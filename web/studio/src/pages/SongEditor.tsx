/**
 * Song page — now primarily a VIEW-ONLY annotation viewer. It renders the
 * song's first PDF (PDF.js) page-by-page, with annotation layers drawn on top
 * via the one renderer (@troubastack/ink, I8). Per-viewer layer visibility and
 * zoom are local view state — there is no annotation editing here.
 *
 * The song Details (metadata edit), Files (upload/rename/reorder/delete), and
 * admin Delete still live below the viewer in a collapsible section; their
 * data-testids are unchanged.
 */
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
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
  type Role,
  type Song,
  type SongFile,
} from "../api";
import { useAuth } from "../auth";
import { ErrorBanner } from "../components/ErrorBanner";

pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

const ZOOM_LEVELS = [0.5, 1, 1.5, 2];

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

          {/* Retained for the existing e2e #4 (data-testid + copy unchanged). */}
          <p className="muted editor-note" data-testid="editor-placeholder">
            View-only. Editor coming soon — annotation editing is deferred.
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

function Viewer({
  bandId,
  songId,
  myUserId,
}: {
  bandId: string;
  songId: string;
  myUserId: string | null;
}) {
  const [pdfFile, setPdfFile] = useState<SongFile | null>(null);
  const [doc, setDoc] = useState<AnnotationDoc>({ layers: [], objects: [] });
  const [visible, setVisible] = useState<LayerVisibility>({});
  const [zoom, setZoom] = useState(1);
  const [numPages, setNumPages] = useState(0);
  const [status, setStatus] = useState<"loading" | "no-pdf" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  // Loaded PDF.js document + cached page viewports (at scale 1) so overlays
  // can size themselves without re-asking PDF.js.
  const pdfDocRef = useRef<pdfjs.PDFDocumentProxy | null>(null);
  const pageCanvasRefs = useRef<(HTMLCanvasElement | null)[]>([]);
  const overlayRefs = useRef<(HTMLCanvasElement | null)[]>([]);

  // ---- load PDF bytes + annotations ----
  useEffect(() => {
    let cancelled = false;
    const task = { current: null as pdfjs.PDFDocumentLoadingTask | null };
    (async () => {
      setStatus("loading");
      setError(null);
      try {
        const [files, annotations] = await Promise.all([
          api.listFiles(bandId, songId),
          api.getAnnotations(bandId, songId),
        ]);
        if (cancelled) return;
        setDoc(annotations);
        setVisible(defaultVisibility(annotations.layers, myUserId));

        const pdf = files.find((f) => f.contentType === "application/pdf");
        if (!pdf) {
          setStatus("no-pdf");
          return;
        }
        setPdfFile(pdf);

        const res = await fetch(api.fileUrl(pdf.id), { credentials: "include" });
        if (!res.ok) throw new Error(`Failed to fetch PDF (${res.status})`);
        const bytes = new Uint8Array(await res.arrayBuffer());
        if (cancelled) return;

        const loadingTask = pdfjs.getDocument({ data: bytes });
        task.current = loadingTask;
        const pdfDoc = await loadingTask.promise;
        if (cancelled) {
          void pdfDoc.destroy();
          return;
        }
        pdfDocRef.current = pdfDoc;
        pageCanvasRefs.current = new Array(pdfDoc.numPages).fill(null);
        overlayRefs.current = new Array(pdfDoc.numPages).fill(null);
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
      const d = pdfDocRef.current;
      pdfDocRef.current = null;
      if (d) void d.destroy();
    };
  }, [bandId, songId, myUserId]);

  // ---- render PDF pages whenever the document or zoom changes ----
  useEffect(() => {
    const pdfDoc = pdfDocRef.current;
    if (status !== "ready" || !pdfDoc) return;
    let cancelled = false;

    (async () => {
      const dpr = window.devicePixelRatio || 1;
      for (let i = 0; i < pdfDoc.numPages; i++) {
        if (cancelled) return;
        const page = await pdfDoc.getPage(i + 1);
        if (cancelled) return;
        const viewport = page.getViewport({ scale: zoom });
        const canvas = pageCanvasRefs.current[i];
        const overlay = overlayRefs.current[i];
        if (!canvas) continue;

        // Crisp raster: backing store at devicePixelRatio, CSS size = viewport.
        const cssW = Math.floor(viewport.width);
        const cssH = Math.floor(viewport.height);
        canvas.width = Math.floor(viewport.width * dpr);
        canvas.height = Math.floor(viewport.height * dpr);
        canvas.style.width = `${cssW}px`;
        canvas.style.height = `${cssH}px`;

        const ctx = canvas.getContext("2d");
        if (!ctx) continue;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        await page.render({ canvasContext: ctx, viewport }).promise;
        if (cancelled) return;

        if (overlay) {
          overlay.width = canvas.width;
          overlay.height = canvas.height;
          overlay.style.width = `${cssW}px`;
          overlay.style.height = `${cssH}px`;
        }
      }
      if (!cancelled) renderOverlays();
    })();

    return () => {
      cancelled = true;
    };
    // renderOverlays is stable enough via the deps it reads; we intentionally
    // re-run on zoom/status/doc. (eslint-disable would go here in a linted repo.)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, zoom, numPages]);

  // ---- overlay rendering (cheap; re-run on visibility/doc/zoom changes) ----
  const layersById = useMemo(() => {
    const m = new Map<string, AnnotationLayer>();
    for (const l of doc.layers) m.set(l.id, l);
    return m;
  }, [doc.layers]);

  const sortedLayers = useMemo(
    () => sortLayers(doc.layers, myUserId),
    [doc.layers, myUserId],
  );

  const renderOverlays = useCallback(() => {
    const dpr = window.devicePixelRatio || 1;
    // Objects grouped by page, in layer z-order.
    const orderIndex = new Map<string, number>();
    sortedLayers.forEach((l, idx) => orderIndex.set(l.id, idx));

    for (let p = 0; p < overlayRefs.current.length; p++) {
      const overlay = overlayRefs.current[p];
      if (!overlay) continue;
      const ctx = overlay.getContext("2d");
      if (!ctx) continue;
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.clearRect(0, 0, overlay.width, overlay.height);

      const objs = doc.objects
        .filter((o) => o.page === p)
        .filter((o) => {
          const l = layersById.get(o.layerId);
          return l && visible[l.id];
        })
        .sort((a, b) => {
          const la = orderIndex.get(a.layerId) ?? 0;
          const lb = orderIndex.get(b.layerId) ?? 0;
          return la - lb;
        });

      // Page rect fills the whole overlay (backing-store pixels).
      const pageRect = { x: 0, y: 0, w: overlay.width, h: overlay.height };
      renderObjects(ctx, objs.map(toInkObject) as InkObject[], pageRect);
      // dpr is already baked into the backing-store size, so width/height are
      // device px and fractions map directly. (No extra dpr scale needed.)
      void dpr;
    }
  }, [doc.objects, layersById, visible, sortedLayers]);

  // Re-render overlays when visibility (or sorted layers / objects) change,
  // without re-rasterizing the PDF.
  useEffect(() => {
    if (status === "ready") renderOverlays();
  }, [status, renderOverlays]);

  function toggle(layerId: string) {
    setVisible((v) => ({ ...v, [layerId]: !v[layerId] }));
  }

  const zoomIdx = ZOOM_LEVELS.indexOf(zoom);
  function zoomOut() {
    const i = zoomIdx <= 0 ? 0 : zoomIdx - 1;
    setZoom(ZOOM_LEVELS[i]);
  }
  function zoomIn() {
    const i = zoomIdx < 0 ? 1 : Math.min(ZOOM_LEVELS.length - 1, zoomIdx + 1);
    setZoom(ZOOM_LEVELS[i]);
  }

  return (
    <section className="card viewer" data-testid="song-viewer">
      <div className="viewer-toolbar">
        <div className="zoom-controls" data-testid="zoom-controls">
          <button type="button" data-testid="zoom-out" onClick={zoomOut} disabled={zoom <= ZOOM_LEVELS[0]}>
            −
          </button>
          <span className="pill" data-testid="zoom-level">
            {Math.round(zoom * 100)}%
          </span>
          <button
            type="button"
            data-testid="zoom-in"
            onClick={zoomIn}
            disabled={zoom >= ZOOM_LEVELS[ZOOM_LEVELS.length - 1]}
          >
            +
          </button>
        </div>
        {pdfFile && <span className="muted">{pdfFile.filename}</span>}
      </div>

      <div className="viewer-body">
        <div className="viewer-scroll" data-testid="viewer-scroll">
          {status === "loading" && <p className="muted">Loading…</p>}
          {status === "no-pdf" && (
            <p className="muted" data-testid="viewer-no-pdf">
              No PDF attached. Upload a PDF in “Details &amp; files” below to view it here.
            </p>
          )}
          {status === "error" && <ErrorBanner message={error} />}
          {status === "ready" &&
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
              </div>
            ))}
        </div>

        <LayersPanel layers={sortedLayers} visible={visible} myUserId={myUserId} onToggle={toggle} />
      </div>
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
