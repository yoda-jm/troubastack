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

## ⟨D1⟩ VLL's ruling, 2026-09-05: **auto-fit becomes opt-in, and is never the default**

*"the autoadjustment should be an opt in, never the default."* This is a decision, not a proposal —
implement it. It belongs in this task because it is the same question as two columns: **what do we do when
a chart does not fit?**

### What auto-fit does today, measured

`autoFitBodyPt` returns the **largest** integer size in **8–16 pt** at which the chart needs no automatic
page break; `defaultBodyPt` is **11**. So it moves size in *both* directions, per song, to maximise "fits
on one page". In one setlist a short chart can render at 16 pt and a 72-line chart at 8 pt — **a 2×
difference between two songs a musician reads back to back.** A manual `size:` already disables it.

### What making it opt-in actually solves — and what it does not

It solves three real things:

1. **One size across the setlist.** Every chart renders at `defaultBodyPt` unless it asks otherwise. This
   is VLL's original complaint, and no amount of per-song cleverness fixes it.
2. **Predictability.** Adding one lyric line can currently re-size an entire chart. With a fixed size,
   adding a line changes only what comes after it.
3. **It shrinks the reflow surface that breaks annotations.** Today layout is a *function of content
   length*, so any edit anywhere can move every mark on the page. **This does NOT replace T145** — a
   renderer change still reflows — but it removes the most frequent trigger.

It does **not** solve: fitting a long chart on one page. That cost is real (more page turns mid-song), and
**two columns is the honest answer to it** — width traded for size, instead of legibility traded for a
page turn. That is why both live in this task.

### ⟨D1.1⟩ The hypothesis this creates, which MUST be tested, not assumed

VLL's charts were rendered **2026-08-22**, before auto-fit existed (`127519fd`, 08-23). If the default
returns to `defaultBodyPt`, the render should come back close to that August layout — **the one where his
mark sat exactly at the end of the text.** If so, opting out would put many existing annotations back
roughly where they belong.

**Do not state this as a benefit until it is measured.** The archived 08-22 blob and T144's golden
machinery make it a direct comparison: re-render the same source with auto-fit off and diff against the
archived PDF. Report the answer at the gate. It may well differ for unrelated reasons (cp1252 rendering,
tab blocks landed since) — say so plainly if it does.

### Required for ⟨D1⟩

- `autoFitBodyPt` runs **only** when the source opts in — same vocabulary as `size:`, e.g. a header
  directive; do not invent a second mechanism.
- With no directive: `defaultBodyPt`, and automatic page breaks where the content needs them.
- A manual `size:` keeps disabling auto-fit, as today.
- **Red first:** a fixture longer than one page renders at `defaultBodyPt` across **two** pages with no
  directive, and at a smaller size on **one** page with the opt-in. Both assertions fail today — the first
  because today it shrinks, the second because there is nothing to opt into.
- Update T144's golden values **in the same commit**, so the layout change is visible in review rather
  than silent. That is exactly the ritual T144 exists to create, and this is its first real exercise.

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
