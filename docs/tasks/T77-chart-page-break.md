# T77 — A meaningful page break: an explicit `{new_page}` marker + no orphaned sections

**Priority:** high (VLL 2026-08-19: *"I need a page break in the chart page language in order
to put a proper page break that has meaning in the chart (not in the middle of a section or
between chords and lyrics)"*) · **Size:** S/M · **Area:** `core/internal/chartpdf` + the
dialect docs. Builds on T75 (landed, `85f7c4c`).

## The two halves of "a page break that has meaning"

A page turn happens **mid-performance, with both hands busy**. A break is only useful where
the music allows it, which the renderer cannot know — so the author needs to say. That is the
explicit marker (§1). But the automatic breaks must also stop putting turns in stupid places
(§2), or an author would have to mark every song defensively.

I verified both halves against landed `main` before writing this:

- **Chord/lyric pairs are already safe.** Scanning chart lengths that straddle the page
  boundary, no break ever separated a chord row from its lyric — `page(leadPair)` reserves the
  whole pair. Good, but **nothing tests it**, so it can regress silently. Lock it (§2).
- **Section headers ARE orphaned — a real defect.** At the right length, page 1 ends with the
  section header `Chorus` and page 2 begins with `Am       E7`. The heading is stranded on the
  previous page, which is exactly "a break in the middle of a section".

## 1. The marker

**Syntax: a line containing only `{new_page}`** (canonical), with **`{np}`** accepted as a
short alias — case-insensitive, surrounding whitespace ignored.

Why braces, when `size:` is a bare `key: value`? Because they are different kinds of thing and
should not look alike: `size:` is **metadata about the whole chart**, scoped to the header
block; a page break is a **flow marker at a position in the body**. Braces also (a) cannot
realistically collide with lyrics, chord rows or tab lines, (b) are instantly recognisable as
"not content" when reading the source, (c) match [ChordPro's `{new_page}` / `{np}`], which is
what these users already know from OnSong and friends, and (d) leave `[...]` free in case we
ever add inline chords (`[C]word`) — a door worth not closing.

Semantics:

- Content after the marker starts at the top of the next page.
- **Never produce a blank page**: a marker before any content, a trailing marker, and
  consecutive markers are all no-ops (they collapse).
- The marker line is **consumed and never rendered** — like `size:`, and it is likewise **not
  a subtitle candidate** (T70 must not pick it up if it sits directly under the title).
- After a break the body starts at the top margin with no extra leading gap (T75's rule).
- No change to what appears on continuation pages (no repeated title) — that is a separate
  question; do not smuggle it in here.

## 2. Automatic breaks must not strand a heading

**Orphan control:** a section label may never be the last rendered element on a page. When a
section header does not have room for **itself plus its first content line** (a chord+lyric
pair counts as one unit), break *before the header* instead. Reserve
`leadSection + first-unit height` rather than `leadSection` alone.

**Keep the pair rule and test it:** a chord row and its lyric never split across pages
(already true — add the regression test so it stays true).

Both rules apply to the automatic breaks only; an explicit `{new_page}` is always obeyed
exactly where the author put it.

## 3. Interaction with auto-fit (T76, not yet built)

Auto-fit's objective generalises from "the chart fits one page" to **"no page overflows"**.
With explicit breaks the author has defined the segments, and auto-fit picks the largest size
where **every segment fits its own page**. A chart containing `{new_page}` therefore still
gets auto-fit — it is not disabled, unlike an explicit `size:`. Record this in T76's spec when
it is picked up.

## Acceptance criteria

- Marker parsing table: `{new_page}` ✓, `{np}` ✓, `{NEW_PAGE}` ✓, leading/trailing spaces ✓;
  `{newpage}`, `{new page}`, `{np} x`, `new_page` and `{{np}}` are **not** markers and render
  as ordinary body text (they are content, and silently swallowing content is the failure mode
  to avoid).
- Placement: content after a marker begins page 2 at the top; a chart with two markers renders
  three pages in the author's segmentation.
- **No blank pages**: leading marker, trailing marker, and `{np}` twice in a row each produce
  the same page count as the marker-free chart.
- **Orphan control, red-first**: reproduce the stranded `Chorus` case (a chart whose length puts
  a section header at the page boundary — it reproduces around 45 filler lines at the default
  size), assert page 1 does **not** end with a section header, and that the header appears at
  the top of page 2 with its first line. The test must fail on today's code.
- **Pair regression test**: over a scan of chart lengths straddling the boundary, no page ever
  ends with a chord row whose lyric is on the next page.
- T70 body-preservation still holds — the marker line is the only line allowed to disappear;
  T73's no-mojibake assertion and T75's overlap/measure guards stay green.
- `measure()` accounts for markers and orphan control, so it still equals the renderer's final
  `y` (`TestT75_MeasureMatchesRender` extended to a multi-page chart) — T76 depends on this
  being honest.
- Dialect docs gain the marker, its aliases, the no-blank-page rule, and one sentence of
  guidance: **put the break where the player has a free hand** (a section end, a rest, an
  instrumental), not merely where the page happens to fill.
- `gofmt -l core` clean; `go vet`; `make test` green.

## Out of scope

- Repeating the title/key on continuation pages; "V.S." turn hints; two-column layout.
- Auto-fit itself (**T76**) — only its stated interaction above.
- Collapsing repeated sections (the separate technique that would let long charts fit at all).

[ChordPro's `{new_page}` / `{np}`]: https://www.chordpro.org/chordpro/directives-new_page/
