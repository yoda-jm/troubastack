/**
 * Song page — editable metadata, attached files (upload / rename / reorder /
 * delete / download), and an admin-only delete. The canvas/annotation editor is
 * still deferred (the placeholder panel at the bottom).
 */
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError, api, type Role, type Song, type SongFile } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

export function SongEditor() {
  const { bandId, songId } = useParams<{ bandId: string; songId: string }>();
  const navigate = useNavigate();
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
    <div className="page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      {error && !song ? <ErrorBanner message={error} /> : null}

      {song && (
        <>
          <h1 data-testid="song-title">{song.title}</h1>
          <Metadata bandId={bandId} song={song} onSaved={setSong} />
          <Files bandId={bandId} songId={songId} />
          {myRole === "admin" && (
            <DeleteSong
              bandId={bandId}
              songId={songId}
              onDeleted={() => navigate(`/bands/${bandId}`)}
            />
          )}
        </>
      )}

      <section className="card editor-placeholder" data-testid="editor-placeholder">
        <h2>Annotation editor — coming soon</h2>
        <p>
          Editor coming soon. The annotation editor (PDF + freehand ink canvas) will live here. It
          is deferred for now.
        </p>
      </section>
    </div>
  );
}

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
