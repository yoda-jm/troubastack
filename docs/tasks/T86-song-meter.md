# T86 — Give a song its metre, and make the beat count bars instead of guessing 4/4

**Priority:** high (VLL 2026-08-21, now that the beat works on stage: *"we probably want to have the
metric of the song instead of just tempo… instead of being 8 beats it should be 2 bars"*) ·
**Size:** M · **Area:** `proto`, `core` (model, API, bake, band export), `web/studio` (Details +
the beat + the shared contract + vectors). The Stage half is **A35**.

**Revised 2026-08-21** after VLL's three-tier proposal and the additive-metre note: §2–§4 now specify
a metric **grid** (group lengths + a tier per unit) rather than a pulses-per-bar integer. Prototype
with every metre in §2 selectable: <https://claude.ai/code/artifact/50e21132-b37f-46de-95cf-87f7a91d491d>

## Why now

A11's own source says it out loud — *"Fixed length: two bars of 4/4. **We don't know the real
meter**, and 8 beats counts in ~anything."* That was honest when nothing could know better. Now the
beat is real and visible, and 4/4 is baked into two places that a musician will notice immediately:
the downbeat lands every 4 pulses, and the count-in is 8 pulses. In 3/4 that means the emphasis walks
around the bar and the count-in is two-and-two-thirds bars — worse than no downbeat at all, because
it is confidently wrong.

**There is no metre anywhere in the product today** — not in `proto`, not in `app.Song`, not in the
app (grep-confirmed). The charts *print* "3/4" and "6/8" in their header text, so the information
exists on the page but not in the model.

## Design (decided)

### 1. Store it as the musician writes it

`app.Song` gains `Meter string` (`json:"meter,omitempty"`), holding a canonical `"N/D"` —
`"4/4"`, `"3/4"`, `"6/8"`. A string round-trips through the API, the export manifest and the UI
unchanged, and it is what a player recognises. Parse to `(numerator, denominator)` for logic only.

**Empty = unset = today's behaviour (4/4).** Every existing song keeps its current beat exactly.

**Validation is lenient by design:** accept `N/D` with `N` 1–32 and `D ∈ {1,2,4,8,16}`; anything else
is treated as unset rather than rejected. A typo in the metre must never fail a save or break a gig.

### 2. A metre is a list of group lengths — that one primitive covers everything

Do **not** model this as "numerator, denominator, and a compound flag". Resolve a metre to its
**group lengths in metric units** and every case below falls out of the same rule:

| metre | groups | units/bar | felt pulses |
|---|---|---|---|
| 4/4 | `[1,1,1,1]` | 4 | 4 |
| 3/4 | `[1,1,1]` | 3 | 3 |
| 2/4, 2/2 | `[1,1]` | 2 | 2 |
| 6/8 | `[3,3]` | 6 | 2 |
| 9/8 | `[3,3,3]` | 9 | 3 |
| 12/8 | `[3,3,3,3]` | 12 | 4 |
| 5/4 | `[1,1,1,1,1]` | 5 | 5 |
| **3+2/8** | `[3,2]` | 5 | 2 |
| **3+4/8** | `[3,4]` | 7 | 2 |
| **2+2+3/8** | `[2,2,3]` | 7 | 3 |

Derivation: an **additive** numerator (`3+4`) *is* the grouping, taken literally. Otherwise
**compound** (`n % 3 == 0 && n > 3`) → `n/3` groups of 3; **simple** → `n` groups of 1.

Accept the additive form in the parser **now** (VLL, 2026-08-21: *"for later we can even have
(3+4)/8 or 3+3+1/4 or crazy stuffs like that"*). It costs one `split("+")` if the grid is the
primitive, and costs a redesign if it is not. Validation stays lenient: each group 1–32, at most 16
groups, `D ∈ {1,2,4,8,16}`, sum ≤ 64; anything else → unset → 4/4.

### 3. Three tiers, not two — every unit flashes, ranked

VLL, 2026-08-21: *"feel bar beat in primary color, normal beat in secondary color and if there is
still metric beat free you fill with grey (1 (orange), 4 (blue), 2/3/5/6 (grey))"*. Adopted — it is
better than the two-tier rule this spec previously carried, because it stops forcing a choice between
*feeling* the metre and *seeing* it.

Given the groups, unit index `u` within the bar:

| tier | when | colour | weight |
|---|---|---|---|
| **0 — bar** | `u == 0` | primary **amber `#ffb02e`** | full width, full glow |
| **1 — felt pulse** | `u` is a group start | secondary **aqua `#3ee0d4`** | full width, full glow |
| **2 — free subdivision** | everything else | **grey `#6b7a90`** | full width, **~45% opacity, no glow** |

How it reads (`*` bar · `o` pulse · `·` subdivision):

```
4/4      1*  2o  3o  4o                       — no tier 2 at all; identical to today
3/4      1*  2o  3o
6/8      1*  2·  3·  4o  5·  6·                — VLL's example exactly
9/8      1*  2·  3·  4o  5·  6·  7o  8·  9·
12/8     1*  2·  3·  4o  5·  6·  7o  8·  9·  10o 11· 12·
5/4      1*  2o  3o  4o  5o                    — no grouping assumed
3+2/8    1*  2·  3·  4o  5·
3+4/8    1*  2·  3·  4o  5·  6·  7·
2+2+3/8  1*  2·  3o  4·  5o  6·  7·
```

Note what tier 2 buys: **4/4 and 3/4 are pixel-identical to what A34 ships today** (no unit is ever
tier 2 when all groups are 1), so this is purely additive for the common case.

**Grey is fine here, and was not fine before.** VLL rejected grey off-beats during the T85 tuning
(*"grey is a little bit too sad"*) — correctly, because there grey was the *second* rank pretending
to be switched off. As the *third* rank it is doing its job: receding. Keep the distinction sharp by
denying tier 2 the glow — hue alone is too weak a rank signal at speed, and a third *width* would
break the equal-width rule VLL settled on.

### 4. The clock ticks on the unit, and tempo says which unit

The beat is scheduled at the **unit**, never at the pulse. That is what makes unequal groups work:
in 3+4/8 the pulses are *not* evenly spaced but the eighths are, so a pulse-level clock cannot
express the metre at all.

```
uniform groups (all equal):  unitInterval = (60000 / tempo) / groups[0]   // tempo = pulses/min
irregular groups:            unitInterval =  60000 / tempo                // tempo = units/min
```

So the tempo label must name its unit: **`♩=NN`** simple, **`♩.=NN`** compound, **`♪=NN`** (or the
denominator's note) for an irregular additive metre, where no single pulse length exists. Display
only; the stored integer is unchanged.

**Mute tier 2 when it would strobe.** Below **130 ms** per unit the greys read as flicker rather than
texture — keep the grid and the clock, simply do not light tier-2 units. (This is the same complaint
VLL made at T85 — *"it is pulsating too quickly"* — one level down.) In 6/8 that threshold bites
around ♩.=154.

### 4b. Count-in = 2 bars

`countInUnits = 2 × unitsPerBar`, measured in units: 4/4 **8** (unchanged), 3/4 **6**, 6/8 **12
units = 4 felt pulses**, 3+4/8 **14**. The musician's rule is "two bars", and in every metre this is
exactly two bars of wall-clock time.

### 4c. The shared contract changes — and the vectors must grow with it

`beatPhase(elapsedMs, intervalMs, beats)` gains the **grid** (`groups: number[]`, or the parsed metre
string — implementer's choice, but one argument, not three); `BEATS_PER_BAR = 4` stops being a
constant. The return gains **`tier: 0 | 1 | 2`**, and **keeps `emphasis` as `tier === 0`** so that

- every existing 4/4 vector in `docs/contracts/beat-phase.vectors.json` passes **untouched** — that
  is the backward-compatibility proof, and
- **A35 runs the same file.** Do not copy it.

Add vectors for: 3/4 (a downbeat at unit 3, which the old `% 4` rule gets wrong), 6/8 (tier 1 at unit
3 and tier 2 at units 1,2,4,5), 12/8, one additive metre (3+4/8 — tier 1 at unit 3 only), and the
tier-2 mute threshold at a tempo either side of 130 ms/unit.

### 5. Everywhere the field has to flow (the miss to avoid)

1. `app.Song` + the song PATCH endpoint (beside key/tempo).
2. Studio Details: a `meta-meter` input next to key/tempo, and the `♩=` / `♩.=` / `♪=` label logic.
3. **`core/internal/app/bandio.go` — the export manifest song entry.** It carries title/artist/key/
   tempo/tags/notes; if metre is not added there, **a band export silently loses it** and an import
   comes back in 4/4. Easy to forget, invisible when it breaks.
4. `proto/troubastack/v1/bundle.proto` — `BakedSong` gains `string meter = 12` (next free; fields
   5–11 are the existing additive metadata). Additive: old bundles stay valid, old loaders ignore it,
   absent = 4/4.
5. The bake populates it from the song.
6. Seed: give the demo songs their real metres — the charts already print them (Amazing Grace **3/4**,
   House of the Rising Sun **6/8**, The Open Road **4/4**). **And give them tempos**: the A34 handoff
   found the demo songs carry none, so the beat is invisible on demo content by default. Fixing both
   together is what makes the feature demonstrable.

## Acceptance criteria

- Round-trip: set `6/8` on a song → GET returns it → **band export → import → still `6/8`** (the
  bandio path is the one that silently drops it, so assert it explicitly).
- Parser table, asserted as **groups**: `4/4→[1,1,1,1]`, `3/4→[1,1,1]`, `2/2→[1,1]`, `6/8→[3,3]`,
  `9/8→[3,3,3]`, `12/8→[3,3,3,3]`, `5/4→[1,1,1,1,1]`, `3/8→[1,1,1]` (3 is not > 3, so simple),
  `3+2/8→[3,2]`, `3+4/8→[3,4]`, `2+2+3/8→[2,2,3]`, `3+3+1/4→[3,3,1]`;
  `""`, `"x/y"`, `"4/5"`, `"0/4"`, `"33/4"`, `"3+0/8"`, `"1+1+…"` (17 groups) → unset → **4/4**.
- Tier table asserted for one bar of each of 4/4, 6/8, 3+4/8 and 2+2+3/8 against §3's diagram.
- **4/4 and 3/4 produce no tier-2 unit at all** — the additive-only proof for the common case.
- Vectors: new 3/4, 6/8, 12/8 and additive cases pass; **every pre-existing 4/4 vector still passes
  untouched**.
- Count-in spans two bars in every metre (4/4 → 8 units, 3/4 → 6, 6/8 → 12 units / 4 pulses), and a
  song with no metre still counts exactly 8 — assert that explicitly, it is the no-regression case.
- Downbeat lands on unit 1 of each bar in 3/4 (the case the old `% 4` gets wrong).
- Tier 2 is muted below 130 ms/unit and lit above it — assert both sides of the threshold.
- Tempo label reads `♩.=NN` compound, `♩=NN` simple, `♪=NN` irregular-additive.
- Demo: seeded songs carry metre **and** tempo, so the beat is visible on demo content without
  hand-editing.
- `tsc -b studio`, full `make e2e` (isolated ports), `gofmt -l core`, `go vet`, `make test`, and the
  proto/mirror drift guard all green.

## Out of scope

- The Stage half (**A35**): reading `meter` from the bundle and driving the stage beat from it.
- Metre *changes* mid-song and pickup bars. If those ever arrive they are per-section data, not a
  song field — a much bigger design.
- **Inferring** a grouping for a plain odd numerator: `5/4` stays `[1,1,1,1,1]` and `7/8` stays seven
  ones. A player who feels 5/4 as 3+2 writes `3+2/4` and gets it exactly; guessing on their behalf
  would be confidently wrong half the time, which is the failure mode this whole task exists to fix.
- Deriving the metre from the chart text or the PDF. The chart prints it, but parsing prose to infer
  musical structure is a guess; a player typing "6/8" once is better data than a heuristic.
