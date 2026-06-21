/**
 * Song page — a live, multi-user annotation EDITOR built on top of the existing
 * viewer. It renders the song's PDF (PDF.js) page-by-page with annotation layers
 * drawn via the one renderer (@troubastack/ink, I8), AND lets you draw/select/
 * move/delete annotations that sync in realtime over a per-song WebSocket
 * (src/sync.ts). Per-viewer layer visibility + zoom remain local view state.
 *
 * The live document (layers + objects) is driven by the WS snapshot + echoes; an
 * initial REST GET still seeds it so the first paint is correct even before the
 * socket opens. Drawing emits optimistic mutations reconciled by uuid on echo
 * (or rolled back on reject).
 *
 * The song Details (metadata edit), Files (upload/rename/reorder/delete), and
 * admin Delete still live below the viewer in a collapsible section; their
 * data-testids are unchanged.
 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import * as pdfjs from "pdfjs-dist";
// Vite resolves ?url to the emitted asset path; PDF.js needs its worker URL.
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { renderObjects, type InkObject } from "@troubastack/ink";
import {
  ApiError,
  api,
  type AnnotationDoc,
  type AnnotationLayer,
  type AnnotationObject,
  type AnnotationStyle,
  type Role,
  type Song,
  type SongFile,
} from "../api";
import { useAuth } from "../auth";
import { ErrorBanner } from "../components/ErrorBanner";
import { Avatar } from "../components/Avatar";
import { SyncClient, type SyncState } from "../sync";
import {
  COLOR_SWATCHES,
  CORNER_HANDLES,
  DEFAULT_STYLE,
  applyPreset,
  buildObject,
  handleAtPx,
  handlesVisible,
  hitTest,
  intersectsRect,
  isMarquee,
  isMeaningfulGesture,
  matchPreset,
  normalizeRect,
  objectBBox,
  objectLabel,
  pointerToPageXY,
  pointsForTool,
  resizeObject,
  translateObject,
  type DrawTool,
  type HandleId,
  type PresetId,
  type SelectRect,
  type TextMeasure,
  type Tool,
} from "../editor";

pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

// Discrete percentage stops the −/+ buttons step through.
const ZOOM_PERCENTS = [50, 75, 100, 125, 150, 200, 300];

/** A zoom selection is either a fit mode or an explicit percentage. */
type ZoomMode = "fit-width" | "fit-page" | number;

export function SongEditor() {
  const { bandId, songId } = useParams<{ bandId: string; songId: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [song, setSong] = useState<Song | null>(null);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!bandId || !songId) return;
    try {
      const [{ myRole }, found] = await Promise.all([
        api.getBand(bandId),
        api.getSong(bandId, songId),
      ]);
      setMyRole(myRole);
      setSong(found);
      if (!found) setError("Song not found");
      else setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load song");
    }
  }, [bandId, songId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!bandId || !songId) return <div className="page">Loading…</div>;

  return (
    <div className="page viewer-page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      {error && !song ? <ErrorBanner message={error} /> : null}

      {song && (
        <>
          <h1 data-testid="song-title">{song.title}</h1>

          <Viewer
            bandId={bandId}
            songId={songId}
            myUserId={user?.id ?? null}
            myRole={myRole}
          />

          <Details title="Details & files">
            <Metadata bandId={bandId} song={song} onSaved={setSong} />
            <Files bandId={bandId} songId={songId} />
            {myRole === "admin" && (
              <DeleteSong
                bandId={bandId}
                songId={songId}
                onDeleted={() => navigate(`/bands/${bandId}`)}
              />
            )}
          </Details>

          <p className="muted editor-note" data-testid="editor-note">
            Pick a tool above and draw on the page. Edits sync live to everyone in the band;
            history/revert is the next step.
          </p>
        </>
      )}
    </div>
  );
}

// ===========================================================================
// Viewer
// ===========================================================================

type LayerVisibility = Record<string, boolean>;

/** Default per-viewer visibility (I-style policy): mandatory + shared + my own
 *  personal layers ON; other members' (non-shared, non-mandatory) OFF. */
function defaultVisibility(layers: AnnotationLayer[], myUserId: string | null): LayerVisibility {
  const vis: LayerVisibility = {};
  for (const l of layers) {
    if (l.mandatory) vis[l.id] = true;
    else if (l.zone === "shared") vis[l.id] = true;
    else if (l.zone === "personal" && myUserId != null && l.ownerId === myUserId) vis[l.id] = true;
    else if (l.zone === "conductor") vis[l.id] = true;
    else vis[l.id] = false;
  }
  return vis;
}

/** z-order rank for a layer: conductor(0) < shared(1) < personal(2), then by
 *  `order`; within personal, my own layers sort ABOVE other members'. */
function zoneRank(zone: AnnotationLayer["zone"]): number {
  return zone === "conductor" ? 0 : zone === "shared" ? 1 : 2;
}

function sortLayers(layers: AnnotationLayer[], myUserId: string | null): AnnotationLayer[] {
  return [...layers].sort((a, b) => {
    const za = zoneRank(a.zone);
    const zb = zoneRank(b.zone);
    if (za !== zb) return za - zb;
    if (a.zone === "personal") {
      const aMine = myUserId != null && a.ownerId === myUserId ? 1 : 0;
      const bMine = myUserId != null && b.ownerId === myUserId ? 1 : 0;
      if (aMine !== bMine) return aMine - bMine; // mine later → drawn on top
    }
    return a.order - b.order;
  });
}

/** Is this file something we can render in the viewer (PDF or image)? */
function isViewable(f: SongFile): boolean {
  return f.contentType === "application/pdf" || f.contentType.startsWith("image/");
}

/** May the current user draw into this layer? Mirrors the server gate (apply.go
 *  canWriteLayer):
 *   - CONDUCTOR zone (#3): editable ONLY by a conductor-role viewer, regardless of
 *     ownership/access — members and plain admins see it read-only;
 *   - any other zone: I OWN it, or it is RW.
 *  mandatory governs VISIBILITY, not editability. */
function isEditableLayer(
  l: AnnotationLayer,
  myUserId: string | null,
  myRole: Role | null,
): boolean {
  if (l.zone === "conductor") return myRole === "conductor";
  return (myUserId != null && l.ownerId === myUserId) || l.access === "rw";
}

/** Format the zoom selection for the zoom-level readout. */
function formatZoom(mode: ZoomMode): string {
  if (mode === "fit-width") return "Fit width";
  if (mode === "fit-page") return "Fit page";
  return `${mode}%`;
}

