/**
 * The song Viewer/editor: composition root over useSongSync (realtime spine) and
 * usePdfDocument (raster/zoom/overlay) — T15 split. Owns the file strip, the
 * layer/editing state + optimistic mutation handlers, and the JSX. Behavior +
 * data-testids unchanged.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  api,
  type AnnotationLayer,
  type AnnotationObject,
  type AnnotationStyle,
  type Role,
  type Song,
  type SongFile,
} from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
import { Metadata, Files, DeleteSong } from "./SongDetails";
import {
  DEFAULT_STYLE,
  buildObject,
  isMeaningfulGesture,
  isNonDraw,
  pointsForTool,
  newUuid,
  type DrawTool,
  type Tool,
} from "../../editor";
import { EditorToolbar } from "./Toolbar";
import { IconGlyphPalette } from "./IconGlyphPalette";
import { EditCanvas } from "./WetCanvas";
import { MyFilesEditor } from "./MyFilesEditor";
import { MyCuesEditor } from "./MyCuesEditor";
import { LayersPanel, AnnotationList, DeleteLayerDialog } from "./SidePanels";
import { isEditableLayer } from "./helpers";
import { useSongSync, defaultVisibility } from "./useSongSync";
import { usePdfDocument, ZOOM_PERCENTS } from "./usePdfDocument";
import { usePanelDismiss } from "./usePanelDismiss";
import { useBeat } from "../../useBeat";
import { meterGroups, tempoUnit } from "../../beatPhase";

// ===========================================================================
// Viewer
// ===========================================================================

/** z-order rank for a layer: conductor(0) < shared(1) < personal(2), then by
 *  `order`; within personal, my own layers sort ABOVE other members'. */
