# Roadmap evaluation — what's nice to do next (Fable, 2026-07-20, for VLL)

**Context I'm evaluating against.** The objective as demonstrated by every request
so far: a **self-hosted band tool VLL's own band actually uses** — Studio to author
(scores, charts, annotations, cues), Stage to perform (offline-first, zero-ceremony,
big-touch-targets), one server artifact a bandleader can run, a trusted-band model
(declutter, not privacy). The working mode: VLL steers by *living with the product
on real devices* and firing short feedback bursts; lanes execute autonomously;
design decisions route through the gate. That mode rewards work that (a) survives
real-stage conditions and (b) shortens the loop from "VLL notices" to "landed".

## Ranked: what I'd do next, and why

**1. T09 — wire the proto codegen (retire the hand-written mirrors).** The
evidence this week is decisive: three real bugs were exactly the hand-mirror class
— the `/api/me` `{"user":…}` wrapper mis-parse (which silently disabled P205
auto-match), the REST-vs-WS type-map divergence T51's e2e caught, and the general
AUTHORITY-comment discipline holding by review alone. Every future proto field
(and P205 added five) multiplies the exposure. `buf generate` for Go/TS/Kotlin
turns a recurring bug class into a compile error. This is the single
highest-leverage engineering item on the board. (Web-core, M; the vectors/glyph
drift-guards show the repo already likes generated-and-guarded contracts.)

**2. A "stage-worthiness" hardening pass — the sacred path under hostile
conditions.** The product's whole point is the 90 minutes on stage. Deliberately
drill: airplane mode mid-set, process death and restore *into the same page*
(nav survives now; Stage position should too), battery-saver throttling, a
20-song bundle's open time, WS reconnect storms during rehearsal live mode. Output:
a checklist like ACCEPTANCE-P205 + fixes for whatever falls out. The A19/A21/A22/
#23 arc shows every real-device drill finds one true bug. (Mobile, M, mostly
scaffolding that pays forever.)

**3. Chord transposition for text charts.** The first *musician-value* feature I'd
add rather than infrastructure: T19 charts already parse chords (the highlight
layer proves it); capo/key changes are weekly reality for a covers band
("Wonderwall, capo 2"). Transpose-on-view in Stage (per-member, like cues — a
personal semitone offset) + transpose-on-edit in Studio. Rides the per-member
metadata rails P205 built. (Cross-lane, M — and it's a feature VLL will *feel* at
the next rehearsal.)

**4. The five-minute-bandmate onboarding.** The pieces all exist now — invites,
the QR/app popover, Connect, auto-identity. Fuse them: ONE QR on the band page
that encodes server + invite so a new member scans once → installs → opens the
app pre-pointed at the server with the invite pre-filled → picks a display name →
sees their parts. Today it's four manual steps with typing an IP on a phone.
(Cross-lane, M; turns the demo into a party trick.)

**5. Sync-under-truth audit (smaller).** The outbox/reconnect paths exist but the
week's lesson ("a cookie is not a session") suggests auditing the WS layer's
staleness assumptions the same way: expiry mid-session, clock skew, double-apply
on reconnect. Mostly tests. (Web-core, S/M.)

**Also queued but low:** T24 (chartpdf/mkcharts convergence — now dispatched),
P204 compaction (wait for disk pressure), T45/N4(b)/T52-p2 (wait for VLL's
appetite), multi-band ergonomics (his reality is one band).

**The strategic gap I can't schedule: iOS.** Half the world's musicians hold
iPads. Everything since A27 (commonMain nav, shared Stage, the vectors) has been
quietly keeping the door open; the blocker is purely a Mac + credentials. When
that unblocks, the app work is genuinely "host wiring", not a rewrite — the
architecture is already paying for this.

## My recommendation

Do **1 + 2 in parallel** (different lanes, both are insurance the product now
deserves), then **3** as the next thing VLL demos to the band, with **4** right
behind it. I'll spec whichever you green-light.
