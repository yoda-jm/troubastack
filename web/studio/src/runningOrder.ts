// T158 — the running-order NUMBERING rule, stated once for TroubaStudio.
//
// THE RULE: a number belongs only to a MAIN-ORDER SONG — a song that is NOT on-call and NOT an
// intermission. An on-call (bench) song or an intermission carries no number and NEVER shifts the number of
// the entry after it. So "7" means the 7th main song in the Studio editor, in the Stage drawer, and on the
// printed export sheet alike.
//
// The rule can't be shared as CODE across TS/Kotlin/Go, so all three surfaces run the SAME vectors —
// docs/contracts/running-order-numbering.vectors.json — as a test (see test/running-order-numbering.test.ts)
// so they cannot silently diverge. The display number is DERIVED, never persisted.

/**
 * T153 — the ONE place Studio decides what a setlist entry IS. Absent ⇒ a song, because every entry
 * written before T153 has no kind and must keep its meaning (the additive contract). Call sites must
 * not compare the string themselves, so the default is stated once and cannot drift — the same rule
 * Go states in SetlistItem.IsIntermission and Kotlin in BAKED_KIND_INTERMISSION.
 */
export const KIND_INTERMISSION = "intermission";
export function isIntermission(entry: { kind?: string }): boolean {
  return entry.kind === KIND_INTERMISSION;
}

export type RunningOrderEntry = {
  kind: string; // "song" | "intermission"
  onCall: boolean; // applies only to a song
};

/** A 1-based running-order number per entry, or null for an entry that carries none (on-call / intermission). */
export function runningOrderNumbers(entries: RunningOrderEntry[]): (number | null)[] {
  let n = 0;
  return entries.map((e) => (e.kind === "song" && !e.onCall ? ++n : null));
}

/**
 * The "Main set" count line (T206 follow-up) — SONGS ONLY. An intermission (T153) sits inline in the main set,
 * so a bare length over-counts it (the demo concert read "5 songs" for 4 songs + 1 intermission). Count
 * intermissions separately in Studio's own word; both words pluralise, and the intermission part is dropped
 * when there are none. [entries] are the main-set entries (bench excluded by the caller). A legacy entry with
 * no `kind` reads as a song.
 * Examples: `0 songs`, `1 song`, `4 songs`, `4 songs · 1 intermission`, `4 songs · 2 intermissions`.
 */
export function mainSetCountLabel(entries: { kind?: string }[]): string {
  const intermissions = entries.filter(isIntermission).length;
  const songs = entries.length - intermissions;
  const songPart = `${songs} song${songs === 1 ? "" : "s"}`;
  if (intermissions === 0) return songPart;
  return `${songPart} · ${intermissions} intermission${intermissions === 1 ? "" : "s"}`;
}
