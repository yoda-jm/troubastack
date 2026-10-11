// T196 — anchoring stage 3: the pure helpers behind what Studio shows of a mark's link.
import { describe, it, expect } from "vitest";
import {
  attentionLabel,
  attentionOrder,
  linkDetails,
  linkStatus,
  linkTie,
  needsAttention,
  nextAttention,
  readShowLinks,
  SHOW_LINKS_KEY,
  stateWord,
  truncateLinkText,
  writeShowLinks,
} from "../src/pages/song-editor/markLink";
import type { AnnotationLink, AnnotationObject } from "../src/api";

function mark(uuid: string, link?: AnnotationLink, over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid,
    layerId: "L1",
    type: "rect",
    points: [
      { x: 0.2, y: 0.4 },
      { x: 0.3, y: 0.45 },
    ],
    page: 0,
    text: "",
    order: 0,
    createdAt: 0,
    style: { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.02 },
    ...(link ? { link } : {}),
    ...over,
  };
}

const AT = { page: 0, x: 0.1, y: 0.2, h: 0.02 };

describe("linkStatus", () => {
  it("is nothing at all on an uploaded file, whatever the mark carries", () => {
    expect(linkStatus(mark("a", { state: "lost" }), false)).toBeNull();
    expect(linkStatus(mark("b"), false)).toBeNull();
  });
  it("reads the served state; at and text only for a live link (linked, check)", () => {
    expect(linkStatus(mark("a", { state: "linked", at: AT, text: "a line" }), true)).toEqual({
      state: "linked",
      legacy: false,
      at: AT,
      text: "a line",
    });
    expect(linkStatus(mark("c", { state: "check", at: AT }), true)?.at).toEqual(AT);
    const lost = linkStatus(mark("l", { state: "lost", at: AT, text: "x" }), true);
    expect(lost).toEqual({ state: "lost", legacy: false, at: undefined, text: undefined });
    expect(linkStatus(mark("f", { state: "fixed" }), true)).toEqual({ state: "fixed", legacy: false, at: undefined, text: undefined });
  });
  it("shows a mark with no link on a generated chart as Fixed, placed before linking existed", () => {
    expect(linkStatus(mark("old"), true)).toEqual({ state: "fixed", legacy: true });
    // an unknown state from a newer server is not guessed at either
    expect(linkStatus(mark("odd", { state: "weird" as AnnotationLink["state"] }), true)).toEqual({ state: "fixed", legacy: true });
  });
  it("drops a malformed at", () => {
    const bad = { page: 0, x: Number.NaN, y: 0.2, h: 0.02 };
    expect(linkStatus(mark("a", { state: "linked", at: bad }), true)?.at).toBeUndefined();
  });
});

describe("the product words", () => {
  it("are VLL's, verbatim", () => {
    expect(linkDetails({ state: "linked", legacy: false, text: "Pack a little light for the road ahead," })).toBe(
      "Linked to “Pack a little light for the…”",
    );
    expect(linkDetails({ state: "check", legacy: false })).toBe(
      "Check: the text under this mark changed, or the lines it spans moved apart. Move it to confirm.",
    );
    expect(linkDetails({ state: "lost", legacy: false })).toBe(
      "Lost: the line it was on was deleted. Move it onto the right words to link it again.",
    );
    expect(linkDetails({ state: "fixed", legacy: false })).toBe("Fixed position on the page.");
    expect(linkDetails({ state: "fixed", legacy: true })).toBe(
      "Fixed position on the page. Placed before linking existed; move it to link it.",
    );
    expect(["linked", "check", "lost", "fixed"].map((s) => stateWord(s as "linked"))).toEqual(["Linked", "Check", "Lost", "Fixed"]);
    expect(attentionLabel(1)).toBe("1 to check");
    expect(attentionLabel(3)).toBe("3 to check");
  });
  it("truncates the line at about 32 characters, at a word", () => {
    expect(truncateLinkText("short line")).toBe("short line");
    expect(truncateLinkText("exactly thirty-two characters ok")).toHaveLength(32);
    expect(truncateLinkText("exactly thirty-two characters ok")).toBe("exactly thirty-two characters ok");
    const long = truncateLinkText("Pack a little light for the road ahead,");
    expect(long).toBe("Pack a little light for the…");
    expect(long.length).toBeLessThanOrEqual(32);
    // one long word: cut inside it rather than lose everything
    expect(truncateLinkText("a" + "b".repeat(60))).toBe("a" + "b".repeat(30) + "…");
    expect(truncateLinkText("  spaced\tout   text  ")).toBe("spaced out text");
  });
});

