/**
 * usePdfDocument (T15 part 2 of the Viewer split) — everything about turning the
 * selected file into pixels: PDF.js load, per-page rasterization at scale×DPR, the
 * zoom model (fit-width/fit-page/explicit %, −/+ stepping, Ctrl/⌘-wheel zoom-to-
 * cursor), and the dry annotation overlay (@troubastack/ink, I8). Extracted VERBATIM
 * from Viewer.tsx; behavior + the load-bearing invariants are unchanged.
 *
 * INVARIANTS preserved here:
 *  - NO re-raster on annotation edit: the render effect depends ONLY on
 *    [selectedFile, status, scale, numPages, zoomMode, renderNonce] — never on
 *    objects/visibility — so editing repaints overlays (renderOverlays) without
 *    re-rasterizing pages (the hidden pdf-render-count probe asserts this).
 *  - Render-timing flip fix: the per-run RenderTask cancel-guard + one-shot settle
 *    redraw (renderNonce/nudgedFileRef) are intact.
 *  - Wheel-zoom decouples visual zoom (live CSS transform) from rasterization
 *    (one crisp re-raster on settle) — T27 stage 1.
 *  - The dry overlay is the ONLY committed-object render path (renderObjects).
 */
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import * as pdfjs from "pdfjs-dist";
// Vite resolves ?url to the emitted asset path; PDF.js needs its worker URL.
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { renderObjects, type InkObject } from "@troubastack/ink";
import { api, type AnnotationLayer, type AnnotationObject, type SongFile } from "../../api";
import { toInkObject, compareObjectZ, rasterDpr, budgetedRasterDpr, type LayerVisibility } from "./helpers";

pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

// Discrete percentage stops the −/+ buttons step through.
export const ZOOM_PERCENTS = [50, 75, 100, 125, 150, 200, 300];

// Continuous Ctrl/⌘-wheel zoom bounds (T27 stage 1). Wider than the discrete stops
// so a pinch can go slightly beyond the button range.
const MIN_ZOOM_SCALE = 0.25; // 25%
const MAX_ZOOM_SCALE = 5.0; // 500%
// Wheel→zoom sensitivity: scale multiplies by exp(-deltaY·k).
const WHEEL_ZOOM_K = 0.0015;
// Idle gap after the last wheel tick before we commit the crisp re-raster.
const WHEEL_SETTLE_MS = 120;

// Fit-width is capped at this scale so on a wide screen the score defaults to a
// comfortable reading size (~230%, VLL-validated) — centered with margins — instead
// of filling the whole column and rendering huge. Narrower columns fit below the cap
// as before (T27 stage 3).
const MAX_FIT_SCALE = 2.3;

/** A zoom selection is either a fit mode or an explicit percentage. */
export type ZoomMode = "fit-width" | "fit-page" | number;
export type ViewerStatus = "loading" | "no-file" | "ready" | "error";

