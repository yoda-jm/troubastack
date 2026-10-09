// T181 — the pure song-search rules. Every case is a §7 acceptance line: a word is loose, a chip is strict,
// chips AND, folded spellings count as one, and every count means "songs if you pick it".
import { describe, it, expect } from "vitest";
import {
  caretWord,
  filterSongs,
  filterSummary,
  groupTags,
  queryWords,
  refineStrip,
  removeCaretWord,
  rowPills,
  suggestForWord,
  type SearchSong,
} from "../src/pages/song-search";

// A fixture built to the acceptance's demands: `ro` matches 3 songs, `opener` is a tag on ONE of them, a song
// is tagged `slow blues` (so the chip `blues` must miss it while the word `blues` hits), `encore`/`Encore` are
// two spellings, and `opener` is on more songs than survive a chip (so a band-wide count would be wrong).
const SONGS: SearchSong[] = [
  { id: "a", title: "Rocket", artist: "The Robins", tags: ["opener", "encore"] }, // ro×2 (Rocket, Robins)
  { id: "b", title: "Roar", artist: "Lions", tags: ["opener"] }, //                 ro (Roar)
  { id: "c", title: "Rose", artist: "Garden", tags: ["slow blues", "encore"] }, //  ro (Rose)
  { id: "d", title: "Opus", artist: "Nobody", tags: ["opener", "Encore"] }, //      op (Opus); opener; Encore
  { id: "e", title: "Ballad", artist: "Someone", tags: ["slow blues"] }, //         no ro/op
];

describe("queryWords / matching (⟨D1⟩ loose words, AND)", () => {
  it("splits the box into words; each is a loose substring over title+artist+tags", () => {
    expect(queryWords("  ro   op ")).toEqual(["ro", "op"]);
  });

  it("VLL's sequence: 'ro' → 3 songs; 'ro op' → the songs with BOTH, non-empty", () => {
    const ro = filterSongs(SONGS, [], ["ro"]);
    expect(ro.map((s) => s.id).sort()).toEqual(["a", "b", "c"]); // Rocket, Roar, Rose
    const roOp = filterSongs(SONGS, [], ["ro", "op"]);
    // 'op' matches "Opus" (title) and "opener" (tag). Among the ro-songs, a (opener) and b (opener) carry op.
    expect(roOp.length).toBeGreaterThan(0);
    expect(roOp.map((s) => s.id).sort()).toEqual(["a", "b"]);
  });

  it("word order does not matter: 'op ro' === 'ro op'", () => {
    const x = filterSongs(SONGS, [], ["ro", "op"]).map((s) => s.id).sort();
    const y = filterSongs(SONGS, [], ["op", "ro"]).map((s) => s.id).sort();
    expect(x).toEqual(y);
  });
});

describe("chips are strict and AND (⟨D3⟩)", () => {
  it("the chip 'blues' does NOT match a song tagged only 'slow blues'; the word 'blues' does", () => {
    // fixture really has such a song (c, e tagged "slow blues")
    expect(SONGS.some((s) => (s.tags ?? []).includes("slow blues"))).toBe(true);
    expect(filterSongs(SONGS, ["blues"], [])).toEqual([]); // strict: nothing is tagged exactly "blues"
    expect(filterSongs(SONGS, [], ["blues"]).map((s) => s.id).sort()).toEqual(["c", "e"]); // loose word
  });

  it("two chips AND — only the song carrying both survives, and the fixture has all three cases", () => {
    const only = (p: (s: SearchSong) => boolean) => SONGS.filter(p).length;
    expect(only((s) => (s.tags ?? []).includes("opener") && !(s.tags ?? []).some((t) => t.toLowerCase() === "encore"))).toBeGreaterThan(0); // opener-only (b)
    expect(only((s) => !(s.tags ?? []).includes("opener") && (s.tags ?? []).some((t) => t.toLowerCase() === "encore"))).toBeGreaterThan(0); // encore-only (c)
    expect(only((s) => (s.tags ?? []).includes("opener") && (s.tags ?? []).some((t) => t.toLowerCase() === "encore"))).toBeGreaterThan(0); // both (a, d)
    const both = filterSongs(SONGS, ["opener", "encore"], []).map((s) => s.id).sort();
    expect(both).toEqual(["a", "d"]); // a: opener+encore; d: opener+Encore (folded-equal)
  });
});

describe("folded spellings (⟨D2⟩/§7)", () => {
  it("'encore' on 2 and 'Encore' on 1 → one group, count 3, chip returns 3", () => {
    const enc = groupTags(SONGS).find((g) => g.folded === "encore");
    expect(enc?.songIds.size).toBe(3); // a, c, d
    expect(filterSongs(SONGS, ["encore"], []).map((s) => s.id).sort()).toEqual(["a", "c", "d"]);
  });
});

