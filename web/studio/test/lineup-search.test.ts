import { describe, it, expect } from "vitest";
import { chipMatches, matchLineupSong, filterLineupSongs, type LineupChip, type LineupSong } from "../src/pages/lineup-search";

// A sings lead; B plays autoharp; C plays bass (optional) in one song.
const songs: LineupSong[] = [
  { id: "s1", title: "One", tags: ["bossa"], lineup: [{ musicianId: "A", partIds: ["lead", "gtr"] }, { musicianId: "B", partIds: ["ah"] }] },
  { id: "s2", title: "Two", tags: ["samba"], lineup: [{ musicianId: "A", partIds: ["back"] }, { musicianId: "C", partIds: ["bass"], optional: true }] },
  { id: "s3", title: "Three", lineup: [{ musicianId: "B", partIds: ["ah"] }, { musicianId: "C", partIds: ["bass"] }] },
];

describe("line-up chips", () => {
  it("A · Lead vocal matches only songs where A has that part", () => {
    const c: LineupChip = { kind: "musicianPart", musicianId: "A", partId: "lead", label: "A · Lead" };
    expect(filterLineupSongs(songs, [c], []).map((s) => s.id)).toEqual(["s1"]);
  });
  it("A matches any song A appears in", () => {
    expect(filterLineupSongs(songs, [{ kind: "musician", id: "A", label: "A" }], []).map((s) => s.id)).toEqual(["s1", "s2"]);
  });
  it("Autoharp matches any song where anyone plays it", () => {
    expect(filterLineupSongs(songs, [{ kind: "part", id: "ah", label: "AH" }], []).map((s) => s.id)).toEqual(["s1", "s3"]);
  });
  it("without C: C absent OR optional", () => {
    // s1 (C absent) + s2 (C optional) match; s3 (C required) does not.
    expect(filterLineupSongs(songs, [{ kind: "withoutMusician", id: "C", label: "C" }], []).map((s) => s.id)).toEqual(["s1", "s2"]);
  });
  it("chips AND with tags and each other (VLL's two-chip example)", () => {
    const chips: LineupChip[] = [
      { kind: "musicianPart", musicianId: "A", partId: "lead", label: "A · Lead" },
      { kind: "part", id: "gtr", label: "Gtr" },
    ];
    expect(filterLineupSongs(songs, chips, []).map((s) => s.id)).toEqual(["s1"]);
    // + a tag chip AND a loose word
    expect(filterLineupSongs(songs, [{ kind: "tag", value: "bossa" }], ["one"]).map((s) => s.id)).toEqual(["s1"]);
    expect(chipMatches(songs[1], { kind: "tag", value: "bossa" })).toBe(false);
  });
});
