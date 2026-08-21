# T86 — Give a song its metre, and make the beat count bars instead of guessing 4/4

**Priority:** high (VLL 2026-08-21, now that the beat works on stage: *"we probably want to have the
metric of the song instead of just tempo… instead of being 8 beats it should be 2 bars"*) ·
**Size:** M · **Area:** `proto`, `core` (model, API, bake, band export), `web/studio` (Details +
the beat + the shared contract + vectors). The Stage half is **A35**.

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

### 2. Pulses per bar — the musical decision, not the arithmetic one

The beat pulses what a player *feels*, not what the denominator counts:

| metre | pulses/bar | note |
|---|---|---|
| 4/4, 3/4, 2/4, 2/2 | numerator (4, 3, 2, 2) | simple — pulse = the denominator unit |
| **6/8, 9/8, 12/8** | **numerator ÷ 3** (2, 3, 4) | **compound — the pulse is the dotted unit** |
| 5/4, 7/8 | numerator | odd — downbeat on 1 only; grouping (3+2 etc.) is out of scope |

Rule: compound when `numerator % 3 == 0 && numerator > 3`. This matters — flashing six times a bar in
6/8 is both musically wrong and visually frantic; a player in 6/8 feels **two**.

**Therefore `tempo` means "pulses per minute of the metre's pulse."** The UI must label it honestly:
render **`♩.=NN`** for compound metres and **`♩=NN`** for simple ones, so the number is never
ambiguous. This is a display change only — the stored integer is unchanged.

### 3. Count-in = 2 bars, not 8 beats

`countInPulses = 2 × pulsesPerBar` → 4/4 **8** (unchanged), 3/4 **6**, 6/8 **4**, 5/4 **10**.
Downbeat is `pulseIndex % pulsesPerBar == 0`, replacing the hardcoded `% 4`.

### 4. The shared contract changes — and the vectors must grow with it

`beatPhase(elapsedMs, intervalMs, beats)` gains **`beatsPerBar`**; `BEATS_PER_BAR = 4` stops being a
constant. Studio owns the contract (T85 precedent), so:

- extend `docs/contracts/beat-phase.vectors.json` with **3/4 and 6/8 cases** — at minimum a downbeat
  at pulse 3 for 3/4 (which the old `% 4` rule gets wrong) and a 2-pulse bar for 6/8;
- keep every existing 4/4 vector passing unchanged — that is the backward-compatibility proof;
- **A35 runs the same file.** Do not copy it.

### 5. Everywhere the field has to flow (the miss to avoid)

1. `app.Song` + the song PATCH endpoint (beside key/tempo).
2. Studio Details: a `meta-meter` input next to key/tempo, and the `♩=` / `♩.=` label logic.
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
- Parser table: `4/4→4`, `3/4→3`, `2/2→2`, `6/8→2`, `9/8→3`, `12/8→4`, `5/4→5`, `3/8→3`;
  `""`, `"x/y"`, `"4/5"`, `"0/4"`, `"33/4"` → unset → **4/4 behaviour**.
- Vectors: new 3/4 and 6/8 cases pass; **every pre-existing 4/4 vector still passes untouched**.
- Count-in length follows the metre (3/4 → 6 pulses, 6/8 → 4), and a song with no metre still counts
  exactly 8 — assert that explicitly, it is the no-regression case.
- Downbeat lands on pulse 1 of each bar in 3/4 (the case the old `% 4` gets wrong).
- Tempo label reads `♩.=NN` for compound and `♩=NN` for simple.
- Demo: seeded songs carry metre **and** tempo, so the beat is visible on demo content without
  hand-editing.
- `tsc -b studio`, full `make e2e` (isolated ports), `gofmt -l core`, `go vet`, `make test`, and the
  proto/mirror drift guard all green.

## Out of scope

- The Stage half (**A35**): reading `meter` from the bundle and driving the stage beat from it.
- Metre *changes* mid-song, pickup bars, and grouping for odd metres (5/4 as 3+2). If those ever
  arrive they are per-section data, not a song field — a much bigger design.
- Deriving the metre from the chart text or the PDF. The chart prints it, but parsing prose to infer
  musical structure is a guess; a player typing "6/8" once is better data than a heuristic.
