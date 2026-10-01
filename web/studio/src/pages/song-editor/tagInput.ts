// T180 — the pure logic behind the tag chip field. Kept out of the component so the rules that matter —
// a space is data not a delimiter, a trailing comma makes no empty tag, backspace removes exactly one, and
// a fold-match joins the existing spelling instead of minting a second — are unit-testable without a DOM.
import { foldText } from "../../foldText";

export type TagSuggestion = { tag: string; count: number };

/** One entry of the band vocabulary, folded once for matching. */
export type Vocab = TagSuggestion[];

/**
 * Commit the typed buffer as (at most) one new chip. The commit gesture — Enter or comma — is the ONLY
 * delimiter (⟨D1⟩): a comma the user types ends the tag, but a space inside it is kept, so "slow blues"
 * is one tag. Pasting "a, b, c" arrives as a buffer with commas and yields several, in order.
 *
 * Returns the next chip list and the residual buffer (empty on a clean commit; a trailing fragment after
 * the last comma stays in the buffer so typing can continue).
 *
 * `vocab` lets a typed spelling fold to an existing tag (⟨D3⟩): committing "encore" when the band already
 * has "Encore" adds "Encore", never a second spelling. Against the chips already chosen it folds too, so
 * the same tag is never added twice regardless of case.
 */
export function commitBuffer(buffer: string, chosen: string[], vocab: Vocab = []): { tags: string[]; buffer: string } {
  const parts = buffer.split(",");
  // Everything before the last comma is committed; the final segment stays in the buffer (no trailing
  // comma means the whole thing is that final segment and nothing commits yet — the caller commits it on
  // Enter by passing a buffer with no comma and reading the single result).
  const toCommit = parts.slice(0, -1);
  const rest = parts[parts.length - 1];

  const tags = [...chosen];
  for (const raw of toCommit) {
    addOne(tags, raw, vocab);
  }
  return { tags, buffer: rest };
}

/** Commit the whole buffer as one tag (the Enter gesture): no comma splitting, spaces preserved. */
export function commitAll(buffer: string, chosen: string[], vocab: Vocab = []): string[] {
  const tags = [...chosen];
  addOne(tags, buffer, vocab);
  return tags;
}

function addOne(tags: string[], raw: string, vocab: Vocab): void {
  const t = raw.trim();
  if (t === "") return; // a trailing/empty segment makes NO tag (⟨acceptance⟩)
  const folded = foldText(t);
  // Already chosen (any spelling) → no duplicate.
  if (tags.some((c) => foldText(c) === folded)) return;
  // Folds to a band tag → adopt THAT spelling rather than mint a variant (⟨D3⟩).
  const known = vocab.find((v) => foldText(v.tag) === folded);
  tags.push(known ? known.tag : t);
}

/** Backspace on an empty input removes exactly the LAST chip — never the one before it (⟨acceptance⟩). */
export function backspaceLast(tags: string[]): string[] {
  return tags.slice(0, -1);
}

/**
 * Suggestions for the current buffer: band tags whose folded form CONTAINS the folded buffer, minus the
 * ones already chosen, most-used first (vocab is already ordered). `createLabel` is the exact typed text
 * to offer as a new tag — null when the buffer is empty or folds to a tag already offered/chosen, so the
 * "Create" row never duplicates an existing word.
 */
export function suggest(
  buffer: string,
  vocab: Vocab,
  chosen: string[],
): { matches: TagSuggestion[]; createLabel: string | null } {
  const typed = buffer.trim();
  const folded = foldText(typed);
  const chosenFolded = new Set(chosen.map(foldText));

  const matches =
    folded === ""
      ? []
      : vocab.filter((v) => !chosenFolded.has(foldText(v.tag)) && foldText(v.tag).includes(folded));

  const foldsToKnown = vocab.some((v) => foldText(v.tag) === folded);
  const alreadyChosen = chosenFolded.has(folded);
  const createLabel = typed !== "" && !foldsToKnown && !alreadyChosen ? typed : null;

  return { matches, createLabel };
}

/**
 * The cloud (⟨D3⟩ the study's A3): the most-used tags, minus what is already chosen, bounded to `n` unless
 * `showAll`. Bounding is the point — a forty-chip cloud serves nobody; the head of the distribution goes in
 * the cloud, the tail is reached by typing.
 */
export function cloud(vocab: Vocab, chosen: string[], n: number, showAll: boolean): { shown: TagSuggestion[]; hidden: number } {
  const chosenFolded = new Set(chosen.map(foldText));
  const available = vocab.filter((v) => !chosenFolded.has(foldText(v.tag)));
  if (showAll || available.length <= n) return { shown: available, hidden: 0 };
  return { shown: available.slice(0, n), hidden: available.length - n };
}

/** The default cloud size before "show all" (⟨D3⟩ starts at 12). */
export const CLOUD_DEFAULT_N = 12;
