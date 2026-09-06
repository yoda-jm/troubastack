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

export type RunningOrderEntry = {
  kind: string; // "song" | "intermission"
  onCall: boolean; // applies only to a song
};

/** A 1-based running-order number per entry, or null for an entry that carries none (on-call / intermission). */
export function runningOrderNumbers(entries: RunningOrderEntry[]): (number | null)[] {
  let n = 0;
  return entries.map((e) => (e.kind === "song" && !e.onCall ? ++n : null));
}
