# T73 — chartpdf: chord rows with a trailing `(…)` note, accented section names, tighter header gap

**Priority:** high (b is a rendering-correctness bug; a is hit by every real chart with
repeat marks) · **Size:** S/M · **Area:** `core/internal/chartpdf`. From live use of a real
band's chart, 2026-08-19. Font-size control is **T74**, deliberately separate.

## a. A trailing parenthetical must not disqualify a chord row

`isChordRow` requires *every* whitespace token to match `chordToken`, so a real chord line
with a performance note loses its chord styling entirely and renders as lyric text.
Verified:

```
isChordRow("Am E7 G D F C Dm E7 (2x, 1x Arpèges, 1x normal)") = false
isChordRow("Am E7 (x2)")                                      = false
isChordRow("Am E7")                                           = true
```

**Decided rule.** A line is a chord row if, after splitting off a **terminal, balanced
parenthetical**, the remaining tokens are non-empty and all chords. The tail counts as an
annotation only when it starts with `(` and the line ends with `)` — so
`Am E7 (x2)` is a chord row, while a lyric like `A (very) long day` is **not** (it doesn't
end in `)`), which is the false positive to avoid. Nothing before the first `(` may be
non-chord.

**Rendering.** Chord tokens keep the chord style; the annotation renders on the same line
in a **muted, non-chord style** (it is an instruction, not something to play) and, like all
other text, goes through `tr(...)` — the reported case contains `Arpèges`.

## b. Accented section names mojibake (BUG)

`sectionLabel` (`chart.go:236`) draws `pdf.Cell(0, 6, label)` — **not** `tr(label)` — unlike
the title, subtitle, chords and lyrics. `tr` is already in scope at the call site
(`chart.go:90`). Verified end to end: `## Verse 7 (Arpèges)` renders as
**`Verse 7 (ArpÃ¨ges)`**.

This is the third instance of one mistake: mkcharts' `sectionLabel` (fixed in B13, which
shipped mojibake into the demo bundle) and now the runtime chart renderer. Fix is
`sectionLabel(pdf, tr, y, label)`.

> **Write the test carefully — the obvious one is toothless.** In the repro, the string
> `Arpèges` *also* appears in a lyric/chord line that renders correctly, so
> `strings.Contains(text, "Arpèges")` passes while the header is still broken. The
> assertion must target the **section line specifically** (e.g. the extracted line equal to
> `Verse 7 (Arpèges)`, or assert `ArpÃ¨ges` appears **nowhere** in the output). A blanket
> "no mojibake sequences (`Ã`, `â€`, `Â`) anywhere in the rendered text" assertion is the
> most durable form and would have caught all three instances.

## c. Halve the gap under the header rule

`header` returns `ruleY + 7`; make it `ruleY + 3.5` so the first section/text sits closer to
the rule. Layout-only, applies to every chart — intended.

## Acceptance criteria

- Chord-row table test covering: plain chord row ✓; `Am E7 (x2)` ✓; the full
  `Am E7 G D F C Dm E7 (2x, 1x Arpèges, 1x normal)` ✓; `A (very) long day` ✗ (stays lyric);
  `(x2)` alone ✗; unbalanced `Am E7 (2x` ✗; a lyric line unchanged ✗.
- Rendered output: for a chord row with an annotation, the chord tokens are chord-styled and
  the annotation is present and visually distinct; the accented annotation text is intact.
- Section-accent test as described above — **plus** the blanket no-mojibake assertion over
  the whole extracted text of a chart exercising title, subtitle, section, chords, annotation
  and lyrics with accents and an em-dash. Red before the fix.
- Existing chartpdf tests stay green, including T70's `TestSubtitleHeader_BodyPreservation`
  (no body line may vanish because a line changed classification).
- `gofmt -l core` clean; `go vet`; `make test` green.

## Out of scope

- Font size / density (**T74**).
- Chord transposition of the annotation text (it is prose, never transposed — if T60's
  transposer touches these lines, confirm it skips the parenthetical and say so in the
  handoff).
- Any dialect change: this task adds no new syntax, it only classifies existing lines better.
