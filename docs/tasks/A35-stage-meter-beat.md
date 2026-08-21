# A35 — Stage: beat the song's metre, not an assumed 4/4

**Priority:** normal — **after T86** (which adds `meter` to the song, carries it into the bundle, and
authors the contract change) · **Size:** S–M (grew with the three-tier grid) · **Area:** `app/shared/.../stage`. Sibling of T86.

## What changes

A34 ports T85's contract with `BEATS_PER_BAR = 4`. T86 makes the metre a property of the song and
replaces "pulses per bar" with a **metric grid**, so the Stage half stays small and mechanical:

1. Read `BakedSong.meter` (proto field 12, additive — **absent means 4/4**, which is every bundle
   baked before T86, so old bundles must keep behaving exactly as they do now).
2. Parse it with the same rules T86 pins — a metre resolves to **group lengths in units**: simple →
   `n` ones, compound (`n % 3 == 0 && n > 3`) → `n/3` threes, additive (`3+4/8`) → the groups written
   literally. Lenient: anything unparseable → unset → 4/4.
3. Feed the grid into `beatPhase`. Each unit gets a **tier**: `0` bar (unit 0), `1` felt pulse (a
   group start), `2` free subdivision (everything else).
4. Paint the three tiers — **tier 0 amber `#ffb02e`, tier 1 aqua `#3ee0d4`, tier 2 grey `#6b7a90` at
   ~45% opacity and no glow**, all at the same width. Tier 2 is **muted below 130 ms/unit**.
5. Schedule on the **unit**, not the pulse: uniform groups → `(60000/tempo)/groups[0]`; irregular
   groups → `60000/tempo`. See T86 §4 for why irregular metres make this mandatory rather than
   stylistic.
6. Count-in becomes **2 bars** = `2 × unitsPerBar` units — 8 in 4/4 as today, 6 in 3/4, 12 units
   (4 felt pulses) in 6/8.
7. Label the tempo chip `♩.=NN` compound, `♩=NN` simple, `♪=NN` irregular-additive.

Interactive reference for all of the above, every metre selectable:
<https://claude.ai/code/artifact/50e21132-b37f-46de-95cf-87f7a91d491d>

## Acceptance criteria

- **Runs T86's `docs/contracts/beat-phase.vectors.json` itself** — the same file, not a copy. The
  3/4, 6/8, 12/8 and additive cases T86 adds must pass in Kotlin, and every existing 4/4 vector must
  still pass. This is the whole point of the contract: if the two runtimes ever disagree about when a
  beat is, or what tier it is, a vector fails on one of them.
- A bundle **without** `meter` (i.e. anything baked before T86) beats exactly as it does today:
  8-unit count-in, amber every 4, **no grey unit ever painted**. Assert it — this is the
  no-regression case for every existing concert.
- A 3/4 song counts in 6 and puts the amber on 1; a 6/8 song counts 12 units with amber on 1, aqua on
  4, grey on 2/3/5/6.
- Tier 2 muted below and lit above the 130 ms/unit threshold — assert both sides.
- Device check with screenshots, as A34 did: one 3/4 song (amber on 1, no grey) and **one 6/8 song**,
  which is where the three tiers are actually visible.
- `:shared:check` green; no new deps; nothing persisted (I12 read-only).
- **Do not port T85b's per-frame `getBoundingClientRect` union** — the standing nit from that review.

## Out of scope

- Anything T86 owns (the field, the API, studio UI, the bake, export/import, the vectors themselves).
- Mid-song metre changes and pickup bars — see T86's out-of-scope.
- Inferring a grouping for a plain odd numerator (5/4 stays five ones; `3+2/4` is how a player asks
  for 3+2).
