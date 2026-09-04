# T144 — Pin the chart render, so a layout change cannot ship silently

**Lane:** web-core (core). **Size:** S/M. **Status:** fixed 2026-09-05 (web-core) — golden layout guard
added (`TestGoldenLayout`, 5 invented fixtures: short/boundary/long/tab/lyric-only; page count + PDF hash).
⟨V1⟩ bisect done and it CORRECTS this file's assumption (see the gate entry): the assumed window
`3999abe0..8f662f60` has **no `chartpdf` change** (byte-identical render), so it is not the cause — the
reflow was the re-seed re-rendering old blobs with the current, more compact renderer (the intended
T73/T75/T76 changes). No revert; the guard is the fix. ⟨R1⟩ satisfied: the three sabotages (font, leading,
margin) each turn it red independently, and the fixtures differ across the T75/T76 compaction boundary
(boundary 2→1, long 5→3 pages), so they cover the affected path.

## The event this exists to prevent

Between two bakes on 2026-09-04, **the same chart source rendered differently**:

| | page 2: where ink stops | pages |
|---|---|---|
| 17:46 bake | `0.409` | 2 |
| 22:20 bake | `0.051` | 2 |

Across the annotated songs, page counts moved `4→3`, `2→1`, `2→1`, and one chart's ink went from `0.734`
to `0.924` of the page. **The source is byte-identical** (same md5 in the current folder, the 16:21
backup, and the pre-v2 archive from 12:16). All 157 blobs were re-rendered at 19:19 by a binary deployed
at 19:18; the earlier pages came from a binary built at 16:32.

So the deployed renderer moved from roughly `main@16:21` to `main@18:31` and the type shrank. **No test
noticed, because no test looks at layout.** That is the defect this task fixes — the reflow itself is a
symptom, and the marks it orphaned are T145.

## ⟨V1⟩ First, establish which change did it — do NOT assume

**Nothing in this file names a cause, and the implementation must not either until this is run.** The
candidate window is `98aafb3d`..`8f662f60` — the 16:32 build to the 19:18 build.

Build at each end, run **the same source** through the **import path** with both, and compare page count
and ink extent. Report the answer in the task's gate entry.

**Bisect the importer, not `chartpdf`.** The evidence points there: no commit in the window touches a
`chartpdf.` call site, while the import vocabulary did change in it, and T141's `size` defect appears at
exactly the same instant (`size` flips from describing the blob to describing the source). It is likely
that **#4/#5 and #6 share one cause** — check that before treating them as separate.

Four things are already **excluded by measurement**; do not re-check them:

- the source did not change (identical md5 across three time-separated snapshots);
- nothing stripped a `size:` directive (0/46 sources carry one, in all three) — though auto-fit itself
  IS new: `127519fd` (T76) landed 08-23, after these blobs were rendered on 08-22;
- re-rendering alone does not change the output (two re-renders inside the old instance produced the
  identical layout);
- T138's default-file rule cannot apply — five of the six affected songs have exactly one file.

If the bisect lands on a change that was *intended* to alter layout, then the fix is only the guard below
plus a note; if it lands on one that was not, it is a regression and gets reverted or corrected on top.

## Required — the guard

A **golden layout test** over committed fixture sources, in `core/internal/chartpdf`:

- Fixtures live in the repo and contain **no band data** — invented lyrics and chords only.
- Cover at least: a short chart (1 page), a chart that lands near a page boundary, a long chart that
  auto-fits, and a chart with a tab block.
- For each, assert **page count** and a **layout hash** — a digest over the per-page glyph positions (or,
  if that is not reachable, over the rendered page rasters at a fixed DPI). The point is that *any*
  metric change moves the value.
- On failure, print the old and new page counts and the first differing page, so the next person sees
  *what* moved rather than "hash mismatch".

**Deliberate layout changes then have a ritual**: update the golden values in the same commit that changes
the metric, which makes the change **visible in review** — today it is invisible in a diff.

## ⟨R1⟩ Red first, with teeth

VLL: *"pour chaque bug un red first."* The assertion must be **seen failing** before the guard is
believed, and its expected value must differ from what a wrong implementation produces:

1. Write the golden test against today's renderer and let it pass.
2. **Sabotage each metric separately** — the base font size, then the leading, then the page margin —
   and confirm the test goes red for **each one independently**. A single sabotage can be caught by luck;
   three cannot.
3. Confirm the test would have caught the real event: render a fixture with `chartpdf` from `3999abe0`
   and from `8f662f60`, and check the golden values differ. **This is the teeth-check** — if they do not
   differ, the fixtures do not cover the affected path and must be extended until they do.

## Acceptance

- The golden test exists, is run by the `go` job, and fails on each of the three independent sabotages.
- The ⟨V1⟩ bisect result is stated at the gate, with the two ends' page counts for one fixture.
- Fixtures contain no band data.
- `gofmt -l core` is clean before landing.

## Out of scope

Changing the layout itself, the left margin (**T146**) and re-anchoring existing marks (**T145**). This
task only makes layout a **checked** property.