describe("suggestForWord (⟨D2⟩ count = songs if you pick it)", () => {
  it("'ro op' suggests 'opener' for the typed 'op'", () => {
    const s = suggestForWord(SONGS, [], ["ro", "op"], "op");
    expect(s.some((x) => x.folded === "opener")).toBe(true);
  });

  it("the count is post-pick, not band-wide — with a chip, it equals what picking returns", () => {
    // chip 'encore' first (a,c,d). Typing 'op' → suggestion 'opener'. opener is on a,b,d band-wide (3),
    // but after the encore chip only a and d survive → count MUST be 2, not 3.
    const chips = ["encore"];
    const sugg = suggestForWord(SONGS, chips, ["op"], "op").find((x) => x.folded === "opener");
    expect(sugg?.count).toBe(2);
    // proven by picking it:
    expect(filterSongs(SONGS, ["encore", "opener"], []).map((s) => s.id).sort()).toEqual(["a", "d"]);
  });

  it("excludes tags already chips, and an empty word offers nothing", () => {
    expect(suggestForWord(SONGS, ["opener"], ["op"], "op").some((x) => x.folded === "opener")).toBe(false);
    expect(suggestForWord(SONGS, [], [], "")).toEqual([]);
  });
});

describe("refineStrip (⟨D5⟩)", () => {
  it("with nothing filtering, shows the band's tags, bounded with a hidden count", () => {
    const { shown, hidden } = refineStrip(SONGS, [], 2);
    expect(shown.length).toBe(2);
    expect(hidden).toBeGreaterThan(0);
  });

  it("omits a tag that EVERY listed song carries (picking it narrows nothing)", () => {
    // list only the two 'slow blues' songs (c, e). 'slow blues' is on both → must be omitted.
    const listed = SONGS.filter((s) => s.id === "c" || s.id === "e");
    const strip = refineStrip(listed, []);
    expect(strip.shown.some((t) => t.folded === "slow blues")).toBe(false);
    // 'encore' is on c only (1 of 2) → it DOES narrow, so it is offered with count 1.
    expect(strip.shown.find((t) => t.folded === "encore")?.count).toBe(1);
  });

  it("a strip count equals the listed songs that would remain after clicking it", () => {
    const listed = filterSongs(SONGS, ["encore"], []); // a, c, d
    const opener = refineStrip(listed, ["encore"]).shown.find((t) => t.folded === "opener");
    expect(opener?.count).toBe(2); // a, d among the listed carry opener
  });
});

describe("rowPills (T181 ⟨D6⟩ C1, T188 ⟨D3⟩ natural order)", () => {
  it("a 5-tag song shows exactly 3 pills + hidden 2, in NATURAL A–Z order (not band-count)", () => {
    const song: SearchSong = { id: "z", title: "Z", tags: ["zeta", "opener", "encore", "alpha", "slow blues"] };
    const { shown, hidden } = rowPills(song, [], 3);
    expect(hidden).toBe(2);
    // A–Z: alpha, encore, opener, slow blues, zeta → top 3.
    expect(shown.map((p) => p.label)).toEqual(["alpha", "encore", "opener"]);
    // the OLD band-count rule hid 'alpha' (count 0) and led with opener/encore (count 3); A–Z leads with alpha.
    expect(shown[0].label).toBe("alpha");
  });

  it("T188: ordered by name, not band use — tv (the most-used tag) is NOT first", () => {
    // A band where `tv` is the most-used tag: on 3 songs, vs cartoon 1, s2 1, video-game 2.
    const band: SearchSong[] = [
      { id: "1", title: "A", tags: ["tv", "video-game"] },
      { id: "2", title: "B", tags: ["tv", "s2", "video-game"] },
      { id: "3", title: "C", tags: ["tv", "cartoon"] },
    ];
    const count = (tag: string) => band.filter((s) => (s.tags ?? []).includes(tag)).length;
    expect(count("tv")).toBeGreaterThan(count("video-game")); // assert the fixture: tv is most-used
    expect(count("tv")).toBeGreaterThan(count("cartoon"));

    const song: SearchSong = { id: "2", title: "B", tags: ["tv", "s2", "video-game", "cartoon"] };
    const { shown, hidden } = rowPills(song, [], 3);
    // A–Z: cartoon, s2, tv, video-game → top 3 + 1 hidden. The old rule would have led with tv.
    expect(shown.map((p) => p.label)).toEqual(["cartoon", "s2", "tv"]);
    expect(hidden).toBe(1);
  });

  it("a pill folding-equal to a chip is active; others are not", () => {
    const song: SearchSong = { id: "z", title: "Z", tags: ["opener", "encore"] };
    const { shown } = rowPills(song, ["Opener"], 3); // chip in a different case
    expect(shown.find((p) => p.folded === "opener")?.active).toBe(true);
    expect(shown.find((p) => p.folded === "encore")?.active).toBe(false);
  });
});

describe("caret word + chip commit (⟨D2⟩ replaces only the typed word)", () => {
  it("finds the word under the caret", () => {
    expect(caretWord("ro op", 5).word).toBe("op"); // caret at end
    expect(caretWord("ro op", 2).word).toBe("ro"); // caret after 'ro'
  });
  it("removing the caret word leaves the rest as clean free text", () => {
    expect(removeCaretWord("ro op", 5)).toBe("ro"); // 'op' becomes a chip; 'ro' stays
    expect(removeCaretWord("a op b", 4)).toBe("a b");
  });
});

describe("filterSummary (⟨D4⟩)", () => {
  it("names chips AND'd and the text; empty when nothing filters", () => {
    expect(filterSummary([], "")).toBe("");
    expect(filterSummary(["slow blues", "encore"], "ro")).toBe('Filtering by tags: slow blues AND encore · text: “ro”');
  });
});
