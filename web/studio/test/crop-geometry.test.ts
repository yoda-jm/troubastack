// T198 — the crop window's policy-A aspect lock (the rect is a SQUARE in page fractions: w == h). Pure
// geometry whose wrong answers are silent, so a sabotage (e.g. letting w and h move independently) must turn
// one of these red.
import { describe, it, expect } from "vitest";
import {
  squareAt,
  moveRect,
  clampSquareOnPage,
  MIN_SIDE,
  snapAngle,
  sameRect,
  notesNewlyChanged,
  applyPageAngle,
} from "../src/pages/song-editor/cropGeometry";
import type { CropRect, RehearsalNote } from "../src/api";

const isSquare = (r: { w: number; h: number }) => Math.abs(r.w - r.h) < 1e-9;
const onPage = (r: { x: number; y: number; w: number; h: number }) =>
  r.x >= -1e-9 && r.y >= -1e-9 && r.x + r.w <= 1 + 1e-9 && r.y + r.h <= 1 + 1e-9;

describe("crop geometry — policy A aspect lock (w == h)", () => {
  it("squareAt always produces a square, whatever the drag shape", () => {
    const r = squareAt(0.1, 0.1, 0.9, 0.2); // a wide, shallow drag
    expect(isSquare(r)).toBe(true);
    expect(onPage(r)).toBe(true);
    expect(r.w).toBeGreaterThan(0.5); // side follows the larger delta (0.8), clamped to the page
  });

  it("squareAt enforces the minimum side", () => {
    const r = squareAt(0.5, 0.5, 0.505, 0.505); // a tiny drag
    expect(isSquare(r)).toBe(true);
    expect(r.w).toBeGreaterThanOrEqual(MIN_SIDE - 1e-9);
    expect(onPage(r)).toBe(true);
  });

  it("squareAt grows toward the drag direction from the fixed corner", () => {
    const r = squareAt(0.9, 0.9, 0.3, 0.3); // anchored bottom-right, drag up-left
    expect(isSquare(r)).toBe(true);
    expect(r.x + r.w).toBeLessThanOrEqual(0.9 + 1e-9);
    expect(r.y + r.h).toBeLessThanOrEqual(0.9 + 1e-9);
    expect(onPage(r)).toBe(true);
  });

  it("moveRect keeps the square on the page and preserves its side", () => {
    const moved = moveRect({ x: 0.3, y: 0.3, w: 0.4, h: 0.4 }, 0.9, -0.9); // shove past both edges
    expect(moved.w).toBeCloseTo(0.4, 9);
    expect(isSquare(moved)).toBe(true);
    expect(onPage(moved)).toBe(true);
    expect(moved.x + moved.w).toBeCloseTo(1, 9);
    expect(moved.y).toBeCloseTo(0, 9);
  });

  it("clampSquareOnPage keeps a valid square unshrunk and on-page", () => {
    const r = clampSquareOnPage({ x: 0.7, y: 0.7, w: 0.6, h: 0.6 });
    expect(isSquare(r)).toBe(true);
    expect(r.w).toBeCloseTo(0.6, 9);
    expect(onPage(r)).toBe(true);
  });
});

// T199 — the deskew angle + the "rehearsal notes newly page-changed" count. Pure helpers; a sabotage (e.g.
// dropping the angle from sameRect, or counting an already-changed note) turns one of these red.
const rect = (x: number, y: number, w: number, h: number, angle?: number): CropRect =>
  ({ x, y, w, h, ...(angle != null ? { angle } : {}), source: "manual" });
const note = (o: Partial<RehearsalNote>): RehearsalNote => o as RehearsalNote;

describe("T199 snapAngle", () => {
  it("clamps to ±10 and snaps to the 0.05° grid", () => {
    expect(snapAngle(2.33)).toBe(2.35);
    expect(snapAngle(2.32)).toBe(2.3);
    expect(snapAngle(99)).toBe(10);
    expect(snapAngle(-99)).toBe(-10);
    expect(snapAngle(0)).toBe(0);
    expect(snapAngle(Number.NaN)).toBe(0);
  });
});

