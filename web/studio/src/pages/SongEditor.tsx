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
import { SyncClient, type SyncState } from "../sync";
import {
  COLOR_SWATCHES,
  DEFAULT_STYLE,
  buildObject,
  hitTest,
  isMeaningfulGesture,
  pointerToPageXY,
  pointsForTool,
  translateObject,
  type DrawTool,
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

          <Viewer bandId={bandId} songId={songId} myUserId={user?.id ?? null} />

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

/** May the current user draw into this layer? My own personal layers, or any
 *  shared read-write layer. (Conductor/mandatory/read-only stay view-only.) */
function isEditableLayer(l: AnnotationLayer, myUserId: string | null): boolean {
  if (l.access === "ro") return false;
  if (l.zone === "personal") return myUserId != null && l.ownerId === myUserId;
  if (l.zone === "shared") return true;
  return false;
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
}: {
  bandId: string;
  songId: string;
  myUserId: string | null;
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
  const [selectedUuid, setSelectedUuid] = useState<string | null>(null);
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
          isEditableLayer(l, myUserId),
      ),
    [doc.layers, myUserId, selectedFileId],
  );

  // Keep a valid active layer: prefer the current one if it's still editable,
  // else fall back to the first editable layer (or null → "New layer" offered).
  useEffect(() => {
    setActiveLayerId((cur) => {
      if (cur && editableLayers.some((l) => l.id === cur)) return cur;
      return editableLayers[0]?.id ?? null;
    });
  }, [editableLayers]);

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
      // A brand-new personal layer should be visible to me immediately.
      setVisible((v) => ({ ...v, [id]: true }));
      return id;
    },
    [myUserId, selectedFileId, doc.layers],
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
      syncRef.current.createObject(obj);
      setSelectedUuid(obj.uuid);
    },
    [ensureActiveLayer, style],
  );

  // Commit a move: send the translated object (move mutation).
  const commitMove = useCallback((obj: AnnotationObject) => {
    if (!syncRef.current) return;
    syncRef.current.updateObject("move", obj);
  }, []);

  // Delete the selected object.
  const deleteSelected = useCallback(() => {
    if (!selectedUuid || !syncRef.current) return;
    syncRef.current.deleteObject(selectedUuid);
    setSelectedUuid(null);
  }, [selectedUuid]);

  // Delete/Backspace removes the current selection (ignored while typing in a
  // form field, so the Details inputs below keep working normally).
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Delete" && e.key !== "Backspace") return;
      const t = e.target as HTMLElement | null;
      if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable)) return;
      if (!selectedUuid) return;
      e.preventDefault();
      deleteSelected();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedUuid, deleteSelected]);

  // Clear a stale selection if the selected object disappears (deleted remotely).
  useEffect(() => {
    if (selectedUuid && !doc.objects.some((o) => o.uuid === selectedUuid)) {
      setSelectedUuid(null);
    }
  }, [doc.objects, selectedUuid]);

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

        // Size the overlay to the canvas's ACTUAL rendered dimensions, then
        // paint THIS page's objects off those dims.
        const overlay = overlayRefs.current[i];
        if (overlay) {
          overlay.width = canvas.width;
          overlay.height = canvas.height;
          overlay.style.width = `${canvas.style.width}`;
          overlay.style.height = `${canvas.style.height}`;
          paintOverlay(overlay, i, dpr);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, scale, numPages, isPdf, zoomMode, paintOverlay]);

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
          if (t !== "select") setSelectedUuid(null);
        }}
        style={style}
        onStyle={setStyle}
        editableLayers={editableLayers}
        activeLayerId={activeLayerId}
        onActiveLayer={setActiveLayerId}
        onNewLayer={() => createPersonalLayer()}
        canDraw={myUserId != null && selectedFileId != null}
        objectCount={doc.objects.length}
        selectedUuid={selectedUuid}
        onDelete={deleteSelected}
        connStatus={connStatus}
      />

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
                  objects={doc.objects}
                  layersById={layersById}
                  visible={visible}
                  selectedUuid={selectedUuid}
                  onSelect={setSelectedUuid}
                  onCommitDraw={commitDraw}
                  onCommitMove={commitMove}
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
                objects={doc.objects}
                layersById={layersById}
                visible={visible}
                selectedUuid={selectedUuid}
                onSelect={setSelectedUuid}
                onCommitDraw={commitDraw}
                onCommitMove={commitMove}
              />
            </div>
          )}
        </div>

        {sidebarOpen && (
          <LayersPanel
            layers={sortedLayers}
            visible={visible}
            myUserId={myUserId}
            onToggle={toggle}
          />
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
  onToggle,
}: {
  layers: AnnotationLayer[];
  visible: LayerVisibility;
  myUserId: string | null;
  onToggle: (id: string) => void;
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
            return (
              <li key={l.id} data-testid="layer-item">
                <label className="layer-row">
                  <input
                    type="checkbox"
                    data-testid="layer-toggle"
                    checked={!!visible[l.id]}
                    disabled={l.mandatory}
                    onChange={() => onToggle(l.id)}
                  />
                  <span className="layer-name">{l.name}</span>
                </label>
                <span className="pill">{tag}</span>
                {l.mandatory && <span className="pill mandatory-pill">required</span>}
              </li>
            );
          })}
        </ul>
      )}
    </aside>
  );
}

// ===========================================================================
// Editor toolbar — tools palette, style controls, active layer, object count
// ===========================================================================

