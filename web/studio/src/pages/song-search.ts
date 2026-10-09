// T181 — the song-search matching rules, kept pure so the behaviours that matter are unit-testable without a
// DOM (the same shape as T180's tagInput.ts): a word is a LOOSE substring over title+artist+tags, a chip is a
// STRICT whole-tag match, chips AND together, and folded spellings of one tag count as one.
import { foldText } from "../foldText";
import { compareTagNames } from "./band-tags";

export type SearchSong = { id: string; title: string; artist?: string; tags?: string[] };

/** A song's searchable text — title, artist and every tag — folded once. Words match as substrings of this. */
function songText(s: SearchSong): string {
  return foldText(`${s.title} ${s.artist ?? ""} ${(s.tags ?? []).join(" ")}`);
}

/** A chip keeps a song when one of its tags folds EQUAL to the chip (whole tag, not substring): `blues` as a
 *  chip does not match `slow blues`, though `blues` as a word does. */
function songHasTag(s: SearchSong, folded: string): boolean {
  return (s.tags ?? []).some((t) => foldText(t) === folded);
}

/** The loose words of the box: whitespace-split, empties dropped. Each is its own term (⟨D1⟩). */
export function queryWords(text: string): string[] {
  return text.split(/\s+/).filter(Boolean);
}

/** A song matches when it contains EVERY word (loose, any order) AND carries EVERY chip (strict). */
export function matchSong(s: SearchSong, chips: string[], words: string[]): boolean {
  const text = songText(s);
  for (const w of words) {
    const fw = foldText(w);
    if (fw && !text.includes(fw)) return false;
  }
  for (const c of chips) {
    if (!songHasTag(s, foldText(c))) return false;
  }
  return true;
}

/** The filtered list: chips (strict, AND) then words (loose, AND), in any order. */
export function filterSongs<T extends SearchSong>(songs: T[], chips: string[], words: string[]): T[] {
  return songs.filter((s) => matchSong(s, chips, words));
}

// --- the word under the caret (⟨D2⟩: suggestions are for THAT word only) ---------------------------------

/** The whitespace-delimited run containing the caret, with its bounds in the text. */
export function caretWord(text: string, caret: number): { word: string; start: number; end: number } {
  let start = caret;
  while (start > 0 && !/\s/.test(text[start - 1])) start--;
  let end = caret;
  while (end < text.length && !/\s/.test(text[end])) end++;
  return { word: text.slice(start, end), start, end };
}

/** Remove the caret word from the box (it is becoming a chip), collapsing the gap it leaves so the remaining
 *  words stay one space apart and there is no leading/trailing space. */
export function removeCaretWord(text: string, caret: number): string {
  const { start, end } = caretWord(text, caret);
  return (text.slice(0, start) + text.slice(end)).replace(/\s+/g, " ").trim();
}

// --- folded tag grouping (one entry per folded spelling, label = most-used spelling) --------------------

export type TagGroup = { label: string; folded: string; songIds: Set<string> };

/** Group a song set's tags by folded form: one entry per folded spelling, its label the most-used actual
 *  spelling (ties alphabetical), and the set of songs carrying ANY spelling of it. Display-time only —
 *  storage is never touched (T180's per-spelling cloud stays honest). */
export function groupTags(songs: SearchSong[]): TagGroup[] {
  const raw = new Map<string, { spellings: Map<string, number>; songIds: Set<string> }>();
  for (const s of songs) {
    for (const t of s.tags ?? []) {
      const f = foldText(t);
      if (!f) continue;
      let g = raw.get(f);
      if (!g) {
        g = { spellings: new Map(), songIds: new Set() };
        raw.set(f, g);
      }
      g.spellings.set(t, (g.spellings.get(t) ?? 0) + 1);
      g.songIds.add(s.id);
    }
  }
  const out: TagGroup[] = [];
  for (const [folded, g] of raw) {
    const label = [...g.spellings.entries()].sort((a, b) => b[1] - a[1] || (a[0] < b[0] ? -1 : 1))[0][0];
    out.push({ label, folded, songIds: g.songIds });
  }
  return out;
}

