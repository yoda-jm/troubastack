# T166 — the chart editor documents none of the directives it accepts

**Surface:** TroubaStudio, the chart editor. **Lane:** web-core. **Kind:** gap (discoverability).
**Number claimed** in the same push as this file.

VLL, 2026-09-07, on the new `columns:` directive: *"pour tout ce vocable n'oublie pas de changer la doc à
côté de l'éditeur."* He is right, and the gap is much larger than the one directive that prompted it.

## What the engine accepts, and what the editor says about it

| the engine accepts | the editor's **Chart format** block says |
|---|---|
| `{new_page}` / `{np}` | — |
| `{footnote}` / `{fn}` | — |
| `{start_of_tab}` / `{sot}` … `{end_of_tab}` / `{eot}` | — *(only a contextual lint hint when a paste looks like tab)* |
| `size: N` | — |
| `fit: page` \| `auto` | — |
| `columns: N` | — |

`ChartEditor.tsx:322` documents `# Title`, `## Section`, chord-over-lyric lines, `**bold**` and the blank
line. **Seven directives exist; zero are documented.**

## The footprint is measurable, which is what makes this worth doing

Across the band library's **178 files**: **zero** use `size:`, `fit:`, `capo:` or `key:`. The only
directives present are **8 files with `{sot}`/`{eot}`** and **one `{new_page}`**.

**And `{sot}` is the one directive the UI actively suggests** — the tab-lint hint at `ChartEditor.tsx:301`
offers it by name when a paste looks like tablature. The single directive family the interface mentions is
the single one that gets used. That is not a coincidence to argue about; it is the case for this task.

The cost is real and specific: a long chart auto-shrinks its type to fit rather than being set in two
columns at a readable size, because the author has no way to learn that either lever exists.

## What to build

**Document the whole vocabulary in the `Chart format` block** — not merely add `columns:`. Each entry needs
its shape, its accepted values and one line on what it does. Group them the way the dialect actually works,
because that grouping is itself the explanation:

- **`{…}` on its own line — a marker at a position in the flow:** `{np}` breaks the page here, `{fn}` starts
  the footnotes, `{sot}`…`{eot}` wraps tablature so the frets stay aligned.
- **`key: value` in the header block — a property of the whole chart:** `size:` sets the body type,
  `fit: auto` lets it shrink to fit, `columns: 2` sets it in two columns (which trades width for a *larger*
  type, the reason to reach for it).

## Second half — the editor draws a directive as if it were a lyric

`chartHighlight.ts` has an `hl-marker` class for the four brace directives and **no class at all for the
header directives**, so a correctly typed `size: 14` or `columns: 2` renders identically to a lyric line.
Documenting a directive that the editor then draws as words is half a fix: the author types it, sees
nothing change in the pane, and reasonably concludes it was not understood.

Give the header directives a token class of their own.

## ⟨R1⟩ Red first

- **The documented set EQUALS the engine's set.** This list is a hand-maintained enumeration mirroring a set
  defined in another language, which is precisely the shape that rots silently — so pin it: a committed
  contract file listing the directives, a Go test in `chartpdf` asserting its regexes cover exactly that
  list, and a Studio test asserting the help block mentions every entry. The `docs/contracts/` +
  `wire_kind_contract` pairing from T153 is the precedent. **Teeth: add a directive to the engine without
  documenting it and the pair must redden.**
  *Cheaper fallback if the contract is judged disproportionate: the Studio test alone, against a list
  duplicated in one named place — but then say so, because it will drift.*
- A header directive is highlighted **distinctly from a lyric line**. Teeth: assert the token class for
  `size: 14`, which today is `hl-plain` — the same class the lyric under it gets.
- The brace directives keep their current class; this must not disturb `{sot}` handling.

## Noted, not in scope — a stated mirror that isn't

`chartHighlight.ts:47` is `^\{(start_of_tab|sot)\}$` while the server's `reTabStart`
(`chart_tab.go:49`) also accepts `{sot original=…}`, and the comment above it claims the predicates mirror
the server. So an opener carrying `original=` is not recognised as a marker and the block's highlighting
state is not entered. **Verified not to bite today** — all 8 tab blocks in the library are the plain
`{sot}` form — so it is a note rather than a defect. Fix it while in the file, or file it; do not silently
leave the comment claiming a mirror that is not one.

## Done means

An author who opens the chart editor can discover, without being told by anyone, that a chart can break its
page where they choose, carry footnotes, hold tablature, and be set in two columns at a readable size.
