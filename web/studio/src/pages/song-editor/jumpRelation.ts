/**
 * P206 ⟨D4⟩ — what a selected jump mark SAYS about itself, and where "go to the other end" leads.
 *
 * The toolbar states the RELATIONSHIP, never a role noun: "Source"/"Destination" are our words for our
 * data model, and what the author needs is what the mark DOES. Two things that buys — the swap button
 * becomes legible (press it and the sentence flips, so the control explains itself), and the same-page
 * pair is finally covered, which until now had a segment and an arrow but nothing naming which end you
 * were holding.
 */
import type { AnnotationObject, AnnotationLayer } from "../../api";

export interface JumpRelation {
  /** The sentence the toolbar prints, and the label of the button that goes there. */
  label: string;
  /** The other end, to scroll to and select. */
  partnerUuid: string;
  /** When set, the affordance is offered but REFUSED, and this says why. */
  disabledReason?: string;
}

/**
 * The relationship of `sel` to its partner, or null when there is nothing to say.
 *
 * Null covers the two states that must stay silent: an ordinary mark (not a jump), and a jump whose
 * partner does not exist — a dangling pointer from an import or a cross-layer delete. The ⟨D2⟩ red flag
 * already owns that second state and must remain the only thing that speaks about it (⟨D4⟩ R4).
 *
 * Visibility gates navigation; EDITABILITY DOES NOT. Going to a mark is read-only, so a partner on a
 * read-only or non-active layer is a perfectly good destination — but a partner on a HIDDEN layer is
 * refused with its reason, because scrolling to something the user cannot see is worse than not moving:
 * the page jumps and there is nothing there. Revealing the layer would be the user's decision, never a
 * side effect of navigation.
 */
export function jumpRelation(
  sel: AnnotationObject,
  fileObjects: readonly AnnotationObject[],
  layersById: ReadonlyMap<string, AnnotationLayer>,
  visible: Readonly<Record<string, boolean>>,
): JumpRelation | null {
  // Guard self-reference: a mark pointing at itself is broken, not a pair (⟨D2⟩ flags it).
  const partner = fileObjects.find(
    (o) => o.uuid !== sel.uuid && (o.uuid === sel.jumpTo || o.jumpTo === sel.uuid),
  );
  if (!partner) return null;

  const outgoing = sel.jumpTo === partner.uuid;
  const samePage = partner.page === sel.page;
  const where = samePage ? "the other mark" : `p.${partner.page + 1}`;
  const label = outgoing ? `Jumps to ${where}` : `Jumped to from ${where}`;

  const layer = layersById.get(partner.layerId);
  if (layer && visible[layer.id] === false) {
    return { label, partnerUuid: partner.uuid, disabledReason: "its other end is on a hidden layer" };
  }
  return { label, partnerUuid: partner.uuid };
}
