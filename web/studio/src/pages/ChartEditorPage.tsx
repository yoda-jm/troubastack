/**
 * T105 — the dedicated, full-page chart editor route: /bands/:bandId/songs/:songId/chart/:fileId.
 *
 * It hosts the SAME `ChartEditor` the Details panel uses (T104) — one editor, two hosts — but with the
 * whole viewport instead of a dialog's leftovers, and a URL that is linkable and reloadable. Reaching it
 * from a link means it WILL be hit with a stale or wrong :fileId, so a file that isn't a generated chart
 * renders an honest not-found rather than an empty editor over nothing (you'd save over a non-chart).
 *
 * Leaving (Save, "Back to song", or the browser Back button) returns to the song with that file selected
 * (?file=), the reader's context. Unsaved edits are not warned about — they're PERSISTED (see ChartEditor
 * `persist`), because Back is the obvious exit here and there is no in-app navigation blocker under
 * <BrowserRouter>.
 */
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ApiError, api, type Song } from "../api";
import { ChartEditor, type ChartEdit } from "./song-editor/ChartEditor";
import { ErrorBanner } from "../components/ErrorBanner";

type State =
  | { kind: "loading" }
  | { kind: "notfound" }
  | { kind: "error"; message: string }
  | { kind: "ready"; initial: ChartEdit; song: Song | null };

export function ChartEditorPage() {
  const { bandId, songId, fileId } = useParams<{ bandId: string; songId: string; fileId: string }>();
  const navigate = useNavigate();
  const [state, setState] = useState<State>({ kind: "loading" });
  // The song key drives the Transpose control; keep a local copy so an "also update key" apply reflects.
  const [songKey, setSongKey] = useState<string | undefined>(undefined);

  const backToSong = useCallback(
    () => navigate(`/bands/${bandId}/songs/${songId}?file=${fileId}`),
    [navigate, bandId, songId, fileId],
  );

  useEffect(() => {
    if (!bandId || !songId || !fileId) return;
    let live = true;
    (async () => {
      try {
        const [chart, song] = await Promise.all([
          api.getChartSource(bandId, songId, fileId),
          api.getSong(bandId, songId),
        ]);
        if (!live) return;
        // Honest 404: only a generated text chart is editable here.
        if (!chart.file.generated) {
          setState({ kind: "notfound" });
          return;
        }
        setSongKey(song?.key);
        setState({
          kind: "ready",
          initial: { fileId, source: chart.source, baseRevision: chart.file.revision ?? 1 },
          song,
        });
      } catch (err) {
        if (!live) return;
        // A missing/invalid file id is a not-found, not an error banner.
        if (err instanceof ApiError && err.status === 404) setState({ kind: "notfound" });
        else setState({ kind: "error", message: err instanceof ApiError ? err.message : "Failed to load chart" });
      }
    })();
    return () => {
      live = false;
    };
  }, [bandId, songId, fileId]);

  if (!bandId || !songId || !fileId) return <div className="page">Loading…</div>;

  const backLink = `/bands/${bandId}/songs/${songId}${fileId ? `?file=${fileId}` : ""}`;

  return (
    <div className="page chart-route-page" data-testid="chart-route-page">
      <Link to={backLink} className="chart-route-back" data-testid="chart-route-back">
        &larr; Back to song
      </Link>

      {state.kind === "loading" && <p className="muted">Loading…</p>}

      {state.kind === "notfound" && (
        <div data-testid="chart-route-notfound">
          <h2>Not an editable chart</h2>
          <p className="muted">
            This file isn’t a generated text chart, so there’s no source to edit. It may have been
            deleted, or the link points at a PDF.
          </p>
        </div>
      )}

      {state.kind === "error" && <ErrorBanner message={state.message} />}

      {state.kind === "ready" && (
        <ChartEditor
          bandId={bandId}
          songId={songId}
          initial={state.initial}
          songKey={songKey}
          onSongKeyChanged={setSongKey}
          persist
          cancelLabel="Back to song"
          onDone={backToSong}
          onCancel={backToSong}
        />
      )}
    </div>
  );
}
