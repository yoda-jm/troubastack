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
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError, api, type Role, type Song } from "../api";
import { Viewer } from "./song-editor/Viewer";
import { useAuth } from "../auth";
import { ErrorBanner } from "../components/ErrorBanner";

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
      {error && !song ? (
        <>
          <Link className="crumb" to={`/bands/${bandId}`}>&larr; Back to band</Link>
          <ErrorBanner message={error} />
        </>
      ) : null}

      {song && (
        <>
          <Viewer
            bandId={bandId}
            songId={songId}
            songTitle={song.title}
            myUserId={user?.id ?? null}
            myRole={myRole}
            song={song}
            onSongSaved={setSong}
            onSongDeleted={() => navigate(`/bands/${bandId}`)}
          />

          {/* T36: the song-management UI (metadata / files / delete) now lives INSIDE the
              editor's Details panel (Viewer). The old clipped <Details> section that used
              to sit here was removed — a clipped, human-unreachable copy is exactly how
              the "Playwright-reachable but human-unreachable" bug class survived. */}
          <p className="muted editor-note" data-testid="editor-note">
            Pick a tool above and draw on the page. Edits sync live to everyone in the band;
            history/revert is the next step.
          </p>
        </>
      )}
    </div>
  );
}