describe("T199 sameRect", () => {
  it("compares the box AND the angle; absent==absent, 0==absent", () => {
    expect(sameRect(undefined, undefined)).toBe(true);
    expect(sameRect(rect(0, 0, 0.8, 0.8), undefined)).toBe(false);
    expect(sameRect(rect(0, 0, 0.8, 0.8), rect(0, 0, 0.8, 0.8))).toBe(true);
    expect(sameRect(rect(0, 0, 0.8, 0.8, 3), rect(0, 0, 0.8, 0.8))).toBe(false); // angle-only differs
    expect(sameRect(rect(0, 0, 0.8, 0.8, 0), rect(0, 0, 0.8, 0.8))).toBe(true); // 0 == absent
  });
});

describe("T199 notesNewlyChanged", () => {
  const FILE = "f1";
  const saved: Record<string, CropRect> = { "0": rect(0, 0, 0.8, 0.8) };
  const angled: Record<string, CropRect> = { "0": rect(0, 0, 0.8, 0.8, 3) }; // angle-only change
  const recropped: Record<string, CropRect> = { "0": rect(0.1, 0.1, 0.7, 0.7) }; // box change
  const resolved = note({ fileId: FILE, filePage: 0, pageChanged: false });

  it("no notes → 0", () => expect(notesNewlyChanged([], FILE, recropped, saved)).toBe(0));
  it("crop-only change on a resolved note → 1", () =>
    expect(notesNewlyChanged([resolved], FILE, recropped, saved)).toBe(1));
  it("angle-only change on a resolved note → 1", () =>
    expect(notesNewlyChanged([resolved], FILE, angled, saved)).toBe(1));
  it("unchanged page → 0", () => expect(notesNewlyChanged([resolved], FILE, saved, saved)).toBe(0));
  it("unresolved note (no fileId) → 0", () =>
    expect(notesNewlyChanged([note({ pageChanged: false })], FILE, angled, saved)).toBe(0));
  it("already page-changed → 0", () =>
    expect(notesNewlyChanged([note({ fileId: FILE, filePage: 0, pageChanged: true })], FILE, angled, saved)).toBe(0));
  it("note on another file → 0", () =>
    expect(notesNewlyChanged([note({ fileId: "other", filePage: 0, pageChanged: false })], FILE, angled, saved)).toBe(0));
  it("pageChanged null (couldn't check) still counts when the rect changes (!== true)", () =>
    expect(notesNewlyChanged([note({ fileId: FILE, filePage: 0, pageChanged: null })], FILE, angled, saved)).toBe(1));
});

describe("T199 applyPageAngle", () => {
  it("straighten-only: an uncropped page gets a full-page window carrying the angle", () => {
    expect(applyPageAngle(undefined, undefined, 3)).toEqual({ x: 0, y: 0, w: 1, h: 1, source: "manual", angle: 3 });
  });
  it("clearing the angle on a synthetic full-page box that was never saved DROPS the entry", () => {
    expect(applyPageAngle(rect(0, 0, 1, 1, 3), undefined, 0)).toBeNull();
  });
  it("clearing the angle on a real crop keeps the box (no angle), never drops it", () => {
    expect(applyPageAngle(rect(0.1, 0.1, 0.8, 0.8, 3), undefined, 0)).toEqual({ x: 0.1, y: 0.1, w: 0.8, h: 0.8, source: "manual" });
  });
  it("clearing the angle on a 0,0,1,1 the user actually SAVED keeps it (not synthetic)", () => {
    expect(applyPageAngle(rect(0, 0, 1, 1, 3), rect(0, 0, 1, 1), 0)).toEqual({ x: 0, y: 0, w: 1, h: 1, source: "manual" });
  });
});
