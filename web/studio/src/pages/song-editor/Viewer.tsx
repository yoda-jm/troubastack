/**
 * The song Viewer/editor: composition root over useSongSync (realtime spine) and
 * usePdfDocument (raster/zoom/overlay) — T15 split. Owns the file strip, the
 * layer/editing state + optimistic mutation handlers, and the JSX. Behavior +
 * data-testids unchanged.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  api,
  type AnnotationLayer,
  type AnnotationObject,
  type AnnotationStyle,
  type Role,
  type SongFile,
} from "../../api";
import { ErrorBanner } from "../../components/ErrorBanner";
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
import { isEditableLayer } from "./helpers";
import { useSongSync, defaultVisibility } from "./useSongSync";
import { usePdfDocument, ZOOM_PERCENTS } from "./usePdfDocument";

// ===========================================================================
// Viewer
// ===========================================================================

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
  // Realtime spine (T15): the live doc, visibility, connection status, reject notice
  // and the WS client live in useSongSync; the load-once REST seed below writes
  // through setDoc/setVisible.
  const { doc, setDoc, visible, setVisible, connStatus, rejectNotice, syncRef } =
    useSongSync(bandId, songId, myUserId);

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
  const [status, setStatus] = useState<"loading" | "no-file" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const selectedFile = useMemo(
    () => files.find((f) => f.id === selectedFileId) ?? null,
    [files, selectedFileId],
  );
  const isPdf = selectedFile?.contentType === "application/pdf";
  const isImage = selectedFile?.contentType.startsWith("image/") ?? false;

  // The floating chrome bar (T27 stage 3); we publish its measured height as
  // --chrome-h so the scroll column's top padding + scroll-padding + the floating
  // panel's top all clear it. Constant across tool changes (stable style-row
  // footprint) → no canvas shift.
  const chromeRef = useRef<HTMLDivElement | null>(null);

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
  } = usePdfDocument({
    selectedFile,
    isPdf,
    isImage,
    status,
    setStatus,
    setError,
    sidebarOpen,
    objects: doc.objects,
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


  function toggle(layerId: string) {
    setVisible((v) => ({ ...v, [layerId]: !v[layerId] }));
  }


  return (
    <section
      className={`card viewer${sidebarOpen ? "" : " sidebar-collapsed"}`}
      data-testid="song-viewer"
    >
      {/* Floating chrome (T27 stage 3): header + tools + zoom/files as one glass bar
          over the canvas (centered, width-capped). */}
      <div className="viewer-chrome" data-testid="viewer-chrome" ref={chromeRef}>
      {/* Compact single-row header (T05): back · title · status. */}
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
      </div>{/* .viewer-chrome */}

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
