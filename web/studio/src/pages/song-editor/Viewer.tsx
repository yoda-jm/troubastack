/**
 * The song Viewer/editor (T10 extraction — moved verbatim from SongEditor.tsx):
 * PDF.js rasterization + zoom/DPR, the dry annotation overlay (@troubastack/ink,
 * I8), the file strip, realtime sync (WebSocket outbox/echo reconcile), and the
 * composed toolbar / wet canvas / side panels. Behavior + data-testids unchanged.
 */
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import * as pdfjs from "pdfjs-dist";
// Vite resolves ?url to the emitted asset path; PDF.js needs its worker URL.
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { renderObjects, type InkObject } from "@troubastack/ink";
import {
  api,
  type AnnotationDoc,
  type AnnotationLayer,
  type AnnotationObject,
  type AnnotationStyle,
  type Role,
  type SongFile,
} from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import { SyncClient, type SyncState } from "../../sync";
import {
  DEFAULT_STYLE,
  buildObject,
  isMeaningfulGesture,
  pointsForTool,
  type DrawTool,
  type Tool,
} from "../../editor";
import { EditorToolbar } from "./Toolbar";
import { EditCanvas } from "./WetCanvas";
import { MyFilesEditor } from "./MyFilesEditor";
import { LayersPanel, AnnotationList } from "./SidePanels";
import { toInkObject, isEditableLayer, compareObjectZ, type LayerVisibility } from "./helpers";

pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

// Discrete percentage stops the −/+ buttons step through.
const ZOOM_PERCENTS = [50, 75, 100, 125, 150, 200, 300];

// Continuous Ctrl/⌘-wheel zoom bounds (T27 stage 1). Wider than the discrete
// stops so a pinch can go slightly beyond the button range.
const MIN_ZOOM_SCALE = 0.25; // 25%
const MAX_ZOOM_SCALE = 5.0; // 500%
// Wheel→zoom sensitivity: scale multiplies by exp(-deltaY·k). Small so a trackpad
// pinch (fine-grained ctrlKey wheel) feels smooth rather than jumpy.
const WHEEL_ZOOM_K = 0.0015;
// Idle gap after the last wheel tick before we commit the crisp re-raster.
const WHEEL_SETTLE_MS = 120;