export type Suggestion = { label: string; folded: string; count: number };

/** Suggestions for the word being typed (⟨D2⟩): band tags whose folded form CONTAINS the word's folded form,
 *  minus tags already chips, each with the count of songs the list would show IF PICKED — the current chips,
 *  the OTHER words, and this tag. Up to `limit`, most-results first (ties by label). A 0-count tag still
 *  shows (hiding it would read as "this tag does not exist"). An empty word offers nothing. */
export function suggestForWord(
  allSongs: SearchSong[],
  chips: string[],
  allWords: string[],
  typedWord: string,
  limit = 5,
): Suggestion[] {
  const fw = foldText(typedWord);
  if (fw === "") return [];
  const chipsFolded = new Set(chips.map(foldText));
  // the words that will REMAIN in the box after this word becomes a chip
  const otherWords = allWords.filter((w) => foldText(w) !== fw);
  const out: Suggestion[] = [];
  for (const g of groupTags(allSongs)) {
    if (chipsFolded.has(g.folded) || !g.folded.includes(fw)) continue;
    const count = filterSongs(allSongs, [...chips, g.label], otherWords).length;
    out.push({ label: g.label, folded: g.folded, count });
  }
  out.sort((a, b) => b.count - a.count || compareTagNames(a.label, b.label));
  return out.slice(0, limit);
}

/** The refine strip (⟨D5⟩): tags carried by at least one LISTED song, each counted over the listed songs
 *  carrying it (the "if you pick it" count), excluding tags already chips and tags carried by EVERY listed
 *  song (picking those narrows nothing). Most-common first, ties by label, bounded to `limit` with the rest
 *  as `hidden`. Empty when there is nothing that would narrow. */
export function refineStrip(
  listed: SearchSong[],
  chips: string[],
  limit = 8,
): { shown: Suggestion[]; hidden: number } {
  const chipsFolded = new Set(chips.map(foldText));
  const candidates: Suggestion[] = [];
  for (const g of groupTags(listed)) {
    if (chipsFolded.has(g.folded)) continue;
    const count = g.songIds.size;
    if (count >= listed.length) continue; // carried by every listed song → a no-op
    candidates.push({ label: g.label, folded: g.folded, count });
  }
  candidates.sort((a, b) => b.count - a.count || compareTagNames(a.label, b.label));
  if (candidates.length <= limit) return { shown: candidates, hidden: 0 };
  return { shown: candidates.slice(0, limit), hidden: candidates.length - limit };
}

export type RowPill = { label: string; folded: string; active: boolean };

/** A song's tags as row pills (T181 ⟨D6⟩ C1), ordered by NATURAL NAME order (T188 ⟨D3⟩: compareTagNames) —
 *  still one global order, so the same tag sits in the same place row to row (what band-count order gave),
 *  but a tag family reads in family order. Bounded to `max` with the rest as `hidden`; a pill is `active`
 *  when it folds-equal to a current chip.
 *  (T188: the hidden pill behind `+N` is now the alphabetically last, not the least used — accepted; a
 *  width-based cap is a separate task.) */
export function rowPills(
  song: SearchSong,
  chips: string[],
  max = 3,
): { shown: RowPill[]; hidden: number } {
  const chipsFolded = new Set(chips.map(foldText));
  const pills: RowPill[] = (song.tags ?? []).map((t) => {
    const folded = foldText(t);
    return { label: t, folded, active: chipsFolded.has(folded) };
  });
  pills.sort((a, b) => compareTagNames(a.label, b.label));
  if (pills.length <= max) return { shown: pills, hidden: 0 };
  return { shown: pills.slice(0, max), hidden: pills.length - max };
}

/** The "what is filtering" line (⟨D4⟩): chips AND'd, then the free text. Empty when nothing filters. */
export function filterSummary(chips: string[], text: string): string {
  const parts: string[] = [];
  if (chips.length > 0) parts.push(`tags: ${chips.join(" AND ")}`);
  const t = text.trim();
  if (t !== "") parts.push(`text: “${t}”`);
  return parts.length ? `Filtering by ${parts.join(" · ")}` : "";
}
