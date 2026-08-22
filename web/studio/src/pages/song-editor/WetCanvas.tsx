/**
 * Wet edit canvas (T10 extraction — moved verbatim from SongEditor.tsx): the
 * pointer gesture state machine (draw/move/resize/multi-move/marquee), the T06
 * low-latency freehand path, and the DOM selection/handle overlays. Behavior +
 * data-testids unchanged. Committed rendering still goes through @troubastack/ink.
 */
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { AnnotationLayer, AnnotationObject, AnnotationStyle } from "../../api";
import { renderObjects, type InkObject } from "@troubastack/ink";
import {
  CORNER_HANDLES,
  cursorForPick,
  handlesVisible,
  hitsMultiSelection,
  intersectsRect,
  isMarquee,
  normalizeRect,
  objectBBox,
  pickAt,
  isNonDraw,
  pointerToPageXY,
  resizeObject,
  translateObject,
  type DrawTool,
  type HandleId,
  type PickContext,
  type SelectRect,
  type TextMeasure,
  type Tool,
} from "../../editor";
import { buildWet, budgetedRasterDpr, compareObjectZ, measureTextWidth, toInkObject, rasterDpr, type PRPoint, type LayerVisibility } from "./helpers";
import { SelectionToolbar } from "./Toolbar";

/** Capture a pointer id, best-effort (T34). Exotic/synthetic pointer ids (e.g. an e2e-
 *  dispatched PointerEvent) can't be captured and throw NotFoundError; capture is a
 *  reliability boost (nav-finger ups reach the canvas even lifting over chrome), never
 *  load-bearing, so a failure is non-fatal. */
function capturePointer(el: Element, id: number) {
  try {
    el.setPointerCapture(id);
  } catch {
    /* exotic/synthetic pointer — non-fatal */
  }
}

// T66 Part D: a Move-mode press within this many CSS px of its start is a TAP, not a pan —
// used both to suppress drift on tap and to pair taps into a double-tap. A real double-tap
// on a phone lands within DOUBLE_TAP_MS of the first with the finger near the same spot.
const TAP_SLOP = 8;
const DOUBLE_TAP_MS = 400;
const DOUBLE_TAP_DIST = 30;

