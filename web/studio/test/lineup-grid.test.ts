import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import {
  buildGrid,
  buildGridByPart,
  inferPartKind,
  voiceKey,
  instKey,
  sharedKey,
  type GridPart,
  type GridMusician,
  type GridSong,
} from "../src/pages/lineup-grid";

// Placeholder band (A/B/C) — never real names.
const A: GridMusician = { id: "A", displayName: "Ann" };
const B: GridMusician = { id: "B", displayName: "Bo" };
const C: GridMusician = { id: "C", displayName: "Cy" };

const lead: GridPart = { id: "lead", name: "Lead vocal", kind: "voice", short: "L", input: "mic" };
const back: GridPart = { id: "back", name: "Backing vocal", kind: "voice", short: "B", input: "mic" };
const gtr: GridPart = { id: "gtr", name: "Guitar", kind: "instrument", short: "Gtr", input: "di" };
const ah: GridPart = { id: "ah", name: "Autoharp", kind: "instrument", short: "AH", input: "mic" };
const bass: GridPart = { id: "bass", name: "Bass", kind: "instrument", short: "Bs", input: "di", shared: true };

const musicians = [A, B, C];
const parts = [lead, back, gtr, ah, bass];

describe("buildGrid — the review checklist", () => {
  it("lead↔backing for one person is ONE Voice column; the letter changes row to row", () => {
    const songs: GridSong[] = [
      { id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["lead"] }] },
      { id: "s2", title: "Two", lineup: [{ musicianId: "A", partIds: ["back"] }] },
    ];
    const g = buildGrid(songs, musicians, parts);
    const aCol = g.columns.find((c) => c.id === "A")!;
    const voiceSubs = aCol.subs.filter((s) => s.kind === "channel" && s.channelKey === "voice");
    expect(voiceSubs).toHaveLength(1); // one mic, one column
    expect(g.rows[0].cells[voiceKey("A")].text).toBe("L");
    expect(g.rows[1].cells[voiceKey("A")].text).toBe("B");
  });

  it("several voice parts in one song join on the single channel, in CHANNEL order not entry order (L+B) [T207]", () => {
    // Enter the two voice parts BACKING-FIRST — the opposite of the vocabulary (channel) order.
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["back", "lead"] }] }];
    const g = buildGrid(songs, musicians, parts);
    const aCol = g.columns.find((c) => c.id === "A")!;
    const voiceSub = aCol.subs.find((s) => s.kind === "channel" && s.channelKey === "voice")!;
    const channelOrder = voiceSub.kind === "channel" ? voiceSub.partIds : [];
    // Guard: the fixture's entry order really CONTRADICTS the channel order, so a canonical-order regression
    // can't pass by luck (and an edit that enters lead-first would trip this guard, not silently pass).
    expect(songs[0].lineup![0].partIds).toEqual(["back", "lead"]);
    expect(channelOrder).toEqual(["lead", "back"]);
    expect(songs[0].lineup![0].partIds).not.toEqual(channelOrder);
    // The cell reads canonical "L+B", never entry-order "B+L".
    expect(g.rows[0].cells[voiceKey("A")].text).toBe("L+B");
  });

  it("a shared instrument moving between people is ONE shared column; the cell shows who", () => {
    const songs: GridSong[] = [
      { id: "s1", title: "One", lineup: [{ musicianId: "C", partIds: ["bass"] }] },
      { id: "s2", title: "Two", lineup: [{ musicianId: "A", partIds: ["bass"] }] },
    ];
    const g = buildGrid(songs, musicians, parts);
    const sharedCols = g.columns.filter((c) => c.kind === "shared");
    expect(sharedCols).toHaveLength(1);
    expect(sharedCols[0].id).toBe("bass");
    expect(g.rows[0].cells[sharedKey("bass")].text).toBe("Cy");
    expect(g.rows[1].cells[sharedKey("bass")].text).toBe("Ann");
    // bass is NOT a per-person sub-column (it's shared).
    const aCol = g.columns.find((c) => c.id === "A");
    expect(aCol?.subs.some((s) => s.kind === "channel" && s.partIds.includes("bass"))).toBeFalsy();
  });

  it("one person with two instruments in a song fills both cells", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr", "ah"] }] }];
    const g = buildGrid(songs, musicians, parts);
    expect(g.rows[0].cells[instKey("A", "gtr")].text).toBe("Gtr");
    expect(g.rows[0].cells[instKey("A", "ah")].text).toBe("AH");
  });

  it("a sub-column unused across the shown songs does not appear", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }] }];
    const g = buildGrid(songs, musicians, parts);
    expect(g.columns.map((c) => c.id)).toEqual(["A"]); // only Ann; B, C, and the bass column are absent
    const aCol = g.columns[0];
    expect(aCol.subs.map((s) => s.header)).toEqual(["Gtr"]); // no Voice sub-column (Ann never sings here)
  });

  it("optional parts are flagged on the cell", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"], optional: true }] }];
    const g = buildGrid(songs, musicians, parts);
    expect(g.rows[0].cells[instKey("A", "gtr")].optional).toBe(true);
  });

  it("musician column order follows explicit order, then first appearance", () => {
    const ordered = [{ ...A, order: 2 }, { ...B, order: 1 }, C];
    const songs: GridSong[] = [
      { id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }, { musicianId: "B", partIds: ["gtr"] }] },
    ];
    const g = buildGrid(songs, ordered, parts);
    expect(g.columns.map((c) => c.id)).toEqual(["B", "A"]); // B's order=1 before A's order=2
  });
});

