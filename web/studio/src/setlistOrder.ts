// T127 — order a band's concerts for the list: the NEXT gig first, then undated drafts, then the
// past most-recent-first, with the past kept separate so the page can head it "Past".
//
// The comparison is on the date-only "YYYY-MM-DD" strings, NOT on Date objects. `new Date("2026-09-05")`
// parses as UTC midnight, so in UTC+2 a `date < new Date()` test would call a concert "past" from
// 02:00 local on the morning of the gig. Lexicographic string compare of ISO dates is chronological
// and timezone-free, and `today` is the viewer's LOCAL date — so the day of the gig stays upcoming.
import type { Setlist } from "./api";

// The viewer's local date as "YYYY-MM-DD" (not UTC). Pass a Date for tests; defaults to now.
export function todayLocal(now: Date = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

// Split into the current list (upcoming ascending, then undated by name) and the past
// (descending). A concert dated exactly `today` is UPCOMING (past means eventDate < today, strictly).
export function partitionSetlists(
  setlists: Setlist[],
  today: string,
): { current: Setlist[]; past: Setlist[] } {
  const upcoming: Setlist[] = [];
  const undated: Setlist[] = [];
  const past: Setlist[] = [];
  for (const s of setlists) {
    if (!s.eventDate) undated.push(s);
    else if (s.eventDate < today) past.push(s);
    else upcoming.push(s);
  }
  upcoming.sort((a, b) => a.eventDate!.localeCompare(b.eventDate!)); // next gig first
  undated.sort((a, b) => a.name.localeCompare(b.name)); // drafts, by name
  past.sort((a, b) => b.eventDate!.localeCompare(a.eventDate!)); // most recent first
  return { current: [...upcoming, ...undated], past };
}