export function EditCanvas({
  page,
  tool,
  style,
  iconGlyph,
  drawLocked,
  objects,
  layersById,
  layerRank,
  visible,
  selectedUuids,
  isObjectEditable,
  isObjectEditableNow,
  onSelect,
  onFocusLayer,
  onCommitDraw,
  onTextResolved,
  onCommitMove,
  onCommitResize,
  onReorder,
  onDuplicate,
  onSetColor,
  onDelete,
  beginGesture,
  onDoubleTapZoom,
  updateGesture,
  endGesture,
}: {
  page: number;
  tool: Tool;
  style: AnnotationStyle;
  // T51: the glyph id the Icon tool will stamp — carried into the WET preview so the
  // chosen glyph shows mid-drag (not the `note` fallback).
  iconGlyph: string;
  drawLocked: boolean;
  objects: AnnotationObject[];
  layersById: Map<string, AnnotationLayer>;
  // layerId → z-rank, for the shared z-order comparator (T27). Ensures the pick
  // order matches the dry overlay's paint order.
  layerRank: Map<string, number>;
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
  // T90 — called after the text prompt resolves (placed OR cancelled), so the caller can disarm the
  // one-shot text tool. Optional: other draw tools don't use it.
  onTextResolved?: () => void;
  onCommitMove: (obj: AnnotationObject) => void;
  onCommitResize: (obj: AnnotationObject) => void;
  // Selection-toolbar actions (T27 stage 2) — all act on the single active-editable
  // selection and are gated in the Viewer (owner/RW/active layer).
  onReorder: (uuid: string, dir: "front" | "back") => void;
  onDuplicate: (uuid: string) => void;
  onSetColor: (uuid: string, color: string) => void;
  onDelete: () => void;
  // Two-finger pinch/pan pipeline (T27 stage 4) — from usePdfDocument. beginGesture
  // returns false when there's nothing to zoom (non-PDF / not ready).
  beginGesture: (clientX: number, clientY: number) => boolean;
  updateGesture: (scaleFactor: number, panDx: number, panDy: number) => void;
  endGesture: () => void;
  // T66 Part D: double-tap / double-click in Move mode → zoom to the point (Fit-width ↔ 2×).
  onDoubleTapZoom?: (clientX: number, clientY: number) => void;
}) {
  // ---- Multi-touch navigation (T27 stage 4) ------------------------------
  // Active pointers by id (client coords), so we can detect a second finger and
  // drive a two-finger pinch/pan. `navRef` holds the live two-finger gesture.
  const pointersRef = useRef<Map<number, { x: number; y: number }>>(new Map());
  const navRef = useRef<{
    idA: number;
    idB: number;
    startDist: number;
    startMidX: number;
    startMidY: number;
  } | null>(null);
  // A ONE-finger pan (Select mode, empty space, touch) — reuses the gesture pipeline
  // with scaleFactor 1 (pan, no zoom). Kept separate from the two-finger navRef.
  const panRef = useRef<{ id: number; startX: number; startY: number } | null>(null);
  // T66 Part D: the last touch tap (Move mode) for double-tap-to-zoom detection.
  const lastTapRef = useRef<{ t: number; x: number; y: number } | null>(null);
  // Palm-rejection idiom (#4): once a PEN is ever used, treat a finger as navigation
  // even with a draw tool (pen draws, finger pans). A pen-less device (phone) keeps
  // the one-finger-draws default (#2).
  const penSeenRef = useRef(false);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  // The active gesture: a draw path, a move drag, a resize-handle drag, a
  // rubber-band marquee, or none. Kept in a ref so pointer handlers read the
  // latest without re-binding.
  const gestureRef = useRef<
    | { mode: "draw"; path: PRPoint[] }
    | { mode: "move"; obj: AnnotationObject; start: PRPoint; preview: AnnotationObject }
    | { mode: "resize"; obj: AnnotationObject; handle: HandleId; start: PRPoint; preview: AnnotationObject }
    // Group move of a multi-selection: each editable-now member carries its own
    // original + live preview; all translate by the same delta (Bug #4).
    | { mode: "multi-move"; start: PRPoint; items: { obj: AnnotationObject; preview: AnnotationObject }[] }
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

  // ---- T06 low-latency wet-ink path (freehand only) ----------------------
  // Cached 2D context requested with { desynchronized: true } for lower latency
  // (harmless where unsupported). getContext returns the same context on repeat
  // calls, so the attribute must be set on the FIRST call — hence the cache.
  const wetCtxRef = useRef<CanvasRenderingContext2D | null>(null);
  // Offscreen cache holding the freehand outline for path[0..cachedUpTo], so each
  // frame only blits the cache + re-strokes the short live tail (bounded work per
  // frame instead of getStroke over the whole, growing path).
  const wetCacheRef = useRef<HTMLCanvasElement | null>(null);
  // Compose surface for uniform-alpha wet compositing (T35): the cache + live tail are
  // built OPAQUE and composed here, then blitted to the wet canvas ONCE at the object's
  // opacity — so overlapping segment seams don't stack alpha into darker bands.
  const wetComposeRef = useRef<HTMLCanvasElement | null>(null);
  // The budgeted DPR the wet canvas was last sized at (T44) — repaint()/drawWetFrame()
  // read this so their transform matches the (possibly clamped) backing store.
  const wetDprRef = useRef<number>(rasterDpr());
  const cachedUpToRef = useRef(0);
  const rafRef = useRef<number | null>(null);
  // Dev perf hook (localStorage.inkPerf === "1"): points received + mean
  // pointer-event→paint latency, so the tablet spike has numbers.
  const perfRef = useRef<{
    points: number;
    frames: number;
    sumLatency: number;
    last: number;
    t0: number;
    renderFirst: number;
    renderLast: number;
    renderMax: number;
  } | null>(null);
  // Bake a stable segment into the cache once the live tail exceeds this; the
  // small overlap keeps the join between cached prefix and live tail continuous.
  const WET_SEG = 24;
  const WET_OVERLAP = 3;
  // T35 capture-time diet: drop freehand points closer than this (page-relative, ~0.15%
  // of page width) to the last KEPT point — slow strokes stop storing hundreds of
  // near-duplicates (payload, store size, cache bakes) with no visible change (streamline
  // covers it). Filtered at CAPTURE so wet/dry/bake/hit-test all see the SAME points.
  const WET_MIN_STEP = 0.0015;

  // Objects on THIS page that are currently visible (for hit-testing on select).
  const pageObjects = useMemo(
    () =>
      objects
        .filter((o) => o.page === page)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        })
        // Same z-order as the dry overlay paints (layer rank, then object order,
        // then insertion): pickAt wants ascending "topmost last" (T27).
        .sort((a, b) => compareObjectZ(a, b, layerRank)),
    [objects, page, layersById, layerRank, visible],
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
    const w = pageCanvas.clientWidth;
    const h = pageCanvas.clientHeight;
    if (w <= 0 || h <= 0) return;
    // T44: budget the wet canvas from ITS OWN box (it maps [0,1] to itself, so its DPR
    // needn't match the raster's). At high zoom the raw-DPR wet canvas is the largest,
    // TOPMOST layer on every page (~4752px at 300%×dpr2 > the 4096 GPU floor) — an
    // uncompositable top layer reads as a BLACK page. Cap it like the raster; store the
    // effective DPR so repaint()/drawWetFrame() use the SAME value (alignment).
    const dpr = budgetedRasterDpr(rasterDpr(), [{ w, h }]);
    wetDprRef.current = dpr;
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
  const getWetCtx = useCallback((): CanvasRenderingContext2D | null => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    if (wetCtxRef.current && wetCtxRef.current.canvas === canvas) return wetCtxRef.current;
    // T44: `desynchronized: true` puts this (topmost, per-page) canvas on Android's
    // hardware-overlay compositing path — a documented BLACK-LAYER failure on some GPUs
    // (content intact via getImageData / software screenshot, black on the physical
    // display; VLL's exact symptom). Gate it to FINE pointers (desktop mouse / pen keep
    // the low-latency win; touch phones/tablets skip the overlay path). T35's bbox blits
    // already bounded the wet render cost, so dropping desync on touch is cheap.
    const desync = window.matchMedia?.("(pointer: fine)")?.matches ?? true;
    const ctx = canvas.getContext("2d", { desynchronized: desync }) as CanvasRenderingContext2D | null;
    wetCtxRef.current = ctx;
    return ctx;
  }, []);

  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = getWetCtx();
    if (!ctx) return;
    const dpr = wetDprRef.current;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, canvas.width / dpr, canvas.height / dpr);
    const box = { x: 0, y: 0, w: canvas.width / dpr, h: canvas.height / dpr };

    const g = gestureRef.current;
    if (g?.mode === "draw" && g.path.length > 0 && !isNonDraw(tool)) {
      const wet = buildWet(tool as DrawTool, g.path, style, tool === "icon" ? iconGlyph : "");
      if (wet) renderObjects(ctx, [toInkObject(wet) as InkObject], box);
    } else if (g?.mode === "move" || g?.mode === "resize") {
      renderObjects(ctx, [toInkObject(g.preview) as InkObject], box);
    } else if (g?.mode === "multi-move") {
      renderObjects(ctx, g.items.map((it) => toInkObject(it.preview) as InkObject), box);
    }
  }, [tool, style, iconGlyph, getWetCtx]);

  // Incremental freehand wet render (T06). Blits the cached prefix outline and
  // re-strokes only the short live tail, so per-frame cost stays bounded as the
  // stroke grows. Segment joins are approximate; pointer-up does one authoritative
  // renderObjects pass (below) so the committed preview matches the dry render (I8).
  const drawWetFrame = useCallback(() => {
    rafRef.current = null;
    const g = gestureRef.current;
    const canvas = canvasRef.current;
    const ctx = getWetCtx();
    if (!canvas || !ctx || g?.mode !== "draw") return;
    const dpr = wetDprRef.current;
    const cssW = canvas.width / dpr;
    const cssH = canvas.height / dpr;
    const box = { x: 0, y: 0, w: cssW, h: cssH };
    const path = g.path;

    let cache = wetCacheRef.current;
    if (!cache) {
      cache = document.createElement("canvas");
      wetCacheRef.current = cache;
    }
    if (cache.width !== canvas.width || cache.height !== canvas.height) {
      cache.width = canvas.width;
      cache.height = canvas.height;
      cachedUpToRef.current = 0;
    }

    // T35: build the wet geometry OPAQUE (strip opacity, keep color/width/blend) so no
    // per-pass alpha stacks; the single alpha application happens at the blit below.
    const opaqueStyle = { ...style, opacity: 1 };

    // Bake the newly-stable segment into the cache when the tail grows past WET_SEG.
    if (path.length - cachedUpToRef.current > WET_SEG) {
      const cctx = cache.getContext("2d");
      if (cctx) {
        cctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        const seg = path.slice(Math.max(0, cachedUpToRef.current - WET_OVERLAP));
        const wet = buildWet("freehand", seg, opaqueStyle);
        if (wet) renderObjects(cctx, [toInkObject(wet) as InkObject], box);
        cachedUpToRef.current = path.length;
      }
    }

    // Per-frame (T35 uniform-alpha): compose the OPAQUE cached prefix + OPAQUE live tail
    // on a compose surface, then blit the whole stroke to the wet canvas ONCE at the
    // object's opacity — one alpha application, so overlapping seams read uniform.
    const renderStart = performance.now();
    let compose = wetComposeRef.current;
    if (!compose) {
      compose = document.createElement("canvas");
      wetComposeRef.current = compose;
    }
    if (compose.width !== canvas.width || compose.height !== canvas.height) {
      compose.width = canvas.width;
      compose.height = canvas.height;
    }
    const octx = compose.getContext("2d");
    if (!octx) return;

    // T35 (b): restrict the clear + both full-canvas blits to the stroke's padded
    // device-px bounding box, so per-frame cost scales with STROKE size, not canvas
    // size. The T06 low-latency invariant matters most on tablets (~3–4× slower than
    // this desktop, where a full-canvas clear+blit can blow the 16.6ms budget); a
    // typical annotation stroke is small, and a page-wide stroke degrades gracefully to
    // the full-canvas cost. The path only grows, so the bbox is monotonic — clearing the
    // current (largest-so-far) box clears every earlier frame's pixels on both surfaces.
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const p of path) {
      if (p.x < minX) minX = p.x;
      if (p.x > maxX) maxX = p.x;
      if (p.y < minY) minY = p.y;
      if (p.y > maxY) maxY = p.y;
    }
    const cw = canvas.width;
    const ch = canvas.height;
    // Pen width is uniform and keyed to page width (ink I3: style.width is a page-width
    // fraction); pad by a full width + a small AA margin so freehand caps/joins never clip.
    const pad = (style.width ?? 0) * cw + 4;
    const bx = Math.max(0, Math.floor(minX * cw - pad));
    const by = Math.max(0, Math.floor(minY * ch - pad));
    const bw = Math.min(cw, Math.ceil(maxX * cw + pad)) - bx;
    const bh = Math.min(ch, Math.ceil(maxY * ch + pad)) - by;
    if (bw <= 0 || bh <= 0) return; // nothing on-canvas to paint this frame

    octx.setTransform(1, 0, 0, 1, 0, 0);
    octx.clearRect(bx, by, bw, bh);
    if (cachedUpToRef.current > 0) octx.drawImage(cache, bx, by, bw, bh, bx, by, bw, bh); // opaque prefix
    const tail = path.slice(Math.max(0, cachedUpToRef.current - WET_OVERLAP));
    octx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const wetTail = buildWet("freehand", tail, opaqueStyle);
    if (wetTail) renderObjects(octx, [toInkObject(wetTail) as InkObject], box); // opaque tail

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(bx, by, bw, bh);
    const prevAlpha = ctx.globalAlpha;
    ctx.globalAlpha = style.opacity ?? 1;
    ctx.drawImage(compose, bx, by, bw, bh, bx, by, bw, bh);
    ctx.globalAlpha = prevAlpha;

    const perf = perfRef.current;
    if (perf) {
      const renderMs = performance.now() - renderStart;
      perf.sumLatency += performance.now() - perf.last;
      if (perf.frames === 0) perf.renderFirst = renderMs;
      perf.renderLast = renderMs;
      if (renderMs > perf.renderMax) perf.renderMax = renderMs;
      perf.frames += 1;
    }
  }, [style, getWetCtx]);

  const scheduleWetFrame = useCallback(() => {
    if (rafRef.current == null) rafRef.current = requestAnimationFrame(drawWetFrame);
  }, [drawWetFrame]);

  // Cancel any pending frame + drop the cached prefix (start of a fresh stroke,
  // or teardown). The wet canvas itself is cleared by repaint().
  const resetWetCache = useCallback(() => {
    if (rafRef.current != null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    cachedUpToRef.current = 0;
    // T44: release the full-size wet scratch buffers between strokes. They were retained
    // for the stroke's life (bake cache + compose surface) and lingered full-viewport
    // afterward, adding to the canvas-memory pressure that blacks out pages on mobile.
    // Both are lazily recreated on the next stroke (drawWetFrame), so dropping is safe.
    wetCacheRef.current = null;
    wetComposeRef.current = null;
  }, []);

  // Cancel a pending frame on unmount.
  useEffect(() => () => resetWetCache(), [resetWetCache]);

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

  // All samples in this move, not just the latest — fast styluses batch several
  // positions per event (T06). Falls back to the single event where unsupported.
  function coalescedPoints(e: React.PointerEvent): PRPoint[] {
    const canvas = canvasRef.current!;
    const ne = e.nativeEvent as PointerEvent;
    const evs = typeof ne.getCoalescedEvents === "function" ? ne.getCoalescedEvents() : null;
    if (evs && evs.length > 0) return evs.map((ev) => pointerToPageXY(ev.clientX, ev.clientY, canvas));
    return [pageRelative(e)];
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

  // Assemble the PickContext used by BOTH the pointer-down gesture and the hover
  // cursor, so the cursor always predicts what the next drag will do. pageObjects
  // is in z-order (earliest first → last is topmost), exactly what pickAt wants.
  function buildPickContext(): PickContext | null {
    const dims = pageDims();
    if (!dims) return null;
    return {
      objects: pageObjects,
      pageW: dims.w,
      pageH: dims.h,
      measure: textMeasure(),
      selected: selectedSingle,
      isEditableNow: isObjectEditableNow,
    };
  }

  // Set the EditCanvas cursor from a hover pick. Cheap: just the existing hit
  // math + an inline-style write, no re-render. For mode "none" (or any non-select
  // tool) we clear the inline cursor so the CSS `.tool-*` default (crosshair for a
  // draw tool, default for select) applies. Resize→directional, move→move,
  // select→pointer.
  function updateHoverCursor(e: React.PointerEvent) {
    const canvas = canvasRef.current;
    if (!canvas) return;
    if (tool === "move") {
      // T65: the Move tool shows a grab hand (grabbing is set while a pan is in flight).
      const want = panRef.current ? "grabbing" : "grab";
      if (canvas.style.cursor !== want) canvas.style.cursor = want;
      return;
    }
    if (tool !== "select") {
      // Draw tools keep their CSS crosshair; nothing to predict on hover.
      if (canvas.style.cursor) canvas.style.cursor = "";
      return;
    }
    const pt = pageRelative(e);
    // A multi-selection grab predicts a group move (Bug #4): hovering inside the
    // selection shows `move` when at least one member is editable-now.
    const dims = pageDims();
    if (selectedOnPage.length > 1 && dims) {
      const measure = textMeasure();
      if (
        hitsMultiSelection(pt, selectedOnPage, dims.w, dims.h, measure) &&
        selectedOnPage.some((o) => isObjectEditableNow(o))
      ) {
        if (canvas.style.cursor !== "move") canvas.style.cursor = "move";
        return;
      }
    }
    const ctx = buildPickContext();
    const pick = ctx ? pickAt(pt, ctx) : { object: null, mode: "none" as const };
    // mode "none" → "" sentinel from cursorForPick, which clears to the CSS default.
    const next = cursorForPick(pick, "");
    if (canvas.style.cursor !== next) canvas.style.cursor = next;
  }

  // Discard the in-progress wet gesture WITHOUT committing (2nd-finger-cancels-stroke,
  // T27 stage 4 #3 — never leave a half-stroke). Clears the wet canvas + marquee.
  function cancelWetGesture() {
    const canvas = canvasRef.current;
    if (gestureRef.current || marquee) {
      gestureRef.current = null;
      setMarquee(null);
      resetWetCache();
      repaint();
      forceHandles((n) => n + 1);
    }
    if (panRef.current && canvas?.hasPointerCapture(panRef.current.id)) {
      canvas.releasePointerCapture(panRef.current.id);
    }
    panRef.current = null;
  }

  function onPointerDown(e: React.PointerEvent) {
    const canvas = canvasRef.current;
    if (!canvas) return;

    // T34 self-heal: a PRIMARY pointer is by spec the only active pointer of its type,
    // so any lingering same-type entries in pointersRef are stale — a missed pointerup
    // or pointercancel (e.g. a nav finger that lifted over a chrome button, whose
    // pointer-events:auto swallowed the up, or off-window). Without this, one stale
    // entry makes every later single touch read as size>=2 → instant nav → no stroke
    // ever starts, and editing is dead until reload (the field bug). Clearing on the
    // primary down heals it; penSeenRef is intentionally left sticky (out of scope).
    if (e.isPrimary) {
      pointersRef.current.clear();
      // If a nav was still LIVE (both fingers' ups were missed — realistic when both
      // lift at the pill edges), it left a live CSS transform + a populated wheel burst
      // that only commitWheelZoom clears, so endGesture() must run to settle it to a
      // crisp raster. Without it the score stays CSS-zoomed/blurry until a later pinch
      // happens to commit (arch T34 pre-land condition).
      if (navRef.current) {
        navRef.current = null;
        endGesture();
      }
    }

    // ---- Multi-touch (T27 stage 4): track pointers; a SECOND pointer starts a
    //      two-finger navigation (pinch-zoom + pan), in every tool. ----
    pointersRef.current.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointersRef.current.size >= 2 && !navRef.current) {
      cancelWetGesture(); // abandon any one-finger stroke/pan; two fingers = navigate
      const ids = [...pointersRef.current.keys()].slice(-2);
      const a = pointersRef.current.get(ids[0])!;
      const b = pointersRef.current.get(ids[1])!;
      const midX = (a.x + b.x) / 2;
      const midY = (a.y + b.y) / 2;
      if (beginGesture(midX, midY)) {
        navRef.current = {
          idA: ids[0],
          idB: ids[1],
          startDist: Math.hypot(a.x - b.x, a.y - b.y) || 1,
          startMidX: midX,
          startMidY: midY,
        };
        // T34: capture BOTH nav pointers so their pointerup/pointercancel are delivered
        // to the canvas even when a finger lifts over a chrome button (pointer-events:
        // auto) or off-window — otherwise the missed up leaves a stale pointersRef entry
        // that jams every later single touch into nav.
        capturePointer(canvas, ids[0]);
        capturePointer(canvas, ids[1]);
      }
      return;
    }
    if (navRef.current) {
      // Already navigating — ignore extra touches, but capture them too so their ups
      // don't leak to the chrome and orphan an entry (T34).
      capturePointer(canvas, e.pointerId);
      return;
    }

    if (e.button !== 0) return;
    const pt = pageRelative(e);

    // A read-only (locked) focused layer blocks all drawing gestures; selecting
    // is still allowed so the user can browse/select the layer's annotations.
    if (drawLocked && !isNonDraw(tool)) return;

    if (e.pointerType === "pen") penSeenRef.current = true;

    // T65 Move tool: a single pointer (mouse OR one finger) pans the document through the
    // SAME gesture pipeline as two-finger pan — no pen-seen requirement, and it draws/selects
    // nothing (it's a NonDrawTool). Two-finger pan/pinch is unchanged in every tool.
    if (tool === "move") {
      if (beginGesture(e.clientX, e.clientY)) {
        canvas.setPointerCapture(e.pointerId);
        panRef.current = { id: e.pointerId, startX: e.clientX, startY: e.clientY };
        canvas.style.cursor = "grabbing";
      }
      return;
    }

    // One-finger PAN via the gesture pipeline, ONLY when a draw tool is armed AND a pen
    // has been seen (palm rejection, #4: pen draws, finger pans; pen-less device keeps
    // drawing on finger — #2, phone default). In SELECT mode a one-finger drag now
    // MARQUEES on empty space / MOVES on an object (T43, VLL) — it falls through to the
    // select block below, same as the mouse. No navigation is lost: two fingers ALWAYS
    // pan/zoom (navRef), which already made one-finger-pan in select mode redundant.
    if (e.pointerType === "touch") {
      const doPan = !isNonDraw(tool) && penSeenRef.current;
      if (doPan && beginGesture(e.clientX, e.clientY)) {
        canvas.setPointerCapture(e.pointerId);
        panRef.current = { id: e.pointerId, startX: e.clientX, startY: e.clientY };
        return;
      }
    }

    if (tool === "select") {
      // Bug #4: a multi-selection group MOVE. If more than one object is
      // selected and the press lands inside the selection (on any selected
      // object or within the combined bbox), drag them ALL by one delta. Only
      // the editable-now members actually translate (others stay put); if none
      // are editable-now, fall through to a normal pick (no group move).
      const dims = pageDims();
      if (selectedOnPage.length > 1 && dims) {
        const measure = textMeasure();
        if (hitsMultiSelection(pt, selectedOnPage, dims.w, dims.h, measure)) {
          const items = selectedOnPage
            .filter((o) => isObjectEditableNow(o))
            .map((o) => ({ obj: o, preview: o }));
          if (items.length > 0) {
            canvas.setPointerCapture(e.pointerId);
            gestureRef.current = { mode: "multi-move", start: pt, items };
            return;
          }
        }
      }

      // ONE shared pick drives both this gesture and the hover cursor, so they
      // never disagree. pickAt resolves resize-handle > move > select > none.
      const ctx = buildPickContext();
      const pick = ctx ? pickAt(pt, ctx) : { object: null, mode: "none" as const };

      if (pick.mode === "resize" && pick.object && pick.handle) {
        // Press on a visible resize handle of the current selection → RESIZE.
        canvas.setPointerCapture(e.pointerId);
        gestureRef.current = {
          mode: "resize",
          obj: pick.object,
          handle: pick.handle,
          start: pt,
          preview: pick.object,
        };
        return;
      }

      if (pick.object) {
        onSelect([pick.object.uuid]);
        // Cross-layer focus: clicking an object focuses its layer (so the scoped
        // annotation list shows it and the user can see which layer it's on) —
        // WITHOUT activating it. Editing stays scoped to the active layer (Bug #2).
        onFocusLayer(pick.object.layerId);
        // Only start a MOVE gesture for objects editable RIGHT NOW (mode "move").
        // A "select"-mode object (locked OR non-active layer) is selectable to
        // inspect but never movable — the cursor already showed `pointer`, so no
        // surprise. The server `forbidden` reject is only a backstop.
        if (pick.mode === "move") {
          canvas.setPointerCapture(e.pointerId);
          gestureRef.current = { mode: "move", obj: pick.object, start: pt, preview: pick.object };
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
      // T90: text is a ONE-SHOT tool — disarm after the prompt resolves whether we placed or
      // cancelled, so the next tap selects/deselects instead of opening yet another prompt (the
      // dead end VLL hit on a phone: every tap was another modal). Revert on cancel too, so two
      // mis-taps can't re-trap.
      onTextResolved?.();
      return;
    }

    canvas.setPointerCapture(e.pointerId);
    gestureRef.current = { mode: "draw", path: [pt] };
    resetWetCache();
    perfRef.current =
      tool === "freehand" && typeof localStorage !== "undefined" && localStorage.inkPerf === "1"
        ? {
            points: 1,
            frames: 0,
            sumLatency: 0,
            last: performance.now(),
            t0: performance.now(),
            renderFirst: 0,
            renderLast: 0,
            renderMax: 0,
          }
        : null;
    forceRepaint((n) => n + 1);
  }

  function onPointerMove(e: React.PointerEvent) {
    // ---- Two-finger navigation: pinch (distance ratio) + pan (midpoint delta). ----
    const nav = navRef.current;
    if (nav) {
      const p = pointersRef.current.get(e.pointerId);
      if (p) {
        p.x = e.clientX;
        p.y = e.clientY;
      }
      const a = pointersRef.current.get(nav.idA);
      const b = pointersRef.current.get(nav.idB);
      if (a && b) {
        const midX = (a.x + b.x) / 2;
        const midY = (a.y + b.y) / 2;
        const dist = Math.hypot(a.x - b.x, a.y - b.y) || 1;
        updateGesture(dist / nav.startDist, midX - nav.startMidX, midY - nav.startMidY);
      }
      return;
    }
    // ---- One-finger pan (Move tool / touch-draw-pan): pan only, no zoom. ----
    if (panRef.current && e.pointerId === panRef.current.id) {
      const dx = e.clientX - panRef.current.startX;
      const dy = e.clientY - panRef.current.startY;
      // T66 Part D: a TAP jitters a few px. Don't pan below the tap slop, else each tap of a
      // double-tap-to-zoom drifts the view ("move") — VLL's report. Past the slop it's a real
      // pan (translate by the full delta — the tiny slop is imperceptible).
      if (Math.hypot(dx, dy) < TAP_SLOP) return;
      updateGesture(1, dx, dy);
      return;
    }

    const g = gestureRef.current;
    if (!g) {
      // No active gesture → this is a HOVER. Set an action-reflecting cursor from
      // the SAME pickAt the press will use, so the cursor predicts the next drag.
      updateHoverCursor(e);
      return;
    }
    const pt = pageRelative(e);
    if (g.mode === "draw") {
      if (tool === "freehand") {
        // Out of React, coalesced, one paint per frame (T06). Keep only points that
        // moved at least WET_MIN_STEP from the last kept point (T35 diet).
        const pts = coalescedPoints(e);
        const eps2 = WET_MIN_STEP * WET_MIN_STEP;
        for (const p of pts) {
          const last = g.path[g.path.length - 1];
          if (!last || (p.x - last.x) ** 2 + (p.y - last.y) ** 2 >= eps2) g.path.push(p);
        }
        const perf = perfRef.current;
        if (perf) {
          perf.points += pts.length; // received rate (latency metric), not kept count
          perf.last = performance.now();
        }
        scheduleWetFrame();
      } else {
        // line/rect/ellipse previews are O(1) geometry — the simple full repaint
        // is already cheap and exact.
        g.path.push(pt);
        repaint();
      }
    } else if (g.mode === "move") {
      const dx = pt.x - g.start.x;
      const dy = pt.y - g.start.y;
      g.preview = translateObject(g.obj, dx, dy);
      repaint();
    } else if (g.mode === "multi-move") {
      const dx = pt.x - g.start.x;
      const dy = pt.y - g.start.y;
      for (const it of g.items) it.preview = translateObject(it.obj, dx, dy);
      repaint();
      forceHandles((n) => n + 1); // move the DOM bboxes with the drag
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
    pointersRef.current.delete(e.pointerId);
    const canvas = canvasRef.current;

    // ---- End a two-finger navigation when either finger lifts → commit ONE raster. ----
    const nav = navRef.current;
    if (nav) {
      if (e.pointerId === nav.idA || e.pointerId === nav.idB) {
        navRef.current = null;
        endGesture();
      }
      return;
    }
    // ---- End a one-finger pan → commit (scroll reconciles). ----
    if (panRef.current && e.pointerId === panRef.current.id) {
      const start = panRef.current;
      panRef.current = null;
      if (canvas?.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
      endGesture();
      if (tool === "move" && canvas) canvas.style.cursor = "grab"; // grabbing → grab (T65)
      // T66 Part D: a Move-mode TAP/CLICK (barely moved — a real pan resets the tracker) that
      // follows another within DOUBLE_TAP_MS at ~the same spot is a double-tap/double-click →
      // zoom to the point. This is the SINGLE detector for BOTH mouse and touch (pointer events
      // unify them); the DOM `dblclick` is intentionally NOT wired, so the browser's synthesized
      // touch-compat dblclick can't double-fire the zoom (that netted it back to nothing — VLL).
      if (tool === "move") {
        const moved = Math.hypot(e.clientX - start.startX, e.clientY - start.startY);
        if (moved < TAP_SLOP) {
          const now = performance.now();
          const last = lastTapRef.current;
          if (
            last &&
            now - last.t < DOUBLE_TAP_MS &&
            Math.hypot(e.clientX - last.x, e.clientY - last.y) < DOUBLE_TAP_DIST
          ) {
            lastTapRef.current = null;
            onDoubleTapZoom?.(e.clientX, e.clientY);
          } else {
            lastTapRef.current = { t: now, x: e.clientX, y: e.clientY };
          }
        } else {
          lastTapRef.current = null; // a real pan is never a double-tap
        }
      }
      return;
    }

    const g = gestureRef.current;
    gestureRef.current = null;
    if (canvas?.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
    if (!g) return;
    if (g.mode === "draw" && tool !== "select") {
      resetWetCache(); // drop any queued frame; commit re-renders authoritatively
      const perf = perfRef.current;
      if (perf) {
        const secs = (performance.now() - perf.t0) / 1000 || 1;
        const mean = perf.frames ? (perf.sumLatency / perf.frames).toFixed(1) : "?";
        // eslint-disable-next-line no-console
        console.log(
          `[inkPerf] ${perf.points} pts / ${secs.toFixed(2)}s = ${Math.round(perf.points / secs)} pts/s · ${perf.frames} frames · ` +
            `mean event→paint ${mean}ms · per-frame render first ${perf.renderFirst.toFixed(2)}ms → last ${perf.renderLast.toFixed(2)}ms (max ${perf.renderMax.toFixed(2)}ms)`,
        );
        perfRef.current = null;
      }
      // T35: always keep the FINAL point — the filter may have dropped a sub-step move
      // near the release, so the committed stroke ends exactly where the finger lifted
      // (and the committed geometry matches the wet preview the user just watched).
      if (tool === "freehand") {
        const end = pageRelative(e);
        const last = g.path[g.path.length - 1];
        if (!last || last.x !== end.x || last.y !== end.y) g.path.push(end);
      }
      onCommitDraw(tool as DrawTool, page, g.path);
    } else if (g.mode === "move") {
      // Only commit if it actually moved.
      const moved =
        g.preview.points.some((p, i) => p.x !== g.obj.points[i].x || p.y !== g.obj.points[i].y);
      if (moved) onCommitMove(g.preview);
    } else if (g.mode === "multi-move") {
      // Commit a move mutation for each member that actually moved (Bug #4).
      for (const it of g.items) {
        const moved = it.preview.points.some(
          (p, i) => p.x !== it.obj.points[i].x || p.y !== it.obj.points[i].y,
        );
        if (moved) onCommitMove(it.preview);
      }
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
        onPointerLeave={() => {
          // Clear the action cursor when leaving the canvas so a stale move/resize
          // cursor doesn't linger; CSS `.tool-*` default takes over.
          const c = canvasRef.current;
          if (c && c.style.cursor) c.style.cursor = "";
        }}
      />
      {/* Selection overlays: DOM elements positioned in % of the page box, so
          they track the page under any zoom AND are queryable in e2e. */}
      <div className="selection-overlay" aria-hidden="true">
        {selectedOnPage.map((o) => {
          // While resizing THIS object, draw the live preview box so the bbox +
          // handles follow the drag.
          const g = gestureRef.current;
          let previewing = o;
          if (g && (g.mode === "resize" || g.mode === "move") && g.obj.uuid === o.uuid) {
            previewing = g.preview;
          } else if (g && g.mode === "multi-move") {
            const it = g.items.find((x) => x.obj.uuid === o.uuid);
            if (it) previewing = it.preview;
          }
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
              data-uuid={o.uuid}
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
      {/* Floating selection toolbar (T27 stage 2): shown by the single,
          active-editable selection. Unlike the aria-hidden .selection-overlay, this
          is interactive — it sits ABOVE the wet canvas with its own pointer-events
          so its buttons are clickable, anchored at the object's bbox and lifted
          above it in CSS. No layout shift (absolute over the canvas). */}
      {selectedSingle && isObjectEditableNow(selectedSingle) && (() => {
        const g = gestureRef.current;
        const previewing =
          g && (g.mode === "resize" || g.mode === "move") && g.obj.uuid === selectedSingle.uuid
            ? g.preview
            : selectedSingle;
        const rm: TextMeasure | undefined = pageBoxPx
          ? { pageW: pageBoxPx.w, pageH: pageBoxPx.h, widthPx: measureTextWidth }
          : undefined;
        const b = objectBBox(previewing, rm);
        return (
          <div
            className="sel-toolbar-anchor"
            style={{
              left: `${b.minX * 100}%`,
              top: `${b.minY * 100}%`,
              width: `${(b.maxX - b.minX) * 100}%`,
            }}
          >
            <SelectionToolbar
              color={selectedSingle.style.color}
              onColor={(c) => onSetColor(selectedSingle.uuid, c)}
              onBringToFront={() => onReorder(selectedSingle.uuid, "front")}
              onSendToBack={() => onReorder(selectedSingle.uuid, "back")}
              onDuplicate={() => onDuplicate(selectedSingle.uuid)}
              onDelete={onDelete}
            />
          </div>
        );
      })()}
    </>
  );
}
