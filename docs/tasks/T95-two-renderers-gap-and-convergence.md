# T95 — The two PDF renderers: gap analysis, and converge what can converge

**Priority:** normal · **Size:** the analysis is done (§1–§3, below); the work is **S + M**, stageable ·
**Area:** `core/internal/chartpdf`, `core/cmd/mkcharts`, `docs/demo-charts`. Lane: Web & Core.
**Supersedes T24** ("converge cmd/mkcharts onto internal/chartpdf"), whose framing was wrong — see §4.

VLL, 2026-08-23: *"is there something to do to make them converge again or share the code?"* then
*"can you spec having only 1 renderer or the gap analysis between both?"*

The gap analysis comes first because it decides the other question. It is **done** — §1–§3 are findings,
not work. §5 is the work.

## 1. The finding that reframes everything: the gap is not in rendering power

Both renderers use nearly the same fpdf primitives. Counted on current main:

| primitive | `mkcharts` | `chartpdf` |
|---|---|---|
| `SetXY` / `Cell` / `SetFont` / `SetTextColor` | heavy | heavy |
| `CellFormat` | 1 | 3 |
| `Line` | 4 | 1 |
| `Rect` | 2 | 0 |
| `MultiCell` | 4 | 0 |

That is the *whole* difference: two `Rect`s, three extra `Line`s, and four `MultiCell`s. **`chartpdf`
is not missing drawing capability.** What it is missing is *dialect syntax* — there is no way to write
"a tab stave" or "a row of chord boxes" in a `.chart` file, so there is nothing for `chartpdf` to
render even though it could draw it.

This is why T24's framing ("converge mkcharts onto chartpdf") never moved: stated that way it is
blocked on growing a user-facing format, which is a much bigger decision than a refactor.

## 2. What each mkcharts output actually needs

| mkcharts output | what it draws beyond the dialect | converges? |
|---|---|---|
| `amazing-grace` | a wrapped prose footnote (public-domain attribution) | **yes**, with §5.1 |
| `open-road-leadsheet` | header rule (chartpdf already draws one); a **header META line** (`Key: … • Capo 2`) | **yes** — see the ⚠ below |
| `open-road-guitar` | **tab staves** (monospace rows + rules) + footnote | no |
| `house-rising-sun-tab` | **tab staves** + footnote | no |
| `house-rising-sun-drums` | **groove grid** (staff lines, bar lines, legend box) | no — and shouldn't |
| `blank-chart` | staff lines + empty chord boxes | **N/A — dropped**, see ⚠ below |

So of six artefacts: **two converge outright**, one converges in its body, three are genuinely
different documents.

> ### ⚠ Two corrections to this table (2026-08-24, on Web-Core's evidence)
>
> **The lead sheet has NO chord-box row.** `openRoadLeadSheet()` is `header` + `sectionLabel`/`chordLine`
> body + `footer` — no `Rect`, no `barChart`. Those are in open-road-**guitar**, which stays on mkcharts.
> So the lead sheet converges **fully**. Its real obstacle is different: mkcharts' `header` takes a third
> **meta** line (`"Key: G major • Tempo: 92 bpm • 4/4 • Capo 2"`) that chartpdf's header has no slot for —
> and B13's showcase mark anchors on `"Capo 2"` inside it. **Ruled: carry the meta as the chart's
> SUBTITLE** (option (a)); the mark stays near the top, no new grammar. A printed "info line" directive
> was rejected on §4's own reasoning — that is exactly the dialect growth §4 refused. Moving the mark to
> a `{footnote}` was rejected as restyling VLL's showcase.
>
> **The lead sheet must NOT be merged into `open-road-lyrics.chart`.** They are different content and the
> demo ships both: the chart's verse 1 is "Morning on the highway…" against the lead sheet's "Pack a
> little light…". The seed keeps the text chart deliberately so the demo shows the T19 chart type
> *alongside* the PDFs (B10). The lead sheet needs its **own** new `.chart` source.
>
> **`blank-chart` is dropped from this task.** It is a **test fixture only** (`anchortext_test`) — never
> seeded, never annotated, not in the demo — and its mkcharts form is staff lines + empty chord boxes,
> which the dialect **cannot** express. "Converging" it would replace a useful fixture with a title-only
> placeholder that loses the look it exists to provide, and it frees no helper (`header` stays for the
> residue builders). §5.2.3 wanted *duplication* gone; blank-chart isn't duplicated. Do not re-open.

## 3. The real blocker for the ones that *can* converge: the anchor manifest

`mkcharts` emits `<name>.anchors.json` — a bounding box in `[0,1]²` for **every text run** — and B13's
seed uses it to place demo annotations by looking up "this word, this page" instead of eyeballed
coordinates. That is what makes VLL's highlight provably cover its text (the standing annotation
quality bar).

`chartpdf` emits no anchors, so converting any chart to it today **breaks anchored demo annotations**.

