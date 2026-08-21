# A35 — Stage: beat the song's metre, not an assumed 4/4

**Priority:** normal — **after T86** (which adds `meter` to the song, carries it into the bundle, and
authors the contract change) · **Size:** S · **Area:** `app/shared/.../stage`. Sibling of T86.

## What changes

A34 ports T85's contract with `BEATS_PER_BAR = 4`. T86 makes pulses-per-bar a property of the song,
so the Stage half is small and mechanical:

1. Read `BakedSong.meter` (proto field 12, additive — **absent means 4/4**, which is every bundle
   baked before T86, so old bundles must keep behaving exactly as they do now).
2. Parse it with the same rules T86 pins: simple → numerator; **compound (`n % 3 == 0 && n > 3`) →
   numerator ÷ 3**, because a player in 6/8 feels two, not six.
3. Feed `beatsPerBar` into `beatPhase`; downbeat becomes `pulseIndex % beatsPerBar == 0`.
4. Count-in becomes **2 bars** (`2 × beatsPerBar`) — 8 in 4/4 as today, 6 in 3/4, 4 in 6/8.
5. Label the tempo chip `♩.=NN` for compound metres, `♩=NN` for simple.

## Acceptance criteria

- **Runs T86's `docs/contracts/beat-phase.vectors.json` itself** — the same file, not a copy. The 3/4
  and 6/8 cases T86 adds must pass in Kotlin, and every existing 4/4 vector must still pass. This is
  the whole point of the contract: if the two runtimes ever disagree about when a beat is, a vector
  fails on one of them.
- A bundle **without** `meter` (i.e. anything baked before T86) beats exactly as it does today: 8-pulse
  count-in, downbeat every 4. Assert it — this is the no-regression case for every existing concert.
- A 3/4 song counts in 6 and puts the downbeat on 1; a 6/8 song counts in 4 pulses, not 12.
- Device check with a screenshot, as A34 did — a 3/4 song showing the amber downbeat on 1 of each bar.
- `:shared:check` green; no new deps; nothing persisted (I12 read-only).

## Out of scope

- Anything T86 owns (the field, the API, studio UI, the bake, export/import, the vectors themselves).
- Mid-song metre changes, pickup bars, odd-metre grouping — see T86's out-of-scope.