describe("buildGrid — shared part with two players (ruling 4)", () => {
  it("shows BOTH players joined in the one song, not a silent first-only", () => {
    const songs: GridSong[] = [
      { id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["bass"] }, { musicianId: "B", partIds: ["bass"] }] },
    ];
    const g = buildGrid(songs, musicians, parts);
    const sharedCols = g.columns.filter((c) => c.kind === "shared");
    expect(sharedCols).toHaveLength(1);
    expect(g.rows[0].cells[sharedKey("bass")].text).toBe("Ann+Bo"); // both, joined — not just "Ann"
  });
});

describe("buildGrid editor mode", () => {
  it("shows a PERSISTED instrument column even with zero usage (ruling 1 — no vanish on toggle-off)", () => {
    const withGtr = [{ ...A, instruments: ["gtr"] }, B, C];
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["lead"] }] }];
    // A sings, plays guitar in NO song, but lists it as an instrument.
    const g = buildGrid(songs, withGtr, parts, { editor: true });
    const aCol = g.columns.find((c) => c.id === "A")!;
    expect(aCol.subs.some((s) => s.kind === "channel" && s.channelKey === "p:gtr")).toBe(true);
    // The sheet projection ignores the persisted list (used-only).
    const sheet = buildGrid(songs, withGtr, parts);
    const aSheet = sheet.columns.find((c) => c.id === "A")!;
    expect(aSheet.subs.some((s) => s.kind === "channel" && s.channelKey === "p:gtr")).toBe(false);
  });
  it("shows every roster musician (even one who plays nothing) so the band can be filled", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }] }];
    const g = buildGrid(songs, musicians, parts, { editor: true });
    expect(g.columns.filter((c) => c.kind === "musician").map((c) => c.id).sort()).toEqual(["A", "B", "C"]);
  });
  it("gives each musician a Voice sub-column when the band has any voice part", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }] }];
    const g = buildGrid(songs, musicians, parts, { editor: true });
    expect(g.rows[0].cells[voiceKey("B")]).toBeDefined(); // B can be given a voice even though they sing nothing yet
  });
  it("shows every shared part's column so one can be assigned", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }] }];
    const g = buildGrid(songs, musicians, parts, { editor: true });
    expect(g.columns.some((c) => c.kind === "shared" && c.id === "bass")).toBe(true);
  });
  it("the sheet projection (no options) still omits unused musicians + shared columns", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["gtr"] }] }];
    const g = buildGrid(songs, musicians, parts);
    expect(g.columns.map((c) => c.id)).toEqual(["A"]);
  });
});

describe("buildGridByPart", () => {
  it("columns are parts, cells are the players", () => {
    const songs: GridSong[] = [
      { id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["lead"] }, { musicianId: "B", partIds: ["bass"] }] },
    ];
    const g = buildGridByPart(songs, musicians, parts);
    const leadCol = g.columns.find((c) => c.partId === "lead")!;
    expect(g.rows[0].cells[leadCol.partId]).toBe("Ann");
    expect(g.rows[0].cells[sharedKeyPart(g, "bass")]).toBe("Bo");
  });
});

// helper: a by-part column is keyed by partId directly
function sharedKeyPart(g: ReturnType<typeof buildGridByPart>, partId: string): string {
  return g.columns.find((c) => c.partId === partId)!.partId;
}

// The ONE shared projection vector set (Fable, 2026-10-24): this TS editor projection (sheet mode) and the Go
// sheet projection (core/internal/app TestSheetProjection_SharedVectors) both load this file and must agree.
describe("T203-FIX2 — inferPartKind matches whole folded WORDS (FR + EN)", () => {
  // Fable's cases — the old substring version is red on "Lead guitar" and "Harmonica".
  const voice = ["Chant lead", "Backing vocals", "Chœurs", "Voix", "Lead", "Voice", "Harmonies", "choir", "Chant", "Choeurs"];
  const inst = ["Lead guitar", "Harmonica", "Guitare", "Guitar", "Bass", "Basse", "Autoharp", "Keys", "Clavier", "Percussion", "Drums", "Batterie", "Shaker"];
  for (const n of voice) it(`"${n}" → voice`, () => expect(inferPartKind(n)).toBe("voice"));
  for (const n of inst) it(`"${n}" → instrument`, () => expect(inferPartKind(n)).toBe("instrument"));
});

