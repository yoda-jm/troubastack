import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { api, type RehearsalNote, type SongFile } from "../../api";
import { noteDate } from "./RehearsalNotes";

/**
 * T186 — look at a rehearsal note on the page it was drawn on. A note is ~99.9% transparent strokes; on its
 * own it is marks on nothing. The viewer composites the note PNG over the BAKE PAGE whose rasterHash is the
 * note's (served by ⟨D1⟩), so it reads as the page the musician wrote on — even when the chart has changed
 * since, and even when the note no longer resolves to any current file (the case VLL asked for). When no rev
 * still holds that page, the background 404s and we show the strokes on plain paper and say so.
 *
 * A real dialog: role=dialog, focus moved in on open and RETURNED to the opener on close, Escape closes, and
 * ‹ › / ← → page across the song's notes.
 */
export function RehearsalNoteViewer({
  notes,
  index,
  bandId,
  songId,
  files,
  onClose,
  onNavigate,
  onGoToNote,
  onRemove,
}: {
  notes: RehearsalNote[];
  index: number;
  bandId: string;
  songId: string;
  files: SongFile[];
  onClose: () => void;
  onNavigate: (delta: number) => void;
  onGoToNote: (note: RehearsalNote) => void;
  onRemove: (page: number) => void;
}) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const [bgFailed, setBgFailed] = useState(false);
  const n = notes[index];

  // The background is per-note; a new note must try its own page again.
  useEffect(() => {
    setBgFailed(false);
  }, [n?.id]);

  // Focus in on open, RETURN focus to the opener (the row/button) on close.
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    dialogRef.current?.focus();
    return () => prev?.focus?.();
  }, []);

  // Escape and ← → are handled at the DOCUMENT level, not on the dialog, because paging to an end disables a
  // button and the browser then moves focus to <body> — a dialog-scoped handler would miss the next key.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (e.key === "ArrowLeft") {
        onNavigate(-1);
      } else if (e.key === "ArrowRight") {
        onNavigate(1);
      }
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose, onNavigate]);

  // Tab stays trapped within the dialog (focus is inside while tabbing, so a dialog handler suffices).
  const onKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key !== "Tab") return;
    const focusables = dialogRef.current?.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
    );
    if (!focusables || focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement;
    if (e.shiftKey && (active === first || active === dialogRef.current)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }, []);

  if (!n) return null;

  const resolved = !!n.fileId;
  const targetFile = resolved ? files.find((f) => f.id === n.fileId) : undefined;
  const info =
    resolved && targetFile
      ? `${targetFile.filename} · page ${(n.filePage ?? 0) + 1} · from rev ${n.concertRev} · taken as ${n.takenAs || "—"} · ${noteDate(n)}`
      : bgFailed
        ? "The page this note was drawn on is no longer available. Showing the strokes alone."
        : `Drawn on a page that is no longer in this song's files, shown as it was at rev ${n.concertRev}.`;

  return createPortal(
    <div className="note-viewer-backdrop" data-testid="note-viewer-backdrop" onMouseDown={onClose}>
      <div
        className="note-viewer"
        data-testid="note-viewer"
        role="dialog"
        aria-modal="true"
        aria-label="Rehearsal note"
        tabIndex={-1}
        ref={dialogRef}
        onKeyDown={onKeyDown}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="note-viewer-info" data-testid="note-viewer-info">
          {info}
        </div>
        <div className={"note-viewer-stage" + (bgFailed ? " no-bg" : "")} data-testid="note-viewer-stage">
          {/* The frame shrink-wraps the background page; the note is positioned to THIS box, not the stage,
              so the strokes land exactly on the page (T186 fix: VLL — the note floated above the page). The
              note and the page share the bake raster's aspect, so filling the frame aligns them. */}
          <div className="note-viewer-frame" data-testid="note-viewer-frame">
            {!bgFailed && (
              <img
                className="note-viewer-bg"
                data-testid="note-viewer-bg"
                src={api.rehearsalNoteBackgroundUrl(bandId, songId, n.pageInSong, n.blobHash)}
                alt=""
                onError={() => setBgFailed(true)}
                draggable={false}
              />
            )}
            <img
              className="note-viewer-note"
              data-testid="note-viewer-note"
              src={api.rehearsalNoteUrl(bandId, songId, n.pageInSong, n.blobHash)}
              alt="A rehearsal note"
              draggable={false}
            />
          </div>
        </div>
        <div className="note-viewer-actions">
          <button
            type="button"
            className="ghost-btn"
            data-testid="note-viewer-prev"
            title="Previous note"
            disabled={index <= 0}
            onClick={() => onNavigate(-1)}
          >
            ‹
          </button>
          <span className="note-viewer-count" data-testid="note-viewer-count">
            {index + 1} / {notes.length}
          </span>
          <button
            type="button"
            className="ghost-btn"
            data-testid="note-viewer-next"
            title="Next note"
            disabled={index >= notes.length - 1}
            onClick={() => onNavigate(1)}
          >
            ›
          </button>
          <span className="note-viewer-spacer" />
          {resolved && targetFile && (
            <button
              type="button"
              className="ghost-btn"
              data-testid="note-viewer-goto"
              title={`Show this note on ${targetFile.filename}`}
              onClick={() => {
                onGoToNote(n);
                onClose();
              }}
            >
              Go to {targetFile.filename}
            </button>
          )}
          <button
            type="button"
            className="ghost-btn"
            data-testid="note-viewer-remove"
            title="Remove this note from Studio (the tablet keeps its copy)"
            onClick={() => {
              onRemove(n.pageInSong);
              onClose();
            }}
          >
            Done, remove
          </button>
          <button
            type="button"
            className="primary"
            data-testid="note-viewer-close"
            onClick={onClose}
          >
            Close
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