const TOOLS: { tool: Tool; label: string; testid: string }[] = [
  { tool: "select", label: "Select", testid: "tool-select" },
  { tool: "freehand", label: "Pen", testid: "tool-freehand" },
  { tool: "line", label: "Line", testid: "tool-line" },
  { tool: "rect", label: "Rect", testid: "tool-rect" },
  { tool: "ellipse", label: "Ellipse", testid: "tool-ellipse" },
  { tool: "highlight", label: "Highlight", testid: "tool-highlight" },
  { tool: "text", label: "Text", testid: "tool-text" },
];

function EditorToolbar({
  tool,
  onTool,
  style,
  onStyle,
  editableLayers,
  activeLayerId,
  onActiveLayer,
  onNewLayer,
  canDraw,
  objectCount,
  selectedUuid,
  onDelete,
  connStatus,
}: {
  tool: Tool;
  onTool: (t: Tool) => void;
  style: AnnotationStyle;
  onStyle: (s: AnnotationStyle) => void;
  editableLayers: AnnotationLayer[];
  activeLayerId: string | null;
  onActiveLayer: (id: string) => void;
  onNewLayer: () => void;
  canDraw: boolean;
  objectCount: number;
  selectedUuid: string | null;
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
            disabled={!canDraw && t.tool !== "select"}
            onClick={() => onTool(t.tool)}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="style-controls">
        <span className="swatches">
          {COLOR_SWATCHES.map((c) => (
            <button
              key={c}
              type="button"
              className={`swatch${style.color === c ? " active" : ""}`}
              style={{ background: c }}
              aria-label={`Color ${c}`}
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
            onChange={(e) => onStyle({ ...style, color: e.target.value })}
          />
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
            onChange={(e) => onStyle({ ...style, opacity: Number(e.target.value) })}
          />
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
            onChange={(e) => onStyle({ ...style, width: Number(e.target.value) })}
          />
        </label>
        <label className="style-field">
          <span>Text size</span>
          <input
            type="range"
            data-testid="style-font"
            min={0.015}
            max={0.08}
            step={0.005}
            value={style.fontSize}
            onChange={(e) => onStyle({ ...style, fontSize: Number(e.target.value) })}
          />
        </label>
      </div>

      <div className="layer-controls">
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
        <button
          type="button"
          data-testid="delete-object"
          className="delete-object-btn"
          disabled={!selectedUuid}
          onClick={onDelete}
        >
          Delete
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
  objects,
  layersById,
  visible,
  selectedUuid,
  onSelect,
  onCommitDraw,
  onCommitMove,
}: {
  page: number;
  tool: Tool;
  style: AnnotationStyle;
  objects: AnnotationObject[];
  layersById: Map<string, AnnotationLayer>;
  visible: LayerVisibility;
  selectedUuid: string | null;
  onSelect: (uuid: string | null) => void;
  onCommitDraw: (tool: DrawTool, page: number, path: PRPoint[], text?: string) => void;
  onCommitMove: (obj: AnnotationObject) => void;
}) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  // The active gesture: a draw path, a move drag, or none. Kept in a ref so the
  // pointer handlers (bound via React props) read the latest without re-binding.
  const gestureRef = useRef<
    | { mode: "draw"; path: PRPoint[] }
    | { mode: "move"; obj: AnnotationObject; start: PRPoint; preview: AnnotationObject }
    | null
  >(null);
  const [, forceRepaint] = useState(0);

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
    repaint();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Paint the wet object (and a selection box) onto the edit canvas.
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
    } else if (g?.mode === "move") {
      renderObjects(ctx, [toInkObject(g.preview) as InkObject], box);
    }

    // Selection outline (page-relative bbox → px).
    if (selectedUuid) {
      const sel = pageObjects.find((o) => o.uuid === selectedUuid);
      if (sel) {
        const xs = sel.points.map((p) => p.x);
        const ys = sel.points.map((p) => p.y);
        const minX = Math.min(...xs) * box.w;
        const minY = Math.min(...ys) * box.h;
        const maxX = Math.max(...xs) * box.w;
        const maxY = Math.max(...ys) * box.h;
        ctx.save();
        ctx.setLineDash([4, 3]);
        ctx.strokeStyle = "#2563eb";
        ctx.lineWidth = 1.5;
        ctx.strokeRect(minX - 4, minY - 4, maxX - minX + 8, maxY - minY + 8);
        ctx.restore();
      }
    }
  }, [tool, style, selectedUuid, pageObjects]);

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

  // Repaint when the selection / page objects / tool / style change.
  useEffect(() => {
    repaint();
  }, [repaint]);

  function pageRelative(e: React.PointerEvent): PRPoint {
    const canvas = canvasRef.current!;
    return pointerToPageXY(e.clientX, e.clientY, canvas);
  }

  function onPointerDown(e: React.PointerEvent) {
    if (e.button !== 0) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const pt = pageRelative(e);

    if (tool === "select") {
      // Hit-test topmost (last drawn) visible object on this page.
      const hit = [...pageObjects].reverse().find((o) => hitTest(o, pt.x, pt.y));
      if (hit) {
        onSelect(hit.uuid);
        canvas.setPointerCapture(e.pointerId);
        gestureRef.current = { mode: "move", obj: hit, start: pt, preview: hit };
      } else {
        onSelect(null);
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
    }
    repaint();
  }

  return (
    <canvas
      ref={canvasRef}
      className={`edit-canvas tool-${tool}`}
      data-testid="edit-canvas"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    />
  );
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