function Viewer({
  bandId,
  songId,
  myUserId,
  myRole,
}: {
  bandId: string;
  songId: string;
  myUserId: string | null;
  myRole: Role | null;
}) {
  // The file strip is MY ordered selection (getMyFiles), not the whole pool.
  const [files, setFiles] = useState<SongFile[]>([]);
  const [customized, setCustomized] = useState(false);
  const [selectedFileId, setSelectedFileId] = useState<string | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [doc, setDoc] = useState<AnnotationDoc>({ layers: [], objects: [] });
  const [visible, setVisible] = useState<LayerVisibility>({});

  // ---- live editing state ----
  const [tool, setTool] = useState<Tool>("select");
  const [style, setStyle] = useState(DEFAULT_STYLE);
  const [activeLayerId, setActiveLayerId] = useState<string | null>(null);
  // The layer the user is "focusing": drives which layer's objects the
  // annotation list shows (works for ANY layer, editable or locked). When the
  // focused layer is editable it also becomes the active DRAW layer; when it's
  // locked, drawing is disabled but its annotations stay browsable.
  const [focusedLayerId, setFocusedLayerId] = useState<string | null>(null);
  // The current selection (single click or rubber-band marquee). Empty = none.
  const [selectedUuids, setSelectedUuids] = useState<string[]>([]);
  // A brief inline notice when the server rejects one of our writes.
  const [rejectNotice, setRejectNotice] = useState<string | null>(null);
  const [connStatus, setConnStatus] = useState<"connecting" | "open" | "closed">("connecting");
  // The realtime client for this song; null until a connection is opened.
  const syncRef = useRef<SyncClient | null>(null);
  // Latest visibility/sortedLayers/style/tool — referenced inside pointer
  // handlers that are bound once per page canvas (avoids stale closures).
  // Zoom: a fit mode (default Fit width) or an explicit percentage.
  const [zoomMode, setZoomMode] = useState<ZoomMode>("fit-width");
  const [numPages, setNumPages] = useState(0);
  const [status, setStatus] = useState<"loading" | "no-file" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  // Measured available width of the PDF column's content box (CSS px). Drives
  // Fit width and Fit page. Updated by a ResizeObserver.
  const [columnWidth, setColumnWidth] = useState(0);

  const selectedFile = useMemo(
    () => files.find((f) => f.id === selectedFileId) ?? null,
    [files, selectedFileId],
  );
  const isPdf = selectedFile?.contentType === "application/pdf";
  const isImage = selectedFile?.contentType.startsWith("image/") ?? false;

  // Loaded PDF.js document. Page/overlay canvases are indexed by page number.
  const pdfDocRef = useRef<pdfjs.PDFDocumentProxy | null>(null);
  const pageCanvasRefs = useRef<(HTMLCanvasElement | null)[]>([]);
  const overlayRefs = useRef<(HTMLCanvasElement | null)[]>([]);
  // Intrinsic (scale-1) page sizes in CSS px, used to compute fit scales.
  const pageSizesRef = useRef<{ w: number; h: number }[]>([]);
  // The scroll column we measure for Fit width / Fit page.
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // Image element box (for image-overlay sizing).
  const imgRef = useRef<HTMLImageElement | null>(null);
  const imgOverlayRef = useRef<HTMLCanvasElement | null>(null);
  // How many times the PDF pages have actually been RASTERIZED (PDF.js page
  // render). The PDF render effect bumps this; an annotation edit must NOT.
  // Surfaced via a hidden data-testid so e2e can assert no re-raster on edit.
  const pdfRenderCountRef = useRef(0);
  const [pdfRenderCount, setPdfRenderCount] = useState(0);

  // ---- refresh MY file strip (getMyFiles) — also after editor changes ----
  // Keeps a sensible selected file: preserves the current one if it survives,
  // otherwise falls back to the first viewable (PDF-preferred) entry.
  const refreshMyFiles = useCallback(async () => {
    const { files: mine, customized: custom } = await api.getMyFiles(bandId, songId);
    // getMyFiles returns my order; honour it (don't re-sort by displayOrder).
    setFiles(mine);
    setCustomized(custom);
    setSelectedFileId((cur) => {
      if (cur && mine.some((f) => f.id === cur && isViewable(f))) return cur;
      const firstPdf = mine.find((f) => f.contentType === "application/pdf");
      const first = firstPdf ?? mine.find(isViewable) ?? null;
      return first ? first.id : null;
    });
    return mine;
  }, [bandId, songId]);

  // ---- load my files + annotations once ----
  useEffect(() => {
    let cancelled = false;
    (async () => {
      setStatus("loading");
      setError(null);
      try {
        const [mine, annotations] = await Promise.all([
          api.getMyFiles(bandId, songId),
          api.getAnnotations(bandId, songId),
        ]);
        if (cancelled) return;
        setFiles(mine.files);
        setCustomized(mine.customized);
        setDoc(annotations);
        setVisible(defaultVisibility(annotations.layers, myUserId));

        // Default selection: first PDF, else first viewable file.
        const firstPdf = mine.files.find((f) => f.contentType === "application/pdf");
        const first = firstPdf ?? mine.files.find(isViewable) ?? null;
        if (!first) {
          setStatus("no-file");
          return;
        }
        setSelectedFileId(first.id);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load files");
        setStatus("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bandId, songId, myUserId]);

  // When my selection is empty, there is nothing to render.
  useEffect(() => {
    if (files.length === 0 && status !== "loading" && status !== "error") {
      setStatus("no-file");
    }
  }, [files.length, status]);

  // ---- realtime sync: one WebSocket per open song ----
  // The live document is driven by the WS (snapshot + echoes). The REST GET
  // above seeds the first paint; once the snapshot lands it becomes authoritative.
  // New layers arriving over the wire default to visible (defaultVisibility).
  useEffect(() => {
    const client = new SyncClient(bandId, songId, {
      onState: (s: SyncState) => {
        setDoc({ layers: s.layers, objects: s.objects });
        // Ensure any layer we don't yet have a visibility entry for gets a sane
        // default (so my new personal layer shows immediately, etc.).
        setVisible((prev) => {
          let changed = false;
          const next = { ...prev };
          const defaults = defaultVisibility(s.layers, myUserId);
          for (const l of s.layers) {
            if (!(l.id in next)) {
              next[l.id] = defaults[l.id];
              changed = true;
            }
          }
          return changed ? next : prev;
        });
      },
      onStatus: setConnStatus,
      onReject: (_uuid, reason) => {
        // The editable-layer model should prevent forbidden writes, but if the
        // server still rejects (e.g. a stale layer), roll back is already done —
        // show a brief inline notice so the user knows the edit didn't stick.
        setRejectNotice(
          reason === "forbidden"
            ? "That layer is read-only — your edit wasn't saved."
            : reason === "deleted-remotely"
              ? "That object was deleted by someone else."
              : "Your edit couldn't be saved (out of date).",
        );
        window.setTimeout(() => setRejectNotice(null), 4000);
      },
    });
    syncRef.current = client;
    client.connect();
    return () => {
      client.close();
      syncRef.current = null;
    };
  }, [bandId, songId, myUserId]);

  // ---- load the selected file (PDF bytes via PDF.js, or just mark image) ----
  useEffect(() => {
    if (!selectedFile) return;
    let cancelled = false;
    setError(null);

    // Tear down any previous PDF doc.
    const prev = pdfDocRef.current;
    pdfDocRef.current = null;
    if (prev) void prev.destroy();
    pageCanvasRefs.current = [];
    overlayRefs.current = [];
    pageSizesRef.current = [];
    setNumPages(0);

    if (selectedFile.contentType.startsWith("image/")) {
      // Image: nothing to rasterize; <img> + overlay handle it.
      setStatus("ready");
      return;
    }
    if (selectedFile.contentType !== "application/pdf") {
      setStatus("no-file");
      return;
    }

    setStatus("loading");
    (async () => {
      try {
        const res = await fetch(api.fileUrl(selectedFile.id), { credentials: "include" });
        if (!res.ok) throw new Error(`Failed to fetch PDF (${res.status})`);
        const bytes = new Uint8Array(await res.arrayBuffer());
        if (cancelled) return;

        const loadingTask = pdfjs.getDocument({ data: bytes });
        const pdfDoc = await loadingTask.promise;
        if (cancelled) {
          void pdfDoc.destroy();
          return;
        }
        pdfDocRef.current = pdfDoc;
        pageCanvasRefs.current = new Array(pdfDoc.numPages).fill(null);
        overlayRefs.current = new Array(pdfDoc.numPages).fill(null);

        // Cache intrinsic page sizes (scale 1) for fit math.
        const sizes: { w: number; h: number }[] = [];
        for (let i = 0; i < pdfDoc.numPages; i++) {
          const page = await pdfDoc.getPage(i + 1);
          if (cancelled) return;
          const vp = page.getViewport({ scale: 1 });
          sizes.push({ w: vp.width, h: vp.height });
        }
        pageSizesRef.current = sizes;
        setNumPages(pdfDoc.numPages);
        setStatus("ready");
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load PDF");
        setStatus("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedFile]);

  // Tear down the PDF doc on unmount.
  useEffect(() => {
    return () => {
      const d = pdfDocRef.current;
      pdfDocRef.current = null;
      if (d) void d.destroy();
    };
  }, []);

  // ---- measure the scroll column (Fit width / Fit page, resize-aware) ----
  // useLayoutEffect so the width is measured synchronously after the scroll
  // column mounts and BEFORE the browser paints — this is what lets the first
  // render compute the correct fit-width scale without any interaction/resize.
  // The ResizeObserver then keeps it current (sidebar toggle, window resize),
  // and crucially fires its first callback if the column wasn't laid out yet.
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const measure = () => {
      const cs = getComputedStyle(el);
      const padX = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight);
      const w = Math.max(0, el.clientWidth - padX);
      // Only update on a real change to avoid extra render passes.
      setColumnWidth((prev) => (Math.abs(prev - w) > 0.5 ? w : prev));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
    // Re-bind when the scroll element appears (status change mounts it) or the
    // sidebar toggles (the column resizes, which ResizeObserver also catches).
  }, [status, sidebarOpen]);

  // ---- effective scale (CSS px per PDF unit) for a given page + zoom mode ----
  // PER-PAGE: pages may have different intrinsic sizes, so fit modes are sized
  // to THAT page's own dimensions (page 0's size is never assumed for others).
  // For an explicit % the scale is constant. Returns 0 when a fit scale cannot
  // be computed yet (width not measured), which gates the render pass.
  const pageScale = useCallback(
    (pageIndex: number): number => {
      if (typeof zoomMode === "number") return zoomMode / 100;
      const sz = pageSizesRef.current[pageIndex];
      if (!sz || columnWidth <= 0) return 0; // not measured yet → wait
      const byW = columnWidth / sz.w;
      if (zoomMode === "fit-width") return byW;
      // fit-page: the page fits the viewport height too.
      const el = scrollRef.current;
      const viewportH = el ? el.clientHeight - 32 /* approx padding */ : 0;
      if (viewportH <= 0) return byW;
      const byH = viewportH / sz.h;
      return Math.min(byW, byH);
    },
    [zoomMode, columnWidth],
  );

  // A representative scale (page 0) for the zoom readout / − + stepping and for
  // gating: 0 means "fit scale not measured yet".
  const scale = useMemo(
    () => pageScale(0),
    // numPages is included so this recomputes once page sizes are cached.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [pageScale, numPages],
  );

  const layersById = useMemo(() => {
    const m = new Map<string, AnnotationLayer>();
    for (const l of doc.layers) m.set(l.id, l);
    return m;
  }, [doc.layers]);

  const sortedLayers = useMemo(
    () => sortLayers(doc.layers, myUserId),
    [doc.layers, myUserId],
  );

  // ---- editable layers (for the active-layer selector) ----
  // Layers I may draw into, scoped to the currently selected file: my own
  // personal layers + any shared RW layer for this file.
  const editableLayers = useMemo(
    () =>
      doc.layers.filter(
        (l) =>
          (selectedFileId == null || l.fileId === selectedFileId) &&
          isEditableLayer(l, myUserId, myRole),
      ),
    [doc.layers, myUserId, selectedFileId],
  );

  // The layer ink currently lands on (only ever an editable layer, never a
  // conductor/other-member RO layer — those are filtered out of editableLayers).
  const activeLayer = useMemo(
    () => editableLayers.find((l) => l.id === activeLayerId) ?? null,
    [editableLayers, activeLayerId],
  );

  // Keep a valid active layer: prefer the current one if it's still editable,
  // else fall back to the first editable layer (or null → "New layer" offered).
  // Never let activeLayerId point at a non-editable layer.
  useEffect(() => {
    setActiveLayerId((cur) => {
      if (cur && editableLayers.some((l) => l.id === cur)) return cur;
      return editableLayers[0]?.id ?? null;
    });
  }, [editableLayers]);

  // ---- focused layer (drives the scoped annotation list) ----
  // The layer whose annotations the list shows. Any layer for the current file
  // may be focused; the focused layer is independent of editability.
  const fileLayers = useMemo(
    () =>
      doc.layers.filter((l) => selectedFileId == null || l.fileId === selectedFileId),
    [doc.layers, selectedFileId],
  );

  const focusedLayer = useMemo(
    () => fileLayers.find((l) => l.id === focusedLayerId) ?? null,
    [fileLayers, focusedLayerId],
  );

  // Is the focused layer one we may NOT draw on (read-only for us)? Drawing is
  // disabled while a locked layer is focused — its objects stay browsable.
  const focusLocked = focusedLayer != null && !isEditableLayer(focusedLayer, myUserId, myRole);

  // Can the focused layer be turned into the active edit target? True when it is
  // editable for me AND not already active — drives the "Edit this layer" CTA.
  const canEditFocusedLayer =
    focusedLayer != null &&
    isEditableLayer(focusedLayer, myUserId, myRole) &&
    focusedLayerId !== activeLayerId;

  // Keep a valid focused layer for the current file. Default to the active
  // (editable) layer so drawing stays enabled out of the box; only an explicit
  // click on a locked row focuses (and locks) it. When there is NO editable
  // layer, leave focus null rather than auto-focusing a locked layer — drawing
  // then provisions a personal layer on demand (Bug 1 flow).
  useEffect(() => {
    setFocusedLayerId((cur) => {
      if (cur && fileLayers.some((l) => l.id === cur)) return cur;
      return activeLayerId;
    });
  }, [fileLayers, activeLayerId]);

  // Focus a layer (click in the Layers panel). If it's editable it also becomes
  // the active DRAW layer; if locked, drawing is disabled but the list scopes
  // to it. Keeps the active-layer selector in sync via activeLayerId.
  const focusLayer = useCallback(
    (id: string) => {
      setFocusedLayerId(id);
      const l = doc.layers.find((x) => x.id === id);
      if (l && isEditableLayer(l, myUserId, myRole)) setActiveLayerId(id);
    },
    [doc.layers, myUserId],
  );

  // The active-layer selector and the focused layer stay in sync: choosing an
  // active layer also focuses it (it is editable, so this is safe).
  const selectActiveLayer = useCallback((id: string) => {
    setActiveLayerId(id);
    setFocusedLayerId(id);
  }, []);

  // Focus a layer WITHOUT activating it (used when clicking an OBJECT on the
  // canvas). This is the heart of Bug #2: clicking an annotation must let you
  // SEE which layer it's on (focus + scoped list) without silently making that
  // layer the edit target. Editing only ever happens on the ACTIVE layer, which
  // is changed explicitly (the active-layer selector or "Edit this layer").
  const focusLayerOnly = useCallback((id: string) => {
    setFocusedLayerId(id);
  }, []);

  // Make the currently-focused layer the active edit target ("Edit this layer").
  // Only ever activates an editable layer (a non-owned RO layer can never become
  // active — the access enforcement is preserved).
  const editFocusedLayer = useCallback(() => {
    if (focusedLayerId == null) return;
    const l = doc.layers.find((x) => x.id === focusedLayerId);
    if (l && isEditableLayer(l, myUserId, myRole)) setActiveLayerId(focusedLayerId);
  }, [focusedLayerId, doc.layers, myUserId]);

  // Create a personal RW layer ("My notes") for the current file and make it
  // active. zone "personal", ownerId = me, access rw — a layer I may edit.
  const createPersonalLayer = useCallback(
    (name = "My notes"): string | null => {
      if (!myUserId || !selectedFileId || !syncRef.current) return null;
      const id = crypto.randomUUID();
      const order = doc.layers.filter((l) => l.fileId === selectedFileId).length;
      syncRef.current.createLayer({
        id,
        fileId: selectedFileId,
        name,
        ownerId: myUserId,
        zone: "personal",
        order,
        access: "rw",
        mandatory: false,
        roleTag: "",
      });
      setActiveLayerId(id);
      setFocusedLayerId(id);
      // A brand-new personal layer should be visible to me immediately.
      setVisible((v) => ({ ...v, [id]: true }));
      return id;
    },
    [myUserId, selectedFileId, doc.layers],
  );

  // Toggle a layer's access (#4): locked = "ro" (others view-only), unlocked = "rw".
  // Sends a layerUpdate carrying the layer with the new access; the server authorizes
  // it to the layer OWNER or a band ADMIN and broadcasts the change.
  const setLayerAccess = useCallback(
    (layerId: string, access: "rw" | "ro") => {
      if (!syncRef.current) return;
      const l = doc.layers.find((x) => x.id === layerId);
      if (!l || l.access === access) return;
      syncRef.current.updateLayer({ ...l, access });
    },
    [doc.layers],
  );

  // May the viewer toggle THIS layer's lock? Only SHARED-zone layers the viewer owns
  // or (as a band admin) administers. Personal layers are inherently private; the
  // conductor zone is role-governed (#3), so no per-layer lock there.
  const canToggleLayerAccess = useCallback(
    (l: AnnotationLayer): boolean => {
      if (l.zone !== "shared") return false;
      return (myUserId != null && l.ownerId === myUserId) || myRole === "admin";
    },
    [myUserId, myRole],
  );

  // Ensure there's a layer to draw into, creating "My notes" on demand. Returns
  // the layer id to draw into, or null if we can't (no file / not signed in).
  const ensureActiveLayer = useCallback((): string | null => {
    if (activeLayerId && editableLayers.some((l) => l.id === activeLayerId)) {
      return activeLayerId;
    }
    if (editableLayers[0]) {
      setActiveLayerId(editableLayers[0].id);
      return editableLayers[0].id;
    }
    return createPersonalLayer();
  }, [activeLayerId, editableLayers, createPersonalLayer]);

  // Commit a finished draw gesture: build the wire object on the active layer
  // and send a create (optimistically added by the sync client).
  const commitDraw = useCallback(
    (tool: DrawTool, page: number, path: { x: number; y: number }[], text?: string) => {
      if (!isMeaningfulGesture(tool, path)) return;
      const layerId = ensureActiveLayer();
      if (!layerId || !syncRef.current) return;
      const obj = buildObject({
        tool,
        points: pointsForTool(tool, path),
        page,
        layerId,
        style,
        text,
      });
      if (tool === "text" && !obj.text) return; // empty text → nothing to create
      // Defense in depth: never commit onto a layer we may not write. ensureActiveLayer
      // only ever returns an editable layer (or a freshly-created personal one), so
      // we only reject a layer that is KNOWN to be non-editable in the current doc.
      const target = doc.layers.find((l) => l.id === layerId);
      if (target && !isEditableLayer(target, myUserId, myRole)) return;
      syncRef.current.createObject(obj);
      setSelectedUuids([obj.uuid]);
    },
    [ensureActiveLayer, doc.layers, myUserId, style],
  );

  // Is THIS object on a layer I may ever edit (owner / rw)? Drives the lock cue
  // (a truly read-only object shows a 🔒). NOT the mutation gate — see below.
  const isEditableObject = useCallback(
    (obj: AnnotationObject): boolean => {
      const l = layersById.get(obj.layerId);
      return l != null && isEditableLayer(l, myUserId, myRole);
    },
    [layersById, myUserId],
  );

  // May the current user edit (move/resize/delete/restyle) THIS object RIGHT
  // NOW? Bug #2's rule: editing happens ONLY on the ACTIVE layer. So an object
  // is mutable iff it lives on the active layer AND that layer is editable for
  // me. An object on a DIFFERENT editable layer is select/inspect-only until the
  // user explicitly activates its layer ("Edit this layer"). Defense in depth
  // alongside the server `forbidden` backstop.
  const isObjectEditableNow = useCallback(
    (obj: AnnotationObject): boolean => {
      if (obj.layerId !== activeLayerId) return false;
      const l = layersById.get(obj.layerId);
      return l != null && isEditableLayer(l, myUserId, myRole);
    },
    [activeLayerId, layersById, myUserId],
  );

  // Commit a move: send the translated object (move mutation). Guarded so an
  // object that is not on the active editable layer never produces a mutation.
  const commitMove = useCallback(
    (obj: AnnotationObject) => {
      if (!syncRef.current) return;
      if (!isObjectEditableNow(obj)) return;
      syncRef.current.updateObject("move", obj);
    },
    [isObjectEditableNow],
  );

  // Commit a resize: send the resized object (resize mutation). Same gate.
  const commitResize = useCallback(
    (obj: AnnotationObject) => {
      if (!syncRef.current) return;
      if (!isObjectEditableNow(obj)) return;
      syncRef.current.updateObject("resize", obj);
    },
    [isObjectEditableNow],
  );

  // Live restyle: send a setStyle mutation with the object carrying the new
  // style. Guarded: only the active layer's objects can be restyled.
  const restyleObject = useCallback(
    (uuid: string, nextStyle: AnnotationStyle) => {
      if (!syncRef.current) return;
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj || !isObjectEditableNow(obj)) return;
      syncRef.current.updateObject("setStyle", { ...obj, style: { ...nextStyle } });
    },
    [doc.objects, isObjectEditableNow],
  );

  // Scroll the page that contains an object into view (objects live on canvas,
  // so "scroll into view" means bringing its page WRAPPER on-screen). The viewer
  // renders ALL pages in one scroll column, so this works cross-page: clicking a
  // list item for an object on page N brings page N into view even when a
  // different page is currently scrolled to (Feature #4). We scroll the column
  // itself (offsetTop is relative to it) so it also works in the embedded webview.
  const scrollObjectIntoView = useCallback(
    (uuid: string) => {
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj) return;
      const scroll = scrollRef.current;
      const pageEls = scroll?.querySelectorAll<HTMLElement>(".pdf-page");
      const el = pageEls?.[obj.page] ?? pageEls?.[0];
      if (!el || !scroll) return;
      const target = el.offsetTop - scroll.clientHeight / 2 + el.clientHeight / 2;
      scroll.scrollTo({ top: Math.max(0, target), behavior: "smooth" });
    },
    [doc.objects],
  );

  // Delete the current selection (one or many objects). Only objects on the
  // ACTIVE editable layer are deleted; everything else (locked OR on a non-active
  // layer) is skipped — no mutation sent.
  const deleteSelected = useCallback(() => {
    if (selectedUuids.length === 0 || !syncRef.current) return;
    let deletedAny = false;
    for (const uuid of selectedUuids) {
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj || !isObjectEditableNow(obj)) continue;
      syncRef.current.deleteObject(uuid);
      deletedAny = true;
    }
    if (deletedAny) setSelectedUuids([]);
  }, [selectedUuids, doc.objects, isObjectEditableNow]);

  // Delete/Backspace removes the current selection (ignored while typing in a
  // form field, so the Details inputs below keep working normally).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Delete" && e.key !== "Backspace") return;
      const t = e.target as HTMLElement | null;
      if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable)) return;
      if (selectedUuids.length === 0) return;
      e.preventDefault();
      deleteSelected();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedUuids, deleteSelected]);

  // Drop any selected uuids whose objects have disappeared (deleted remotely).
  useEffect(() => {
    setSelectedUuids((cur) => {
      const next = cur.filter((u) => doc.objects.some((o) => o.uuid === u));
      return next.length === cur.length ? cur : next;
    });
  }, [doc.objects]);

  // ---- selection ↔ style controls ----
  // The single selected object (only when exactly one is selected). Drives the
  // style controls (reflect its style) and live restyle.
  const selectedObject = useMemo(
    () => (selectedUuids.length === 1 ? doc.objects.find((o) => o.uuid === selectedUuids[0]) ?? null : null),
    [selectedUuids, doc.objects],
  );
  // Is the (single) selection editable RIGHT NOW (on the active editable layer)?
  // Drives whether the style controls are live. An object on a locked layer OR
  // on a non-active editable layer is inspect-only → controls disabled.
  const selectionEditable = selectedObject != null && isObjectEditableNow(selectedObject);
  // Are the style controls locked? Whenever a single non-active/locked object is
  // selected (it can be inspected but not restyled until its layer is active).
  const controlsLocked = selectedObject != null && !selectionEditable;
  // Is the single selection on an editable layer that just isn't ACTIVE yet?
  // (As opposed to a truly read-only layer.) Drives the "Edit this layer" CTA.
  const selectionOnInactiveEditable =
    selectedObject != null &&
    isEditableObject(selectedObject) &&
    selectedObject.layerId !== activeLayerId;
  // The style the controls REFLECT: the selected object's style, else the draw
  // default. When nothing is selected, controls set the next-drawn-object style.
  const effectiveStyle = selectedObject ? selectedObject.style : style;
  // Applying a style change: restyle the selected (active-editable) object live,
  // else update the draw default for the next object.
  const applyStyle = useCallback(
    (next: AnnotationStyle) => {
      if (selectedObject) {
        if (selectionEditable) restyleObject(selectedObject.uuid, next);
        return; // inspect-only selection: controls are disabled, nothing to apply
      }
      setStyle(next);
    },
    [selectedObject, selectionEditable, restyleObject],
  );

  // ---- overlay rendering: object coords [0,1] → page box. ----
  // We scale the ctx by DPR and draw using the LOGICAL (CSS-px) page box, so a
  // rect at {0.1..0.9} maps to exactly 10%..90% of the rendered page. The
  // backing store is at scale × DPR for crispness; the DPR ctx-scale undoes it.
  // Paint ONE page's overlay: clears it, then draws only that page's visible
  // objects into the logical (CSS-px) page box derived from the overlay's OWN
  // backing-store size. The ctx is scaled by DPR so [0,1] coords map to the box.
  const paintOverlay = useCallback(
    (overlay: HTMLCanvasElement, page: number, dpr: number) => {
      const ctx = overlay.getContext("2d");
      if (!ctx) return;
      const orderIndex = new Map<string, number>();
      sortedLayers.forEach((l, idx) => orderIndex.set(l.id, idx));

      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, overlay.width / dpr, overlay.height / dpr);
      const objs = doc.objects
        .filter((o) => o.page === page)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        })
        .sort((a, b) => (orderIndex.get(a.layerId) ?? 0) - (orderIndex.get(b.layerId) ?? 0));
      // Logical page box (CSS px) fills the whole overlay — sized from THIS
      // page's actual canvas dimensions, not page 0's.
      const pageRect = { x: 0, y: 0, w: overlay.width / dpr, h: overlay.height / dpr };
      renderObjects(ctx, objs.map(toInkObject) as InkObject[], pageRect);
    },
    [doc.objects, layersById, visible, sortedLayers],
  );

  // Latest paintOverlay, referenced by the PDF rasterization effect WITHOUT
  // making that effect depend on it. This is the crux of the no-flicker fix:
  // the PDF render pass must depend ONLY on [selectedFile, scale, numPages,
  // zoomMode], never on objects/visibility/paintOverlay — otherwise every draw/
  // move/restyle re-rasterizes every page (visible flicker). Overlay repaint on
  // object/visibility change is a SEPARATE effect (renderOverlays).
  const paintOverlayRef = useRef(paintOverlay);
  useEffect(() => {
    paintOverlayRef.current = paintOverlay;
  }, [paintOverlay]);

  // Repaint every mounted overlay (used when only visibility/objects change and
  // the canvases keep their current size — no re-rasterization needed).
  const renderOverlays = useCallback(() => {
    const dpr = window.devicePixelRatio || 1;
    if (isImage) {
      const o = imgOverlayRef.current;
      if (o) paintOverlay(o, 0, dpr);
      return;
    }
    for (let p = 0; p < overlayRefs.current.length; p++) {
      const overlay = overlayRefs.current[p];
      if (overlay) paintOverlay(overlay, p, dpr);
    }
  }, [isImage, paintOverlay]);

  // ---- render PDF pages on scale/document change, then overlays ----
  // Robust per-page flow: for each page p, render PDF.js page → canvas C_p at
  // (pageScale(p) × dpr), then size overlay O_p to C_p's ACTUAL backing-store
  // and CSS dimensions, scale O_p's ctx by dpr, and draw that page's objects
  // into the logical page box derived from C_p (never page 0's dims or a stale
  // scale). The pass is keyed on scale/numPages/file so it re-runs on zoom,
  // file switch, and (via columnWidth → scale) once the column is measured.
  useEffect(() => {
    const pdfDoc = pdfDocRef.current;
    if (status !== "ready" || !pdfDoc || !isPdf) return;
    // For fit modes, wait until the column width has been measured. pageScale
    // returns 0 until then; the ResizeObserver's first callback bumps
    // columnWidth → scale, which re-runs this effect with a real scale.
    if (typeof zoomMode !== "number" && scale <= 0) return;

    let cancelled = false;

    (async () => {
      const dpr = window.devicePixelRatio || 1;
      for (let i = 0; i < pdfDoc.numPages; i++) {
        if (cancelled) return;
        const s = pageScale(i);
        if (s <= 0) continue; // page size not cached yet; skip until measured
        const page = await pdfDoc.getPage(i + 1);
        if (cancelled) return;

        const canvas = pageCanvasRefs.current[i];
        if (!canvas) continue;
        // Backing store at this page's scale × DPR; CSS size at logical scale.
        const viewport = page.getViewport({ scale: s * dpr });
        const cssW = viewport.width / dpr;
        const cssH = viewport.height / dpr;

        canvas.width = Math.round(viewport.width);
        canvas.height = Math.round(viewport.height);
        canvas.style.width = `${cssW}px`;
        canvas.style.height = `${cssH}px`;

        const ctx = canvas.getContext("2d");
        if (!ctx) continue;
        await page.render({ canvasContext: ctx, viewport }).promise;
        if (cancelled) return;
        // Count an actual rasterization (one per page render). Editing an
        // annotation must never reach here, so this stays put on edits.
        pdfRenderCountRef.current += 1;
        setPdfRenderCount(pdfRenderCountRef.current);

        // Size the overlay to the canvas's ACTUAL rendered dimensions, then
        // paint THIS page's objects off those dims. We call the LATEST paint via
        // a ref so this effect does not depend on paintOverlay (which changes on
        // every object/visibility change and would otherwise re-rasterize).
        const overlay = overlayRefs.current[i];
        if (overlay) {
          overlay.width = canvas.width;
          overlay.height = canvas.height;
          overlay.style.width = `${canvas.style.width}`;
          overlay.style.height = `${canvas.style.height}`;
          paintOverlayRef.current(overlay, i, dpr);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
    // Depends ONLY on what changes the rasterized pixels: the file, the scale,
    // the page count, and the zoom mode. NOT objects/visibility/paintOverlay —
    // those repaint the overlay canvases via renderOverlays without re-raster.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedFile, status, scale, numPages, zoomMode]);

  // ---- size + render the image overlay (image mode) ----
  const layoutImageOverlay = useCallback(() => {
    if (!isImage) return;
    const img = imgRef.current;
    const overlay = imgOverlayRef.current;
    if (!img || !overlay) return;
    const dpr = window.devicePixelRatio || 1;
    const w = img.clientWidth;
    const h = img.clientHeight;
    overlay.width = Math.round(w * dpr);
    overlay.height = Math.round(h * dpr);
    overlay.style.width = `${w}px`;
    overlay.style.height = `${h}px`;
    renderOverlays();
  }, [isImage, renderOverlays]);

  // Re-render overlays when visibility / sorted layers / objects change.
  useEffect(() => {
    if (status === "ready") renderOverlays();
  }, [status, renderOverlays]);

  // Re-layout the image overlay when the column resizes (image fills width).
  useEffect(() => {
    if (status === "ready" && isImage) layoutImageOverlay();
  }, [status, isImage, columnWidth, layoutImageOverlay]);

  function toggle(layerId: string) {
    setVisible((v) => ({ ...v, [layerId]: !v[layerId] }));
  }

  // −/+ step through the percentage stops. When in a fit mode, snap to the
  // nearest percentage to the currently-rendered scale, then step from there.
  function currentPercent(): number {
    if (typeof zoomMode === "number") return zoomMode;
    return Math.round(scale * 100);
  }
  function stepZoom(dir: -1 | 1) {
    const cur = currentPercent();
    // Find the first stop strictly above (for +) or below (for −) current.
    if (dir === 1) {
      const next = ZOOM_PERCENTS.find((p) => p > cur);
      setZoomMode(next ?? ZOOM_PERCENTS[ZOOM_PERCENTS.length - 1]);
    } else {
      const below = [...ZOOM_PERCENTS].reverse().find((p) => p < cur);
      setZoomMode(below ?? ZOOM_PERCENTS[0]);
    }
  }

  function onZoomSelect(value: string) {
    if (value === "fit-width" || value === "fit-page") setZoomMode(value);
    else setZoomMode(Number(value));
  }

  const zoomSelectValue =
    typeof zoomMode === "number" ? String(zoomMode) : zoomMode;

  return (
    <section
      className={`card viewer${sidebarOpen ? "" : " sidebar-collapsed"}`}
      data-testid="song-viewer"
    >
      <EditorToolbar
        tool={tool}
        onTool={(t) => {
          setTool(t);
          if (t !== "select") setSelectedUuids([]);
        }}
        style={effectiveStyle}
        onStyle={applyStyle}
        controlsLocked={controlsLocked}
        selectedType={selectedObject?.type ?? null}
        editableLayers={editableLayers}
        activeLayerId={activeLayerId}
        activeLayer={activeLayer}
        onActiveLayer={selectActiveLayer}
        onNewLayer={() => createPersonalLayer()}
        canDraw={myUserId != null && selectedFileId != null}
        drawLocked={focusLocked}
        canEditFocusedLayer={canEditFocusedLayer}
        focusedLayerName={focusedLayer?.name ?? null}
        onEditLayer={editFocusedLayer}
        showEditLayerHint={selectionOnInactiveEditable}
        objectCount={doc.objects.length}
        selectionCount={selectedUuids.length}
        canDeleteSelection={selectedUuids.some((u) => {
          const o = doc.objects.find((x) => x.uuid === u);
          return o != null && isObjectEditableNow(o);
        })}
        onDelete={deleteSelected}
        connStatus={connStatus}
      />

      {rejectNotice && (
        <p className="notice editor-reject-notice" data-testid="reject-notice" role="alert">
          {rejectNotice}
        </p>
      )}

      {/* Hidden render-count probe: how many times PDF pages have actually been
          rasterized. e2e asserts this does NOT change across an annotation edit
          (no re-raster = no flicker). */}
      <span
        data-testid="pdf-render-count"
        style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", opacity: 0 }}
        aria-hidden="true"
      >
        {pdfRenderCount}
      </span>

      <div className="viewer-toolbar">
        <div className="zoom-controls" data-testid="zoom-controls">
          <button type="button" data-testid="zoom-out" onClick={() => stepZoom(-1)}>
            −
          </button>
          <select
            data-testid="zoom-mode"
            className="zoom-select"
            value={zoomSelectValue}
            onChange={(e) => onZoomSelect(e.target.value)}
            aria-label="Zoom"
          >
            <option value="fit-width">Fit width</option>
            <option value="fit-page">Fit page</option>
            {ZOOM_PERCENTS.map((p) => (
              <option key={p} value={p}>
                {p === 100 ? "Actual size (100%)" : `${p}%`}
              </option>
            ))}
          </select>
          <button type="button" data-testid="zoom-in" onClick={() => stepZoom(1)}>
            +
          </button>
          <span className="pill" data-testid="zoom-level">
            {formatZoom(zoomMode)}
          </span>
        </div>

        <div className="my-files-controls">
          <button
            type="button"
            className="my-files-edit-btn"
            data-testid="my-files-edit"
            aria-expanded={editorOpen}
            onClick={() => setEditorOpen((o) => !o)}
          >
            Choose files
          </button>
          {customized && (
            <span className="pill my-files-custom-pill" data-testid="my-files-custom">
              custom
            </span>
          )}
        </div>

        {files.length >= 1 && (
          <div
            className="file-picker"
            data-testid="file-picker"
            role="tablist"
            aria-label="Files"
          >
            {files.map((f) => {
              const viewable = isViewable(f);
              const active = f.id === selectedFileId;
              const badge =
                f.contentType === "application/pdf"
                  ? "PDF"
                  : f.contentType.startsWith("image/")
                    ? "IMG"
                    : "—";
              return (
                <button
                  key={f.id}
                  type="button"
                  className={`file-tab card${active ? " active" : ""}`}
                  data-testid="file-tab"
                  role="tab"
                  aria-selected={active}
                  disabled={!viewable}
                  title={viewable ? f.filename : `${f.filename} (not viewable)`}
                  onClick={() => viewable && setSelectedFileId(f.id)}
                >
                  <span className={`pill file-tab-badge badge-${badge.toLowerCase()}`}>
                    {badge}
                  </span>
                  <span className="file-tab-name">{f.filename}</span>
                </button>
              );
            })}
          </div>
        )}

        <button
          type="button"
          className="sidebar-toggle"
          data-testid="sidebar-toggle"
          aria-expanded={sidebarOpen}
          onClick={() => setSidebarOpen((o) => !o)}
        >
          {sidebarOpen ? "Hide layers ▸" : "◂ Show layers"}
        </button>
      </div>

      {editorOpen && (
        <MyFilesEditor
          bandId={bandId}
          songId={songId}
          selected={files}
          onChanged={refreshMyFiles}
          onError={setError}
        />
      )}

      <div className="viewer-body">
        <div className="viewer-scroll" data-testid="viewer-scroll" ref={scrollRef}>
          {status === "loading" && <p className="muted">Loading…</p>}
          {status === "no-file" && files.length === 0 && (
            <p className="muted" data-testid="viewer-no-pdf">
              No files selected —{" "}
              <button
                type="button"
                className="link-button"
                onClick={() => setEditorOpen(true)}
              >
                choose some
              </button>
              , or upload a PDF or image in “Details &amp; files” below.
            </p>
          )}
          {status === "no-file" && files.length > 0 && (
            <p className="muted" data-testid="viewer-no-pdf">
              No viewable file. Upload a PDF or image in “Details &amp; files” below.
            </p>
          )}
          {status === "error" && <ErrorBanner message={error} />}

          {status === "ready" && isPdf &&
            Array.from({ length: numPages }, (_, i) => (
              <div className="pdf-page" data-testid="pdf-page" key={i}>
                <canvas
                  ref={(el) => {
                    pageCanvasRefs.current[i] = el;
                  }}
                  className="pdf-canvas"
                />
                <canvas
                  ref={(el) => {
                    overlayRefs.current[i] = el;
                  }}
                  className="annotation-overlay"
                  data-testid="annotation-overlay"
                />
                <EditCanvas
                  page={i}
                  tool={tool}
                  style={style}
                  drawLocked={focusLocked}
                  objects={doc.objects}
                  layersById={layersById}
                  visible={visible}
                  selectedUuids={selectedUuids}
                  isObjectEditable={isEditableObject}
                  isObjectEditableNow={isObjectEditableNow}
                  onSelect={setSelectedUuids}
                  onFocusLayer={focusLayerOnly}
                  onCommitDraw={commitDraw}
                  onCommitMove={commitMove}
                  onCommitResize={commitResize}
                />
              </div>
            ))}

          {status === "ready" && isImage && selectedFile && (
            <div className="pdf-page" data-testid="pdf-page">
              <img
                ref={imgRef}
                className="pdf-canvas image-page"
                src={api.fileUrl(selectedFile.id)}
                alt={selectedFile.filename}
                onLoad={layoutImageOverlay}
              />
              <canvas
                ref={imgOverlayRef}
                className="annotation-overlay"
                data-testid="annotation-overlay"
              />
              <EditCanvas
                page={0}
                tool={tool}
                style={style}
                drawLocked={focusLocked}
                objects={doc.objects}
                layersById={layersById}
                visible={visible}
                selectedUuids={selectedUuids}
                isObjectEditable={isEditableObject}
                isObjectEditableNow={isObjectEditableNow}
                onSelect={setSelectedUuids}
                onFocusLayer={focusLayerOnly}
                onCommitDraw={commitDraw}
                onCommitMove={commitMove}
                onCommitResize={commitResize}
              />
            </div>
          )}
        </div>

        {sidebarOpen && (
          <div className="viewer-sidebar">
            <AnnotationList
              objects={doc.objects}
              focusedLayerId={focusedLayerId}
              focusedLayer={focusedLayer}
              focusLocked={focusLocked}
              selectedUuids={selectedUuids}
              onSelect={(uuid) => {
                setSelectedUuids([uuid]);
                scrollObjectIntoView(uuid);
              }}
            />
            <LayersPanel
              layers={sortedLayers}
              visible={visible}
              myUserId={myUserId}
              myRole={myRole}
              activeLayerId={activeLayerId}
              focusedLayerId={focusedLayerId}
              onToggle={toggle}
              onFocus={focusLayer}
              canToggleAccess={canToggleLayerAccess}
              onSetAccess={setLayerAccess}
            />
          </div>
        )}
      </div>
    </section>
  );
}

// ===========================================================================
// My files editor
// ===========================================================================

/** Editor over the shared file POOL (listFiles): include/exclude each pool file
 *  in my selection, reorder my included files, or reset to default-all. Changes
 *  apply immediately (PUT the resulting ordered fileIds, or DELETE on reset),
 *  then ask the viewer to refresh the strip via onChanged. */
function MyFilesEditor({
  bandId,
  songId,
  selected,
  onChanged,
  onError,
}: {
  bandId: string;
  songId: string;
  selected: SongFile[];
  onChanged: () => Promise<SongFile[]>;
  onError: (msg: string | null) => void;
}) {
  const [pool, setPool] = useState<SongFile[]>([]);
  // My ordered selection of fileIds — seeded from the current strip, then kept
  // in sync after each apply (onChanged returns the server's canonical order).
  const [order, setOrder] = useState<string[]>(selected.map((f) => f.id));
  const [busy, setBusy] = useState(false);

  // Load the whole pool once.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listFiles(bandId, songId);
        if (cancelled) return;
        list.sort((a, b) => a.displayOrder - b.displayOrder);
        setPool(list);
      } catch (err) {
        if (cancelled) return;
        onError(err instanceof Error ? err.message : "Failed to load files");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bandId, songId, onError]);

  const poolById = useMemo(() => {
    const m = new Map<string, SongFile>();
    for (const f of pool) m.set(f.id, f);
    return m;
  }, [pool]);

  // Apply an ordered selection: PUT, then refresh the strip and re-seed order
  // from the server's canonical response (drops any files removed from the pool).
  const apply = useCallback(
    async (nextOrder: string[]) => {
      setBusy(true);
      onError(null);
      try {
        await api.setMyFiles(bandId, songId, nextOrder);
        const mine = await onChanged();
        setOrder(mine.map((f) => f.id));
      } catch (err) {
        onError(err instanceof Error ? err.message : "Failed to update selection");
      } finally {
        setBusy(false);
      }
    },
    [bandId, songId, onChanged, onError],
  );

  function toggleInclude(id: string) {
    const next = order.includes(id) ? order.filter((x) => x !== id) : [...order, id];
    void apply(next);
  }

  function moveIncluded(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= order.length) return;
    const next = [...order];
    [next[index], next[other]] = [next[other], next[index]];
    void apply(next);
  }

  async function reset() {
    setBusy(true);
    onError(null);
    try {
      await api.clearMyFiles(bandId, songId);
      const mine = await onChanged();
      setOrder(mine.map((f) => f.id));
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to reset");
    } finally {
      setBusy(false);
    }
  }

  // Included files (in MY order) first, then the rest of the pool.
  const includedFiles = order.map((id) => poolById.get(id)).filter((f): f is SongFile => !!f);
  const excludedFiles = pool.filter((f) => !order.includes(f.id));

  return (
    <section className="my-files-panel card" data-testid="my-files-panel">
      <div className="my-files-panel-head">
        <h2>My files</h2>
        <button
          type="button"
          className="my-files-reset-btn"
          data-testid="my-files-reset"
          disabled={busy}
          onClick={() => void reset()}
        >
          Reset to all
        </button>
      </div>
      <p className="muted my-files-hint">
        Pick which files appear in your strip and in what order. Everyone shares the same
        pool (managed in “Details &amp; files”).
      </p>

      {pool.length === 0 ? (
        <p className="muted">No files in the pool yet.</p>
      ) : (
        <ul className="list my-files-list">
          {includedFiles.map((f, i) => (
            <li key={f.id} data-testid="my-files-row" className="my-files-row included">
              <label className="my-files-row-main">
                <input
                  type="checkbox"
                  data-testid="my-files-include"
                  checked
                  disabled={busy}
                  onChange={() => toggleInclude(f.id)}
                />
                <span className="my-files-name">{f.filename}</span>
              </label>
              <span className="actions">
                <button
                  type="button"
                  data-testid="my-files-up"
                  disabled={busy || i === 0}
                  onClick={() => moveIncluded(i, -1)}
                >
                  ↑
                </button>
                <button
                  type="button"
                  data-testid="my-files-down"
                  disabled={busy || i === includedFiles.length - 1}
                  onClick={() => moveIncluded(i, 1)}
                >
                  ↓
                </button>
              </span>
            </li>
          ))}
          {excludedFiles.map((f) => (
            <li key={f.id} data-testid="my-files-row" className="my-files-row excluded">
              <label className="my-files-row-main">
                <input
                  type="checkbox"
                  data-testid="my-files-include"
                  checked={false}
                  disabled={busy}
                  onChange={() => toggleInclude(f.id)}
                />
                <span className="my-files-name muted">{f.filename}</span>
              </label>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function LayersPanel({
  layers,
  visible,
  myUserId,
  myRole,
  activeLayerId,
  focusedLayerId,
  onToggle,
  onFocus,
  canToggleAccess,
  onSetAccess,
}: {
  layers: AnnotationLayer[];
  visible: LayerVisibility;
  myUserId: string | null;
  myRole: Role | null;
  activeLayerId: string | null;
  focusedLayerId: string | null;
  onToggle: (id: string) => void;
  onFocus: (id: string) => void;
  // Whether the viewer may flip THIS layer's lock (shared-zone owner/admin only).
  canToggleAccess: (l: AnnotationLayer) => boolean;
  onSetAccess: (id: string, access: "rw" | "ro") => void;
}) {
  return (
    <aside className="layers-panel" data-testid="layers-panel">
      <h2>Layers</h2>
      {layers.length === 0 ? (
        <p className="muted" data-testid="layers-empty">
          No annotation layers.
        </p>
      ) : (
        <ul className="list layers-list">
          {layers.map((l) => {
            const tag =
              l.zone === "personal" && myUserId != null && l.ownerId === myUserId
                ? "personal · mine"
                : l.zone;
            // Non-editable = a layer I may not write (RO, or someone else's
            // personal layer). Shown with a lock; never the active draw target.
            const locked = !isEditableLayer(l, myUserId, myRole);
            const isActive = l.id === activeLayerId;
            const isFocused = l.id === focusedLayerId;
            return (
              <li
                key={l.id}
                data-testid="layer-item"
                className={`layer-item${isActive ? " active-layer" : ""}${
                  isFocused ? " focused-layer" : ""
                }${locked ? " locked" : ""}`}
              >
                <div className="layer-row-wrap">
                  <input
                    type="checkbox"
                    data-testid="layer-toggle"
                    aria-label={`Show ${l.name}`}
                    checked={!!visible[l.id]}
                    disabled={l.mandatory}
                    onChange={() => onToggle(l.id)}
                  />
                  {/* Click focuses the layer: scopes the annotation list to it,
                      and (if editable) makes it the active draw layer. */}
                  <button
                    type="button"
                    data-testid="layer-row"
                    className="layer-row"
                    aria-pressed={isFocused}
                    title="Show this layer's annotations"
                    onClick={() => onFocus(l.id)}
                  >
                    <span data-testid="layer-owner" title={l.ownerId === myUserId ? "Your layer" : "Another member's layer"}>
                      <Avatar user={{ displayName: l.name, avatarKind: "neutral" }} size={20} />
                    </span>
                    <span className="layer-name">{l.name}</span>
                  </button>
                </div>
                <span className="pill">{tag}</span>
                {l.mandatory && <span className="pill mandatory-pill">required</span>}
                {isActive && (
                  <span className="pill active-pill" data-testid="layer-active">
                    drawing
                  </span>
                )}
                {isFocused && (
                  <span className="pill focused-pill" data-testid="layer-focused">
                    viewing
                  </span>
                )}
                {locked && (
                  <span
                    className="pill lock-pill"
                    data-testid="layer-lock"
                    title="Read-only — you can't draw on this layer"
                    aria-label="Read-only layer"
                  >
                    🔒 locked
                  </span>
                )}
                {/* #4: lock/unlock toggle for shared-zone layers I own or admin.
                    locked(ro) = others view-only; unlocked(rw) = others can edit. */}
                {canToggleAccess(l) && (
                  <button
                    type="button"
                    data-testid="layer-access-toggle"
                    className={`layer-access-btn${l.access === "ro" ? " is-locked" : ""}`}
                    aria-pressed={l.access === "ro"}
                    title={
                      l.access === "ro"
                        ? "Locked — others can only view. Click to unlock (allow edits)."
                        : "Unlocked — others can edit. Click to lock (view-only)."
                    }
                    aria-label={l.access === "ro" ? "Unlock layer" : "Lock layer"}
                    onClick={() => onSetAccess(l.id, l.access === "ro" ? "rw" : "ro")}
                  >
                    {l.access === "ro" ? "🔒" : "🔓"}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </aside>
  );
}

// ===========================================================================
// Annotation list — objects on the current editing (active) layer
// ===========================================================================

/** A list of the objects on the FOCUSED layer (the layer the user clicked in
 *  the Layers panel — editable or locked). Each row shows the type + a short
 *  label; clicking selects + highlights that object (and the caller scrolls it
 *  into view). When the focused layer is locked, an inline hint explains that
 *  drawing is disabled while you can still browse/select its annotations. */
function AnnotationList({
  objects,
  focusedLayerId,
  focusedLayer,
  focusLocked,
  selectedUuids,
  onSelect,
}: {
  objects: AnnotationObject[];
  focusedLayerId: string | null;
  focusedLayer: AnnotationLayer | null;
  focusLocked: boolean;
  selectedUuids: string[];
  onSelect: (uuid: string) => void;
}) {
  const items = useMemo(
    () => objects.filter((o) => o.layerId === focusedLayerId),
    [objects, focusedLayerId],
  );
  const selected = new Set(selectedUuids);
  return (
    <aside className="annotation-list-panel" data-testid="annotation-list">
      <h2 data-testid="annotation-list-title">
        Annotations{focusedLayer ? ` · ${focusedLayer.name}` : ""}
      </h2>
      {focusLocked && (
        <p className="muted annotation-list-locked-hint" data-testid="annotation-list-locked">
          read-only layer — pick an editable layer to draw
        </p>
      )}
      {!focusedLayer ? (
        <p className="muted" data-testid="annotation-list-empty">
          No layer selected — pick a layer to see its annotations.
        </p>
      ) : items.length === 0 ? (
        <p className="muted" data-testid="annotation-list-empty">
          No annotations on this layer.
        </p>
      ) : (
        <ul className="list annotation-items">
          {items.map((o) => (
            <li key={o.uuid}>
              <button
                type="button"
                data-testid="annotation-item"
                className={`annotation-item${selected.has(o.uuid) ? " selected" : ""}`}
                aria-pressed={selected.has(o.uuid)}
                onClick={() => onSelect(o.uuid)}
              >
                <span className={`pill ann-type ann-type-${o.type}`}>{o.type}</span>
                <span className="ann-label">{objectLabel(o)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}

// ===========================================================================
// Editor toolbar — tools palette, style controls, active layer, object count
// ===========================================================================

// The Highlight tool is gone (#5) — it is now a STYLE PRESET on rect/ellipse.
const TOOLS: { tool: Tool; label: string; testid: string }[] = [
  { tool: "select", label: "Select", testid: "tool-select" },
  { tool: "freehand", label: "Pen", testid: "tool-freehand" },
  { tool: "line", label: "Line", testid: "tool-line" },
  { tool: "rect", label: "Rect", testid: "tool-rect" },
  { tool: "ellipse", label: "Ellipse", testid: "tool-ellipse" },
  { tool: "text", label: "Text", testid: "tool-text" },
];

// The shape-style presets shown as one-click buttons (#5).
const PRESET_BUTTONS: { id: PresetId; label: string; testid: string }[] = [
  { id: "outline", label: "Outline", testid: "preset-outline" },
  { id: "box", label: "Box", testid: "preset-box" },
  { id: "highlight", label: "Highlight", testid: "preset-highlight" },
];

/** Do the style controls (fill/stroke/blend/presets) apply to the current target?
 *  Only for shape tools (rect/ellipse) when drawing, or a selected rect/ellipse/
 *  legacy-highlight object. */
function isShapeTarget(tool: Tool, selectedType: AnnotationObject["type"] | null): boolean {
  if (selectedType) return selectedType === "rect" || selectedType === "ellipse" || selectedType === "highlight";
  return tool === "rect" || tool === "ellipse";
}

function EditorToolbar({
  tool,
  onTool,
  style,
  onStyle,
  controlsLocked,
  selectedType,
  editableLayers,
  activeLayerId,
  activeLayer,
  onActiveLayer,
  onNewLayer,
  canDraw,
  drawLocked,
  canEditFocusedLayer,
  focusedLayerName,
  onEditLayer,
  showEditLayerHint,
  objectCount,
  selectionCount,
  canDeleteSelection,
  onDelete,
  connStatus,
}: {
  tool: Tool;
  onTool: (t: Tool) => void;
  style: AnnotationStyle;
  onStyle: (s: AnnotationStyle) => void;
  // The selected object is on a locked layer → style controls reflect but are disabled.
  controlsLocked: boolean;
  // The selected object's type (drives the tool/shape indicator), or null.
  selectedType: AnnotationObject["type"] | null;
  editableLayers: AnnotationLayer[];
  activeLayerId: string | null;
  activeLayer: AnnotationLayer | null;
  onActiveLayer: (id: string) => void;
  onNewLayer: () => void;
  canDraw: boolean;
  drawLocked: boolean;
  // The focused layer is editable but not active → offer "Edit this layer".
  canEditFocusedLayer: boolean;
  focusedLayerName: string | null;
  onEditLayer: () => void;
  // A non-active editable object is selected → show the inline "edit this layer" hint.
  showEditLayerHint: boolean;
  objectCount: number;
  selectionCount: number;
  canDeleteSelection: boolean;
  onDelete: () => void;
  connStatus: "connecting" | "open" | "closed";
}) {
  return (
    <div className="editor-toolbar" data-testid="editor-toolbar">
      <div className="tool-palette" role="toolbar" aria-label="Annotation tools">
        {TOOLS.map((t) => (
          <button
            key={t.tool}
            type="button"
            data-testid={t.testid}
            className={`tool-btn${tool === t.tool ? " active" : ""}`}
            aria-pressed={tool === t.tool}
            disabled={(!canDraw || drawLocked) && t.tool !== "select"}
            onClick={() => onTool(t.tool)}
          >
            {t.label}
          </button>
        ))}
        {drawLocked && (
          <span className="draw-locked-hint" data-testid="draw-locked-hint" role="status">
            read-only layer — pick an editable layer to draw
          </span>
        )}
      </div>

      <div
        className={`style-controls${selectedType ? " editing-selection" : ""}${
          controlsLocked ? " controls-locked" : ""
        }`}
        data-testid="style-controls"
      >
        {/* Shape/type indicator: the selected object's type, else the draw tool. */}
        <span className="pill style-target" data-testid="style-target">
          {selectedType ? `Editing: ${selectedType}` : `Draw: ${tool}`}
        </span>
        <span className="swatches">
          {COLOR_SWATCHES.map((c) => (
            <button
              key={c}
              type="button"
              className={`swatch${style.color === c ? " active" : ""}`}
              style={{ background: c }}
              aria-label={`Color ${c}`}
              disabled={controlsLocked}
              onClick={() => onStyle({ ...style, color: c })}
            />
          ))}
        </span>
        <label className="style-field">
          <span>Color</span>
          <input
            type="color"
            data-testid="style-color"
            value={style.color}
            disabled={controlsLocked}
            onChange={(e) => onStyle({ ...style, color: e.target.value })}
          />
          <span className="style-value" data-testid="style-color-value">
            {style.color.toUpperCase()}
          </span>
        </label>
        <label className="style-field">
          <span>Opacity</span>
          <input
            type="range"
            data-testid="style-opacity"
            min={0.1}
            max={1}
            step={0.05}
            value={style.opacity}
            disabled={controlsLocked}
            onChange={(e) => onStyle({ ...style, opacity: Number(e.target.value) })}
          />
          <span className="style-value" data-testid="style-opacity-value">
            {Math.round(style.opacity * 100)}%
          </span>
        </label>
        <label className="style-field">
          <span>Width</span>
          <input
            type="range"
            data-testid="style-width"
            min={0.001}
            max={0.02}
            step={0.001}
            value={style.width}
            disabled={controlsLocked}
            onChange={(e) => onStyle({ ...style, width: Number(e.target.value) })}
          />
          <span className="style-value" data-testid="style-width-value">
            {(style.width * 1000).toFixed(1)}
          </span>
        </label>
        {/* Shape style (#5): fill / border(stroke) / blend + presets. Shown for shape
            tools (rect/ellipse) or a selected shape; disabled when controls are locked. */}
        {isShapeTarget(tool, selectedType) && (
          <div className="shape-style" data-testid="shape-style">
            <span className="preset-buttons" role="group" aria-label="Shape presets">
              {PRESET_BUTTONS.map((p) => {
                const active = matchPreset(style) === p.id;
                return (
                  <button
                    key={p.id}
                    type="button"
                    data-testid={p.testid}
                    className={`preset-btn${active ? " active" : ""}`}
                    aria-pressed={active}
                    disabled={controlsLocked}
                    onClick={() => onStyle(applyPreset(style, p.id))}
                  >
                    {p.label}
                  </button>
                );
              })}
            </span>
            <label className="style-field shape-toggle">
              <input
                type="checkbox"
                data-testid="style-fill"
                checked={style.fill ?? false}
                disabled={controlsLocked}
                onChange={(e) => onStyle({ ...style, fill: e.target.checked })}
              />
              <span>Fill</span>
            </label>
            <label className="style-field shape-toggle">
              <input
                type="checkbox"
                data-testid="style-stroke"
                checked={style.stroke ?? true}
                disabled={controlsLocked}
                onChange={(e) => onStyle({ ...style, stroke: e.target.checked })}
              />
              <span>Border</span>
            </label>
            <label className="style-field">
              <span>Blend</span>
              <select
                data-testid="style-blend"
                value={style.blend ?? "normal"}
                disabled={controlsLocked}
                onChange={(e) => onStyle({ ...style, blend: e.target.value as "normal" | "multiply" })}
              >
                <option value="normal">Normal</option>
                <option value="multiply">Multiply</option>
              </select>
            </label>
          </div>
        )}
        <label className="style-field">
          <span>Text size</span>
          <input
            type="range"
            data-testid="style-font"
            min={0.015}
            max={0.08}
            step={0.005}
            value={style.fontSize}
            disabled={controlsLocked}
            onChange={(e) => onStyle({ ...style, fontSize: Number(e.target.value) })}
          />
          <span className="style-value" data-testid="style-font-value">
            {(style.fontSize * 1000).toFixed(0)}
          </span>
        </label>
      </div>

      <div className="layer-controls">
        {/* Prominent, brand-colored chip: always shows where ink will land. */}
        <span
          className="pill active-layer-indicator"
          data-testid="active-layer-indicator"
          title="New annotations are drawn on this layer"
        >
          Drawing on: {activeLayer ? activeLayer.name : "no editable layer — draw to create one"}
        </span>
        <label className="style-field">
          <span>Active layer</span>
          <select
            data-testid="active-layer"
            value={activeLayerId ?? ""}
            disabled={editableLayers.length === 0}
            onChange={(e) => onActiveLayer(e.target.value)}
          >
            {editableLayers.length === 0 && <option value="">No editable layer</option>}
            {editableLayers.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          data-testid="new-layer"
          className="new-layer-btn"
          disabled={!canDraw}
          onClick={onNewLayer}
        >
          + New layer
        </button>
        {/* "Edit this layer": activates the focused (editable, non-active) layer
            so its objects become editable. The active layer is the ONLY edit
            target (Bug #2), changed explicitly here or via the selector. */}
        {canEditFocusedLayer && (
          <button
            type="button"
            data-testid="edit-this-layer"
            className="edit-layer-btn"
            onClick={onEditLayer}
            title="Make this layer the active edit target"
          >
            Edit this layer{focusedLayerName ? `: ${focusedLayerName}` : ""}
          </button>
        )}
        {showEditLayerHint && (
          <span
            className="edit-layer-hint"
            data-testid="edit-layer-hint"
            role="status"
          >
            Editing happens on the active layer — Edit this layer?
          </span>
        )}
        <button
          type="button"
          data-testid="delete-object"
          className="delete-object-btn"
          disabled={selectionCount === 0 || !canDeleteSelection}
          onClick={onDelete}
        >
          Delete{selectionCount > 1 ? ` (${selectionCount})` : ""}
        </button>
        <span className="pill" data-testid="object-count" title="Live annotation count">
          {objectCount} objects
        </span>
        <span
          className={`pill conn-pill conn-${connStatus}`}
          data-testid="conn-status"
          title="Realtime connection"
        >
          {connStatus === "open" ? "live" : connStatus}
        </span>
      </div>
    </div>
  );
}

// ===========================================================================
// Edit canvas — per-page pointer capture + wet-object rendering
// ===========================================================================

/** A page-relative point captured during a gesture. */
type PRPoint = { x: number; y: number };

/**
 * The editing surface that sits ABOVE a page's dry annotation overlay. It is an
 * absolutely-positioned canvas filling the page wrapper; it captures pointer
 * events and renders the in-progress "wet" object via @troubastack/ink.
 *
 * Coordinate mapping: pointer clientX/Y → page-relative [0,1] via the page
 * canvas's bounding rect (pointerToPageXY). Because getBoundingClientRect is in
 * CSS px after layout, the mapping is correct under any zoom/scroll. The canvas
 * backing store is sized to the page box × DPR so the wet preview is crisp and
 * matches the dry overlay exactly.
 */
function EditCanvas({
  page,
  tool,
  style,
  drawLocked,
  objects,
  layersById,
  visible,
  selectedUuids,
  isObjectEditable,
  isObjectEditableNow,
  onSelect,
  onFocusLayer,
  onCommitDraw,
  onCommitMove,
  onCommitResize,
}: {
  page: number;
  tool: Tool;
  style: AnnotationStyle;
  drawLocked: boolean;
  objects: AnnotationObject[];
  layersById: Map<string, AnnotationLayer>;
  visible: LayerVisibility;
  selectedUuids: string[];
  // Whether an object's layer is editable at all (owner/rw) — drives the lock cue.
  isObjectEditable: (obj: AnnotationObject) => boolean;
  // Whether an object may be moved/resized/deleted/restyled RIGHT NOW: it is on
  // the ACTIVE editable layer. Gates the move/resize gestures (Bug #2).
  isObjectEditableNow: (obj: AnnotationObject) => boolean;
  onSelect: (uuids: string[]) => void;
  // Focus an object's layer when it is single-click selected (cross-layer focus,
  // WITHOUT activating it — editing stays scoped to the active layer).
  onFocusLayer: (layerId: string) => void;
  onCommitDraw: (tool: DrawTool, page: number, path: PRPoint[], text?: string) => void;
  onCommitMove: (obj: AnnotationObject) => void;
  onCommitResize: (obj: AnnotationObject) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  // The active gesture: a draw path, a move drag, a resize-handle drag, a
  // rubber-band marquee, or none. Kept in a ref so pointer handlers read the
  // latest without re-binding.
  const gestureRef = useRef<
    | { mode: "draw"; path: PRPoint[] }
    | { mode: "move"; obj: AnnotationObject; start: PRPoint; preview: AnnotationObject }
    | { mode: "resize"; obj: AnnotationObject; handle: HandleId; start: PRPoint; preview: AnnotationObject }
    | { mode: "marquee"; start: PRPoint }
    | null
  >(null);
  const [, forceRepaint] = useState(0);
  // The live rubber-band rect (page-relative [0,1]) while marqueeing, for the
  // DOM selection-box overlay. null when not marqueeing.
  const [marquee, setMarquee] = useState<SelectRect | null>(null);
  // The page box size in CSS px (set by sizeToPage). Drives the render-time
  // TextMeasure so text bboxes / handles position correctly in the DOM overlay.
  const [pageBoxPx, setPageBoxPx] = useState<{ w: number; h: number } | null>(null);
  // Bumped on each resize-preview tick so the DOM handles follow the drag.
  const [, forceHandles] = useState(0);

  // Objects on THIS page that are currently visible (for hit-testing on select).
  const pageObjects = useMemo(
    () =>
      objects
        .filter((o) => o.page === page)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        }),
    [objects, page, layersById, visible],
  );

  // Selected objects on THIS page (for the DOM bbox highlights).
  const selectedSet = useMemo(() => new Set(selectedUuids), [selectedUuids]);
  const selectedOnPage = useMemo(
    () => pageObjects.filter((o) => selectedSet.has(o.uuid)),
    [pageObjects, selectedSet],
  );

  // Match the edit canvas's backing store + CSS box to the page box (the sibling
  // .pdf-canvas), so [0,1] maps identically to the dry overlay. Re-measure on
  // layout changes (zoom/resize) via a ResizeObserver on the page wrapper.
  const sizeToPage = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const wrapper = canvas.parentElement;
    const pageCanvas = wrapper?.querySelector<HTMLElement>(".pdf-canvas");
    if (!pageCanvas) return;
    const dpr = window.devicePixelRatio || 1;
    const w = pageCanvas.clientWidth;
    const h = pageCanvas.clientHeight;
    if (w <= 0 || h <= 0) return;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    canvas.style.width = `${w}px`;
    canvas.style.height = `${h}px`;
    setPageBoxPx((prev) => (prev && prev.w === w && prev.h === h ? prev : { w, h }));
    repaint();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Paint the wet object / move preview onto the edit canvas. (Selection boxes
  // and per-object bbox highlights are DOM overlays, not canvas — so e2e can
  // query them and they stay crisp at any zoom.)
  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    const box = { x: 0, y: 0, w: canvas.width / dpr, h: canvas.height / dpr };

    const g = gestureRef.current;
    if (g?.mode === "draw" && g.path.length > 0 && tool !== "select") {
      const wet = buildWet(tool as DrawTool, g.path, style);
      if (wet) renderObjects(ctx, [toInkObject(wet) as InkObject], box);
    } else if (g?.mode === "move" || g?.mode === "resize") {
      renderObjects(ctx, [toInkObject(g.preview) as InkObject], box);
    }
  }, [tool, style]);

  // Keep the canvas sized to its page; observe the page wrapper for zoom/resize.
  useLayoutEffect(() => {
    sizeToPage();
    const canvas = canvasRef.current;
    const wrapper = canvas?.parentElement;
    if (!wrapper) return;
    const ro = new ResizeObserver(() => sizeToPage());
    ro.observe(wrapper);
    return () => ro.disconnect();
  }, [sizeToPage]);

  // Repaint when the page objects / tool / style change.
  useEffect(() => {
    repaint();
  }, [repaint]);

  function pageRelative(e: React.PointerEvent): PRPoint {
    const canvas = canvasRef.current!;
    return pointerToPageXY(e.clientX, e.clientY, canvas);
  }

  // Build a TextMeasure for the CURRENT page box (CSS px). measureText runs on a
  // shared offscreen ctx with the SAME font the renderer uses, so a text bbox
  // matches the painted glyphs. The page box is the edit canvas's CSS size.
  const textMeasure = useCallback((): TextMeasure | undefined => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;
    const pageW = parseFloat(canvas.style.width) || canvas.clientWidth;
    const pageH = parseFloat(canvas.style.height) || canvas.clientHeight;
    if (pageW <= 0 || pageH <= 0) return undefined;
    return {
      pageW,
      pageH,
      widthPx: (text, fontPx) => measureTextWidth(text, fontPx),
    };
  }, []);

  // The page box size in CSS px (for px↔[0,1] handle math). null until laid out.
  const pageDims = useCallback((): { w: number; h: number } | null => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const w = parseFloat(canvas.style.width) || canvas.clientWidth;
    const h = parseFloat(canvas.style.height) || canvas.clientHeight;
    return w > 0 && h > 0 ? { w, h } : null;
  }, []);

  // The bbox of the single selected object on this page (drives resize handles).
  const selectedSingle = selectedOnPage.length === 1 ? selectedOnPage[0] : null;

  function onPointerDown(e: React.PointerEvent) {
    if (e.button !== 0) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const pt = pageRelative(e);

    // A read-only (locked) focused layer blocks all drawing gestures; selecting
    // is still allowed so the user can browse/select the layer's annotations.
    if (drawLocked && tool !== "select") return;

    if (tool === "select") {
      const measure = textMeasure();
      const dims = pageDims();
      // If a single editable-NOW object is selected, a press within a small corner
      // handle zone starts a RESIZE — but ONLY when the bbox is big enough to show
      // handles (handleAtPx returns null for a small object, so its body stays a
      // MOVE zone). Otherwise the press falls through to the move/select path (Bug #2).
      if (selectedSingle && isObjectEditableNow(selectedSingle) && dims) {
        const b = objectBBox(selectedSingle, measure);
        const handle = handleAtPx(b, pt, dims.w, dims.h);
        if (handle) {
          canvas.setPointerCapture(e.pointerId);
          gestureRef.current = {
            mode: "resize",
            obj: selectedSingle,
            handle,
            start: pt,
            preview: selectedSingle,
          };
          return;
        }
      }
      // Hit-test topmost (last drawn) visible object on this page — across ALL
      // visible layers, not just the focused one.
      const hit = [...pageObjects].reverse().find((o) => hitTest(o, pt.x, pt.y, 0.02, measure));
      if (hit) {
        onSelect([hit.uuid]);
        // Cross-layer focus: clicking an object focuses its layer (so the scoped
        // annotation list shows it and the user can see which layer it's on) —
        // WITHOUT activating it. Editing stays scoped to the active layer (Bug #2).
        onFocusLayer(hit.layerId);
        // Only start a MOVE gesture for objects editable RIGHT NOW (on the active
        // editable layer). An object on a locked OR non-active layer is selectable
        // (to inspect / show its cue) but never movable — no transient move, no
        // mutation. The server `forbidden` reject is only a backstop.
        if (isObjectEditableNow(hit)) {
          canvas.setPointerCapture(e.pointerId);
          gestureRef.current = { mode: "move", obj: hit, start: pt, preview: hit };
        }
      } else {
        // Empty space → start a rubber-band marquee.
        onSelect([]);
        canvas.setPointerCapture(e.pointerId);
        gestureRef.current = { mode: "marquee", start: pt };
        setMarquee(normalizeRect(pt, pt));
      }
      return;
    }

    if (tool === "text") {
      // Click → inline prompt → text object at the anchor.
      const text = window.prompt("Text annotation");
      if (text && text.trim()) onCommitDraw("text", page, [pt], text.trim());
      return;
    }

    canvas.setPointerCapture(e.pointerId);
    gestureRef.current = { mode: "draw", path: [pt] };
    forceRepaint((n) => n + 1);
  }

  function onPointerMove(e: React.PointerEvent) {
    const g = gestureRef.current;
    if (!g) return;
    const pt = pageRelative(e);
    if (g.mode === "draw") {
      g.path.push(pt);
      repaint();
    } else if (g.mode === "move") {
      const dx = pt.x - g.start.x;
      const dy = pt.y - g.start.y;
      g.preview = translateObject(g.obj, dx, dy);
      repaint();
    } else if (g.mode === "resize") {
      const dx = pt.x - g.start.x;
      const dy = pt.y - g.start.y;
      g.preview = resizeObject(g.obj, g.handle, dx, dy, textMeasure());
      repaint();
      forceHandles((n) => n + 1); // move the DOM handles + bbox with the drag
    } else if (g.mode === "marquee") {
      setMarquee(normalizeRect(g.start, pt));
    }
  }

  function onPointerUp(e: React.PointerEvent) {
    const g = gestureRef.current;
    gestureRef.current = null;
    const canvas = canvasRef.current;
    if (canvas?.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
    if (!g) return;
    if (g.mode === "draw" && tool !== "select") {
      onCommitDraw(tool as DrawTool, page, g.path);
    } else if (g.mode === "move") {
      // Only commit if it actually moved.
      const moved =
        g.preview.points.some((p, i) => p.x !== g.obj.points[i].x || p.y !== g.obj.points[i].y);
      if (moved) onCommitMove(g.preview);
    } else if (g.mode === "resize") {
      // Commit if the geometry (points) or the text fontSize actually changed.
      const changed =
        g.preview.points.some((p, i) => p.x !== g.obj.points[i].x || p.y !== g.obj.points[i].y) ||
        g.preview.style.fontSize !== g.obj.style.fontSize;
      if (changed) onCommitResize(g.preview);
    } else if (g.mode === "marquee") {
      const pt = pageRelative(e);
      const rect = normalizeRect(g.start, pt);
      setMarquee(null);
      if (isMarquee(rect)) {
        const measure = textMeasure();
        const hits = pageObjects
          .filter((o) => intersectsRect(o, rect, measure))
          .map((o) => o.uuid);
        onSelect(hits);
      }
    }
    repaint();
  }

  return (
    <>
      <canvas
        ref={canvasRef}
        className={`edit-canvas tool-${tool}`}
        data-testid="edit-canvas"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
      />
      {/* Selection overlays: DOM elements positioned in % of the page box, so
          they track the page under any zoom AND are queryable in e2e. */}
      <div className="selection-overlay" aria-hidden="true">
        {selectedOnPage.map((o) => {
          // While resizing THIS object, draw the live preview box so the bbox +
          // handles follow the drag.
          const g = gestureRef.current;
          const previewing =
            g && (g.mode === "resize" || g.mode === "move") && g.obj.uuid === o.uuid
              ? g.preview
              : o;
          const renderMeasure: TextMeasure | undefined = pageBoxPx
            ? { pageW: pageBoxPx.w, pageH: pageBoxPx.h, widthPx: measureTextWidth }
            : undefined;
          const b = objectBBox(previewing, renderMeasure);
          const everEditable = isObjectEditable(o);
          const locked = !everEditable; // truly read-only layer
          const editableNow = isObjectEditableNow(o);
          // Editable layer but NOT the active one → inspect-only, distinct cue.
          const inactiveEditable = everEditable && !editableNow;
          // Bug #2: suppress handles when the on-screen bbox is too small to show them
          // safely — the object is then move-only (handles would overlap the body).
          const bigEnough = pageBoxPx ? handlesVisible(b, pageBoxPx.w, pageBoxPx.h) : false;
          const showHandles = editableNow && selectedOnPage.length === 1 && bigEnough;
          return (
            <div
              key={o.uuid}
              className={`selected-bbox${locked ? " readonly" : ""}${
                inactiveEditable ? " inactive" : ""
              }`}
              data-testid="selected-bbox"
              style={{
                left: `${b.minX * 100}%`,
                top: `${b.minY * 100}%`,
                width: `${(b.maxX - b.minX) * 100}%`,
                height: `${(b.maxY - b.minY) * 100}%`,
              }}
            >
              {/* Locked cue: a small greyed lock pinned at the bbox corner. */}
              {locked && (
                <span
                  className="bbox-lock"
                  data-testid="bbox-lock"
                  title="Read-only layer — this annotation can't be edited"
                  aria-label="Read-only annotation"
                >
                  🔒
                </span>
              )}
              {/* Resize handles: only for the single, active-editable selection. */}
              {showHandles &&
                CORNER_HANDLES.map((h) => {
                  // Handle position relative to THIS bbox (the box is the parent,
                  // so 0%/100% are its own corners).
                  const left = h === "nw" || h === "sw" ? 0 : 100;
                  const top = h === "nw" || h === "ne" ? 0 : 100;
                  return (
                    <span
                      key={h}
                      className={`resize-handle handle-${h}`}
                      data-testid="resize-handle"
                      data-handle={h}
                      style={{ left: `${left}%`, top: `${top}%` }}
                    />
                  );
                })}
            </div>
          );
        })}
        {marquee && (
          <div
            className="selection-box"
            data-testid="selection-box"
            style={{
              left: `${marquee.x0 * 100}%`,
              top: `${marquee.y0 * 100}%`,
              width: `${(marquee.x1 - marquee.x0) * 100}%`,
              height: `${(marquee.y1 - marquee.y0) * 100}%`,
            }}
          />
        )}
      </div>
    </>
  );
}

// A shared offscreen 2D context for text measurement (font matches web/ink).
let measureCtx: CanvasRenderingContext2D | null = null;
function measureTextWidth(text: string, fontPx: number): number {
  if (!measureCtx) {
    const c = document.createElement("canvas");
    measureCtx = c.getContext("2d");
  }
  if (!measureCtx) return text.length * fontPx * 0.5; // crude fallback
  measureCtx.font = `${fontPx}px system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
  return measureCtx.measureText(text).width;
}

/** Build a wet preview object from an in-progress gesture (no uuid needed). */
function buildWet(tool: DrawTool, path: PRPoint[], style: AnnotationStyle): AnnotationObject | null {
  if (path.length === 0) return null;
  return {
    uuid: "wet",
    layerId: "wet",
    type: tool,
    points: pointsForTool(tool, path),
    page: 0,
    text: "",
    style,
  };
}

/** Map an API annotation object to the renderer's InkObject view (same shape). */
function toInkObject(o: AnnotationObject): InkObject {
  return {
    uuid: o.uuid,
    layerId: o.layerId,
    type: o.type,
    points: o.points,
    page: o.page,
    text: o.text,
    style: o.style,
  };
}

// ===========================================================================
// Collapsible details wrapper
// ===========================================================================

function Details({ title, children }: { title: string; children: React.ReactNode }) {
  // Default OPEN: the viewer is the headline, but the existing flows expect the
  // metadata/files controls to be reachable without an extra click.
  const [open, setOpen] = useState(true);
  return (
    <section className="card details-section" data-testid="details-section">
      <button
        type="button"
        className="details-toggle"
        data-testid="details-toggle"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        {open ? "▾" : "▸"} {title}
      </button>
      {open && <div className="details-body">{children}</div>}
    </section>
  );
}

// ===========================================================================
// Details / Files / Delete (unchanged behavior + data-testids)
// ===========================================================================

function Metadata({
  bandId,
  song,
  onSaved,
}: {
  bandId: string;
  song: Song;
  onSaved: (s: Song) => void;
}) {
  const [title, setTitle] = useState(song.title);
  const [artist, setArtist] = useState(song.artist ?? "");
  const [key, setKey] = useState(song.key ?? "");
  const [tempo, setTempo] = useState(song.tempo != null ? String(song.tempo) : "");
  const [tags, setTags] = useState((song.tags ?? []).join(", "));
  const [notes, setNotes] = useState(song.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      const tagList = tags
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);
      const updated = await api.updateSong(bandId, song.id, {
        title,
        artist,
        key,
        tempo: tempo === "" ? 0 : Number(tempo),
        tags: tagList,
        notes,
      });
      onSaved(updated);
      setNotice("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Details</h2>
      <form onSubmit={onSave} data-testid="song-meta-form">
        <label>
          Title
          <input
            data-testid="meta-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            required
          />
        </label>
        <label>
          Artist
          <input
            data-testid="meta-artist"
            value={artist}
            onChange={(e) => setArtist(e.target.value)}
          />
        </label>
        <label>
          Key
          <input data-testid="meta-key" value={key} onChange={(e) => setKey(e.target.value)} />
        </label>
        <label>
          Tempo (BPM)
          <input
            data-testid="meta-tempo"
            type="number"
            value={tempo}
            onChange={(e) => setTempo(e.target.value)}
          />
        </label>
        <label>
          Tags (comma-separated)
          <input data-testid="meta-tags" value={tags} onChange={(e) => setTags(e.target.value)} />
        </label>
        <label>
          Notes
          <textarea
            data-testid="meta-notes"
            rows={3}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </label>
        <div className="inline-form">
          <button type="submit" data-testid="meta-save" disabled={busy}>
            Save details
          </button>
          {notice && (
            <span className="notice" data-testid="meta-notice">
              {notice}
            </span>
          )}
        </div>
      </form>
      <ErrorBanner message={error} />
    </section>
  );
}

function fmtSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function Files({ bandId, songId }: { bandId: string; songId: string }) {
  const [files, setFiles] = useState<SongFile[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    try {
      const list = await api.listFiles(bandId, songId);
      list.sort((a, b) => a.displayOrder - b.displayOrder);
      setFiles(list);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load files");
    }
  }, [bandId, songId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onUpload(e: FormEvent) {
    e.preventDefault();
    const f = fileInput.current?.files?.[0];
    if (!f) return;
    setError(null);
    setBusy(true);
    try {
      await api.uploadFile(bandId, songId, f);
      if (fileInput.current) fileInput.current.value = "";
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to upload");
    } finally {
      setBusy(false);
    }
  }

  async function rename(file: SongFile) {
    const next = window.prompt("New filename", file.filename);
    if (!next || next === file.filename) return;
    setError(null);
    try {
      await api.updateFile(bandId, songId, file.id, { filename: next });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to rename");
    }
  }

  async function remove(file: SongFile) {
    if (!window.confirm(`Delete "${file.filename}"?`)) return;
    setError(null);
    try {
      await api.deleteFile(bandId, songId, file.id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete");
    }
  }

  async function move(index: number, dir: -1 | 1) {
    const other = index + dir;
    if (other < 0 || other >= files.length) return;
    const a = files[index];
    const b = files[other];
    setError(null);
    try {
      // Swap display orders.
      await api.updateFile(bandId, songId, a.id, { displayOrder: b.displayOrder });
      await api.updateFile(bandId, songId, b.id, { displayOrder: a.displayOrder });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reorder");
    }
  }

  return (
    <section className="card">
      <h2>Files</h2>

      <form onSubmit={onUpload} className="inline-form" data-testid="file-upload-form">
        <input ref={fileInput} type="file" data-testid="file-input" />
        <button type="submit" data-testid="file-upload" disabled={busy}>
          Upload
        </button>
      </form>

      <ErrorBanner message={error} />

      {files.length === 0 ? (
        <p className="muted" data-testid="files-empty">
          No files yet.
        </p>
      ) : (
        <ul className="list" data-testid="files-list">
          {files.map((f, i) => (
            <li key={f.id} data-testid="file-row">
              <span>
                <a
                  href={api.fileUrl(f.id)}
                  target="_blank"
                  rel="noreferrer"
                  data-testid="file-download"
                >
                  {f.filename}
                </a>{" "}
                <span className="muted">{fmtSize(f.size)}</span>
              </span>
              <span className="actions">
                <button
                  type="button"
                  data-testid="file-up"
                  disabled={i === 0}
                  onClick={() => move(i, -1)}
                >
                  ↑
                </button>
                <button
                  type="button"
                  data-testid="file-down"
                  disabled={i === files.length - 1}
                  onClick={() => move(i, 1)}
                >
                  ↓
                </button>
                <button type="button" data-testid="file-rename" onClick={() => rename(f)}>
                  Rename
                </button>
                <button type="button" data-testid="file-delete" onClick={() => remove(f)}>
                  Delete
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function DeleteSong({
  bandId,
  songId,
  onDeleted,
}: {
  bandId: string;
  songId: string;
  onDeleted: () => void;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onDelete() {
    if (!window.confirm("Delete this song? This cannot be undone.")) return;
    setError(null);
    setBusy(true);
    try {
      await api.deleteSong(bandId, songId);
      onDeleted();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete song");
      setBusy(false);
    }
  }

  return (
    <section className="card">
      <h2>Danger zone</h2>
      <div className="inline-form">
        <button type="button" data-testid="delete-song" disabled={busy} onClick={onDelete}>
          Delete song
        </button>
      </div>
      <ErrorBanner message={error} />
    </section>
  );
}