**But `chartpdf` is most of the way there.** `layout()` is already the single source of truth for
element positions — the property T75, T76 and T77 all lean on — and it already feeds a `trace` hook
recording page + y per drawn element (`placed`, the pagination test hook). Anchors need x, width and
the text alongside. That is an extension of existing machinery, not new machinery.

## 4. RULING: "only one renderer" is rejected — and this is the honest reason

To retire `mkcharts` the dialect would have to grow **three** new block syntaxes: a prose footnote, a
tab stave, and a chord-box row. The `.chart` dialect is user-facing, shared with the studio editor,
pinned by golden tests, and now carries auto-fit (T76), compaction (T75) and page breaks (T77). Adding
tab-stave and chord-box grammar to it — **so that demo fixtures can share code** — is the tail wagging
the dog. A guitar tab is a genuinely different document from a chord-over-lyric chart, and the tool
that draws one is allowed to be a different tool.

What *is* worth fixing is the accident: today the two renderers overlap on the part they have in
common (`header`, `chordLine`, `sectionLabel` are hand-written twice), and `amazing-grace` exists
**twice** — once as an mkcharts PDF, once as `amazing-grace.chart` rendered by `chartpdf`. That
duplication is real and removable. The tab and the drum groove are not duplication; they are the
residue, and after §5 that is all `mkcharts` should contain.

**T76 made this more pressing, not less.** Auto-fit now applies to `chartpdf` output and not to
mkcharts' own drawing, so the two have drifted further apart — and `docs/demo/README.md` now leans on
exactly that gap to justify skipping a demo re-bake. The split is load-bearing in a place it wasn't
before.

## 5. The work, in two stages that can land separately

### 5.1 Stage A (S) — `chartpdf` emits anchors, and gains a footnote block

1. **Anchors.** Widen `layout()`'s existing `trace` to carry x, width and the text of each run, and add
   an exported entry point that returns the PDF **and** its anchor manifest in the same shape
   `mkcharts` writes today (page + `[0,1]²` box + text). Do not fork the layout — the whole value is
   that anchors come from the same walk that draws, so they cannot disagree with the ink.
   - **Guard it with a golden.** Anchors that silently drift are worse than no anchors, because they
     place VLL's annotations. A sha or a pinned table over a small fixture; teeth-check it by nudging
     one advance and confirming the golden fails.
   - Assert the invariant that matters: **every anchor's box contains the text it names** at the size
     actually rendered. A test that only checks "some boxes exist" would pass on garbage.
2. **A footnote/attribution block in the dialect.** The smallest useful addition, and it is *not*
   demo-only: real charts carry public-domain credits and "chord shapes" notes. Wrapped prose, after
   the body, visually distinct from lyrics. It must interact correctly with T76 auto-fit (it is part of
   the content being fitted) and T77 breaks — say so in the tests, don't discover it.

### 5.2 Stage B (M) — converge the two that converge, shrink mkcharts

3. Convert `amazing-grace` (done, Stage B part 1) to a `.chart` source rendered through `chartpdf`, and the
   lead sheet's **chord/lyric body** likewise (its chord-box row stays in `mkcharts`). They then
   inherit T75 compaction, T76 auto-fit, T77 breaks and transpose **permanently**, instead of being
   frozen at whatever `mkcharts` hardcoded.
4. Delete the now-duplicate `header` / `chordLine` / `sectionLabel` helpers from `mkcharts`, leaving it
   as exactly "the documents the dialect cannot express": tab, drum groove, chord boxes.
5. **Re-bake the demo bundle** — Stage B changes the demo PDFs' bytes, which forces the re-bake T76
   legitimately skipped. Update `docs/demo/README.md`: its current note explains why T76 needed no
   re-bake, and after this that explanation is stale.

## 6. Acceptance criteria

- Anchors for a converted chart **place a demo annotation over the right word** — assert against the
  seed's real lookup path, not against a hand-written box. This is the B13 quality bar and the reason
  anchors exist.
- Every anchor box contains its own text at the rendered size; teeth-check by perturbing one advance.
- The converted charts are byte-stable (a pinned sha, as T76 established for both the auto-fit and
  explicit-size paths) and visibly unchanged or better — attach before/after for each.
- `mkcharts` still produces the tab, drums and chord-box artefacts **byte-identically** to today: those
  are untouched, so prove it with a sha rather than by inspection.
- `go test ./...`, `gofmt -l core` clean; demo bundle re-baked with the README note rewritten.

## 7. Out of scope — deliberately

- **Tab-stave and chord-box grammar in the dialect.** §4 rules against it. If a *user* ever asks to
  write tab in the studio, that is a product decision with its own spec, not a fixture-sharing
  exercise.
- The drum groove. It is a one-off illustration; leave it in `mkcharts` forever.
- Any change to how the studio renders or edits charts.
