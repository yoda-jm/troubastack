import { describe, it, expect } from "vitest";
import { badgeTitle, withNotesFirst } from "../src/pages/song-editor/noteBadge";

const songs = [
  { id: "a", title: "Alpha" },
  { id: "b", title: "Bravo" },
  { id: "c", title: "Charlie" },
  { id: "d", title: "Delta" },
];

describe("T173 — songs with notes come first, and nothing else moves", () => {
  it("puts noted songs first, most notes first", () => {
    const out = withNotesFirst(songs, { c: 1, a: 3 });
    expect(out.map((s) => s.id)).toEqual(["a", "c", "b", "d"]);
  });

  // The incoming order is somebody's choice — the caller sorted it, or the user filtered it. A song
  // with no notes must not move relative to its neighbours because a DIFFERENT song gained one, and
  // two songs with the SAME count must not swap on a re-render. A non-stable sort passes the test
  // above and fails this one.
  it("is a stable partition, not a re-sort", () => {
    expect(withNotesFirst(songs, {}).map((s) => s.id)).toEqual(["a", "b", "c", "d"]);
    expect(withNotesFirst(songs, { b: 2, d: 2 }).map((s) => s.id)).toEqual(["b", "d", "a", "c"]);
    // b and d tie: they keep their incoming order, and reversing the input reverses only them
    const rev = [...songs].reverse();
    expect(withNotesFirst(rev, { b: 2, d: 2 }).map((s) => s.id)).toEqual(["d", "b", "c", "a"]);
  });

  it("treats an absent song id as zero, so the client never distinguishes absent from 0", () => {
    expect(withNotesFirst(songs, { zzz: 9 }).map((s) => s.id)).toEqual(["a", "b", "c", "d"]);
    expect(withNotesFirst(songs, { a: 0 }).map((s) => s.id)).toEqual(["a", "b", "c", "d"]);
  });

  it("does not drop or duplicate a song", () => {
    const out = withNotesFirst(songs, { a: 1, b: 1, c: 1, d: 1 });
    expect([...out].map((s) => s.id).sort()).toEqual(["a", "b", "c", "d"]);
  });

  // D6: "2 notes" on a song row is ambiguous between the tablet and Studio, and only one of the two is
  // actionable from here. The hover text has to say WHERE, and what to do about it.
  it("says where the notes are and what to do", () => {
    expect(badgeTitle(1)).toContain("1 rehearsal note");
    expect(badgeTitle(3)).toContain("3 rehearsal notes");
    for (const n of [1, 3]) {
      expect(badgeTitle(n)).toContain("Studio");
      expect(badgeTitle(n).toLowerCase()).toContain("annotation");
    }
  });
});

// ---- ⟨D1⟩ + ⟨D4⟩ as source guards -----------------------------------------------------

import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const read = (rel: string) => readFileSync(join(HERE, rel), "utf8");

describe("T173 — where the badge lives, and what it may not do", () => {
  // ⟨D4⟩ "The list may never mutate a note … no 'clear from here', no bulk delete on this surface."
  // The badge sits one click from the thing it points at, so adding a delete here would feel helpful
  // and would be exactly wrong: a note is a reference underlay, and the only place to act on it is the
  // editor where you can see what you are about to discard.
  it("the song list calls nothing that mutates a note", () => {
    const src = read("../src/pages/BandDetail.tsx");
    expect(src, "the list must read the aggregate").toContain("bandRehearsalNoteCounts");
    for (const forbidden of ["deleteRehearsalNote", "putRehearsalNote", "rehearsalNoteUrl"]) {
      expect(src, `the song list must not call ${forbidden} — ⟨D4⟩ is read-only, always`).not.toContain(
        forbidden,
      );
    }
  });

  // ⟨D1⟩ "The band library list, not the setlist page … a setlist is the wrong frame and would put a
  // nag in front of someone getting ready to play." A setlist is performance prep; recopying is editing.
  //
  // This WALKS src/pages rather than naming the two files it must stay out of. A hand-listed pair is
  // complete today and silently incomplete the day a third song-listing surface appears — the same
  // enumeration rot the ⟨D4⟩ guard above avoids by asserting its positive case first. So: exactly one
  // page may carry the marker, it must be BandDetail, and the walk has to have seen a plausible number
  // of files or it is proving nothing.
  it("exactly one page surface carries the badge, and it is the band library", () => {
    const dir = join(HERE, "../src/pages");
    const pages = readdirSync(dir).filter((f) => f.endsWith(".tsx"));
    expect(pages.length, "the page walk found almost nothing — it is looking in the wrong place").toBeGreaterThan(8);
    const carriers = pages.filter((f) => {
      const src = readFileSync(join(dir, f), "utf8");
      return src.includes("song-note-badge") || src.includes("bandRehearsalNoteCounts");
    });
    expect(carriers, "⟨D1⟩: the badge belongs to the band library and nowhere else").toEqual([
      "BandDetail.tsx",
    ]);
  });

  // ⟨D6⟩'s other half: when there are NO badges the explanation has nowhere to live, because the tooltip
  // is on the badge. Three states render as an unbadged list — nothing waiting, nothing SENT, and "we
  // could not check" — and only the first means there is no work. The line must distinguish them.
  it("the zero-state line names where notes are counted, and admits a failed lookup", () => {
    const src = read("../src/pages/BandDetail.tsx");
    expect(src).toContain('data-testid="note-scope-note"');
    // the failed branch must exist and must not claim there is nothing waiting
    expect(src).toMatch(/noteFetch === "failed"/);
    expect(src).toMatch(/Couldn’t check for rehearsal notes/);
    // and both non-failed wordings have to say that a tablet-held note is not counted here
    const tabletMentions = src.match(/still on a tablet isn’t counted here/g) ?? [];
    expect(tabletMentions.length, "both the empty and the non-empty wording must say it").toBe(2);
  });
});
