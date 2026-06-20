/**
 * SongEditor — PLACEHOLDER for the deferred canvas/annotation editor (I10).
 *
 * The real editor lands here later: a PDF.js dry layer + the single in-progress
 * wet stroke on a `desynchronized` canvas, all rendered through @troubastack/ink
 * (the one renderer, I8), with optimistic objects reconciled to core echoes (I6).
 * See docs/design/03-rendering-and-ink.md. Until then this route just shows the
 * song metadata and an "editor coming soon" panel so the navigation works.
 */
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError, api, type Song } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";

export function SongEditor() {
  const { bandId, songId } = useParams<{ bandId: string; songId: string }>();
  const [song, setSong] = useState<Song | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!bandId || !songId) return;
    void api
      .listSongs(bandId)
      .then((songs) => {
        const found = songs.find((s) => s.id === songId) ?? null;
        setSong(found);
        if (!found) setError("Song not found");
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load song"));
  }, [bandId, songId]);

  return (
    <div className="page">
      <Link to={`/bands/${bandId}`}>&larr; Back to band</Link>
      <ErrorBanner message={error} />

      {song && (
        <>
          <h1 data-testid="song-title">{song.title}</h1>
          {song.artist && <p className="muted">{song.artist}</p>}
        </>
      )}

      <section className="card editor-placeholder" data-testid="editor-placeholder">
        <h2>Editor coming soon</h2>
        <p>
          The annotation editor (PDF + freehand ink canvas) will live here. It is
          deferred for now.
        </p>
      </section>
    </div>
  );
}
