# T74 — Per-chart font size, as a header-block directive (`size: N`)

**Priority:** normal · **Size:** M — small code, but it **extends the chart dialect**, so the
design is the work · **Area:** `core/internal/chartpdf` + the chart-dialect docs. Requested
2026-08-19 ("a way to pick the font size"). Depends on nothing, but land **after T73** to
keep the rendering fixes separate from a syntax change.

## The decision you asked me to make

Three mechanisms were on the table — a dialect directive, a stored per-file setting, or a
render option. **Decided: a directive in the source, scoped to the header block.** Reasons:

- The chart **source is the artifact**. It is what the user edits, what round-trips through
  the pool, what the seed reproduces from a folder, and what gets re-rendered on every save.
  A setting that lives beside the source (new column, new API field, new bake concern) would
  have to be carried by all of those paths to survive — that is a lot of machinery for a
  number.
- A render option doesn't persist, so it cannot answer "my chart should always print this
  size", which is the actual request.

The cost of a directive is dialect ambiguity, so the design's whole job is to make collision
impossible.

## Design (decided)

1. **Scope: the header block only.** Directives are recognized only on the contiguous
   non-blank lines immediately after `# Title`, before the first blank line or `##` section.
   Body lines are never scanned — a lyric `size: 13` renders as a lyric, as it does today.
2. **Exactly one recognized key: `size`.** A header-block line matching
   `^size\s*:\s*(\d+)$` (case-insensitive on the key) is a directive. **Any other `key: value`
   line is not a directive** — it stays whatever it already was (subtitle or body), so an
   artist line like `Foo: Bar` is unaffected. Adding a second key later is a deliberate
   decision, not an accident.
3. **Interaction with T70's subtitle rule — specify it, don't discover it.** The subtitle is
   currently the line at exactly `titleIndex+1`. With directives, the subtitle is the first
   header-block line after the title that is **not** a directive; directive lines are removed
   from the body and never rendered. Both orders must work:

   ```
   # My Song            # My Song
   The Artist           size: 13
   size: 13             The Artist
   ```

4. **Bounds and failure mode.** Accept **8–16 pt** (default = today's size, unchanged). Out
   of range or malformed → **ignore the directive and render at the default**; the line is
   still consumed (not printed as body). A chart must never fail to render because of it.
5. **Scaling.** `size` sets the body/lyric font size; chords, sections and the header scale
   **proportionally** from it, so the chart stays balanced rather than having one row change
   size. Keep the existing ratios — derive them from the current constants rather than
   inventing new ones. Line spacing scales with it too, or the change is pointless.

## Acceptance criteria

- Directive parsing table test: `size: 13` ✓; `Size:13` ✓; `size: 99` → default (ignored);
  `size: abc` → not a directive → **body**? No — decided: it does not match the numeric
  pattern, so it is *not* a directive and stays subtitle/body, exactly as today. Cover both
  that case and the out-of-range case, since they behave differently on purpose.
- T70 interaction tests: both orders in §3 yield subtitle `The Artist` **and** size 13, and
  neither prints a `size:` line in the body.
- A chart with no directive renders **byte-identically** to before this task (assert against
  a fixture render) — this is the guard that an existing chart's appearance is untouched.
- Rendered size actually changes: a chart at `size: 8` and the same chart at `size: 16`
  produce different page content, with the smaller fitting strictly more lines per page.
- `TestSubtitleHeader_BodyPreservation` and the T73 no-mojibake assertion stay green; a
  directive line is the only line allowed to disappear from the body.
- Docs: the chart-dialect documentation gains the directive, its one key, its range, its
  header-block scoping and the "unknown keys are not directives" rule, with an example.
- `gofmt -l core` clean; `go vet`; `make test` green.

## Out of scope

- Any second directive key (`margin`, `columns`, …) — add one only on a real request, and
  it is a spec decision each time, not an open extension point.
- Auto-fit / shrink-to-page. It is the obvious next idea and it is deliberately **not**
  bundled: it changes existing charts' appearance implicitly, which is precisely the class of
  surprise T70 was written to avoid. If wanted, it is its own task with its own regression
  guard.
- Per-file stored settings or a render-time override API.
