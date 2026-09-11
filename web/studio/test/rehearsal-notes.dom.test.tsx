// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
import {
  RehearsalNotesChip,
  RehearsalUnderlay,
  noteDate,
  noteForPage,
} from "../src/pages/song-editor/RehearsalNotes";
import type { RehearsalNote } from "../src/api";

afterEach(cleanup);

function note(over: Partial<RehearsalNote> = {}): RehearsalNote {
  return {
    id: "n" + (over.pageInSong ?? 0),
    bandId: "b",
    songId: "s",
    pageInSong: 0,
    rasterHash: "rh",
    concertId: "c",
    concertRev: 8,
    takenAs: "member-7",
    blobHash: "bh",
    width: 1600,
    height: 2261,
    uploadedAt: "2026-09-11T21:00:00Z",
    pageChanged: false,
    ...over,
  };
}

describe("T170 — the note is a reference, and the UI must not promise more than it knows", () => {
  it("picks the note drawn on the page, and nothing for a page without one", () => {
    const notes = [note({ pageInSong: 0 }), note({ pageInSong: 3 })];
    expect(noteForPage(notes, 3)?.pageInSong).toBe(3);
    expect(noteForPage(notes, 1)).toBeUndefined();
  });

  // The tablet has no wall clock for a note (A70 stamps SystemClock.elapsedRealtime), so
  // capturedAt arrives absent. Showing the upload date UNLABELLED would tell the musician the
  // note was drawn when it was in fact merely sent — a small lie about the one thing the row is
  // for. "drawn" vs "sent" is the whole point of this function.
  it("labels which date it is showing", () => {
    expect(noteDate(note({ capturedAt: "2026-09-01T10:00:00Z" }))).toMatch(/^drawn /);
    expect(noteDate(note())).toMatch(/^sent /);
    // Go serialises a zero time.Time as year 1; that is "no date", not the year 1.
    expect(noteDate(note({ capturedAt: "0001-01-01T00:00:00Z" }))).toMatch(/^sent /);
  });

  it("renders no chip at all when this user has no notes", () => {
    const { container } = render(
      <RehearsalNotesChip notes={[]} shown onToggle={() => {}} onRemove={() => {}} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("counts the notes and reflects whether the underlay is on", () => {
    const { rerender } = render(
      <RehearsalNotesChip
        notes={[note({ pageInSong: 0 }), note({ pageInSong: 1 })]}
        shown
        onToggle={() => {}}
        onRemove={() => {}}
      />,
    );
    const chip = screen.getByTestId("rehearsal-notes-chip");
    expect(chip.textContent).toContain("(2)");
    expect(chip.getAttribute("aria-pressed")).toBe("true");
    rerender(
      <RehearsalNotesChip
        notes={[note()]}
        shown={false}
        onToggle={() => {}}
        onRemove={() => {}}
      />,
    );
    expect(screen.getByTestId("rehearsal-notes-chip").getAttribute("aria-pressed")).toBe("false");
  });

  // pageChanged is three-valued and `null` means NOBODY CHECKED. Tagging a null as changed
  // cries wolf on every note when the server has no bake; treating it as unchanged tells the
  // musician their reference is current on no evidence. Neither may happen: null shows nothing.
  it("tags 'page changed' only when the server actually said true", () => {
    for (const [value, wanted] of [
      [true, true],
      [false, false],
      [null, false],
    ] as const) {
      const { unmount } = render(
        <RehearsalNotesChip
          notes={[note({ pageChanged: value })]}
          shown
          onToggle={() => {}}
          onRemove={() => {}}
        />,
      );
      fireEvent.click(screen.getByTestId("rehearsal-notes-more"));
      expect(
        screen.queryAllByTestId("rehearsal-note-changed").length > 0,
        `pageChanged=${String(value)} should ${wanted ? "" : "NOT "}show the tag`,
      ).toBe(wanted);
      unmount();
    }
  });

  it("Done, remove asks for the note's own page", () => {
    const onRemove = vi.fn();
    render(
      <RehearsalNotesChip
        notes={[note({ pageInSong: 4 })]}
        shown
        onToggle={() => {}}
        onRemove={onRemove}
      />,
    );
    fireEvent.click(screen.getByTestId("rehearsal-notes-more"));
    fireEvent.click(screen.getByTestId("rehearsal-note-done"));
    expect(onRemove).toHaveBeenCalledWith(4);
  });

  // The bytes behind the URL change when a note is re-sent, and the URL is otherwise identical
  // (it is keyed by page). Without the hash the browser serves the previous note — including
  // one the musician has just removed and re-sent.
  it("busts the cache with the blob hash", () => {
    render(<RehearsalUnderlay note={note({ blobHash: "deadbeef" })} bandId="b" songId="s" />);
    const img = screen.getByTestId("rehearsal-underlay");
    expect(img.getAttribute("src")).toContain("deadbeef");
    expect(img.getAttribute("aria-hidden")).toBe("true"); // decoration, not content
  });
});

// ---- source guards (§5.3) --------------------------------------------------------------

function walk(dir: string, out: string[] = []): string[] {
  for (const e of readdirSync(dir)) {
    const p = join(dir, e);
    if (statSync(p).isDirectory()) walk(p, out);
    else if (/\.(ts|tsx|mjs|js)$/.test(e)) out.push(p);
  }
  return out;
}

describe("T170 §5.3 — a rehearsal note must not reach the renderer or the bake", () => {
  // If "rehearsal" ever appears in ink or bake, someone has started teaching the renderer about
  // notes — which is the exact line this feature exists on the right side of. A note is not an
  // object, so nothing that draws objects may know it exists.
  it.each(["../../ink/src", "../../bake/src"])("%s never mentions a rehearsal note", (rel) => {
    const files = walk(join(HERE, rel));
    expect(files.length, `no sources found under ${rel} — this guard is looking at nothing`).toBeGreaterThan(0);
    const guilty = files.filter((f) => /rehearsal/i.test(readFileSync(f, "utf8")));
    expect(guilty, `these mention a rehearsal note: ${guilty}`).toEqual([]);
  });

  // DOM order IS the feature: the note prints between the chart and the annotation layers. A
  // reordering that put it above .annotation-overlay would hide the musician's own marks behind
  // the scribble they are copying. The e2e measures the pixels; this catches the edit.
  it("Viewer.tsx puts the underlay above the raster and below the annotations, in BOTH stacks", () => {
    const src = readFileSync(join(HERE, "../src/pages/song-editor/Viewer.tsx"), "utf8");
    const raster = [...src.matchAll(/className="pdf-canvas/g)].map((m) => m.index!);
    const under = [...src.matchAll(/<RehearsalUnderlay/g)].map((m) => m.index!);
    const overlay = [...src.matchAll(/className="annotation-overlay"/g)].map((m) => m.index!);
    expect(raster.length, "expected two page stacks (canvas pages + image pages)").toBe(2);
    expect(under.length).toBe(2);
    expect(overlay.length).toBe(2);
    for (let i = 0; i < 2; i++) {
      expect(raster[i], `stack ${i}: raster must come first`).toBeLessThan(under[i]);
      expect(under[i], `stack ${i}: the underlay must come BEFORE .annotation-overlay`).toBeLessThan(
        overlay[i],
      );
    }
  });
});

// Fable, dd6e359c: the tag states the FACT, never the cause. A re-encode, a re-render, an edit
// and a reflow are indistinguishable from a hash — and the label's first real mass firing was a
// RENDERER change, so "the chart changed under your note" would have been false exactly when it
// first mattered. Pinned as words, because this is a wording ruling and wording is what rots.
describe("T170 — the changed tag names what is checkable and nothing else", () => {
  it("says the page is not in the current bake, and blames nothing", () => {
    render(
      <RehearsalNotesChip
        notes={[note({ pageChanged: true })]}
        shown
        onToggle={() => {}}
        onRemove={() => {}}
      />,
    );
    fireEvent.click(screen.getByTestId("rehearsal-notes-more"));
    const tag = screen.getByTestId("rehearsal-note-changed");
    const words = (tag.textContent + " " + (tag.getAttribute("title") ?? "")).toLowerCase();
    expect(words).toContain("current bake");
    for (const cause of ["re-render", "rerender", "edited", "you changed", "the chart changed"]) {
      expect(words, `the tag blames a cause it cannot know: ${cause}`).not.toContain(cause);
    }
  });
});
