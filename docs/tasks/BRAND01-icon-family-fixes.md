# BRAND01 — Icon family: correctness fixes on the generated brand assets

**Priority:** normal · **Size:** S · **Area:** `docs/brand` (build.py, sheet.py, dist/ regenerated).
**For:** the agent that built the icon family (commit `5e98306c`, currently on a detached HEAD in
its own worktree — not landed). Source: architect audit 2026-09-01. **VLL is happy with the look:
no visual redesign.** Except where a fix IS the point (the four adaptive foregrounds), every file
in `dist/` must come out byte-identical.

The architecture is right and stays: bricks + recipes, one generator, measured numbers, no
hand-edits in dist/, no SVG filters. Determinism verified during the audit (rebuild → zero diff).
The findings below are correctness holes, three of them verified against the committed output.

## Findings → fixes

### 1. Adaptive foreground busts the Android safe circle — REQUIRED, measured

README and code both say the artwork is "scaled into the 66/108 safe circle", but the scale is a
hand-typed `scale(0.66)` (`build.py` main, adaptive block) — and 66/108 = **0.611**, not 0.66.
Worse, what matters is the artwork's own extent, and it was never measured against the circle:

> rendered `troubastack-adaptive-foreground.svg` at 432px; max radial extent of any pixel with
> alpha > 8 = **170.2px** from center vs safe-circle radius **132px** (= 66/108 × 216). Same for
> every mark (the tile-less compact art has identical extent). **~29% overshoot.**

On any round-masked launcher — most of them — the corners of the artwork get clipped off.

**Fix:** derive the foreground scale from the artwork's measured extent so the furthest opaque
pixel sits at ≤ 66/108 of the half-canvas (leave a small margin, ~2%); do not hand-type the
number. **Guard:** add a raster containment check to the `--png` path (rsvg-convert + Pillow are
both already used by sheet.py): render each `*-adaptive-foreground.svg`, fail the build if any
pixel with alpha > 8 lies outside the safe circle. **Red-first:** the check must fail against the
committed dist/ before the fix — paste that failure in the handoff.

### 2. Every adaptive foreground carries TWO `<defs>` blocks — REQUIRED, verified

The foreground is extracted from `compose()` output by string surgery
(`art.split(">\n", 2)[2].rsplit("</svg>", 1)[0]`) — which keeps compose's `<defs>` — and is then
wrapped in `TEMPLATE` with `defs=brick("_defs")` **again**. Verified: every gradient id
(`gTile`, `gLayer1-3`, `gSheen`, …) is defined **twice** in all four files. Duplicate XML ids are
invalid; which definition a renderer uses is unspecified, and the VectorDrawable conversion these
files exist for is exactly the kind of consumer that may choke or pick differently.

**Fix:** stop the string surgery — build the foreground body directly:
`body(mark, "compact", tile=False)` wrapped in the scale group, with ONE `<defs>` =
`brick("_defs")` + `ink(mark, "compact")`. **Guard with teeth:** extend `check()` to also assert
id uniqueness across the document — that check would have caught this. Red-first against the
committed dist/, same as §1.

### 3. PNG ladder writes `{mark}-192.png` twice — REQUIRED

`PNG_SIZES` lists 192 under **both** `full` and `compact`; the filename carries no variant, so
the compact render silently overwrites the full one (dict order). The README even contradicts
the table: "full — 512px and up", yet full renders at 192.

**Fix:** `full: [1024, 512]` — compact owns 192, per the README's own ranges. Assert in the
`--png` loop that no output path is written twice (cheap, permanent). Update the commit-message
habit of counting PNGs only after this — today's "30 PNGs" doesn't match either interpretation.

### 4. sheet.py hard-crashes without Pillow while calling it optional

`from PIL import Image, ImageDraw  # (optional dependency, only for the compare)` — no
try/except. On a machine without Pillow, sheet.py tracebacks AFTER writing the sheet, even when
`reference/` is empty and no compare would run. Wrap the import; on ImportError print a skip note
and exit 0, mirroring the existing missing-reference skip.

### 5. The palette anti-drift guard is circular for two of its six swatches

`sheet.py` builds its haystack as `_defs text + B.SHARED_HL + B.tile_colour()` and then checks
every swatch is `in` it. `SHARED_HL` and the tile swatch are in the haystack **by construction**
— those two rows can never fail, under the caption that says the panel cannot drift (the exact
failure mode this guard was written for, twice).

**Fix:** haystack = the actual paint documents only — `src/_defs.svg` plus the generated
`ink(mark, variant)` output for every pair (that is where `SHARED_HL` genuinely appears when
used). Keep `tile_colour()` as the one explicitly-derived exemption, with a comment saying so.
Compare case-insensitively. While in there: collapse the doubled draft of the same comment at
sheet.py:24-29 (two overlapping versions survived an edit).

### 6. Minor hardening (do in the same pass)

- `_layer()` indexes stop 3 (and 4 for layer 3) out of a regex over `_defs.svg`. Assert the
  expected stop count (6) before indexing, so a re-measured ramp fails loudly instead of
  shipping a neighbouring colour as the official swatch.
- `import re` twice inside functions in build.py — move to the top.
- `band()`: after the angle normalization `a1 > a0` always holds, so `sw` is constantly 1 —
  simplify or comment; today it reads like a case that can happen.

### 7. CI regen guard — the repo pattern this work is missing

build.py is deterministic and stdlib-only (verified). Add the standard drift guard next to the
glyphs/mirrors steps in `.github/workflows/ci.yml`:
`python3 docs/brand/build.py && git diff --exit-code docs/brand/dist`.
Do **NOT** guard sheet.py in CI — `family-sheet.svg` embeds rsvg-rasterised base64 PNGs, so its
bytes depend on rsvg/font versions. Say so in a comment where the guard is added.

## Deliberately NOT in this task

- **Any visual change** to full/compact/minimal/wordmarks/bricks — all dist/ SVGs except the four
  adaptive foregrounds byte-identical before/after (scoped `git diff` in the handoff).
- **Wordmark live `<text>` outlining** — stays a tracked known gap until the website ships.
- **Integration**: favicon.ico + site PNG set into `web/studio`, VectorDrawable conversion and
  launcher wiring in `app/` — separate tasks, to be specced once this lands. This task only makes
  the assets safe to integrate.

## Acceptance criteria

- §1 containment check and §2 id-uniqueness check both shown **red against the committed dist/**,
  then green after the fix (outputs in the handoff).
- Handoff shows one adaptive foreground before/after behind a round mask — the clipping is the
  point, make it visible.
- `git diff --stat` over dist/ shows exactly the four adaptive foregrounds changed.
- `--png` run: number of files written = number of renders; the duplicate-path assert is in.
- sheet.py completes without Pillow (skip note, exit 0) and with Pillow but no `reference/`
  (existing skip) — both demonstrated.
- Build run twice in a row → zero diff (determinism kept).
- README updated where it touches §1/§3 (safe-circle wording, PNG ranges).
- CI guard from §7 in place and green.
- Land through the gate as usual: reviewer verdict, `Approved:` trailer, fast-forward onto main.
