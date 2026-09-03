// T127 — the concert partition. The load-bearing vector is a concert dated EXACTLY today: it must be
// UPCOMING, not past. That is where the UTC bug shows — `new Date("2026-09-05") < new Date()` calls
// today's gig "past" from 02:00 local in UTC+2. Teeth-check: swap partitionSetlists' string compare
// for `new Date(s.eventDate) < new Date()` and the "today is upcoming" case fails.
import { describe, it, expect } from "vitest";
import { partitionSetlists, todayLocal } from "../src/setlistOrder";
import type { Setlist } from "../src/api";

function sl(name: string, eventDate?: string): Setlist {
  return { id: name, bandId: "b", name, eventDate, createdAt: "2026-01-01T00:00:00Z" };
}
const TODAY = "2026-09-03";

describe("partitionSetlists (T127)", () => {
  it("a concert dated exactly today is UPCOMING, not past", () => {
    const { current, past } = partitionSetlists([sl("Gig", TODAY)], TODAY);
    expect(current.map((s) => s.name)).toEqual(["Gig"]);
    expect(past).toEqual([]);
  });

  it("yesterday is past, tomorrow is upcoming", () => {
    const { current, past } = partitionSetlists(
      [sl("Yesterday", "2026-09-02"), sl("Tomorrow", "2026-09-04")],
      TODAY,
    );
    expect(current.map((s) => s.name)).toEqual(["Tomorrow"]);
    expect(past.map((s) => s.name)).toEqual(["Yesterday"]);
  });

  it("orders: upcoming ascending (next first), then undated by name; past descending", () => {
    const { current, past } = partitionSetlists(
      [
        sl("Far", "2026-12-01"),
        sl("Soon", "2026-09-10"),
        sl("Draft B"),
        sl("Draft A"),
        sl("Old", "2025-01-01"),
        sl("Recent", "2026-08-01"),
      ],
      TODAY,
    );
    expect(current.map((s) => s.name)).toEqual(["Soon", "Far", "Draft A", "Draft B"]);
    expect(past.map((s) => s.name)).toEqual(["Recent", "Old"]);
  });
});

describe("todayLocal", () => {
  it("formats the LOCAL date as YYYY-MM-DD (from local components, not UTC)", () => {
    // Built from local Y/M/D components, so this holds in any timezone.
    expect(todayLocal(new Date(2026, 8, 3))).toBe("2026-09-03");
    expect(todayLocal(new Date(2026, 0, 9))).toBe("2026-01-09");
  });
});
