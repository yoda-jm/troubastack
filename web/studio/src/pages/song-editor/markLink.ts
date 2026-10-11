/**
 * T196 — anchoring stage 3: what Studio SHOWS of a mark's link to its chart line (pure helpers).
 *
 * The server decides every link (T195) and serves its state on each mark — `link.state` is one of VLL's
 * four words, Linked / Check / Lost / Fixed — plus, for a Linked or Check mark, where its linked text is on
 * the current render (`link.at`) and what the line says (`link.text`). Studio never computes a link: it has
 * no manifest. Everything here reads the served object, and nothing here writes anything.
 *
 * Product words (VLL's; a label is a product word — change them only with him):
 *   - the states: Linked / Check / Lost / Fixed;
 *   - the counter: "N to check" — it counts Check AND Lost marks, the ones that need a look;
 *   - the details sentences in `linkDetails`.
 */
import type { AnnotationObject } from "../../api";

export type LinkState = "linked" | "check" | "lost" | "fixed";

/** The served `link.at`: the linked character's top-left on the current render ([0,1] page coords) and
 *  the linked line's height. */
export type LinkAt = { page: number; x: number; y: number; h: number };

/** What Studio shows for one mark. `legacy` is a mark placed before linking existed (an old-style T145
 *  anchor, or no link at all) on a generated chart: shown as Fixed, with its own note. */
export type LinkStatus = { state: LinkState; legacy: boolean; at?: LinkAt; text?: string };

const STATES: ReadonlySet<string> = new Set(["linked", "check", "lost", "fixed"]);

/**
 * The status of a mark, or null when Studio shows nothing for it: a mark on an uploaded file (nothing to
 * link to — `onGeneratedChart` false). A mark on a generated chart with no link (or an unknown state) was
 * placed before linking existed: Fixed, legacy.
 */
export function linkStatus(obj: AnnotationObject, onGeneratedChart: boolean): LinkStatus | null {
  if (!onGeneratedChart) return null;
  const l = obj.link;
  if (!l || !STATES.has(l.state)) return { state: "fixed", legacy: true };
  const st = l.state as LinkState;
  const live = st === "linked" || st === "check";
  return {
    state: st,
    legacy: false,
    at: live && l.at && finiteAt(l.at) ? l.at : undefined,
    text: live ? l.text : undefined,
  };
}

function finiteAt(a: LinkAt): boolean {
  return [a.page, a.x, a.y, a.h].every((v) => typeof v === "number" && Number.isFinite(v));
}

/** Does this status ask the writer to look at the mark? Check and Lost — the counter's population. */
export function needsAttention(s: LinkStatus | null): boolean {
  return s != null && (s.state === "check" || s.state === "lost");
}

/** The link text as quoted in the details: about 32 characters, cut at a word, with an ellipsis. */
export const LINK_TEXT_MAX = 32;
export function truncateLinkText(text: string, max = LINK_TEXT_MAX): string {
  const t = text.replace(/\s+/g, " ").trim();
  if (t.length <= max) return t;
  let cut = t.slice(0, max - 1);
  const sp = cut.lastIndexOf(" ");
  if (sp >= max / 2) cut = cut.slice(0, sp); // a whole word, unless that loses more than half
  return `${cut.replace(/[\s,;:.]+$/, "")}…`;
}

/** The details sentence for a status — VLL's product words, verbatim. */
export function linkDetails(s: LinkStatus): string {
  switch (s.state) {
    case "linked":
      return s.text ? `Linked to “${truncateLinkText(s.text)}”` : "Linked";
    case "check":
      return "Check: the text under this mark changed, or the lines it spans moved apart. Move it to confirm.";
    case "lost":
      return "Lost: the line it was on was deleted. Move it onto the right words to link it again.";
    case "fixed":
      return s.legacy
        ? "Fixed position on the page. Placed before linking existed; move it to link it."
        : "Fixed position on the page.";
  }
}

/** The short state word shown before the sentence (and in the bake list). */
export function stateWord(state: LinkState): string {
  return state === "linked" ? "Linked" : state === "check" ? "Check" : state === "lost" ? "Lost" : "Fixed";
}

/** The counter label: "1 to check", "3 to check". Counts Check + Lost. */
export function attentionLabel(n: number): string {
  return `${n} to check`;
}

/** The marks that need attention, in reading order (page, then top, then left): the order the counter's
 *  clicks cycle through. `bboxOf` gives a mark's box (the editor's objectBBox). */
export function attentionOrder(
  objects: AnnotationObject[],
  onGeneratedChart: boolean,
  bboxOf: (o: AnnotationObject) => { minX: number; minY: number },
): AnnotationObject[] {
  return objects
    .filter((o) => needsAttention(linkStatus(o, onGeneratedChart)))
    .map((o) => ({ o, b: bboxOf(o) }))
    .sort((a, b) => a.o.page - b.o.page || a.b.minY - b.b.minY || a.b.minX - b.b.minX || (a.o.uuid < b.o.uuid ? -1 : 1))
    .map((x) => x.o);
}

/** The next mark to select when the counter is clicked: the one after the current selection in
 *  `order`, wrapping; the first when nothing (or something else) is selected. */
export function nextAttention(order: AnnotationObject[], selectedUuid: string | null): AnnotationObject | null {
  if (order.length === 0) return null;
  const i = selectedUuid == null ? -1 : order.findIndex((o) => o.uuid === selectedUuid);
  return order[(i + 1) % order.length];
}

/**
 * The dot and the thin line, in PIXELS of a page box w × h: the dot centred on the linked character's
 * left edge at mid-line, and the line from the dot to the NEAREST point of the mark's box edge. `end` is
 * null when the dot is inside the box or within `minLen` px of it (a dot on its own mark needs no line).
 */
export function linkTie(
  at: LinkAt,
  box: { minX: number; minY: number; maxX: number; maxY: number },
  w: number,
  h: number,
  minLen = 3,
): { dot: { x: number; y: number }; end: { x: number; y: number } | null } {
  const dot = { x: at.x * w, y: (at.y + at.h / 2) * h };
  const x0 = box.minX * w, x1 = box.maxX * w, y0 = box.minY * h, y1 = box.maxY * h;
  const inside = dot.x >= x0 && dot.x <= x1 && dot.y >= y0 && dot.y <= y1;
  if (inside) return { dot, end: null };
  const end = { x: Math.min(Math.max(dot.x, x0), x1), y: Math.min(Math.max(dot.y, y0), y1) };
  return { dot, end: Math.hypot(end.x - dot.x, end.y - dot.y) < minLen ? null : end };
}

// ---- the "Show links" toggle: per browser, off by default, broken storage = off ----

export const SHOW_LINKS_KEY = "trouba_show_links";

export function readShowLinks(storage: Pick<Storage, "getItem"> | null | undefined): boolean {
  try {
    return storage?.getItem(SHOW_LINKS_KEY) === "1";
  } catch {
    return false;
  }
}

export function writeShowLinks(storage: Pick<Storage, "setItem" | "removeItem"> | null | undefined, on: boolean): void {
  try {
    if (on) storage?.setItem(SHOW_LINKS_KEY, "1");
    else storage?.removeItem(SHOW_LINKS_KEY);
  } catch {
    /* broken storage: the toggle still works for this page, it just isn't remembered */
  }
}

/** window.localStorage, or null where even touching it throws (privacy modes, sandboxed frames). */
export function safeLocalStorage(): Storage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}
