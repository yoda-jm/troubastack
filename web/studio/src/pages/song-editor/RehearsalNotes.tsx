import { useCallback, useEffect, useState } from "react";
import { api, type RehearsalNote } from "../../api";

/**
 * T170 — rehearsal notes as a REFERENCE UNDERLAY.
 *
 * VLL's rule, which shapes every line here: a note is a bitmap precisely so that it can NOT be
 * mistaken for an annotation. You cannot import it as a layer and do things to it; you print it
 * between the PDF and your own marks and recopy what still matters by hand. So this module
 * renders an `<img>` and a list, and deliberately offers no way to select, move, re-place or
 * bake one. If any of that appears here, the wrong feature has been built.
 */

/** useRehearsalNotes loads the signed-in user's notes for a song. Notes are per-owner on the
 * server, so there is nothing to filter here — an empty list is the normal case. A failure is
 * swallowed to an empty list on purpose: the editor must open with or without this. */
export function useRehearsalNotes(bandId: string, songId: string) {
  const [notes, setNotes] = useState<RehearsalNote[]>([]);
  const reload = useCallback(() => {
    let live = true;
    api
      .listRehearsalNotes(bandId, songId)
      .then((n) => {
        if (live) setNotes(n);
      })
      .catch(() => {
        if (live) setNotes([]);
      });
    return () => {
      live = false;
    };
  }, [bandId, songId]);
  useEffect(() => reload(), [reload]);

  const remove = useCallback(
    async (page: number) => {
      await api.deleteRehearsalNote(bandId, songId, page);
      setNotes((prev) => prev.filter((n) => n.pageInSong !== page));
    },
    [bandId, songId],
  );
  return { notes, reload, remove };
}

/**
 * The underlay for ONE page. It goes between `.pdf-canvas` and `.annotation-overlay`, absolutely
 * positioned to the same page box, and is never hit-testable — a click on note ink must reach
 * whatever is underneath, because the note is not a thing you can pick up.
 *
 * Full opacity, no transparency slider (VLL). The PNG is itself ~99.9% transparent, so the chart
 * reads through it; dimming it would only make the handwriting harder to copy.
 */
export function RehearsalUnderlay({
  note,
  bandId,
  songId,
}: {
  note: RehearsalNote;
  bandId: string;
  songId: string;
}) {
  return (
    <img
      className="rehearsal-underlay"
      data-testid="rehearsal-underlay"
      data-page={note.pageInSong}
      src={api.rehearsalNoteUrl(bandId, songId, note.pageInSong, note.blobHash)}
      alt=""
      aria-hidden="true"
      draggable={false}
    />
  );
}

/** noteForPage picks the (at most one) note drawn on a page. The server's unique key is
 * (owner, song, page), so a second match would be a server bug, not a display choice. */
export function noteForPage(notes: RehearsalNote[], page: number): RehearsalNote | undefined {
  return notes.find((n) => n.pageInSong === page);
}

/** noteDate is the one honest date this row can show. `capturedAt` is when it was DRAWN and is
 * what the musician wants — but the tablet has no wall clock for a note today (A70 stamps a
 * boot-relative counter), so it arrives absent and we fall back to the upload date AND SAY SO.
 * Rendering the fallback unlabelled, as though it were the drawing time, is the failure here. */
export function noteDate(n: RehearsalNote): string {
  const captured = n.capturedAt && !n.capturedAt.startsWith("0001-01-01") ? n.capturedAt : "";
  const iso = captured || n.uploadedAt;
  const when = new Date(iso);
  const text = Number.isNaN(when.getTime()) ? iso : when.toLocaleDateString();
  return captured ? `drawn ${text}` : `sent ${text}`;
}

/**
 * The chip + its popover. Present only when the signed-in user has notes on this song; toggles
 * the underlay, default ON the first time (they sent them in order to see them).
 */
export function RehearsalNotesChip({
  notes,
  shown,
  onToggle,
  onRemove,
}: {
  notes: RehearsalNote[];
  shown: boolean;
  onToggle: () => void;
  onRemove: (page: number) => void;
  }) {
  const [open, setOpen] = useState(false);
  if (notes.length === 0) return null;
  return (
    <span className="rehearsal-chip-wrap">
      <button
        type="button"
        className={"pill-btn" + (shown ? " active" : "")}
        data-testid="rehearsal-notes-chip"
        aria-pressed={shown}
        title={shown ? "Hide the rehearsal notes underlay" : "Show the rehearsal notes underlay"}
        onClick={onToggle}
        onContextMenu={(e) => {
          e.preventDefault();
          setOpen((v) => !v);
        }}
      >
        <span className="pill-label">Rehearsal notes ({notes.length})</span>
      </button>
      <button
        type="button"
        className="pill-btn rehearsal-chip-more"
        data-testid="rehearsal-notes-more"
        aria-expanded={open}
        title="List the rehearsal notes on this song"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="pill-label">⋯</span>
      </button>
      {open && (
        <div className="rehearsal-popover" data-testid="rehearsal-notes-popover">
          {notes.map((n) => (
            <div className="rehearsal-note-row" data-testid="rehearsal-note-row" key={n.id}>
              <span className="rehearsal-note-what">
                page {n.pageInSong + 1} · from rev {n.concertRev} · taken as {n.takenAs || "—"} ·{" "}
                {noteDate(n)}
              </span>
              {/* Fable (dd6e359c): state the FACT, never the cause. A re-encode, a re-render, an
                  edit and a reflow are indistinguishable from a hash — and the label's first real
                  mass firing was a RENDERER change (two consecutive bakes moved every raster hash
                  without one page's content changing), so any wording of the form "the chart
                  changed under your note" would be false at the exact moment it first matters. */}
              {n.pageChanged === true && (
                <span
                  className="rehearsal-note-changed"
                  data-testid="rehearsal-note-changed"
                  title="This page isn't in the current bake — the reference may no longer line up"
                >
                  not in the current bake
                </span>
              )}
              <button
                type="button"
                className="rehearsal-note-done"
                data-testid="rehearsal-note-done"
                title="Remove this note from Studio (the tablet keeps its copy)"
                onClick={() => void onRemove(n.pageInSong)}
              >
                Done, remove
              </button>
            </div>
          ))}
        </div>
      )}
    </span>
  );
}