export function usePdfDocument(args: {
  selectedFile: SongFile | null;
  isPdf: boolean;
  isImage: boolean;
  status: ViewerStatus;
  setStatus: (s: ViewerStatus) => void;
  setError: (e: string | null) => void;
  sidebarOpen: boolean;
  // Dry-overlay inputs (annotation data lives in useSongSync / Viewer):
  objects: AnnotationObject[];
  layersById: Map<string, AnnotationLayer>;
  visible: LayerVisibility;
  layerRank: Map<string, number>;
}) {
  const { selectedFile, isPdf, isImage, status, setStatus, setError, sidebarOpen } = args;
  const { objects, layersById, visible, layerRank } = args;

  // Loaded PDF.js document. Page/overlay canvases are indexed by page number.
  const pdfDocRef = useRef<pdfjs.PDFDocumentProxy | null>(null);
  const pageCanvasRefs = useRef<(HTMLCanvasElement | null)[]>([]);
  const overlayRefs = useRef<(HTMLCanvasElement | null)[]>([]);
  // Canvases already wired with contextlost/restored recovery listeners (T44), so we
  // attach exactly once per canvas element across re-renders.
  const wiredCanvasesRef = useRef<WeakSet<HTMLCanvasElement>>(new WeakSet());
  // The budgeted raster DPR the last PDF render used (T44). Overlays repaint on their own
  // trigger, but their canvas is sized to the raster — so their transform MUST use the
  // same value or the ink misaligns at high zoom. Seeded to the unclamped clamp.
  const effectiveDprRef = useRef<number>(rasterDpr());
  // Intrinsic (scale-1) page sizes in CSS px, used to compute fit scales.
  const pageSizesRef = useRef<{ w: number; h: number }[]>([]);
  // The scroll column we measure for Fit width / Fit page.
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // The pages wrapper — the CSS-transform target for live wheel-zoom (T27 s1).
  const contentRef = useRef<HTMLDivElement | null>(null);
  // Image element box (for image-overlay sizing).
  const imgRef = useRef<HTMLImageElement | null>(null);
  const imgOverlayRef = useRef<HTMLCanvasElement | null>(null);
  // How many times the PDF pages have actually been RASTERIZED. The render effect
  // bumps this; an annotation edit must NOT. Surfaced via a hidden data-testid.
  const pdfRenderCountRef = useRef(0);
  const [pdfRenderCount, setPdfRenderCount] = useState(0);
  // One-shot "settle" redraw (flip fix): after a file's FIRST successful render pass
  // we schedule exactly one more clean render on the next frame.
  const [renderNonce, setRenderNonce] = useState(0);
  const nudgedFileRef = useRef<string | null>(null);
  const nudgeRafRef = useRef<number | null>(null);
  // Measured available width of the PDF column's content box (CSS px). Drives Fit
  // width / Fit page. Updated by a ResizeObserver.
  const [columnWidth, setColumnWidth] = useState(0);
  // Zoom: a fit mode (default Fit width) or an explicit percentage.
  const [zoomMode, setZoomMode] = useState<ZoomMode>("fit-width");
  const [numPages, setNumPages] = useState(0);

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
        const res = await fetch(api.fileUrl(selectedFile.id, selectedFile.revision), {
          credentials: "include",
        });
        if (!res.ok) {
          // T69: a 404 means the rendered blob is gone. Generated charts self-heal server-side
          // (re-rendered from source), so a 404 that reaches here is an UPLOADED file whose
          // bytes are genuinely lost — show a clear, actionable state, not a raw status code.
          // The file strip stays put (rendered outside this pane), so other files still open.
          if (res.status === 404) {
            throw new Error("This file's data is missing — re-upload it to restore it.");
          }
          throw new Error(`Failed to fetch PDF (${res.status})`);
        }
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
  }, [selectedFile, setStatus, setError]);

  // Tear down the PDF doc on unmount.
  useEffect(() => {
    return () => {
      const d = pdfDocRef.current;
      pdfDocRef.current = null;
      if (d) void d.destroy();
    };
  }, []);

  // ---- measure the scroll column (Fit width / Fit page, resize-aware) ----
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const measure = () => {
      const cs = getComputedStyle(el);
      const padX = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight);
      const w = Math.max(0, el.clientWidth - padX);
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
  const pageScale = useCallback(
    (pageIndex: number): number => {
      if (typeof zoomMode === "number") return zoomMode / 100;
      const sz = pageSizesRef.current[pageIndex];
      if (!sz || columnWidth <= 0) return 0; // not measured yet → wait
      const byW = Math.min(columnWidth / sz.w, MAX_FIT_SCALE);
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

  // A representative scale (page 0) for the zoom readout / − + stepping and gating.
  const scale = useMemo(
    () => pageScale(0),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [pageScale, numPages],
  );

  // ---- dry overlay: object coords [0,1] → page box (I8) ----
  const paintOverlay = useCallback(
    (overlay: HTMLCanvasElement, page: number, dpr: number) => {
      const ctx = overlay.getContext("2d");
      if (!ctx) return;

      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, overlay.width / dpr, overlay.height / dpr);
      const objs = objects
        .filter((o) => o.page === page)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        })
        // Layer z-rank, then per-object order, then insertion (stable) — same
        // comparator as the hit-test so paint order matches pick order (T27).
        .sort((a, b) => compareObjectZ(a, b, layerRank));
      const pageRect = { x: 0, y: 0, w: overlay.width / dpr, h: overlay.height / dpr };
      renderObjects(ctx, objs.map(toInkObject) as InkObject[], pageRect);
    },
    [objects, layersById, visible, layerRank],
  );

  // Latest paintOverlay, referenced by the PDF rasterization effect WITHOUT making
  // that effect depend on it (the crux of the no-flicker fix).
  const paintOverlayRef = useRef(paintOverlay);
  useEffect(() => {
    paintOverlayRef.current = paintOverlay;
  }, [paintOverlay]);

  // Repaint every mounted overlay (visibility/objects change; no re-raster).
  const renderOverlays = useCallback(() => {
    const dpr = effectiveDprRef.current; // T44: match the raster's budgeted DPR
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
  useEffect(() => {
    const pdfDoc = pdfDocRef.current;
    if (status !== "ready" || !pdfDoc || !isPdf) return;
    if (typeof zoomMode !== "number" && scale <= 0) return;

    let cancelled = false;
    const tasks: pdfjs.RenderTask[] = [];

    (async () => {
      // T44: derive the raster DPR from the WHOLE document's canvas footprint at the
      // current zoom — cap the summed backing store + per-side dimension so a phone
      // never paints a page black (see budgetedRasterDpr). Uses CSS display sizes
      // (pageScale × intrinsic), which encode the current zoom; display size itself is
      // unchanged (zero-shift intact).
      const cssSizes: { w: number; h: number }[] = [];
      for (let i = 0; i < pdfDoc.numPages; i++) {
        const sz = pageSizesRef.current[i];
        const s = pageScale(i);
        if (sz && s > 0) cssSizes.push({ w: sz.w * s, h: sz.h * s });
      }
      const dpr = budgetedRasterDpr(rasterDpr(), cssSizes);
      effectiveDprRef.current = dpr; // so overlay repaints match this raster (T44)
      let renderedAny = false;
      for (let i = 0; i < pdfDoc.numPages; i++) {
        if (cancelled) return;
        const s = pageScale(i);
        if (s <= 0) continue; // page size not cached yet; skip until measured
        const page = await pdfDoc.getPage(i + 1);
        if (cancelled) return;

        const canvas = pageCanvasRefs.current[i];
        if (!canvas) continue;
        // T44 recover: if the browser evicts this canvas's backing store under memory
        // pressure (contextlost), it stays black until a re-raster — so on restore, bump
        // renderNonce to re-render. preventDefault marks the 2D context restorable.
        // Attached once per canvas (deduped).
        if (!wiredCanvasesRef.current.has(canvas)) {
          wiredCanvasesRef.current.add(canvas);
          canvas.addEventListener("contextlost", (e) => e.preventDefault());
          canvas.addEventListener("contextrestored", () => setRenderNonce((n) => n + 1));
        }
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
          if ((err as { name?: string })?.name === "RenderingCancelledException") return;
          throw err;
        }
        if (cancelled) return;
        pdfRenderCountRef.current += 1;
        setPdfRenderCount(pdfRenderCountRef.current);
        renderedAny = true;

        const overlay = overlayRefs.current[i];
        if (overlay) {
          overlay.width = canvas.width;
          overlay.height = canvas.height;
          overlay.style.width = `${canvas.style.width}`;
          overlay.style.height = `${canvas.style.height}`;
          paintOverlayRef.current(overlay, i, dpr);
        }
      }

      if (!cancelled && renderedAny && nudgedFileRef.current !== (selectedFile?.id ?? null)) {
        nudgedFileRef.current = selectedFile?.id ?? null;
        nudgeRafRef.current = requestAnimationFrame(() => setRenderNonce((n) => n + 1));
      }
    })();

    return () => {
      cancelled = true;
      for (const t of tasks) t.cancel();
      if (nudgeRafRef.current != null) cancelAnimationFrame(nudgeRafRef.current);
    };
    // Depends ONLY on what changes the rasterized pixels — NOT objects/visibility.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedFile, status, scale, numPages, zoomMode, renderNonce]);

  // ---- size + render the image overlay (image mode) ----
  const layoutImageOverlay = useCallback(() => {
    if (!isImage) return;
    const img = imgRef.current;
    const overlay = imgOverlayRef.current;
    if (!img || !overlay) return;
    const w = img.clientWidth;
    const h = img.clientHeight;
    // T44: budget the image overlay from its own box too (same GPU side-cap hole as the
    // wet canvas — a zoomed image overlay could exceed 4096 and black out). effectiveDprRef
    // so renderOverlays paints at the matching DPR.
    const dpr = budgetedRasterDpr(rasterDpr(), [{ w, h }]);
    effectiveDprRef.current = dpr;
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

  // ---- zoom controls ----
  const currentPercent = useCallback((): number => {
    if (typeof zoomMode === "number") return zoomMode;
    return Math.round(scale * 100);
  }, [zoomMode, scale]);
  const stepZoom = useCallback(
    (dir: -1 | 1) => {
      const cur = currentPercent();
      if (dir === 1) {
        const next = ZOOM_PERCENTS.find((p) => p > cur);
        setZoomMode(next ?? ZOOM_PERCENTS[ZOOM_PERCENTS.length - 1]);
      } else {
        const below = [...ZOOM_PERCENTS].reverse().find((p) => p < cur);
        setZoomMode(below ?? ZOOM_PERCENTS[0]);
      }
    },
    [currentPercent],
  );
  const onZoomSelect = useCallback((value: string) => {
    if (value === "fit-width" || value === "fit-page") setZoomMode(value);
    else setZoomMode(Number(value));
  }, []);
  const zoomSelectValue = typeof zoomMode === "number" ? String(zoomMode) : zoomMode;
  const customZoomPercent =
    typeof zoomMode === "number" && !ZOOM_PERCENTS.includes(zoomMode) ? zoomMode : null;

  // ---- Zoom burst pipeline (T27 stage 1 Ctrl/⌘-wheel; reused by stage-4 pinch) ----
  // A "burst" is a live zoom/pan gesture: the pages scale cheaply via a CSS transform
  // on the content wrapper (anchored at vx/vy in the scroll viewport, content-fraction
  // fx/fy), plus a live pan translate (panDx/panDy — 0 for wheel, non-zero for a
  // two-finger drag). On settle we commit ONE crisp re-raster and reconcile scroll.
  const wheelBurstRef = useRef<{
    base: number;
    target: number;
    vx: number;
    vy: number;
    fx: number;
    fy: number;
    panDx: number;
    panDy: number;
  } | null>(null);
  const wheelSettleRef = useRef<number | null>(null);

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
      // The anchor content-point (fx,fy) should end under the gesture's CURRENT
      // viewport position — the initial anchor (vx,vy) plus any two-finger pan.
      scroll.scrollLeft = Math.max(0, burst.fx * contentW - burst.vx - burst.panDx);
      scroll.scrollTop = Math.max(0, burst.fy * contentH - burst.vy - burst.panDy);
    }
    if (!(typeof zoomMode === "number" && zoomMode === targetPct)) {
      setZoomMode(targetPct);
    }
  }, [zoomMode]);

  const wheelZoomHandler = useCallback(
    (e: WheelEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (!isPdf) return;
      const scroll = scrollRef.current;
      const content = contentRef.current;
      if (!scroll || !content) return;
      e.preventDefault();

      let dy = e.deltaY;
      if (e.deltaMode === 1) dy *= 16;
      else if (e.deltaMode === 2) dy *= 100;

      let burst = wheelBurstRef.current;
      if (!burst) {
        const cs = getComputedStyle(scroll);
        const padL = parseFloat(cs.paddingLeft) || 0;
        const padR = parseFloat(cs.paddingRight) || 0;
        const padT = parseFloat(cs.paddingTop) || 0;
        const padB = parseFloat(cs.paddingBottom) || 0;
        const bL = parseFloat(cs.borderLeftWidth) || 0;
        const bT = parseFloat(cs.borderTopWidth) || 0;
        const rect = scroll.getBoundingClientRect();
        const vx = e.clientX - rect.left - bL - padL;
        const vy = e.clientY - rect.top - bT - padT;
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
          panDx: 0,
          panDy: 0,
        };
        wheelBurstRef.current = burst;
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

  // ---- Two-finger pinch/pan gesture API (T27 stage 4) ----
  // Drives the SAME burst pipeline as the wheel zoom above: a live CSS transform on
  // the content wrapper during the gesture, then ONE crisp re-raster committed on
  // gesture end. WetCanvas detects the two-pointer gesture and calls these.
  const beginGesture = useCallback(
    (clientX: number, clientY: number): boolean => {
      if (!isPdf) return false;
      const scroll = scrollRef.current;
      const content = contentRef.current;
      if (!scroll || !content) return false;
      const cs = getComputedStyle(scroll);
      const padL = parseFloat(cs.paddingLeft) || 0;
      const padR = parseFloat(cs.paddingRight) || 0;
      const padT = parseFloat(cs.paddingTop) || 0;
      const padB = parseFloat(cs.paddingBottom) || 0;
      const bL = parseFloat(cs.borderLeftWidth) || 0;
      const bT = parseFloat(cs.borderTopWidth) || 0;
      const rect = scroll.getBoundingClientRect();
      const vx = clientX - rect.left - bL - padL;
      const vy = clientY - rect.top - bT - padT;
      const contentW = Math.max(1, scroll.scrollWidth - padL - padR);
      const contentH = Math.max(1, scroll.scrollHeight - padT - padB);
      const base = scale > 0 ? scale : 1;
      wheelBurstRef.current = {
        base,
        target: base,
        vx,
        vy,
        fx: (scroll.scrollLeft + vx) / contentW,
        fy: (scroll.scrollTop + vy) / contentH,
        panDx: 0,
        panDy: 0,
      };
      const wr = content.getBoundingClientRect();
      content.style.transformOrigin = `${clientX - wr.left}px ${clientY - wr.top}px`;
      content.style.willChange = "transform";
      if (wheelSettleRef.current != null) {
        window.clearTimeout(wheelSettleRef.current);
        wheelSettleRef.current = null;
      }
      return true;
    },
    [isPdf, scale],
  );

  // scaleFactor is relative to the gesture START (pinch distance ratio); panDx/panDy
  // is the midpoint drag in CSS px since the start.
  const updateGesture = useCallback((scaleFactor: number, panDx: number, panDy: number) => {
    const burst = wheelBurstRef.current;
    const content = contentRef.current;
    if (!burst || !content) return;
    burst.target = Math.min(MAX_ZOOM_SCALE, Math.max(MIN_ZOOM_SCALE, burst.base * scaleFactor));
    burst.panDx = panDx;
    burst.panDy = panDy;
    content.style.transform = `translate(${panDx}px, ${panDy}px) scale(${burst.target / burst.base})`;
  }, []);

  // Commit ONE crisp raster on gesture end (fingers up) — the stage-1 settle path.
  const endGesture = useCallback(() => {
    commitWheelZoom();
  }, [commitWheelZoom]);

  // ---- Double-tap / double-click zoom-to-point (T66 Part D) ----
  // Deliberately NOT the pinch/burst pipeline. A double-tap is instantaneous, so there's no
  // live gesture to animate; driving begin→update→commit in a single tick raced the CSS
  // transform against layout on-device and reconciled to a RE-CENTRE, not a zoom-to-point
  // (VLL's report). Instead reuse the reliable setZoomMode zoom (the +/- stepper path) and
  // anchor scroll explicitly: size the page canvases to `targetScale` synchronously so the
  // content size is measurable NOW, place scroll so the tapped content-point (fx,fy) stays
  // under the cursor, then setZoomMode(mode) commits the crisp re-raster at that same size.
  // `mode` must render at `targetScale` — "fit-width" only when targetScale IS the fit scale,
  // else the matching numeric percent.
  const anchorZoom = useCallback(
    (clientX: number, clientY: number, targetScale: number, mode: ZoomMode): boolean => {
      if (!isPdf) return false;
      const scroll = scrollRef.current;
      if (!scroll) return false;
      const cs = getComputedStyle(scroll);
      const padL = parseFloat(cs.paddingLeft) || 0;
      const padR = parseFloat(cs.paddingRight) || 0;
      const padT = parseFloat(cs.paddingTop) || 0;
      const padB = parseFloat(cs.paddingBottom) || 0;
      const bL = parseFloat(cs.borderLeftWidth) || 0;
      const bT = parseFloat(cs.borderTopWidth) || 0;
      const rect = scroll.getBoundingClientRect();
      const vx = clientX - rect.left - bL - padL;
      const vy = clientY - rect.top - bT - padT;
      const contentW0 = Math.max(1, scroll.scrollWidth - padL - padR);
      const contentH0 = Math.max(1, scroll.scrollHeight - padT - padB);
      const fx = (scroll.scrollLeft + vx) / contentW0; // content-fraction under the tap
      const fy = (scroll.scrollTop + vy) / contentH0;
      const s = Math.min(MAX_ZOOM_SCALE, Math.max(MIN_ZOOM_SCALE, targetScale));
      // Resize the canvases to the target NOW so scrollWidth/Height reflect the new layout
      // when we anchor (the setZoomMode re-raster below repaints them crisply at this size).
      for (let i = 0; i < pageCanvasRefs.current.length; i++) {
        const sz = pageSizesRef.current[i];
        const canvas = pageCanvasRefs.current[i];
        if (!sz || !canvas) continue;
        canvas.style.width = `${sz.w * s}px`;
        canvas.style.height = `${sz.h * s}px`;
      }
      const contentW = Math.max(1, scroll.scrollWidth - padL - padR);
      const contentH = Math.max(1, scroll.scrollHeight - padT - padB);
      scroll.scrollLeft = Math.max(0, fx * contentW - vx); // keep (fx,fy) under the cursor
      scroll.scrollTop = Math.max(0, fy * contentH - vy);
      setZoomMode(mode);
      return true;
    },
    [isPdf],
  );

  // The double-tap zoom TOGGLE — decided STATELESSLY from the actual current zoom, never a
  // private boolean (a boolean desynced from reality: reach fit via the stepper/pinch while
  // the flag still read "zoomed" → double-tap re-applied fit while already there → a bare
  // re-centre, VLL's report). Rule: at/near fit (or zoomed out) → zoom IN to ~2× fit; clearly
  // zoomed in → back to fit. BOTH directions anchor the tapped content-point (RULED Option A,
  // 2026-07-27) — symmetric, the universal maps/PDF idiom, and a same-spot double-tap is an
  // exact inverse (zoom-out restores the pre-zoom fit view, fixing VLL's "fit lands too low").
  const toggleZoomAtPoint = useCallback(
    (clientX: number, clientY: number): boolean => {
      const sz = pageSizesRef.current[0];
      if (!sz || columnWidth <= 0) return false;
      const fit = Math.min(columnWidth / sz.w, MAX_FIT_SCALE);
      const cur = scale > 0 ? scale : fit;
      if (cur > fit * 1.25) {
        // zoomed in → back to fit, ANCHORED at the tapped point (stays fit-width mode).
        return anchorZoom(clientX, clientY, fit, "fit-width");
      }
      const target = Math.min(MAX_ZOOM_SCALE, Math.max(MIN_ZOOM_SCALE, 2 * fit));
      return anchorZoom(clientX, clientY, target, Math.round(target * 100)); // → 2× fit at point
    },
    [scale, columnWidth, anchorZoom],
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

  return {
    scrollRef,
    contentRef,
    pageCanvasRefs,
    overlayRefs,
    imgRef,
    imgOverlayRef,
    numPages,
    pdfRenderCount,
    zoomSelectValue,
    customZoomPercent,
    stepZoom,
    onZoomSelect,
    renderOverlays,
    layoutImageOverlay,
    // Two-finger pinch/pan gesture API (T27 stage 4) — WetCanvas drives these.
    beginGesture,
    updateGesture,
    endGesture,
    // Double-tap / double-click zoom toggle, decided from the actual zoom (T66 Part D).
    toggleZoomAtPoint,
  };
}