describe("T203-FIX2 — the editor column order is stable under cell edits", () => {
  // A zero-order roster + vocabulary (the case that broke): the tie-break must be the ROSTER/VOCAB index, not
  // usage, so toggling cells never reorders the wide columns.
  const M: GridMusician[] = [
    { id: "A", displayName: "Ann" },
    { id: "B", displayName: "Bo" },
    { id: "C", displayName: "Cy" },
  ];
  const P: GridPart[] = [
    { id: "lead", name: "Lead vocal", kind: "voice", short: "L" },
    { id: "bass", name: "Bass", kind: "instrument", short: "Bs", shared: true },
    { id: "bongo", name: "Bongos", kind: "instrument", short: "Bgo", shared: true },
  ];
  const colIDs = (songs: GridSong[]) => buildGrid(songs, M, P, { editor: true }).columns.map((c) => c.id);
  const baseline = colIDs([]); // [A, B, C, bass, bongo] — roster then vocab order

  it("a musician used only in a LATE song keeps its roster position (the exact bug)", () => {
    // Ann (roster-first) sings only in song 5; under a usage tie-break she'd jump to the end.
    const songs: GridSong[] = Array.from({ length: 6 }, (_, i) => ({ id: `s${i}`, title: `s${i}`, lineup: i === 5 ? [{ musicianId: "A", partIds: ["lead"] }] : [] }));
    expect(colIDs(songs)).toEqual(baseline);
  });

  it("200 random cell toggles never change the column order", () => {
    let seed = 12345;
    const rnd = (n: number) => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) % n);
    const songs: GridSong[] = Array.from({ length: 5 }, (_, i) => ({ id: `s${i}`, title: `s${i}`, lineup: [] as GridSong["lineup"] }));
    for (let it = 0; it < 200; it++) {
      const s = songs[rnd(songs.length)];
      const mid = M[rnd(M.length)].id;
      const pid = P[rnd(P.length)].id;
      const lineup = (s.lineup ?? []).map((e) => ({ ...e, partIds: [...e.partIds] }));
      let e = lineup.find((x) => x.musicianId === mid);
      if (!e) { e = { musicianId: mid, partIds: [] }; lineup.push(e); }
      e.partIds = e.partIds.includes(pid) ? e.partIds.filter((p) => p !== pid) : [...e.partIds, pid];
      s.lineup = lineup.filter((x) => x.partIds.length > 0);
      expect(colIDs(songs)).toEqual(baseline);
    }
  });
});

describe("shared projection vectors (guards TS editor ↔ Go sheet)", () => {
  type Vec = {
    cases: {
      name: string;
      musicians: { id: string; name: string; order?: number }[];
      parts: { id: string; name: string; kind: string; short?: string; shared?: boolean; order?: number; channelId?: string }[];
      channels?: { id: string; name: string; order?: number }[];
      songs: { id: string; lineup: { musicianId: string; partIds: string[] }[] }[];
      expect: { subcols: string[]; cells: Record<string, Record<string, string>> };
    }[];
  };
  const vec: Vec = JSON.parse(readFileSync(new URL("../../../testdata/lineup-sheet-vectors.json", import.meta.url), "utf8"));

  for (const c of vec.cases) {
    it(c.name, () => {
      const musicians: GridMusician[] = c.musicians.map((m) => ({ id: m.id, displayName: m.name, order: m.order }));
      const parts: GridPart[] = c.parts.map((p) => ({ id: p.id, name: p.name, kind: p.kind as GridPart["kind"], short: p.short, shared: p.shared, order: p.order, channelId: p.channelId }));
      const songs: GridSong[] = c.songs.map((s) => ({ id: s.id, title: s.id, lineup: s.lineup }));
      const g = buildGrid(songs, musicians, parts, { channels: c.channels }); // SHEET mode (used-only), to match the Go projection
      const keys = g.columns.flatMap((col) => col.subs.map((s) => s.key));
      expect(keys).toEqual(c.expect.subcols);
      for (const row of g.rows) {
        const want = c.expect.cells[row.songId];
        if (!want) continue;
        for (const [key, text] of Object.entries(want)) {
          expect(row.cells[key]?.text ?? "").toBe(text);
        }
      }
    });
  }
});
