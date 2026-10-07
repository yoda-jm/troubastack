// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { RehearsalNoteViewer } from "../src/pages/song-editor/RehearsalNoteViewer";
import type { RehearsalNote, SongFile } from "../src/api";

afterEach(cleanup);

function note(over: Partial<RehearsalNote> = {}): RehearsalNote {
  return {
    id: "n" + (over.pageInSong ?? 0),
    bandId: "b",
    songId: "s",
    pageInSong: 0,
    rasterHash: "rh",
    concertId: "c",
    concertRev: 12,
    takenAs: "member-7",
    blobHash: "bh",
    width: 1600,
    height: 2261,
    uploadedAt: "2026-09-11T21:00:00Z",
    pageChanged: false,
    ...over,
  };
}

const files: SongFile[] = [
  { id: "f-bass", filename: "bass.pdf" } as SongFile,
  { id: "f-lyrics", filename: "lyrics.pdf" } as SongFile,
];

function view(notes: RehearsalNote[], index = 0, props: Partial<Parameters<typeof RehearsalNoteViewer>[0]> = {}) {
  return render(
    <RehearsalNoteViewer
      notes={notes}
      index={index}
      bandId="b"
      songId="s"
      files={files}
      onClose={() => {}}
      onNavigate={() => {}}
      onGoToNote={() => {}}
      onRemove={() => {}}
      {...props}
    />,
  );
}

describe("T186 — the rehearsal-note viewer", () => {
  it("is a dialog that composites the note over the bake-page background", () => {
    view([note({ pageInSong: 2, fileId: "f-bass", filePage: 0 })]);
    const dlg = screen.getByTestId("note-viewer");
    expect(dlg.getAttribute("role")).toBe("dialog");
    expect(dlg.getAttribute("aria-modal")).toBe("true");
    // background is the /background endpoint; the note is the note image.
    expect(screen.getByTestId("note-viewer-bg").getAttribute("src")).toContain("/rehearsal-notes/2/background");
    expect(screen.getByTestId("note-viewer-note").getAttribute("src")).toContain("/rehearsal-notes/2");
  });

  it("a RESOLVED note names its file and page, and offers Go to", () => {
    view([note({ pageInSong: 2, fileId: "f-bass", filePage: 0 })]);
    expect(screen.getByTestId("note-viewer-info").textContent).toContain("bass.pdf · page 1 · from rev 12");
    expect(screen.getByTestId("note-viewer-goto").textContent).toContain("bass.pdf");
  });

  it("an UNRESOLVED note with a background says the page is no longer in the files, never a page number", () => {
    view([note({ pageInSong: 2 })]); // no fileId
    const info = screen.getByTestId("note-viewer-info").textContent ?? "";
    expect(info).toBe("Drawn on a page that is no longer in this song's files, shown as it was at rev 12.");
    expect(info).not.toMatch(/page \d/); // never imply a page number
    expect(screen.queryByTestId("note-viewer-goto")).toBeNull(); // nowhere to go
  });

  it("when the background 404s, it shows the strokes alone and says so", () => {
    view([note({ pageInSong: 2 })]);
    fireEvent.error(screen.getByTestId("note-viewer-bg")); // the /background endpoint returned 404
    expect(screen.queryByTestId("note-viewer-bg")).toBeNull(); // no background shown
    expect(screen.getByTestId("note-viewer-info").textContent).toBe(
      "The page this note was drawn on is no longer available. Showing the strokes alone.",
    );
    expect(screen.getByTestId("note-viewer-note")).toBeTruthy(); // the strokes are still there
  });

  it("‹ › page across the notes, disabled at the ends, and ← → keys navigate", () => {
    const onNavigate = vi.fn();
    const notes = [note({ pageInSong: 0 }), note({ pageInSong: 1 }), note({ pageInSong: 2 })];
    const { rerender } = view(notes, 0, { onNavigate });
    expect(screen.getByTestId("note-viewer-count").textContent).toBe("1 / 3");
    expect((screen.getByTestId("note-viewer-prev") as HTMLButtonElement).disabled).toBe(true); // first
    expect((screen.getByTestId("note-viewer-next") as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(screen.getByTestId("note-viewer-next"));
    expect(onNavigate).toHaveBeenCalledWith(1);
    fireEvent.keyDown(screen.getByTestId("note-viewer"), { key: "ArrowLeft" });
    expect(onNavigate).toHaveBeenCalledWith(-1);
    // at the last note, next is disabled
    rerender(
      <RehearsalNoteViewer
        notes={notes}
        index={2}
        bandId="b"
        songId="s"
        files={files}
        onClose={() => {}}
        onNavigate={onNavigate}
        onGoToNote={() => {}}
        onRemove={() => {}}
      />,
    );
    expect((screen.getByTestId("note-viewer-next") as HTMLButtonElement).disabled).toBe(true);
  });

  it("Escape and Close both close; Done/remove removes the note's page", () => {
    const onClose = vi.fn();
    const onRemove = vi.fn();
    view([note({ pageInSong: 4 })], 0, { onClose, onRemove });
    fireEvent.keyDown(screen.getByTestId("note-viewer"), { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("note-viewer-remove"));
    expect(onRemove).toHaveBeenCalledWith(4);
  });
});
