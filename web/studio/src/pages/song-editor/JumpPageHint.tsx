/**
 * P206 — the direction cue for a jump whose partner is on ANOTHER PAGE.
 *
 * The dashed segment only exists when both ends are co-visible, and the cross-page case was deferred at
 * spec time as "a page hint, later". In practice that is the COMMON case — a jump usually goes somewhere
 * else in the chart — so selecting a real jump showed nothing at all: no link, and no way to tell a source
 * from a destination (VLL, 2026-09-09). This is that hint: which way, and to which page.
 *
 * Its own component for the same reason as JumpFlags — it renders off derived state, so it can be tested
 * at the render level without authoring a two-page pair through the UI.
 *
 * WHY THIS SAYS A PAGE NUMBER WHILE STAGE DELIBERATELY DOES NOT (Fable, ⟨GO⟩ 086ea07d). Stage shows no page
 * index on a jump — VLL's ruling at `aced8d3f`: "a jump is a matched SYMBOL pair, so the reader follows the
 * glyph, not a page index". That is not in tension with this: AUTHORING needs to know where a mark points,
 * READING needs to follow a symbol. A page number is a fact about the document, useful at a desk; on a stand
 * mid-song it is a second vocabulary competing with the glyph at the moment there is no attention to spare.
 * So: number here, never there. If you are here to "harmonise" the two surfaces, the cheap direction is to
 * add the number to Stage, and that undoes a ruling — read both notes first.
 */
import { objectBBox, type TextMeasure } from "../../editor";
import type { AnnotationObject } from "../../api";

export interface JumpPageHintItem {
  uuid: string;
  /** true when the SELECTED mark carries the pointer (it jumps away), false when it is the target. */
  outgoing: boolean;
  /** 1-based page of the partner, as a reader counts pages. */
  page: number;
}

/** The hints for the selected marks on this page whose partner is elsewhere in the file. */
export function jumpPageHints(
  selectedOnPage: readonly AnnotationObject[],
  fileObjects: readonly AnnotationObject[],
  thisPage: number,
): JumpPageHintItem[] {
  const out: JumpPageHintItem[] = [];
  for (const sel of selectedOnPage) {
    // Guard self-reference: a mark pointing at itself is broken, not a pair (⟨D2⟩ flags it separately).
    const partner = fileObjects.find(
      (o) => o.uuid !== sel.uuid && (o.uuid === sel.jumpTo || o.jumpTo === sel.uuid),
    );
    if (!partner || partner.page === thisPage) continue; // co-visible pairs get the segment instead
    out.push({ uuid: sel.uuid, outgoing: sel.jumpTo === partner.uuid, page: partner.page + 1 });
  }
  return out;
}

export function JumpPageHints({
  objects,
  hints,
  measure,
}: {
  /** The objects on THIS page (for placing each hint on its mark). */
  objects: AnnotationObject[];
  hints: JumpPageHintItem[];
  measure?: TextMeasure;
}) {
  return (
    <>
      {hints.map((h) => {
        const o = objects.find((x) => x.uuid === h.uuid);
        if (!o) return null;
        const b = objectBBox(o, measure);
        return (
          <div
            key={`hint-${h.uuid}`}
            className="jump-page-hint"
            data-testid="jump-page-hint"
            data-uuid={h.uuid}
            // BELOW the mark, not above it (VLL: "the chip on the selected jumpmark is still hidden by
            // the toolbar"). The selection toolbar is `bottom:100%; left:50%` on the same box and is far
            // wider than a landmark — ~200px of buttons over a ~65px mark — so it overhangs both sides by
            // about a mark's width, and anything placed above the box lands under it. Below is free.
            style={{ left: `${b.maxX * 100}%`, top: `${b.maxY * 100}%` }}
          >
            {h.outgoing ? `→ p.${h.page}` : `← p.${h.page}`}
          </div>
        );
      })}
    </>
  );
}
