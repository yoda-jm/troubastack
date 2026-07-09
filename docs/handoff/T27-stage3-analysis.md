# T27 stage 3 — WIP branch: evidence pack for arch review

**Prepared by:** Web-Core (Opus)  ·  **Date:** 2026-07-10
**For:** arch (Fable) — independent review + go-forward call.
**Requested by:** VLL — *"ask again to Fable … a thorough opinion of what you did on that
branch, and how we can go forward, with a detailed analysis in a file that I can audit."*

This is a **handoff of evidence, not a verdict.** §A is reproducible fact. §B is Web-Core's
interpretation, explicitly flagged as *claims to verify* — arch owns the actual judgment.
VLL flagged (correctly) that Web-Core writing its own assessment is self-grading; so the
conclusions below are deliberately left open for arch.

**Branch:** `task/T27-stage3-fullscreen`, tip `3e9fe60`, rebased onto `main` `399904d`.
**Nothing landed. `main` is untouched.**

---

## A. Facts (reproducible — commands in §C)

### A1. Diff shape vs main
`git diff --stat origin/main...HEAD` → **14 files, +217/−62**:
- **Source (5 files, +105/−29):** `Shell.tsx` (+9/−1), `usePdfDocument.ts` (+8/−1),
  `Toolbar.tsx` (+4), `Viewer.tsx` (+30/−4), `styles.css` (+83/−16).
- **Specs (9 files, +112/−33):** `editor.spec`, `editor-active-layer`, `editor-ed5`,
  `editor-features`, `editor-locked-restyle`, `editor-noflicker`, `editor-pick`,
  `editor-uxfix`, `editor-zorder`.

### A2. What the source changes are
- `Shell.tsx`: hide the app top bar on the editor route (`fullbleed` regex) → class
  `shell-fullbleed`.
- `usePdfDocument.ts`: `const MAX_FIT_SCALE = 2.3` capping the fit-width scale.
- `Toolbar.tsx`: `if (neutral) return null` — the style row renders nothing when Select is
  active with nothing selected.
- `Viewer.tsx`: wrap header+toolbar+zoom/files in `.viewer-chrome`
  (`data-testid="viewer-chrome"`); a ResizeObserver publishes its height as `--chrome-h`.
- `styles.css`: `.shell-fullbleed` (100dvh, overflow hidden); `.viewer-chrome` (absolute,
  centered, glass, `pointer-events:none` with `auto` on its controls); `.viewer-scroll`
  fills height and top-pads by `--chrome-h`; `.viewer-sidebar` absolute (top-right).

### A3. Assertion diff (the assertion-freeze question)
Whitespace-normalized set of `expect(...)`/`expect.*` lines, `main` vs branch, per spec:
**all 9 files report identical** (`ALL_FROZEN=1`). The added spec lines are of two kinds:
1. `const top = Math.max(box.y, chrome ? chrome.y + chrome.height + 6 : 0)` (chrome-inset
   band top) in `editor`, `editor-features`, `editor-noflicker`.
2. A `pageFracPoint`/scroll-into-view helper (`viewer-scroll.scrollBy`) in `editor-pick`,
   `editor-ed5`, `editor-uxfix`, `editor-zorder`.
Reproduce with the loop in §C — it prints `FROZEN`/`CHANGED` per file.

### A4. Typecheck
`npx tsc -b` → clean (exit 0).

### A5. e2e baseline — **RAW, full run** (`npx playwright test editor box-render viewer`)
**35 passed / 10 failed (9.8m), retries:0.** The 10 failures:

| # | Spec | wall |
|---|---|---|
| 1 | `editor-ed5.spec.ts:225` dragging a small rect's center moves it | 17.8s |
| 2 | `editor-ed5.spec.ts:300` Highlight vs Outline preset | 19.4s |
| 3 | `editor-features.spec.ts:275` resize handles resize | 17.1s |
| 4 | `editor-locked-restyle.spec.ts:238` locked object can't move/delete | **30.4s (timeout)** |
| 5 | `editor-locked-restyle.spec.ts:322` clicking an object focuses its layer | **30.4s (timeout)** |
| 6 | `editor-locked-restyle.spec.ts:355` editable object reflects/restyles | **30.4s (timeout)** |
| 7 | `editor-noflicker.spec.ts:93` add/move does NOT re-rasterize | **30.4s (timeout)** |
| 8 | `editor.spec.ts:134` draw rect+freehand+text persists | **30.6s (timeout)** |
| 9 | `editor.spec.ts:175` realtime: A draws → B sees it | **30.0s (timeout)** |
| 10 | `editor.spec.ts:228` select + delete removes an object | **30.5s (timeout)** |