// T84: group tools that share a remembered stroke width — rect and ellipse are both "shapes".
function toolWidthKey(t: Tool): string {
  return t === "rect" || t === "ellipse" ? "shape" : t;
}

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
  song,
  onSongSaved,
  onSongDeleted,
}: {
  bandId: string;
  songId: string;
  songTitle: string;
  myUserId: string | null;
  myRole: Role | null;
  // The song's metadata (title/artist/key/tempo/tags/notes) + a save callback, so the
  // fullscreen editor's Details panel can edit it — the SongEditor page's own Details
  // section is clipped off-screen by the full-bleed layout (fixed: song info reachable).
  song: Song;
  onSongSaved: (s: Song) => void;
  // T36: the whole song-management surface (files + delete) lives in this panel now,
  // since SongEditor's clipped <Details> was the substrate of the unreachable-UI class
  // and is removed. onSongDeleted navigates away (the song no longer exists).
  onSongDeleted: () => void;
}) {
  // The file strip is MY ordered selection (getMyFiles), not the whole pool.
  // T138: `files` is the POOL (a browser over every file), not the bake selection. `mySelection` marks
  // which pool files are in the member's stage selection (an at-a-glance dot, never a re-order or a filter).
  const [files, setFiles] = useState<SongFile[]>([]);
  const [mySelection, setMySelection] = useState<Set<string>>(() => new Set());
  // The member's selection as an ORDERED list — what MyFilesEditor must seed from. `mySelection` is a Set
  // for the strip's O(1) mark and DISCARDS order, and my-files is an ordered list (it decides the page
  // sequence on stage). Passing `files` (the pool) here would tick every box and, on the first toggle,
  // PUT the whole pool over the member's curated selection.
  const [myOrdered, setMyOrdered] = useState<SongFile[]>([]);
  const [customized, setCustomized] = useState(false);
  // T68: the open file is mirrored into ?file=<id> so a hard refresh restores it instead of
  // snapping to the first file. Seed from the URL on mount; the initial-load pick validates it
  // against my pool before use (a stale/foreign id degrades gracefully to the first PDF).
  const [searchParams, setSearchParams] = useSearchParams();
  const initialFileParamRef = useRef<string | null>(searchParams.get("file"));
  const [selectedFileId, setSelectedFileId] = useState<string | null>(() =>
    searchParams.get("file"),
  );
  const [editorOpen, setEditorOpen] = useState(false);
  const detailsPanelRef = useRef<HTMLDivElement>(null);
  // T89's Details dismiss contract (Escape + outside-click, portal-aware) is now the shared
  // usePanelDismiss hook (T94), applied below to BOTH Details and the file rail so they can't drift.
  // Realtime spine (T15): the live doc, visibility, connection status, reject notice
  // and the WS client live in useSongSync; the load-once REST seed below writes
  // through setDoc/setVisible.
  const { doc, setDoc, visible, setVisible, connStatus, rejectNotice, syncRef } =
    useSongSync(bandId, songId, myUserId);

  // T30 — "no silent ink": while the realtime connection is down, ink cannot land,
  // so the editor presents READ-ONLY up-front (draw tools grayed via canDraw, wet
  // gestures blocked via WetCanvas drawLocked, and an explanatory chip in the
  // chrome) instead of letting strokes silently evaporate. Presentation only —
  // the sync client's reconnect semantics are untouched.
  const offline = connStatus !== "open";
  // T30 — commit-time notice for client-side declines (rendered through the same
  // alert surface as server rejects; cleared on the next successful commit).
  const [localNotice, setLocalNotice] = useState<string | null>(null);
  // P201 stage 2b: names of the live-mode setlists containing this song (empty = not
  // live). Polled so the editor's LIVE banner reflects an admin toggling it elsewhere.
  const [liveSetlists, setLiveSetlists] = useState<string[]>([]);
  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      try {
        const sls = await api.liveSetlistsForSong(bandId, songId);
        if (!cancelled) setLiveSetlists(sls.map((s) => s.name));
      } catch {
        /* transient — keep the last state; a real auth error surfaces elsewhere */
      }
    };
    void poll();
    const t = window.setInterval(poll, 20_000);
    return () => {
      cancelled = true;
      window.clearInterval(t);
    };
  }, [bandId, songId]);

  // ---- live editing state ----
  // T66: the editor opens in PAN mode — the first thing you do with a score is navigate,
  // not select. Selecting objects is now an explicit Select-tool choice.
  const [tool, setTool] = useState<Tool>("move");
  const [style, setStyle] = useState(DEFAULT_STYLE);
  // T84: remember the draw-default stroke width per tool GROUP (freehand / line / rect+ellipse) for
  // the session — a marker swipe and a box outline want different weights. Switching tools restores
  // that group's last width; changing width while drawing records it for the current group.
  const widthByTool = useRef<Record<string, number>>({});
  // T85 — the visual beat, driven by the song's tempo. Tap = count-in (self-stopping);
  // the ∞ toggle keeps it running. The frame pulses on the page border via a ref, so a
  // 60 fps beat never re-renders the editor.
  const beat = useBeat(song.tempo, song.meter);
  // T51 — the glyph the "Icon" tool stamps next (its id rides in the object's text).
  const [activeGlyph, setActiveGlyph] = useState("mic");
  // T54 — Details panel tab by AUDIENCE (Band 👥 / Mine 👤 / Admin). Remembered per
  // session (default Band — metadata is the commonest first read). Only the active
  // tab's content mounts, so switching to Mine re-fetches the pool (self-sufficient
  // my-files picker — Fable's workflow-trap pin).
  const [detailsTab, setDetailsTabState] = useState<"band" | "mine" | "admin">(() => {
    try {
      const v = sessionStorage.getItem("trouba_details_tab");
      if (v === "band" || v === "mine" || v === "admin") return v;
    } catch {
      /* sessionStorage unavailable (private mode / insecure ctx) — default below */
    }
    return "band";
  });
  const setDetailsTab = useCallback((t: "band" | "mine" | "admin") => {
    setDetailsTabState(t);
    try {
      sessionStorage.setItem("trouba_details_tab", t);
    } catch {
      /* ignore */
    }
  }, []);
  const [activeLayerId, setActiveLayerId] = useState<string | null>(null);
  // The layer the user is "focusing": drives which layer's objects the
  // annotation list shows (works for ANY layer, editable or locked). When the
  // focused layer is editable it also becomes the active DRAW layer; when it's
  // locked, drawing is disabled but its annotations stay browsable.
  const [focusedLayerId, setFocusedLayerId] = useState<string | null>(null);
  // The current selection (single click or rubber-band marquee). Empty = none.
  const [selectedUuids, setSelectedUuids] = useState<string[]>([]);
  const [status, setStatus] = useState<"loading" | "no-file" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  // The on-demand right drawer (T27 stage 3): `sidebarOpen` gates it; `drawerTab`
  // picks the Layers vs Annotations face. Toggled from the top-bar pills. Starts
  // closed so the canvas owns the viewport (mockup default).
  const [sidebarOpen, setSidebarOpen] = useState(false);
  // T94 §3.5: "annotations" is the CODE name for the tab the UI calls "Notes". The identifier and
  // AnnotationList stay; only the user-facing word is Notes. drawerTab persists for the editing
  // session, so reopening the rail returns to the last tab (§3.1).
  const [drawerTab, setDrawerTab] = useState<"layers" | "annotations">("layers");
  const railRef = useRef<HTMLElement>(null);
  // T94 §3.4 — at most one of {rail, Details} is open. Opening either closes the other; there is no
  // "restore the rail behind Details" (two overlapping panels was the reported mess).
  const closeDetails = useCallback(() => setEditorOpen(false), []);
  const closeRail = useCallback(() => setSidebarOpen(false), []);
  const openDetails = useCallback(() => {
    setSidebarOpen(false);
    setEditorOpen(true);
  }, []);
  const toggleRail = useCallback(() => {
    setEditorOpen(false); // §3.4: opening (or toggling) the rail never leaves Details open
    setSidebarOpen((open) => !open);
  }, []);
  // T94 §3.3 — ONE dismissal helper for both floating surfaces (extracted from T89). Details keeps
  // ✕ + Escape + outside-click; the RAIL opts out of outside-click (last arg false) because it is a
  // working inspector used WHILE clicking the score/toolbar — an outside-click arm dismisses it
  // mid-edit. Both keep ✕ + Escape. [gate flag: documented deviation from §3.3.]
  usePanelDismiss(editorOpen, closeDetails, detailsPanelRef, "my-files-edit");
  usePanelDismiss(sidebarOpen, closeRail, railRef, "sidebar-toggle", false);

  const selectedFile = useMemo(
    () => files.find((f) => f.id === selectedFileId) ?? null,
    [files, selectedFileId],
  );
  const isPdf = selectedFile?.contentType === "application/pdf";
  const isImage = selectedFile?.contentType.startsWith("image/") ?? false;

  // T68: mirror the open file into ?file=<id>. Single sync point — every selection path
  // (strip click, initial load, refresh) funnels through setSelectedFileId. `replace` so
  // switching files doesn't grow history (Back still exits the editor, not step through file
  // picks). Guarded to a no-op when already in sync, so it never loops on its own writes.
  useEffect(() => {
    const cur = searchParams.get("file");
    if ((selectedFileId ?? null) === (cur ?? null)) return;
    const next = new URLSearchParams(searchParams);
    if (selectedFileId) next.set("file", selectedFileId);
    else next.delete("file");
    setSearchParams(next, { replace: true });
  }, [selectedFileId, searchParams, setSearchParams]);

  // The floating chrome bar (T27 stage 3); we publish its measured height as
  // --chrome-h so the scroll column's top padding + scroll-padding + the floating
  // panel's top all clear it. Constant across tool changes (stable style-row
  // footprint) → no canvas shift.
  const chromeRef = useRef<HTMLDivElement | null>(null);
  // T66: the phone chrome's single horizontal-scroll region (tools · zoom · Layers/Notes/
  // Details). An edge fade (.of-start/.of-end) shows only when it actually overflows.
  const tbScrollRef = useRef<HTMLDivElement | null>(null);

  // ---- refresh the file strip (the POOL) + my selection marks — also after editor changes ----
  // T138: the strip browses the whole pool (in pool order), so switching to another part to annotate no
  // longer depends on it being in my bake selection. getMyFiles supplies only the MARK (which are mine) and
  // the `custom` pill. Keeps a sensible open file: the current one if it survives, else first-PDF/viewable.
  const refreshMyFiles = useCallback(async () => {
    const [pool, mine] = await Promise.all([api.listFiles(bandId, songId), api.getMyFiles(bandId, songId)]);
    setFiles(pool);
    setMySelection(new Set(mine.files.map((f) => f.id)));
    setMyOrdered(mine.files);
    setCustomized(mine.customized);
    setSelectedFileId((cur) => {
      if (cur && pool.some((f) => f.id === cur && isViewable(f))) return cur;
      const firstPdf = pool.find((f) => f.contentType === "application/pdf");
      const first = firstPdf ?? pool.find(isViewable) ?? null;
      return first ? first.id : null;
    });
    return pool;
  }, [bandId, songId]);

  // ---- load my files + annotations once ----
  useEffect(() => {
    let cancelled = false;
    (async () => {
      setStatus("loading");
      setError(null);
      try {
        const [pool, mine, annotations] = await Promise.all([
          api.listFiles(bandId, songId),
          api.getMyFiles(bandId, songId),
          api.getAnnotations(bandId, songId),
        ]);
        if (cancelled) return;
        // T138: the strip is the pool; the selection only marks which pool files are mine.
        setFiles(pool);
        setMySelection(new Set(mine.files.map((f) => f.id)));
        setMyOrdered(mine.files);
        setCustomized(mine.customized);
        setDoc(annotations);
        setVisible(defaultVisibility(annotations.layers, myUserId));

        // T68: honour a ?file=<id> from the URL if it names a real, viewable file in the pool
        // (restored INSIDE the load so it resolves before the layer/annotation effects run);
        // otherwise the default first-PDF (else first-viewable) pick. A stale/foreign id
        // degrades to that default — never a blank/wedged viewer.
        const urlPick = pool.find((f) => f.id === initialFileParamRef.current && isViewable(f));
        const firstPdf = pool.find((f) => f.contentType === "application/pdf");
        const first = urlPick ?? firstPdf ?? pool.find(isViewable) ?? null;
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

  // (Realtime sync → useSongSync; PDF load/raster/zoom/overlay → usePdfDocument — T15.)

  // Publish the floating chrome bar's height as --chrome-h on the .viewer card.
  useEffect(() => {
    const chrome = chromeRef.current;
    if (!chrome) return;
    const section = chrome.parentElement; // the .viewer card
    const apply = () => {
      section?.style.setProperty("--chrome-h", `${Math.round(chrome.getBoundingClientRect().height)}px`);
    };
    apply();
    const ro = new ResizeObserver(apply);
    ro.observe(chrome);
    return () => ro.disconnect();
  }, [status]);

  // T66: toggle .of-start/.of-end on the phone chrome's scroll region so its edge fade shows
  // only when it can scroll that way (dependency-free; a no-op on desktop where it's
  // display:contents and never scrolls). Same idiom as the T65 tool-row fade.
  useEffect(() => {
    const el = tbScrollRef.current;
    if (!el) return;
    const update = () => {
      el.classList.toggle("of-start", el.scrollLeft > 1);
      el.classList.toggle("of-end", el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
    };
    update();
    el.addEventListener("scroll", update, { passive: true });
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => {
      el.removeEventListener("scroll", update);
      ro.disconnect();
    };
  }, [status]);

  const layersById = useMemo(() => {
    const m = new Map<string, AnnotationLayer>();
    for (const l of doc.layers) m.set(l.id, l);
    return m;
  }, [doc.layers]);

  // Objects scoped to the file currently on screen (T40). Annotations are keyed by
  // song, but every layer binds to ONE file (layer.fileId); the render/hit-test must
  // only ever see the selected file's objects, or a layer bound to the Score paints
  // its ink onto the Vocals/Guitar parts (they share page indices) — the cross-file
  // bleed VLL hit. The layer PANELS already filter by selectedFileId; this is the same
  // filter for the canvas, applied once so the dry overlay + wet EditCanvas agree.
  const objectsForFile = useMemo(
    () =>
      doc.objects.filter((o) => {
        if (selectedFileId == null) return true;
        return layersById.get(o.layerId)?.fileId === selectedFileId;
      }),
    [doc.objects, layersById, selectedFileId],
  );

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

  // The Layers drawer must list only the on-screen file's layers (T40 follow-up).
  // sortedLayers spans the whole song; without this filter the panel showed every
  // file's layers (e.g. the Score's cues while viewing the Vocals part). layerRank
  // stays over ALL layers — it only drives z-order, and only this file's objects
  // render, so the relative stacking is preserved.
  const sortedFileLayers = useMemo(
    () => sortedLayers.filter((l) => selectedFileId == null || l.fileId === selectedFileId),
    [sortedLayers, selectedFileId],
  );

  // PDF raster / zoom / dry-overlay machinery (T15). Owns the scroll/content/canvas
  // refs the JSX binds, the zoom controls, and renderOverlays; preserves the
  // no-re-raster-on-edit + render-timing + wheel-zoom invariants internally.
  const {
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
    layoutImageOverlay,
    beginGesture,
    updateGesture,
    endGesture,
    toggleZoomAtPoint,
  } = usePdfDocument({
    selectedFile,
    isPdf,
    isImage,
    status,
    setStatus,
    setError,
    sidebarOpen,
    objects: objectsForFile,
    layersById,
    visible,
    layerRank,
  });

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
      try {
        const id = newUuid();
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
      } catch (err) {
        // T32: surface a layer-create failure instead of dying silently.
        console.error("createPersonalLayer failed", err);
        setLocalNotice(
          `Couldn't create a layer — ${err instanceof Error ? err.message : String(err)}`,
        );
        return null;
      }
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

  // T83: may the viewer DELETE this layer? Mirror the server's write-rights (apply.go): a
  // conductor-zone layer needs the conductor ROLE; any other zone, the layer's owner or a band admin.
  // The button is only an affordance — the server enforces the same rule and rejects otherwise.
  const canDeleteLayer = useCallback(
    (l: AnnotationLayer): boolean => {
      if (l.zone === "conductor") return myRole === "conductor";
      return (myUserId != null && l.ownerId === myUserId) || myRole === "admin";
    },
    [myUserId, myRole],
  );
  // Objects on a layer, across all pages of the file (objects carry layerId; objectsForFile spans the
  // file) — the count the hard-confirm dialog names (§5).
  const layerObjectCount = useCallback(
    (layerId: string): number => objectsForFile.filter((o) => o.layerId === layerId).length,
    [objectsForFile],
  );
  const [layerToDelete, setLayerToDelete] = useState<AnnotationLayer | null>(null);
  const confirmDeleteLayer = useCallback(() => {
    const l = layerToDelete;
    setLayerToDelete(null);
    if (!l || !syncRef.current) return;
    syncRef.current.deleteLayer(l);
    // If the deleted layer was active/focused, hand off to the next remaining layer (§6: no special
    // case — new-layer already exists, so nobody is stuck).
    const next = sortedFileLayers.find((x) => x.id !== l.id)?.id ?? null;
    setActiveLayerId((cur) => (cur === l.id ? next : cur));
    setFocusedLayerId((cur) => (cur === l.id ? null : cur));
  }, [layerToDelete, syncRef, sortedFileLayers]);

  // Ensure there's a layer to draw into, creating "My notes" on demand. Returns
  // the layer id to draw into, or null if we can't (no file / not signed in).
  // T28: the resolved layer is AUTO-REVEALED — the wet stroke renders regardless of
  // visibility, but the committed object is filtered by `visible`, so drawing on a
  // hidden layer used to silently swallow the annotation (it existed, synced, and was
  // never painted). Drawing IS the intent to see it (the Photoshop/GoodNotes idiom).
  const ensureActiveLayer = useCallback((): string | null => {
    let id: string | null = null;
    if (activeLayerId && editableLayers.some((l) => l.id === activeLayerId)) {
      id = activeLayerId;
    } else if (editableLayers[0]) {
      setActiveLayerId(editableLayers[0].id);
      id = editableLayers[0].id;
    } else {
      id = createPersonalLayer(); // already made visible on creation
    }
    if (id) {
      const layerId = id;
      setVisible((v) => (v[layerId] ? v : { ...v, [layerId]: true }));
    }
    return id;
  }, [activeLayerId, editableLayers, createPersonalLayer, setVisible]);

  // Commit a finished draw gesture: build the wire object on the active layer
  // and send a create (optimistically added by the sync client).
  const commitDraw = useCallback(
    (tool: DrawTool, page: number, path: { x: number; y: number }[], text?: string) => {
      if (!isMeaningfulGesture(tool, path)) return;
      const layerId = ensureActiveLayer();
      if (!layerId || !syncRef.current) {
        // T30: never swallow a gesture silently — say why it didn't land. (The wet
        // stroke is already cleared by WetCanvas after every gesture.)
        setLocalNotice("Couldn't place the annotation — no layer to draw on.");
        return;
      }
      try {
        const obj = buildObject({
          tool,
          points: pointsForTool(tool, path),
          page,
          layerId,
          style,
          // T51: an icon stamp carries its glyph id in `text` (from the palette).
          text: tool === "icon" ? activeGlyph : text,
        });
        if (tool === "text" && !obj.text) return; // empty text → nothing to create
        // Defense in depth: never commit onto a layer we may not write. ensureActiveLayer
        // only ever returns an editable layer (or a freshly-created personal one), so
        // we only reject a layer that is KNOWN to be non-editable in the current doc.
        const target = doc.layers.find((l) => l.id === layerId);
        if (target && !isEditableLayer(target, myUserId, myRole)) {
          setLocalNotice("Couldn't place the annotation — the layer isn't editable."); // T30
          return;
        }
        syncRef.current.createObject(obj);
        // T155: a freshly drawn shape must NOT be left selected. While a drawing tool is armed, nothing is
        // selected — the selection box + handles say "you can move me" (you can't, not without the select
        // tool), and for a line or freehand stroke they sit ON TOP of the ink and hide the very mark you
        // just made. Marking up a chart is several strokes in a row, not draw-then-move, so the tool stays
        // armed and the canvas shows the clean stroke. The object is still fully created (createObject
        // above) — the reference is not lost, so a future undo still targets it. Selection is a state of
        // the SELECT tool (pick-to-select is untouched). EXCEPTION: text — its flow switches to the select
        // tool on resolve, so the new text is left selected for immediate restyle/reposition (VLL's stated
        // carve-out).
        if (tool === "text") {
          setSelectedUuids([obj.uuid]);
        } else {
          setSelectedUuids([]);
        }
        setLocalNotice(null); // T30: a successful commit clears any stale decline notice
      } catch (err) {
        // T32: a create path must NEVER throw into the void (the insecure-context
        // crypto.randomUUID class — fixed at the source by newUuid, but any future throw
        // here would otherwise vanish silently). Surface it through the same T30 notice
        // and log for forensics.
        console.error("commitDraw failed", err);
        setLocalNotice(
          `Couldn't place the annotation — ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    },
    [ensureActiveLayer, doc.layers, myUserId, style, activeGlyph],
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
        uuid: newUuid(),
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
      widthByTool.current[toolWidthKey(tool)] = next.width; // T84: remember this tool group's width
      setStyle(next);
    },
    [selectedObject, selectionEditable, restyleObject, tool],
  );


  function toggle(layerId: string) {
    setVisible((v) => ({ ...v, [layerId]: !v[layerId] }));
  }


  // The style/layer row + tool cluster share one big prop set; build it once.
  const toolbarProps = {
    tool,
    onTool: (t: Tool) => {
      setTool(t);
      if (t !== "select") setSelectedUuids([]);
      // T84: restore this tool group's remembered draw width (freehand vs line vs shape).
      const remembered = widthByTool.current[toolWidthKey(t)];
      if (remembered != null) setStyle((s) => ({ ...s, width: remembered }));
    },
    style: effectiveStyle,
    onStyle: applyStyle,
    controlsLocked,
    multiSelected: selectedUuids.length > 1,
    selectedType: selectedObject?.type ?? null,
    editableLayers,
    activeLayerId,
    activeLayer,
    onActiveLayer: selectActiveLayer,
    onNewLayer: () => createPersonalLayer(),
    // T30: offline grays the draw tools via canDraw (NOT drawLocked — that prop
    // drives the "read-only layer" hint text, which would mislead here; the
    // offline chip in the chrome is the explanation instead).
    canDraw: myUserId != null && selectedFileId != null && !offline,
    drawLocked: focusLocked,
    canEditFocusedLayer,
    focusedLayerName: focusedLayer?.name ?? null,
    onEditLayer: editFocusedLayer,
    showEditLayerHint: selectionOnInactiveEditable,
    selectionCount: selectedUuids.length,
    canDeleteSelection: selectedUuids.some((u) => {
      const o = doc.objects.find((x) => x.uuid === u);
      return o != null && isObjectEditableNow(o);
    }),
    onDelete: deleteSelected,
  };
  // The contextual style pill (.ctx) shows only when there is something to style: a DRAW
  // tool is active, or one/more objects are selected. The non-drawing tools (Select, Move
  // — T66) are neutral: no style bar unless a selection exists (mockup behavior).
  const ctxShown = !isNonDraw(tool) || selectedUuids.length > 0;

  // T66 Part D: double-tap / double-click in MOVE mode zooms to the tapped point. The
  // in/out decision is STATELESS — toggleZoomAtPoint reads the ACTUAL current zoom (at/near
  // fit → zoom IN to 2× at the point; zoomed in → back to fit), so it can never desync into
  // a bare re-centre the way a private boolean did (VLL: double-tap-at-fit only re-centred).
  // Scoped to Move so it never collides with object editing in Select/draw. (Touch double-tap
  // and the browser's synthesized dblclick are de-duped in WetCanvas by pointer type, so this
  // runs exactly once per gesture.)
  const onDoubleTapZoom = (clientX: number, clientY: number) => {
    if (tool !== "move") return;
    toggleZoomAtPoint(clientX, clientY);
  };

  return (
    <section
      className={`card viewer${sidebarOpen ? "" : " sidebar-collapsed"}${
        liveSetlists.length > 0 ? " has-live-banner" : ""
      }`}
      data-testid="song-viewer"
    >
      {liveSetlists.length > 0 && (
        <div className="editor-live-banner" data-testid="editor-live-banner" role="status">
          <span className="live-dot" aria-hidden="true" />
          LIVE — your edits are publishing to performers ({liveSetlists.join(", ")})
        </div>
      )}
      {/* ---- Floating TOP BAR pill (T27 stage 3, matches the approved mockup):
          back · title · tool cluster · zoom · Layers/Notes/Details toggles. One
          slim glass row over the canvas; pointer-events pass through to the score
          except on the controls. ---- */}
      <div className="viewer-chrome topbar-pill" data-testid="viewer-chrome" ref={chromeRef}>
        {/* T66: on phone the pill is ONE row — Back + title PINNED (never scroll away),
            everything else in a single horizontal-scroll region (.tb-scroll) with the T65
            fade. Both wrappers are display:contents on desktop, so its layout is unchanged. */}
        <div className="tb-nav">
          <Link
            className="tb-back"
            to={`/bands/${bandId}`}
            aria-label="Back to band"
            title="Back to band"
          >
            &larr;
          </Link>
          <span className="tb-title" data-testid="song-title" title={songTitle}>
            {songTitle}
          </span>
        </div>
        <div className="tb-scroll" data-testid="tb-scroll" ref={tbScrollRef}>
        <span className="tb-divider" aria-hidden="true" />

        <EditorToolbar part="tools" {...toolbarProps} />

        <span className="tb-spring" />

        <div className="zoom-controls" data-testid="zoom-controls">
          <button type="button" data-testid="zoom-out" onClick={() => stepZoom(-1)} aria-label="Zoom out">
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
          <button type="button" data-testid="zoom-in" onClick={() => stepZoom(1)} aria-label="Zoom in">
            +
          </button>
        </div>

        <span className="tb-divider" aria-hidden="true" />

        {/* T85/T86 — visual beat. Only when the song has a tempo. One segmented capsule (A34
            parity): the metronome taps a count-in (∞ off) or a continuous beat (∞ on). The label
            names the tempo's UNIT — ♩ simple / ♩. compound / ♪ irregular-additive. */}
        {song.tempo ? (
          (() => {
            const unit = tempoUnit(meterGroups(song.meter));
            const glyph = unit === "dotted-quarter" ? "♩." : unit === "eighth" ? "♪" : "♩";
            return (
              <div className="beat-controls beat-capsule" data-testid="beat-controls">
                <button
                  type="button"
                  className={`pill-btn${beat.running ? " active" : ""}`}
                  data-testid="beat-toggle"
                  aria-pressed={beat.running}
                  title={`Visual beat — ${glyph}=${song.tempo}${song.meter ? ` (${song.meter})` : ""}${beat.continuous ? " · continuous" : " · count-in"}`}
                  onClick={() => (beat.running ? beat.stop() : beat.start(beat.continuous))}
                >
                  <svg className="pill-icon" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
                    <path d="M6 2h4l3 12H3L6 2z" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
                    <path d="M8 12L12.5 4.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
                    <circle cx="12.5" cy="4.5" r="1.1" fill="currentColor" />
                  </svg>
                  <span className="pill-label" data-testid="beat-tempo-label">
                    {glyph}={song.tempo}
                  </span>
                </button>
                <button
                  type="button"
                  className={`pill-btn beat-loop${beat.continuous ? " active" : ""}`}
                  data-testid="beat-loop"
                  aria-pressed={beat.continuous}
                  aria-label="Keep the beat running"
                  title="Keep the beat running (otherwise it counts in and stops)"
                  onClick={() => beat.setContinuous(!beat.continuous)}
                >
                  ∞
                </button>
              </div>
            );
          })()
        ) : null}

        <span className="tb-divider" aria-hidden="true" />

        {/* T105 — for a generated text chart the source IS the file, so offer to edit it from where you
            are reading it, without opening the files panel. Navigates to the dedicated editor route.
            Absent on PDFs (nothing to edit) — the type-awareness mirrors T104's row control. */}
        {selectedFile?.generated && (
          <Link
            to={`/bands/${bandId}/songs/${songId}/chart/${selectedFile.id}`}
            className="pill-btn"
            data-testid="viewer-edit-chart"
            title="Edit this chart"
          >
            <svg className="pill-icon" viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
              <path
                fill="currentColor"
                d="M3 17.25V21h3.75L17.8 9.94l-3.75-3.75L3 17.25zM20.7 7.04a1 1 0 000-1.41l-2.34-2.34a1 1 0 00-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
              />
            </svg>
            <span className="pill-label">Edit chart</span>
          </Link>
        )}

        {/* T94 §3.1 — ONE pill opens/closes the file rail; Layers ↔ Notes switch on the tab row inside
            it (that row is unavoidable once the rail is open, so a second top-bar pill was redundant).
            "This file" names the SCOPE — the rail inspects the file you are viewing. `sidebar-toggle`
            keeps its testid here; `drawer-notes` moves to the Notes tab inside the rail. */}
        <button
          type="button"
          className={`pill-btn${sidebarOpen ? " active" : ""}`}
          data-testid="sidebar-toggle"
          aria-pressed={sidebarOpen}
          aria-expanded={sidebarOpen}
          onClick={toggleRail}
          title="This file — layers & notes"
        >
          {/* T66: icon-only on phone (chrome row-count budget), text on desktop. */}
          <svg className="pill-icon" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
            <path d="M8 1.5l6.5 3.3L8 8.1 1.5 4.8 8 1.5z" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
            <path d="M1.5 8L8 11.3 14.5 8" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinejoin="round" />
          </svg>
          <span className="pill-label">This file</span>
        </button>
        {/* T94 §3.2 — Details is the SONG's properties, not a peer of the file rail: a ⚙ gear, and it
            takes over the screen (opening it closes the rail, §3.4). */}
        <button
          type="button"
          className={`pill-btn${editorOpen ? " active" : ""}`}
          data-testid="my-files-edit"
          aria-expanded={editorOpen}
          onClick={() => (editorOpen ? setEditorOpen(false) : openDetails())}
          title="Song properties & files"
        >
          <svg className="pill-icon" viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
            <path fill="currentColor" d="M12 8a4 4 0 100 8 4 4 0 000-8zm0 6a2 2 0 110-4 2 2 0 010 4z" />
            <path
              fill="currentColor"
              d="M19.4 13a7.6 7.6 0 000-2l2-1.6-2-3.4-2.4 1a7.3 7.3 0 00-1.7-1L14.9 2h-3.8l-.4 2.6a7.3 7.3 0 00-1.7 1l-2.4-1-2 3.4L4.6 11a7.6 7.6 0 000 2l-2 1.6 2 3.4 2.4-1a7.3 7.3 0 001.7 1l.4 2.6h3.8l.4-2.6a7.3 7.3 0 001.7-1l2.4 1 2-3.4-2-1.6z"
            />
          </svg>
          <span className="pill-label">Details</span>
        </button>
        </div>
      </div>

      {/* Hidden render-count probe: how many times PDF pages have actually been
          rasterized. e2e asserts this does NOT change across an annotation edit. */}
      <span
        data-testid="pdf-render-count"
        style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", opacity: 0 }}
        aria-hidden="true"
      >
        {pdfRenderCount}
      </span>

      {/* ---- Contextual STYLE pill (.ctx): slides in below the top bar only when a
          draw tool or a selection is active. Absolute over the canvas → zero-shift. ---- */}
      {ctxShown && (
        <div className="ctx-bar" data-testid="ctx-bar">
          <EditorToolbar part="style" {...toolbarProps} />
        </div>
      )}

      {/* T51 — glyph palette for the Icon tool (color/opacity stay in the style row). */}
      {tool === "icon" && toolbarProps.canDraw && (
        <IconGlyphPalette
          active={activeGlyph}
          color={style.color}
          onPick={setActiveGlyph}
          // T88: re-measure the page hug when zoom / page count / viewed file changes.
          reflowKey={`${zoomSelectValue}|${customZoomPercent ?? ""}|${numPages}|${selectedFileId ?? ""}`}
        />
      )}

      {(rejectNotice ?? localNotice) && (
        <p className="notice editor-reject-notice" data-testid="reject-notice" role="alert">
          {rejectNotice ?? localNotice}
        </p>
      )}

      <div className="viewer-body">
        {/* T85 — the beat frame: a rounded rail in the padding around the page, pulsing
            on the border and never over the music. The rAF loop writes its style via the
            ref; it starts invisible. */}
        <div
          className="beat-frame"
          data-testid="beat-frame"
          ref={beat.frameRef}
          aria-hidden="true"
          style={{ opacity: 0 }}
        />
        <div className="viewer-scroll" data-testid="viewer-scroll" ref={scrollRef}>
          {status === "loading" && <p className="muted">Loading…</p>}
          {status === "no-file" && files.length === 0 && (
            <p className="muted" data-testid="viewer-no-pdf">
              No files selected —{" "}
              <button
                type="button"
                className="link-button"
                onClick={openDetails}
              >
                choose some
              </button>
              , or upload a PDF or image under Details → “Shared with the band”.
            </p>
          )}
          {status === "no-file" && files.length > 0 && (
            <p className="muted" data-testid="viewer-no-pdf">
              No viewable file. Upload a PDF or image under Details → “Shared with the band”.
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
                  iconGlyph={activeGlyph}
                  drawLocked={focusLocked || offline}
                  objects={objectsForFile}
                  layersById={layersById}
                  layerRank={layerRank}
                  visible={visible}
                  selectedUuids={selectedUuids}
                  isObjectEditable={isEditableObject}
                  isObjectEditableNow={isObjectEditableNow}
                  onSelect={setSelectedUuids}
                  onFocusLayer={focusLayerOnly}
                  onCommitDraw={commitDraw}
                  onTextResolved={() => setTool("select")}
                  onCommitMove={commitMove}
                  onCommitResize={commitResize}
                  onReorder={reorderSelected}
                  onDuplicate={duplicateSelected}
                  onSetColor={setObjectColor}
                  onDelete={deleteSelected}
                  beginGesture={beginGesture}
                  updateGesture={updateGesture}
                  endGesture={endGesture}
                  onDoubleTapZoom={onDoubleTapZoom}
                />
              </div>
            ))}

          {status === "ready" && isImage && selectedFile && (
            <div className="pdf-page" data-testid="pdf-page">
              <img
                ref={imgRef}
                className="pdf-canvas image-page"
                src={api.fileUrl(selectedFile.id, selectedFile.revision)}
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
                iconGlyph={activeGlyph}
                drawLocked={focusLocked || offline}
                objects={objectsForFile}
                layersById={layersById}
                layerRank={layerRank}
                visible={visible}
                selectedUuids={selectedUuids}
                isObjectEditable={isEditableObject}
                isObjectEditableNow={isObjectEditableNow}
                onSelect={setSelectedUuids}
                onFocusLayer={focusLayerOnly}
                onCommitDraw={commitDraw}
                onTextResolved={() => setTool("select")}
                onCommitMove={commitMove}
                onCommitResize={commitResize}
                onReorder={reorderSelected}
                onDuplicate={duplicateSelected}
                onSetColor={setObjectColor}
                onDelete={deleteSelected}
                beginGesture={beginGesture}
                updateGesture={updateGesture}
                endGesture={endGesture}
                  onDoubleTapZoom={onDoubleTapZoom}
              />
            </div>
          )}
          </div>
        </div>

        {/* ---- On-demand right DRAWER: one tabbed glass dropdown (Layers |
            Annotations), toggled from the top-bar pills. Absolute over the canvas
            → opening/closing it never shifts the score. The Layers tab hosts layer
            management (active layer, +New layer, Edit-this-layer, Delete). ---- */}
        {sidebarOpen && (
          <aside className="drawer open" data-testid="viewer-drawer" ref={railRef}>
            {/* T94 — sticky tabs row: the two tabs are the only Layers↔Notes switch (the top-bar pill
                just opens/closes the rail), plus a ✕ that survives scrolling the rail body (§4). */}
            <div className="drawer-tabs">
              <button
                type="button"
                className={`drawer-tab${drawerTab === "layers" ? " active" : ""}`}
                data-testid="drawer-layers"
                aria-pressed={drawerTab === "layers"}
                onClick={() => setDrawerTab("layers")}
              >
                Layers
              </button>
              <button
                type="button"
                className={`drawer-tab${drawerTab === "annotations" ? " active" : ""}`}
                data-testid="drawer-notes"
                aria-pressed={drawerTab === "annotations"}
                onClick={() => setDrawerTab("annotations")}
              >
                Notes
              </button>
              <button
                type="button"
                className="drawer-close icon-btn"
                data-testid="drawer-close"
                aria-label="Close panel"
                title="Close"
                onClick={() => setSidebarOpen(false)}
              >
                ✕
              </button>
            </div>
            <div className="drawer-body">
              {drawerTab === "layers" ? (
                <>
                  <LayersPanel
                    layers={sortedFileLayers}
                    visible={visible}
                    myUserId={myUserId}
                    myRole={myRole}
                    activeLayerId={activeLayerId}
                    focusedLayerId={focusedLayerId}
                    onToggle={toggle}
                    onFocus={focusLayer}
                    canToggleAccess={canToggleLayerAccess}
                    onSetAccess={setLayerAccess}
                    canDelete={canDeleteLayer}
                    onDelete={setLayerToDelete}
                  />
                  <EditorToolbar part="layers" {...toolbarProps} />
                </>
              ) : (
                <AnnotationList
                  objects={objectsForFile}
                  focusedLayerId={focusedLayerId}
                  focusedLayer={focusedLayer}
                  focusLocked={focusLocked}
                  selectedUuids={selectedUuids}
                  onSelect={(uuid) => {
                    setSelectedUuids([uuid]);
                    scrollObjectIntoView(uuid);
                  }}
                />
              )}
            </div>
          </aside>
        )}
      </div>

      {layerToDelete && (
        <DeleteLayerDialog
          layer={layerToDelete}
          objectCount={layerObjectCount(layerToDelete.id)}
          onConfirm={confirmDeleteLayer}
          onCancel={() => setLayerToDelete(null)}
        />
      )}

      {/* ---- Floating BOTTOM BAR pill: file tabs (parts) + "＋ Add file" · live
          status (N objects · ● live). ---- */}
      <div className="bottombar-pill" data-testid="viewer-bottombar">
        <div className="parts" role="tablist" aria-label="Files" data-testid="file-picker">
          {files.map((f) => {
            const viewable = isViewable(f);
            const active = f.id === selectedFileId;
            const mine = mySelection.has(f.id); // T138: in my bake selection (an additive mark, not a filter)
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
                className={`part file-tab${active ? " active" : ""}${mine ? " mine" : ""}`}
                data-testid="file-tab"
                data-mine={mine ? "true" : undefined}
                role="tab"
                aria-selected={active}
                disabled={!viewable}
                title={viewable ? (mine ? `${f.filename} — in my stage selection` : f.filename) : `${f.filename} (not viewable)`}
                onClick={() => viewable && setSelectedFileId(f.id)}
              >
                <span className={`pill file-tab-badge badge-${badge.toLowerCase()}`}>{badge}</span>
                <span className="file-tab-name">{f.filename}</span>
                {/* T138 (VLL): mark MINE — never dim the others (they open fine; dimming would falsely say
                    "unavailable"). An unset member sees exactly one marked: the default the stage takes. */}
                {mine && (
                  <span className="file-tab-mine" data-testid="file-tab-mine" aria-label="In my stage selection" title="In my stage selection">
                    ●
                  </span>
                )}
              </button>
            );
          })}
          <button
            type="button"
            className="part add-file"
            data-testid="add-file"
            onClick={openDetails}
            title="Add or choose files"
          >
            ＋ Add file
          </button>
          {customized && (
            <span className="pill my-files-custom-pill" data-testid="my-files-custom">
              custom
            </span>
          )}
        </div>
        <div className="status">
          {/* T30: while disconnected the editor is read-only — say so, prominently,
              instead of letting strokes silently evaporate. */}
          {offline && (
            <span className="editor-readonly-chip" data-testid="editor-readonly" role="status">
              Read-only — offline, changes can't be saved
            </span>
          )}
          <span data-testid="object-count" title="Live annotation count">
            {doc.objects.length} objects
          </span>
          <span
            className={`live-status conn-${connStatus}`}
            data-testid="conn-status"
            title="Realtime connection"
          >
            {connStatus === "open" ? "live" : connStatus}
          </span>
        </div>
      </div>

      {/* Wheel/zoom affordance hint (desktop). */}
      <div className="wheelhint" aria-hidden="true">
        scroll to zoom toward the cursor · ⌘/ctrl+scroll fine · drag to pan
      </div>

      {/* ---- Details & files panel (floating; opened by the top-bar Details pill).
          Keeps metadata / upload / the T19 chart editor + T25 preview reachable —
          they must NOT regress behind the fullscreen chrome. ---- */}
      {editorOpen && (
        <div className="details-panel" data-testid="details-panel" ref={detailsPanelRef}>
          {/* T54 — tabs BY AUDIENCE (global-vs-personal legibility, scheme A): the
              audience split IS the tab structure, so it fixes both the illegible mix
              AND the long-scrolling modal. Band 👥 = shared with everyone (metadata +
              the file pool); Mine 👤 = just for you (my-files order + my-cues); Admin =
              the destructive delete (admins only). Only the active tab mounts. */}
          {(() => {
            // A non-admin can never sit on the Admin tab (session value or otherwise).
            const tab = detailsTab === "admin" && myRole !== "admin" ? "band" : detailsTab;
            return (
              <>
                <div className="details-tabs" role="tablist" aria-label="Song details">
                  <button
                    type="button"
                    role="tab"
                    data-testid="details-tab-band"
                    className={`details-tab${tab === "band" ? " active" : ""}`}
                    aria-selected={tab === "band"}
                    onClick={() => setDetailsTab("band")}
                  >
                    <span aria-hidden="true">👥</span> Shared with the band
                  </button>
                  <button
                    type="button"
                    role="tab"
                    data-testid="details-tab-mine"
                    className={`details-tab${tab === "mine" ? " active" : ""}`}
                    aria-selected={tab === "mine"}
                    onClick={() => setDetailsTab("mine")}
                  >
                    <span aria-hidden="true">👤</span> Just for you
                  </button>
                  {myRole === "admin" && (
                    <button
                      type="button"
                      role="tab"
                      data-testid="details-tab-admin"
                      className={`details-tab details-tab-admin${tab === "admin" ? " active" : ""}`}
                      aria-selected={tab === "admin"}
                      onClick={() => setDetailsTab("admin")}
                    >
                      Admin
                    </button>
                  )}
                  {/* T89 — the panel's own exit, in the sticky tabs row so it survives scrolling the
                      body. Right-aligned: the Admin tab already has margin-left:auto (stays far
                      right), so the ✕ sits after it; with no Admin tab the ✕ pushes itself right. */}
                  <button
                    type="button"
                    className={`icon-btn details-close${myRole === "admin" ? "" : " push-right"}`}
                    data-testid="details-close"
                    aria-label="Close details"
                    title="Close details"
                    onClick={() => setEditorOpen(false)}
                  >
                    ✕
                  </button>
                </div>
                <div className="details-tab-body" data-testid={`details-body-${tab}`}>
                  {tab === "band" && (
                    <>
                      <Metadata bandId={bandId} song={song} onSaved={onSongSaved} />
                      <Files
                        bandId={bandId}
                        songId={songId}
                        songTitle={song.title}
                        songArtist={song.artist}
                        songKey={song.key}
                        onSongKeyChanged={(k) => onSongSaved({ ...song, key: k })}
                        onFilesChanged={refreshMyFiles}
                      />
                    </>
                  )}
                  {tab === "mine" && (
                    <>
                      <MyFilesEditor
                        bandId={bandId}
                        songId={songId}
                        selected={myOrdered}
                        onChanged={refreshMyFiles}
                        onError={setError}
                      />
                      <MyCuesEditor bandId={bandId} songId={songId} onError={setError} />
                    </>
                  )}
                  {tab === "admin" && myRole === "admin" && (
                    <DeleteSong bandId={bandId} songId={songId} onDeleted={onSongDeleted} />
                  )}
                </div>
              </>
            );
          })()}
        </div>
      )}
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
