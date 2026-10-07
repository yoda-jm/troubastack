import { useCallback, useEffect, useState } from "react";
import { api, type RehearsalNote, type SongFile } from "../../api";

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

/** noteForPage picks the (at most one) note drawn on a given page of the OPEN file. A resolved note
 * (T185 ⟨D2⟩) matches its own fileId + filePage, so it draws only on its file; an unresolved note keeps
 * the single-file rule (pageInSong on whatever file is open), which is right when the song has one file. */
export function noteForPage(
  notes: RehearsalNote[],
  fileId: string | null,
  page: number,
): RehearsalNote | undefined {
  return notes.find((n) =>
    n.fileId ? n.fileId === fileId && n.filePage === page : n.pageInSong === page,
  );
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
  numPages,
  files,
  selectedFileId,
  onToggle,
  onRemove,
  onGoToNote,
  onView,
}: {
  notes: RehearsalNote[];
  shown: boolean;
  numPages: number;
  // T185 ⟨D3⟩ — the song's files, to name a resolved note's file and know whether "Go to" must switch.
  files: SongFile[];
  selectedFileId: string | null;
  onToggle: () => void;
  onRemove: (page: number) => void;
  onGoToNote: (note: RehearsalNote) => void;
  // T186 — open the note viewer at this row's note (by its index in the song's note list).
  onView: (index: number) => void;
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
          {notes.map((n, i) => {
            // T185 ⟨D3⟩ — a resolved note names its file and the page within it; "Go to" switches to that
            // file when it is not the open one. An unresolved note keeps the single-file "page N" rule.
            const resolved = !!n.fileId;
            const targetFile = resolved ? files.find((f) => f.id === n.fileId) : undefined;
            const targetPage = resolved ? n.filePage ?? 0 : n.pageInSong;
            const where =
              resolved && targetFile
                ? `${targetFile.filename} · page ${targetPage + 1}`
                : `page ${n.pageInSong + 1}`;
            const switches = resolved && !!targetFile && n.fileId !== selectedFileId;
            // Go-to is possible for a resolved note whenever its file is still on the song (its page exists
            // there by construction); for an unresolved note, only when the page is in the OPEN file.
            const canGoTo = resolved ? !!targetFile : n.pageInSong < numPages;
            const disabledTitle = resolved
              ? "That file is no longer on this song"
              : `This file has no page ${n.pageInSong + 1}`;
            const gotoLabel = switches
              ? `Go to ${targetFile!.filename}`
              : `Go to page ${targetPage + 1}`;
            const gotoTitle = switches
              ? `Switch to ${targetFile!.filename} and show this note on page ${targetPage + 1}`
              : `Show this note and scroll to page ${targetPage + 1}`;
            return (
            <div className="rehearsal-note-row" data-testid="rehearsal-note-row" key={n.id}>
              {/* T186 — the description is the obvious way in: click it to open the viewer. */}
              <button
                type="button"
                className="rehearsal-note-what"
                data-testid="rehearsal-note-what"
                title="Look at this note on the page it was drawn on"
                onClick={() => onView(i)}
              >
                {where} · from rev {n.concertRev} · taken as {n.takenAs || "—"} · {noteDate(n)}
              </button>
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
              {/* T186 — look at the note on the page it was drawn on. The obvious action; works for an
                  unresolved note too, which "Go to" cannot. */}
              <button
                type="button"
                className="rehearsal-note-view"
                data-testid="rehearsal-note-view"
                title="Look at this note on the page it was drawn on"
                onClick={() => onView(i)}
              >
                View
              </button>
              {/* T185 — go to the note. A resolved note (⟨D2⟩) goes to its own file + page, switching files
                  if needed; its page always exists in its file, so the button is enabled whenever that file
                  is still on the song. An UNRESOLVED note keeps T183's rule: enabled only when its page is in
                  the OPEN file, else it states that and does nothing rather than guess. */}
              {canGoTo ? (
                <button
                  type="button"
                  className="rehearsal-note-goto"
                  data-testid="rehearsal-note-goto"
                  title={gotoTitle}
                  onClick={() => {
                    onGoToNote(n);
                    setOpen(false);
                  }}
                >
                  {gotoLabel}
                </button>
              ) : (
                <button
                  type="button"
                  className="rehearsal-note-goto"
                  data-testid="rehearsal-note-goto"
                  disabled
                  title={disabledTitle}
                >
                  {gotoLabel}
                </button>
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
            );
          })}
        </div>
      )}
    </span>
  );
}
