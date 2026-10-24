import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import {
  buildGrid,
  buildGridByPart,
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
    const voiceSubs = aCol.subs.filter((s) => s.kind === "voice");
    expect(voiceSubs).toHaveLength(1); // one mic, one column
    expect(g.rows[0].cells[voiceKey("A")].text).toBe("L");
    expect(g.rows[1].cells[voiceKey("A")].text).toBe("B");
  });

  it("several voice parts in one song join on the single channel (L+B)", () => {
    const songs: GridSong[] = [{ id: "s1", title: "One", lineup: [{ musicianId: "A", partIds: ["lead", "back"] }] }];
    const g = buildGrid(songs, musicians, parts);
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
    expect(aCol?.subs.some((s) => s.kind === "inst" && s.partId === "bass")).toBeFalsy();
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
    expect(aCol.subs.some((s) => s.kind === "inst" && s.partId === "gtr")).toBe(true);
    // The sheet projection ignores the persisted list (used-only).
    const sheet = buildGrid(songs, withGtr, parts);
    const aSheet = sheet.columns.find((c) => c.id === "A")!;
    expect(aSheet.subs.some((s) => s.kind === "inst" && s.partId === "gtr")).toBe(false);
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
describe("shared projection vectors (guards TS editor ↔ Go sheet)", () => {
  type Vec = {
    cases: {
      name: string;
      musicians: { id: string; name: string; order?: number }[];
      parts: { id: string; name: string; kind: string; short?: string; shared?: boolean; order?: number }[];
      songs: { id: string; lineup: { musicianId: string; partIds: string[] }[] }[];
      expect: { subcols: string[]; cells: Record<string, Record<string, string>> };
    }[];
  };
  const vec: Vec = JSON.parse(readFileSync(new URL("../../../testdata/lineup-sheet-vectors.json", import.meta.url), "utf8"));

  for (const c of vec.cases) {
    it(c.name, () => {
      const musicians: GridMusician[] = c.musicians.map((m) => ({ id: m.id, displayName: m.name, order: m.order }));
      const parts: GridPart[] = c.parts.map((p) => ({ id: p.id, name: p.name, kind: p.kind as GridPart["kind"], short: p.short, shared: p.shared, order: p.order }));
      const songs: GridSong[] = c.songs.map((s) => ({ id: s.id, title: s.id, lineup: s.lineup }));
      const g = buildGrid(songs, musicians, parts); // SHEET mode (used-only), to match the Go projection
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