**Correction to an earlier claim:** an earlier note (and the first draft of this file) said
"4 known failures." That was a run I **killed at test ~20**, not a complete run. The
complete run above is **10 failures**, seven of them 30s **timeouts**, including a core
invariant (`editor-noflicker`) and core flows (draw-persist, realtime, select+delete). The
branch is **not near green.**

### A6. Zero-shift close-out
`editor-zeroshift.spec.ts` currently contains only the **draw-tool** no-shift test (live,
passing in A5's run). The **panel-toggle** zero-shift `test()` that Fable named as the
stage-3 close-out (guardrail #3) is **not written yet.**

---

## B. Web-Core's reading — CLAIMS TO VERIFY (arch owns the verdict)

These are hypotheses, not findings. Each is stated so arch can confirm or refute.

- **B1 (assertion-freeze):** *Claim* — the spec diff changes only helper mechanics, no
  assertion. Evidence: A3. *To verify:* arch's own diff of the `expect` lines.
- **B2 (nature of the failures):** *Claim* — the 10 failures are un-migrated mechanics
  (targets landing off the visible band / under the floating chrome), not product
  regressions. *Unverified risk:* the seven 30s **timeouts** may indicate something
  stronger than "coords land off-band" — e.g., a target that never scrolls into reach, an
  overlay intercepting pointer events, or (worst case) a real interaction regression. I have
  **not** proven these are purely mechanics. This needs arch's eyes / a trace review before
  anyone claims the invariants hold.
- **B3 (no-reraster invariant):** `editor-noflicker:93` is currently **failing (timeout)**.
  Whether the invariant itself is intact or the test just can't reach the target under the
  new layout is **unknown** from a red/timeout result alone.
- **B4 (contextual style row):** making the style row `neutral → null` is what the mockup
  shows; it is also plausibly why some `editor-locked-restyle` / style-readout paths can't
  find controls that are no longer rendered in the neutral state.

## C. How to audit / reproduce

```bash
git fetch origin && git checkout task/T27-stage3-fullscreen   # tip 3e9fe60
git diff --stat origin/main...HEAD                            # A1
git diff origin/main...HEAD -- 'web/studio/src/**'            # A2

# A3 — assertion-freeze proof (prints FROZEN per spec):
for f in $(git diff --name-only origin/main...HEAD -- 'web/studio/e2e/*.spec.ts'); do
  diff <(git show origin/main:"$f" | grep -oE 'expect[.(].*' | tr -d '[:space:]' | sort) \
       <(git show HEAD:"$f"        | grep -oE 'expect[.(].*' | tr -d '[:space:]' | sort) \
    >/dev/null && echo "FROZEN  $f" || echo "CHANGED $f"; done

cd web/studio && npx tsc -b                                   # A4
npx playwright test editor box-render viewer                  # A5 (needs :8080 free; ~10m)
```

Mockup ground truth: `scratchpad/editor-redesign.html` + renders `editor-light.png`,
`editor-draw.png`, `editor-wide.png`, `editor-tablet.png`, `editor-phone.png`.

## D. Design-vs-mockup gap (fact)

The branch floats + contextually-hides the chrome, but does **not** yet reproduce the
approved mockup's visual language: the mockup has a **pill** top bar (back·title·tools·zoom
·Layers/Notes/Details toggles), a **separate floating bottom pill** (file tabs + `N objects
· ● live`), and a **tabbed `Layers | Annotations` drawer**. The branch keeps the old
stacked header/toolbar/zoom/files in one rounded-rect and the old stacked sidebar. VLL's
note *"the bottom and top flyout do not look like the design"* still stands.

## E. Open questions for arch

- **Q1 (ordering):** design reshape → migrate helpers once, or land a functional close-out
  on the *current* chrome first (get green sooner) then polish? (Note A5: "current chrome"
  is 10-red today, so "green sooner" is not free either.)
- **Q2 (control relocation):** the mockup moves `active-layer`/`new-layer` into the drawer
  and delete into the selbar. Specs expect them always-visible. Relocate (wider mechanics
  touch) or keep always-mounted (less churn)?
- **Q3 (contextual style row):** confirm adapting the affected specs to activate a tool /
  make a selection before reaching style controls is a sanctioned *flow change* (guardrail
  #4), vs. a signal the contextual-hide is too aggressive.
- **Q4 (the timeouts):** do you want a trace/pixels pass on the seven 30s timeouts before
  any further work, to rule out a real regression hiding behind "just mechanics"?
- **Q5 (`MAX_FIT_SCALE = 2.3`):** acceptable product default (VLL validated ~229%) or make
  it configurable?

---
*Web-Core is holding. No further branch work until arch rules on the go-forward.*
