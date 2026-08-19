# T76 — Auto-fit: pick the largest size that keeps a chart on one page

**Priority:** normal — **sequenced after T75** (compaction), by ruling · **Size:** S/M ·
**Area:** `core/internal/chartpdf`.

VLL's requirement: *"maximize the size of the text but do not exceed a page (portrait) for
most songs … maybe not all of them, but the normal-length ones."*

## Why after T75

Auto-fit's only lever is shrinking type. Run against today's loose layout it would buy a
one-page fit by making text ~20% smaller; run after compaction it starts from a page that
holds ~25% more, so the size it picks is bigger. Same feature, better answer. T75 also lands
the pure `measure()` pass this task needs.

## Design (decided) — including the calls the lane asked me for

- **Mechanism:** the `measure(lines, bodyPt)` pass from T75. Search sizes from the ceiling
  down and render **once** at the largest whose measured body height fits the usable page.
  No double render, no guessing.
- **Range: reuse T74's 8–16 pt.** Auto-fit must **not** exceed the manual ceiling — one
  bounded range for the dialect, or `size:` and auto-fit start disagreeing about what is
  legal and we get to explain why a chart may be 20 pt only when we chose it. The 8 pt floor
  stays a readability floor: below it we accept the overflow rather than print something
  nobody can read on a stand.
- **An explicit `size:` disables auto-fit** for that chart (T74 stays a hard manual
  override). This also preserves byte-identity for charts that opted in.
- **Overflow is allowed.** If a chart does not fit at 8 pt, it spills to a second page as
  today. "Most songs, not all" is the requirement; a guaranteed one-page fit would mean
  unreadable type or silent content loss, and both are worse.
- **Demo charts: accept the change and regenerate the goldens** (the lane's option (i)).
  Scoping auto-fit to folder/Studio charts and leaving demo charts at a fixed size would give
  us **two rendering behaviours for one dialect** — the demo bundle, the screenshots and
  DEMO-VID would stop showing what users actually get. We have already been burned by
  demo/product divergence (B13 shipped mojibake into the bundle because the demo generator
  and the runtime renderer had drifted). One renderer, one behaviour; regenerate what that
  invalidates.

## Amendment (T77, ruled 2026-08-19): measure by `contentHeight`, per segment

T77 split the measurement primitives, and that split is **correct — my original wording here and
in T77 was imprecise**. Use them as follows:

- **`contentHeight(source)`** — the continuous "how tall is this" number (pagination disabled).
  **This is the primitive auto-fit consumes**, not `measure()`.
- `measure(source)` is the *paginated* final y and exists to prove the renderer and the layout walk
  agree; it is a drift guard, not a height.
- Both are one `layout()` walk with a `paginate` flag, shared with `renderChart`, so a measurement
  cannot drift from what is drawn.

**Per-segment rule.** With explicit `{new_page}` markers the author defines the segments; auto-fit
picks the largest size where **every segment's `contentHeight` fits its own page**. Orphan control
never perturbs this: if a segment fits one page, nothing inside it overflows, so no header is
pushed.

**Watch the first segment's budget.** The title block is drawn once, at the top of the chart, and
continuation pages start at the top margin with no repeated title. So segment 1's budget is
`usable page − header height` while later segments get the full usable page. Compute it that way
rather than assuming a uniform budget.

## Amendment (T77): charts with explicit page breaks

T77 adds a `{new_page}` marker. The objective generalises from "the chart fits one page" to
**"no page overflows"**: with explicit breaks the author has defined the segments, and
auto-fit picks the largest size where **every segment fits its own page**. A chart containing
`{new_page}` still gets auto-fit — unlike an explicit `size:`, a break does not disable it.

## Acceptance criteria

- Property test: for a normal-length fixture the chart occupies **exactly one page**, and the
  chosen size is **maximal** — rendering at chosen+1 pt overflows. This is the real assertion;
  "it fits" alone would pass at 8 pt for everything.
- A deliberately over-long fixture falls back to the 8 pt floor and is allowed to be
  multi-page (assert it does not error and does not lose content — T70's body-preservation
  property must hold at the floor too).
- A chart with an explicit `size:` renders at exactly that size regardless of fit, and
  byte-identically to its T74/T75 output.
- Chosen size is deterministic and stable across runs (same input → same size → same bytes).
- Goldens/demo artefacts regenerated per the decision above, including a re-bake if any baked
  default part is a text chart; note it in `docs/demo/README.md`.
- Handoff shows the same real chart before/after with the chosen size stated.
- `gofmt -l core` clean; `go vet`; `make test`.

## Out of scope

- Multi-column and inline-chord layouts (separate tasks; they change the chart's appearance,
  and after T75+T76 we should re-measure whether they are still needed).
- Per-part or per-concert overrides; auto-fit for anything but text charts.
