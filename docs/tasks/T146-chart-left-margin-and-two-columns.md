# T146 — Shrink the chart's left margin, and open the door to two columns

**Lane:** web-core (core, `chartpdf`). **Size:** S for the margin, M for the column option.
**Status:** spec, 2026-09-05. Enhancement #7 from the rehearsal field report.

## What VLL asked for

*"diminuer la marge a gauche des fichiers textes rendu"* — the rendered chart's left margin is sized for a
single column and wastes width on a tablet held in portrait.

It is worth doing for its own sake, and it is the **prerequisite** for the thing behind it: a chart that
no longer has to shrink to fit. Today a long chart auto-fits by making the type smaller — which is exactly
what made a 72-line chart unreadable next to a 41-line one at the rehearsal. **Two columns trade width for
type size**, which is the trade a musician on a stand actually wants.

## Required

**Stage 1 — the margin becomes a named constant.**

- Replace the hard-coded left margin with a named constant, with the chosen value and its reason in a
  comment.
- **T144's golden test must pin it**, so the value cannot drift the way the type size did on 2026-09-04.
  If T144 has not landed yet, this task adds the assertion for the margin alone.
- Reduce it to the new value in the same commit, and state the before/after in millimetres.

**Stage 2 — the two-column option.**

- A chart may render in two columns. **Opt-in per chart via a directive** in the source, consistent with
  how `size:` already works — this is the same vocabulary, so do not invent a second mechanism.
- A column is a layout unit: a chord/lyric pair never splits across the column break, and a tab stave
  never splits at all (the existing rule from T135).
- Auto-fit still applies **within** the chosen column count.

## The question to settle while specifying stage 2

**Does two columns interact with annotations?** It must be answered before, not after: a mark anchored to
page coordinates on a one-column render is meaningless on a two-column one. If **T145** lands first with a
source anchor, this is free. If it has not, stage 2 **waits** — shipping a column mode that silently
invalidates every existing mark would repeat the exact failure of 2026-09-04.

Say which of those two situations applies at the gate.

## ⟨R1⟩ Red first

- **Margin:** assert the rendered left edge of the first glyph is at the new value. Seen red against
  today's value first — the expected number must differ from what today's code produces.
- **Two columns:** assert a chart that needs two pages in one column fits **one page in two columns at a
  larger type size than the one-column render**. That comparison is the whole point of the feature, so it
  is the thing to assert; a test that only checks "two columns appear" would pass a version that shrank
  the type anyway.
- Fixtures contain **no band data** — invented lyrics only.

## Acceptance

- The margin is a named, tested constant, reduced, with before/after stated.
- Stage 2 either lands with the annotation question answered, or is explicitly deferred behind T145.
- `gofmt -l core` clean before landing.
