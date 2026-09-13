import type { Song } from "../../api";

/**
 * T173 — the song list shows which songs carry a rehearsal note, and puts them first.
 *
 * A rehearsal note exists to be recopied into a real annotation and then deleted (T170 §1). Until this,
 * nothing in Studio said which songs had one waiting, so notes rot — the exact failure T170 is built to
 * prevent. Sorting is what turns the list into the worklist: VLL asked for a badge *"and the ability to
 * filter them, or maybe to show them first because the goal is to remove them"*, and ⟨D2⟩ picks
 * sort-first for that reason — a filter is a mode you must choose to enter, sorting puts the work under
 * your nose. The filter is deferred, not refused.
 *
 * ⟨D6⟩ — what the count MEANS, stated here because a reader of `counts[id]` cannot see it: it is notes
 * sitting in STUDIO. A note the tablet has not sent is invisible (the tablet nags for those, and it is
 * the only surface that can act on them); "Done, remove" clears the badge while the tablet keeps its own
 * copy, because removal never travels back (T170 §7). Zero here means "nothing waiting in Studio".
 */

/** withNotesFirst returns songs reordered so those carrying notes come first, MOST notes first, and
 *  everything else keeps the order it arrived in. It is a stable partition, not a full re-sort: the
 *  list's existing order is somebody's choice (title, or whatever the caller sorted by) and a song with
 *  no notes must not move relative to its neighbours just because another song gained one. */
export function withNotesFirst<T extends { id: string }>(
  songs: readonly T[],
  counts: Record<string, number>,
): T[] {
  const withNotes: T[] = [];
  const rest: T[] = [];
  for (const s of songs) ((counts[s.id] ?? 0) > 0 ? withNotes : rest).push(s);
  // Only the noted group is sorted, and only by count — within an equal count the incoming order stands.
  withNotes.sort((a, b) => (counts[b.id] ?? 0) - (counts[a.id] ?? 0));
  return [...withNotes, ...rest];
}

/** badgeTitle is the hover/aria text. It says where the notes ARE, because "2 notes" on a song list is
 *  ambiguous between the tablet and Studio and only one of the two is actionable from here. */
export function badgeTitle(n: number): string {
  return n === 1
    ? "1 rehearsal note waiting in Studio — open the song to copy it into an annotation"
    : `${n} rehearsal notes waiting in Studio — open the song to copy them into annotations`;
}

/** noteCount totals the aggregate. Used to choose the zero-state wording, so "no badges" can be
 *  reported as a fact rather than left to look identical to a failed lookup. */
export function noteCount(counts: Record<string, number>): number {
  let n = 0;
  for (const v of Object.values(counts)) n += v;
  return n;
}

export type SongLike = Pick<Song, "id">;