describe("the counter", () => {
  const box = (o: AnnotationObject) => ({ minX: o.points[0].x, minY: o.points[0].y });
  const at = (x: number, y: number, page = 0) => ({ points: [{ x, y }, { x: x + 0.05, y: y + 0.02 }], page });
  const objs = [
    mark("lostLow", { state: "lost" }, at(0.2, 0.8)),
    mark("linked", { state: "linked", at: AT }, at(0.2, 0.1)),
    mark("checkTop", { state: "check" }, at(0.5, 0.3)),
    mark("checkLeft", { state: "check" }, at(0.1, 0.3)),
    mark("fixed", { state: "fixed" }, at(0.1, 0.05)),
    mark("legacy", undefined, at(0.1, 0.06)),
    mark("page2", { state: "lost" }, at(0.1, 0.01, 1)),
  ];
  it("counts Check and Lost only — never Linked, Fixed or a mark placed before linking", () => {
    expect(needsAttention(linkStatus(objs[0], true))).toBe(true);
    expect(needsAttention(linkStatus(objs[1], true))).toBe(false);
    expect(needsAttention(linkStatus(objs[4], true))).toBe(false);
    expect(needsAttention(linkStatus(objs[5], true))).toBe(false);
    expect(needsAttention(null)).toBe(false);
  });
  it("orders them for reading: page, then top, then left", () => {
    expect(attentionOrder(objs, true, box).map((o) => o.uuid)).toEqual(["checkLeft", "checkTop", "lostLow", "page2"]);
    expect(attentionOrder(objs, false, box)).toEqual([]);
  });
  it("cycles: first, then the next after the selection, wrapping", () => {
    const order = attentionOrder(objs, true, box);
    expect(nextAttention(order, null)?.uuid).toBe("checkLeft");
    expect(nextAttention(order, "checkLeft")?.uuid).toBe("checkTop");
    expect(nextAttention(order, "page2")?.uuid).toBe("checkLeft");
    expect(nextAttention(order, "linked")?.uuid).toBe("checkLeft"); // something else selected
    expect(nextAttention([], null)).toBeNull();
  });
});

describe("linkTie", () => {
  const W = 1000, H = 1400;
  it("puts the dot on the character's left edge at mid-line, and the line to the NEAREST box edge", () => {
    const t = linkTie({ page: 0, x: 0.1, y: 0.2, h: 0.02 }, { minX: 0.3, minY: 0.25, maxX: 0.4, maxY: 0.3 }, W, H);
    expect(t.dot).toEqual({ x: 100, y: 0.21 * H });
    expect(t.end).toEqual({ x: 300, y: 350 }); // the box's top-left corner is the nearest point
  });
  it("meets a side of the box when the dot is level with it", () => {
    const t = linkTie({ page: 0, x: 0.1, y: 0.25, h: 0.02 }, { minX: 0.3, minY: 0.2, maxX: 0.4, maxY: 0.35 }, W, H);
    expect(t.end).toEqual({ x: 300, y: t.dot.y });
  });
  it("draws no line when the dot is inside the box or right next to it", () => {
    expect(linkTie({ page: 0, x: 0.35, y: 0.26, h: 0.02 }, { minX: 0.3, minY: 0.2, maxX: 0.4, maxY: 0.35 }, W, H).end).toBeNull();
    expect(linkTie({ page: 0, x: 0.299, y: 0.26, h: 0.02 }, { minX: 0.3, minY: 0.2, maxX: 0.4, maxY: 0.35 }, W, H).end).toBeNull();
  });
});

describe("Show links storage", () => {
  const mem = () => {
    const m = new Map<string, string>();
    return {
      getItem: (k: string) => m.get(k) ?? null,
      setItem: (k: string, v: string) => void m.set(k, v),
      removeItem: (k: string) => void m.delete(k),
      m,
    };
  };
  it("is off by default and remembered when turned on", () => {
    const s = mem();
    expect(readShowLinks(s)).toBe(false);
    writeShowLinks(s, true);
    expect(s.m.get(SHOW_LINKS_KEY)).toBe("1");
    expect(readShowLinks(s)).toBe(true);
    writeShowLinks(s, false);
    expect(readShowLinks(s)).toBe(false);
  });
  it("reads broken or missing storage as off, and never throws", () => {
    const broken = {
      getItem: () => {
        throw new Error("denied");
      },
      setItem: () => {
        throw new Error("denied");
      },
      removeItem: () => {
        throw new Error("denied");
      },
    };
    expect(readShowLinks(broken)).toBe(false);
    expect(() => writeShowLinks(broken, true)).not.toThrow();
    expect(readShowLinks(null)).toBe(false);
    expect(() => writeShowLinks(undefined, true)).not.toThrow();
  });
});
