# T75 — Chart compaction: reclaim wasted page space at the SAME font size

**Priority:** high (this is what the "songs overflow a page" complaint is really about) ·
**Size:** M · **Area:** `core/internal/chartpdf`. **Do this before auto-fit (T76)** — see
the ruling.

## Ruling: compaction first, auto-fit second

VLL: *"I would rather work on compacity instead of zoom ratio … autofit is probably still a
nice feature, but compacity should come first."* That ordering is right, and not just as a
preference:

**Auto-fit's only lever is making the text smaller.** If the layout spends ~20% of the page
on leading, margins and a large title, auto-fit "solves" an overflow by shrinking the type
by roughly that much — the opposite of the stated goal (*"maximize the size of the text but
do not exceed a page"*). Compaction converts wasted space into font size: every millimetre
reclaimed here is a millimetre auto-fit never has to buy by shrinking. Compaction raises the
ceiling; auto-fit then just picks a point beneath it.

Two more reasons for this order: compaction is deterministic and reviewable (fixed metrics,
diffable renders) where auto-fit is content-dependent; and T75 introduces the **pure
measurement pass** that T76 needs anyway.

## Where the space actually goes (measured on `origin/main`, 11 pt default)

| Element | Now | Text height | Effective leading |
|---|---|---|---|
| lyric-only line | 6.5 mm | ~3.9 mm | **1.68×** |
| chord-only line | 6.0 mm | ~3.9 mm | 1.55× |
| chord + lyric pair | 11.5 mm | 2 × 3.9 mm | 1.48×/row |
| section label | 8.0 mm + 4 mm gap | ~3.9 mm | **2.06× + gap** |
| header (title 22 pt → rule → body) | ~30.5 mm before line 1 | — | ~10% of the page |
| margins | 18 mm top **and** bottom | — | 36 mm (12%) |

Body text conventionally sets at ~1.2× leading; we are at 1.5–1.7×, plus a 22 pt title and
36 mm of vertical margin. There is roughly a quarter of a page of recoverable space **before
touching the font size**.

## Design (decided)

**The defining constraint: the body font size does not change.** Default stays 11 pt. This
task must demonstrate a materially shorter chart at *identical type size*.

Levers, in order — all metric changes, no dialect change:

1. **Leading.** Bring per-line advances toward typographic norms, keeping the chord row
   tighter than the lyric row (chord tokens are short and mostly descender-free): lyric-only
   ~5.3 mm, chord-only ~5.0 mm, chord+lyric pair ~9.6 mm, section label ~6.5 mm. Treat these
   as targets, not gospel — derive them from a stated ratio (e.g. lyric 1.35×, chord 1.28×)
   so the numbers have a reason and scale correctly with `size:`.
2. **Structure.** Collapse consecutive blank source lines to a single paragraph gap; drop the
   paragraph gap immediately after the header and before the first section; no trailing gap
   at the end of the body. These remove space nobody asked for.
3. **Margins** 18 mm → **12 mm** left/right/top/bottom. Gains ~12 mm of height and a wider
   text column (fewer wrapped lines, which is a second-order height win). 12 mm still prints
   safely and leaves a binder edge.
4. **Header.** The performer knows the song; the title does not need 22 pt. Title ~16 pt,
   subtitle ~11 pt, with the rule and body start tightened proportionally. Reclaims ~8–10 mm.
5. **A pure `measure(lines, bodyPt) → height` pass** built from the *same* per-line metrics
   the renderer uses (one source of truth — a measurement that can drift from the renderer is
   worse than none). T76 consumes this directly.

**Explicitly NOT in this task** (each is a real technique, each is its own decision):
multi-column layout, inline chords (`[C]word`), collapsing repeated sections into repeat
marks, and section labels set inline with their first line. Multi-column and inline chords are
the two biggest further wins and are how ChordPro/OnSong solve this; they change what the
chart *looks like*, so they get their own task and their own review.

## Readability floors (non-negotiable)

This is read on a stand at roughly an arm's length, mid-performance. Compaction must not
produce a wall of text:

- A chord row must clear the previous line's descenders — assert no computed row overlap.
- Section labels must stay visually separable from body lines (keep the bold + colour and a
  gap that is still visibly larger than the intra-stanza gap).
- The paragraph gap must remain distinguishable from the line gap (a stanza break that reads
  as a line break defeats the point).

## Acceptance criteria

- **Body font size unchanged** (11 pt default; `size:` directive still honoured and still
  scales everything proportionally).
- **Quantified win:** rendered body height for the committed `docs/demo-charts/*.chart`
  fixtures **and** a long real-world-shaped fixture drops by **≥15%** versus `origin/main`;
  report the per-fixture numbers in the handoff. A test asserts the measured height of a
  fixture is below an explicit millimetre budget.
- **Overlap guard:** a test over every fixture asserting consecutive rendered rows never
  overlap, at 8 pt, 11 pt and 16 pt (the compaction must hold across the `size:` range, not
  just the default).
- `measure()` exists as a pure function and is the single source of the per-line advances
  used by the renderer — a test asserts a chart's measured height equals the renderer's final
  `y` (drift guard).
- **Goldens updated deliberately:** T74's `TestRender_NoDirectiveByteStable` necessarily
  changes — regenerate it and say so; it is the point of this task, not a surprise. Check
  whether any baked demo part is a text chart and, if so, re-bake `demo-concert.tstage` and
  note it in `docs/demo/README.md`.
- Before/after renders of the same real chart (VLL's *Hotel California* shape: several
  sections, chord rows with `(…)` notes) attached to the handoff — this is a visual change and
  the gate reviews it on pixels.
- T70's body-preservation and T73's no-mojibake assertions stay green; `gofmt -l core` clean;
  `go vet`; `make test`.

## Out of scope

- Auto-fit (**T76**), which lands on top of this.
- Multi-column, inline chords, repeat collapsing, inline section labels (see above).
- Any change to the chart dialect.