/** A zoom selection is either a fit mode or an explicit percentage. */
type ZoomMode = "fit-width" | "fit-page" | number;
// ===========================================================================
// Viewer
// ===========================================================================

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
export function Viewer({
  bandId,
  songId,
  songTitle,
  myUserId,
  myRole,
}: {
  bandId: string;
  songId: string;
  songTitle: string;
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
  // The pages wrapper — the CSS-transform target for live wheel-zoom (T27 s1).
  const contentRef = useRef<HTMLDivElement | null>(null);
  // Image element box (for image-overlay sizing).
  const imgRef = useRef<HTMLImageElement | null>(null);
  const imgOverlayRef = useRef<HTMLCanvasElement | null>(null);
  // How many times the PDF pages have actually been RASTERIZED (PDF.js page
  // render). The PDF render effect bumps this; an annotation edit must NOT.
  // Surfaced via a hidden data-testid so e2e can assert no re-raster on edit.
  const pdfRenderCountRef = useRef(0);
  const [pdfRenderCount, setPdfRenderCount] = useState(0);
  // One-shot "settle" redraw (T-fix): after a file's FIRST successful render pass
  // we schedule exactly one more clean render on the next frame. This is the
  // belt-and-suspenders for the intermittent upside-down/blank first paint some
  // browsers show on initial load — the same thing a manual zoom (redraw) cures.
  // Bumping renderNonce re-runs the render effect; nudgedFileRef makes it fire at
  // most once per file open (not per zoom), so it can never loop.
  const [renderNonce, setRenderNonce] = useState(0);
  const nudgedFileRef = useRef<string | null>(null);
  const nudgeRafRef = useRef<number | null>(null);

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

  // layerId → z-rank (index in the sorted stack). Drives the shared z-order
  // comparator used by BOTH the dry overlay paint and the wet-canvas hit-test, so
  // render order and pick order stay identical (T27).
  const layerRank = useMemo(() => {
    const m = new Map<string, number>();
    sortedLayers.forEach((l, idx) => m.set(l.id, idx));
    return m;
  }, [sortedLayers]);

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

  // ---- selection-toolbar actions (T27 stage 2) ---------------------------
  // Recolor the selected object (a scoped setStyle carrying only a new color).
  const setObjectColor = useCallback(
    (uuid: string, color: string) => {
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj) return;
      restyleObject(uuid, { ...obj.style, color });
    },
    [doc.objects, restyleObject],
  );

  // Bring-to-front / send-to-back WITHIN the object's layer+page: compute a new
  // `order` from the current siblings (max+1 / min−1) and send a `reorder` (gated
  // + LWW server-side). Only the active-editable selection can be reordered.
  const reorderSelected = useCallback(
    (uuid: string, dir: "front" | "back") => {
      if (!syncRef.current) return;
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj || !isObjectEditableNow(obj)) return;
      const siblings = doc.objects.filter(
        (o) => o.uuid !== uuid && o.layerId === obj.layerId && o.page === obj.page,
      );
      if (siblings.length === 0) return; // nothing to move relative to
      const orders = siblings.map((o) => o.order ?? 0);
      const nextOrder =
        dir === "front" ? Math.max(...orders) + 1 : Math.min(...orders) - 1;
      if (nextOrder === (obj.order ?? 0)) return; // already there
      syncRef.current.reorderObject({ ...obj, order: nextOrder });
    },
    [doc.objects, isObjectEditableNow],
  );

  // Duplicate the selected object: a copy on the SAME (active editable) layer,
  // nudged down-right so it's visibly distinct, and selected in its place.
  const duplicateSelected = useCallback(
    (uuid: string) => {
      if (!syncRef.current) return;
      const obj = doc.objects.find((o) => o.uuid === uuid);
      if (!obj || !isObjectEditableNow(obj)) return;
      const off = 0.02;
      const copy: AnnotationObject = {
        ...obj,
        uuid: crypto.randomUUID(),
        points: obj.points.map((p) => ({
          ...p,
          x: Math.min(1, p.x + off),
          y: Math.min(1, p.y + off),
        })),
      };
      syncRef.current.createObject(copy);
      setSelectedUuids([copy.uuid]);
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

      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, overlay.width / dpr, overlay.height / dpr);
      const objs = doc.objects
        .filter((o) => o.page === page)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        })
        // Layer z-rank, then per-object order, then insertion (stable) — same
        // comparator as the hit-test so paint order matches pick order (T27).
        .sort((a, b) => compareObjectZ(a, b, layerRank));
      // Logical page box (CSS px) fills the whole overlay — sized from THIS
      // page's actual canvas dimensions, not page 0's.
      const pageRect = { x: 0, y: 0, w: overlay.width / dpr, h: overlay.height / dpr };
      renderObjects(ctx, objs.map(toInkObject) as InkObject[], pageRect);
    },
    [doc.objects, layersById, visible, layerRank],
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
    // In-flight PDF.js render tasks for THIS effect run. When the effect re-runs
    // (zoom, file switch, or the first 0→measured scale bump), React calls this
    // run's cleanup first — we cancel these tasks so an old render can't keep
    // painting a canvas that the new run has already resized. Resizing a canvas
    // (`canvas.width = …`) resets its 2D transform to identity, and a stale render
    // continuing under identity draws in PDF-native (Y-up) space → the page comes
    // out UPSIDE DOWN until the next clean re-render. Cancelling prevents that.
    const tasks: pdfjs.RenderTask[] = [];

    (async () => {
      const dpr = window.devicePixelRatio || 1;
      let renderedAny = false;
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
        const task = page.render({ canvasContext: ctx, viewport });
        tasks.push(task);
        try {
          await task.promise;
        } catch (err) {
          // A superseded render (cancelled in cleanup) rejects with
          // RenderingCancelledException — expected; stop this stale pass quietly.
          if ((err as { name?: string })?.name === "RenderingCancelledException") return;
          throw err;
        }
        if (cancelled) return;
        // Count an actual rasterization (one per page render). Editing an
        // annotation must never reach here, so this stays put on edits.
        pdfRenderCountRef.current += 1;
        setPdfRenderCount(pdfRenderCountRef.current);
        renderedAny = true;

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

      // One-shot settle redraw: once per file open, after its first full pass has
      // completed and laid out, schedule a single clean re-render on the next
      // frame. Cures the intermittent bad first paint without ever looping (the
      // re-run keeps the same selectedFileId, so it won't schedule again).
      if (!cancelled && renderedAny && nudgedFileRef.current !== selectedFileId) {
        nudgedFileRef.current = selectedFileId ?? null;
        nudgeRafRef.current = requestAnimationFrame(() => setRenderNonce((n) => n + 1));
      }
    })();

    return () => {
      cancelled = true;
      for (const t of tasks) t.cancel(); // stop any in-flight render on our canvases
      if (nudgeRafRef.current != null) cancelAnimationFrame(nudgeRafRef.current);
    };
    // Depends ONLY on what changes the rasterized pixels: the file, the scale,
    // the page count, and the zoom mode. NOT objects/visibility/paintOverlay —
    // those repaint the overlay canvases via renderOverlays without re-raster.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedFile, status, scale, numPages, zoomMode, renderNonce]);

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
  // A continuous wheel-zoom can land on a percentage that isn't one of the
  // discrete stops; surface it as an extra option so the readout stays truthful
  // and the <select> doesn't render blank.
  const customZoomPercent =
    typeof zoomMode === "number" && !ZOOM_PERCENTS.includes(zoomMode) ? zoomMode : null;

  // ---- Ctrl/⌘-wheel zoom-to-cursor (T27 stage 1) --------------------------
  // Plain wheel scrolls natively (never intercepted). Ctrl/⌘+wheel — and a
  // trackpad pinch, which the browser delivers as a wheel event with
  // ctrlKey===true — zooms toward the pointer. The load-bearing invariant
  // (Fable): DECOUPLE the visual zoom from rasterization. During a burst we only
  // apply a cheap CSS transform on the pages wrapper (instant feedback, zero
  // re-raster); the crisp PDF re-raster (the scale-keyed effect) is committed
  // exactly ONCE, on wheel-settle. A fast pinch therefore costs one raster, not
  // one per tick — which also keeps the flip-fix cancel-guard from thrashing.

  // Live burst state (mutated per tick, no re-render): the committed base scale,
  // the running target scale, and the pointer anchor (viewport offset vx/vy +
  // scale-invariant content fraction fx/fy) used to re-place the scroll on settle.
  const wheelBurstRef = useRef<{
    base: number; target: number; vx: number; vy: number; fx: number; fy: number;
  } | null>(null);
  const wheelSettleRef = useRef<number | null>(null);

  // Commit the burst: bake the live zoom into layout, then trigger exactly one
  // crisp re-raster. To avoid any size jump we first CSS-resize the existing page
  // canvases to the target (the browser stretches the current bitmap — instant,
  // momentarily soft), drop the transform, re-anchor the scroll against the
  // now-correct geometry, and only then setZoomMode (which re-rasters sharply in
  // place). The overlay/edit canvases are inset:0 over .pdf-page and follow.
  const commitWheelZoom = useCallback(() => {
    wheelSettleRef.current = null;
    const burst = wheelBurstRef.current;
    wheelBurstRef.current = null;
    if (!burst) return;
    const content = contentRef.current;
    const scroll = scrollRef.current;
    const targetPct = Math.min(
      Math.round(MAX_ZOOM_SCALE * 100),
      Math.max(Math.round(MIN_ZOOM_SCALE * 100), Math.round(burst.target * 100)),
    );
    const targetScale = targetPct / 100;

    for (let i = 0; i < pageCanvasRefs.current.length; i++) {
      const sz = pageSizesRef.current[i];
      const canvas = pageCanvasRefs.current[i];
      if (!sz || !canvas) continue;
      canvas.style.width = `${sz.w * targetScale}px`;
      canvas.style.height = `${sz.h * targetScale}px`;
    }
    if (content) {
      content.style.transform = "";
      content.style.transformOrigin = "";
      content.style.willChange = "";
    }
    if (scroll) {
      const cs = getComputedStyle(scroll);
      const padL = parseFloat(cs.paddingLeft) || 0;
      const padR = parseFloat(cs.paddingRight) || 0;
      const padT = parseFloat(cs.paddingTop) || 0;
      const padB = parseFloat(cs.paddingBottom) || 0;
      const contentW = Math.max(1, scroll.scrollWidth - padL - padR);
      const contentH = Math.max(1, scroll.scrollHeight - padT - padB);
      scroll.scrollLeft = Math.max(0, burst.fx * contentW - burst.vx);
      scroll.scrollTop = Math.max(0, burst.fy * contentH - burst.vy);
    }
    // One crisp re-raster (skip if the scale is effectively unchanged).
    if (!(typeof zoomMode === "number" && zoomMode === targetPct)) {
      setZoomMode(targetPct);
    }
  }, [zoomMode]);

  // The wheel handler. Held in a ref so the non-passive listener binds once (and
  // cleans up once) while always seeing the latest scale/zoomMode/isPdf.
  const wheelZoomHandler = useCallback(
    (e: WheelEvent) => {
      // Plain wheel → let the browser scroll the column (multi-page nav).
      if (!(e.ctrlKey || e.metaKey)) return;
      // Only PDFs rasterize at a scale; images are width-fit (zoom is a no-op),
      // so leave their ctrl-wheel to the browser rather than swallowing it.
      if (!isPdf) return;
      const scroll = scrollRef.current;
      const content = contentRef.current;
      if (!scroll || !content) return;
      // The ONLY branch that cancels the event — suppressing the browser's own
      // ctrl+wheel page-zoom (Fable: preventDefault ONLY on the ctrl/meta branch).
      e.preventDefault();

      // Normalize delta across deltaMode (0 px / 1 line / 2 page).
      let dy = e.deltaY;
      if (e.deltaMode === 1) dy *= 16;
      else if (e.deltaMode === 2) dy *= 100;

      let burst = wheelBurstRef.current;
      if (!burst) {
        // Burst start: anchor on the current committed scale + pointer. Geometry
        // is read from the SCROLL CONTAINER synchronously (Fable) — never the
        // async-decoded canvas, which hasn't re-rendered at the new scale.
        const cs = getComputedStyle(scroll);
        const padL = parseFloat(cs.paddingLeft) || 0;
        const padR = parseFloat(cs.paddingRight) || 0;
        const padT = parseFloat(cs.paddingTop) || 0;
        const padB = parseFloat(cs.paddingBottom) || 0;
        const bL = parseFloat(cs.borderLeftWidth) || 0;
        const bT = parseFloat(cs.borderTopWidth) || 0;
        const rect = scroll.getBoundingClientRect();
        // Pointer offset within the content viewport (inside border + padding).
        const vx = e.clientX - rect.left - bL - padL;
        const vy = e.clientY - rect.top - bT - padT;
        // Scale-invariant content fraction under the pointer — re-anchors
        // correctly after the raster grows/shrinks the content.
        const contentW = Math.max(1, scroll.scrollWidth - padL - padR);
        const contentH = Math.max(1, scroll.scrollHeight - padT - padB);
        const base = scale > 0 ? scale : 1;
        burst = {
          base,
          target: base,
          vx,
          vy,
          fx: (scroll.scrollLeft + vx) / contentW,
          fy: (scroll.scrollTop + vy) / contentH,
        };
        wheelBurstRef.current = burst;
        // Anchor the live scale at the pointer (wrapper-local coords).
        const wr = content.getBoundingClientRect();
        content.style.transformOrigin = `${e.clientX - wr.left}px ${e.clientY - wr.top}px`;
        content.style.willChange = "transform";
      }

      const factor = Math.exp(-dy * WHEEL_ZOOM_K);
      burst.target = Math.min(MAX_ZOOM_SCALE, Math.max(MIN_ZOOM_SCALE, burst.target * factor));
      content.style.transform = `scale(${burst.target / burst.base})`;

      if (wheelSettleRef.current != null) window.clearTimeout(wheelSettleRef.current);
      wheelSettleRef.current = window.setTimeout(commitWheelZoom, WHEEL_SETTLE_MS);
    },
    [isPdf, scale, commitWheelZoom],
  );

  // Bind the non-passive wheel listener once; call the latest handler via a ref.
  const wheelZoomRef = useRef(wheelZoomHandler);
  useEffect(() => {
    wheelZoomRef.current = wheelZoomHandler;
  }, [wheelZoomHandler]);
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const listener = (e: WheelEvent) => wheelZoomRef.current(e);
    el.addEventListener("wheel", listener, { passive: false });
    return () => {
      el.removeEventListener("wheel", listener);
      if (wheelSettleRef.current != null) window.clearTimeout(wheelSettleRef.current);
    };
  }, []);

  return (
    <section
      className={`card viewer${sidebarOpen ? "" : " sidebar-collapsed"}`}
      data-testid="song-viewer"
    >
      {/* Compact single-row header (T05): back · title · status. Replaces the
          old centered back-link + big <h1>, and hosts the status pills that used
          to sit between the toolbar's action buttons. */}
      <div className="editor-header" data-testid="editor-header">
        <Link
          className="editor-back"
          to={`/bands/${bandId}`}
          aria-label="Back to band"
          title="Back to band"
        >
          &larr;
        </Link>
        <span className="editor-song-title" data-testid="song-title">
          {songTitle}
        </span>
        <span className="editor-status">
          <span className="pill" data-testid="object-count" title="Live annotation count">
            {doc.objects.length} objects
          </span>
          <span
            className={`pill conn-pill conn-${connStatus}`}
            data-testid="conn-status"
            title="Realtime connection"
          >
            {connStatus === "open" ? "live" : connStatus}
          </span>
        </span>
      </div>

      <EditorToolbar
        tool={tool}
        onTool={(t) => {
          setTool(t);
          if (t !== "select") setSelectedUuids([]);
        }}
        style={effectiveStyle}
        onStyle={applyStyle}
        controlsLocked={controlsLocked}
        multiSelected={selectedUuids.length > 1}
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
        selectionCount={selectedUuids.length}
        canDeleteSelection={selectedUuids.some((u) => {
          const o = doc.objects.find((x) => x.uuid === u);
          return o != null && isObjectEditableNow(o);
        })}
        onDelete={deleteSelected}
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
            {customZoomPercent != null && (
              <option value={customZoomPercent}>{customZoomPercent}%</option>
            )}
          </select>
          <button type="button" data-testid="zoom-in" onClick={() => stepZoom(1)}>
            +
          </button>
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

          {/* Transform target for the live wheel-zoom (T27 stage 1): the pages
              scale cheaply here during a Ctrl/⌘-wheel burst; the crisp raster is
              committed on settle and this transform is reset. */}
          <div className="viewer-content" ref={contentRef}>
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
                  layerRank={layerRank}
                  visible={visible}
                  selectedUuids={selectedUuids}
                  isObjectEditable={isEditableObject}
                  isObjectEditableNow={isObjectEditableNow}
                  onSelect={setSelectedUuids}
                  onFocusLayer={focusLayerOnly}
                  onCommitDraw={commitDraw}
                  onCommitMove={commitMove}
                  onCommitResize={commitResize}
                  onReorder={reorderSelected}
                  onDuplicate={duplicateSelected}
                  onSetColor={setObjectColor}
                  onDelete={deleteSelected}
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
                layerRank={layerRank}
                visible={visible}
                selectedUuids={selectedUuids}
                isObjectEditable={isEditableObject}
                isObjectEditableNow={isObjectEditableNow}
                onSelect={setSelectedUuids}
                onFocusLayer={focusLayerOnly}
                onCommitDraw={commitDraw}
                onCommitMove={commitMove}
                onCommitResize={commitResize}
                onReorder={reorderSelected}
                onDuplicate={duplicateSelected}
                onSetColor={setObjectColor}
                onDelete={deleteSelected}
              />
            </div>
          )}
          </div>
        </div>

        {sidebarOpen && (
          <div className="viewer-sidebar">
            {/* Layers panel ABOVE the annotation list so its position stays
                stable; only the variable-length annotation list (below) grows
                or shrinks as the layer/selection changes. */}
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
