# T110 — Unit tests for the code that has none

**Priority:** normal, after T108 · **Size:** M · **Area:** `web/ink`, `web/studio` (Web & Core lane).
From the 2026-08-25 project audit, §4.3. This is the sharpest asymmetry in the repo: Go has 238 tests
and a conformance suite, the app has 169 JVM tests — and **studio and ink have zero unit tests**.

## 1. The problem

`web/ink` is *the* single authoritative stroke renderer (invariant I8, reused server-side by the bake
worker), and 713 lines of hit-testing geometry sit in studio. Both are validated **only** through a
serial browser suite at `workers: 1, retries: 0`, where one flake reds the push and a single assertion
costs tens of seconds. Pure geometry is the easiest code in the repo to test and currently the most
expensive to check.

## 2. What to build

**(a) Vitest**, wired into `make` and CI alongside the existing gates.

**(b) A first suite aimed at the pure, load-bearing functions** — the stroke geometry and
`strokeWidth` behaviour in ink, studio's hit-testing, and the DPR/canvas-budget arithmetic. Pick the
functions where a wrong answer is *silent* today.

**(c) Move the beat-vector test out of e2e** if it is a straight port — a shared-vector check does not
need a browser.

## 3. Rules

- **Do not restate the implementation in the test.** A vector with a hand-derived expected value is
  worth ten that recompute the function under test. The repo already has this idiom in
  `docs/contracts/*.vectors.json` — follow it, and prefer extending those files to inventing a
  parallel fixture format.
- **A test must discriminate.** For each new test, confirm that the obvious wrong implementation
  reddens it; if it doesn't, the test guards nothing.
- Don't duplicate e2e coverage — this is for what a browser test can only reach expensively.
- Keep it a *first* suite. Breadth beats a perfect corner: a thin test on ten functions is worth more
  here than an exhaustive one on two.

## 4. Acceptance criteria

- `vitest` runs from `make` and in CI, with its own job or step, and fails the build on red.
- A named set of pure functions in ink and studio is covered.
- For at least the geometry and `strokeWidth` cases, the teeth-check is reported: what wrong
  implementation was tried, and that it reddened.
- Full `make e2e` still green and its count reconciled.

## 5. Out of scope

Component/React rendering tests. Visual regression (`toHaveScreenshot`) — separate task, later.
Coverage measurement/thresholds.
