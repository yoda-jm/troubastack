/**
 * T182 — the pure logic behind the band Tags panel: validating a new spelling, the ⟨D3⟩ adopt-existing
 * offer (so a rename cannot mint a third spelling of a tag that already fragments), telling a plain rename
 * from a merge, and wording the consequence the confirmation states in numbers (⟨D5⟩). Kept out of the
 * component so every rule here is unit-pinned; the component only wires these to the dialog and the server.
 */
import { foldText } from "../foldText";

export type TagCount = { tag: string; count: number };

/** A validation error for a typed `to`, or null when it is usable. Mirrors the server's ErrInvalidInput
 * cases (⟨D3⟩): empty after trimming, or containing a comma (T180's tag delimiter). */
export function validateNewTag(to: string): string | null {
  const t = to.trim();
  if (t === "") return "Tag cannot be empty.";
  if (t.includes(",")) return "A tag cannot contain a comma — it is the tag separator.";
  return null;
}

/**
 * adoptOffer returns an EXISTING spelling the member should adopt instead of the one they typed: a tag that
 * folds equal to the trimmed `to` but is spelled differently (typing `ENCORE` when `encore` exists). Null
 * when there is none — including when `to` exactly equals an existing spelling (that is a merge, offered
 * plainly, not an adopt) and, crucially, when the only fold-equal spelling is `from` itself: recasing your
 * own tag (`encore` → `Encore`) with no other variant is a plain rename, per ⟨D3⟩, not an adopt. When
 * several OTHER spellings fold equal, the most-used wins (tags are count-desc), steering to the convention.
 */
export function adoptOffer(to: string, tags: TagCount[], from: string): TagCount | null {
  const t = to.trim();
  const f = foldText(t);
  if (f === "") return null;
  let best: TagCount | null = null;
  for (const tc of tags) {
    if (tc.tag === from) continue; // your own spelling is never an "adopt this instead"
    if (tc.tag === t) return null; // another exact match already exists → a merge, not an adopt
    if (foldText(tc.tag) === f) {
      if (best === null || tc.count > best.count) best = tc;
    }
  }
  return best;
}

export type RenamePlan =
  | { kind: "rename"; from: string; to: string; message: string }
  | { kind: "merge"; from: string; to: string; message: string };

/** The minimal song shape planRename needs: just the stored tags, which the band page already holds. */
export type TaggedSong = { tags?: string[] };

/**
 * planRename describes what renaming `from` to `to` will do, counting DISTINCT songs from the store the
 * client holds. It is a MERGE when some song carries the exact spelling `to` (other than `from`); otherwise a
 * plain rename. The message is the consequence in numbers — the only safety this feature has (no undo), so
 * every number is exact, never estimated (Fable, T182 ⟨1⟩): "will be on N" counts the distinct songs carrying
 * `from` OR `to`, so a song carrying BOTH — the merge's own case — is counted once, matching the result,
 * not the fromCount+toCount sum that would double-count it.
 */
export function planRename(from: string, to: string, songs: TaggedSong[]): RenamePlan {
  const carriesFrom = (s: TaggedSong) => (s.tags ?? []).includes(from);
  const carriesTo = (s: TaggedSong) => (s.tags ?? []).includes(to);
  const fromCount = songs.filter(carriesFrom).length;
  const toExists = to !== from && songs.some(carriesTo);
  if (toExists) {
    const union = songs.filter((s) => carriesFrom(s) || carriesTo(s)).length;
    const verb = fromCount === 1 ? "song changes" : "songs change";
    return {
      kind: "merge",
      from,
      to,
      message: `"${from}" will be merged into "${to}": ${fromCount} ${verb}, "${to}" will be on ${union}.`,
    };
  }
  return {
    kind: "rename",
    from,
    to,
    message: `"${from}" will become "${to}" on ${fromCount} ${songWord(fromCount)}.`,
  };
}

/** deleteMessage is the ⟨D5⟩ consequence for removing a spelling from every song that carries it. */
export function deleteMessage(tag: string, count: number): string {
  return `"${tag}" will be removed from ${count} ${songWord(count)}.`;
}

function songWord(n: number): string {
  return n === 1 ? "song" : "songs";
}
