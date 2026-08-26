# A47 — `:androidApp` has no test source set

**Priority:** normal · **Size:** S · **Area:** `app/androidApp` (Mobile lane).
From the audit's §4.3 testing bullet, and the half T109 did not take. T109 gave `internal/sync` its first
in-package tests; the same bullet also named this, and it is still true.

## 1. Measured today

`app/androidApp/src/` contains exactly two source sets:

```
  debug
  main
```

**There is no `test`.** So the Android-only pure functions the audit named — `sessionCookieFor`,
`safeSegment` — have never been executed by a test, on any machine.

This is the app-side twin of what T110 found in studio: the easiest code in the module to test, and the
only code with no way to run a test at all.

## 2. What to build

**(a) A `test` source set** on `:androidApp`, wired into `:androidApp:test` and into whatever CI job
already runs `:shared:check`, failing the build on red.

**(b) A first suite on the pure functions**, chosen the way T110 chose: the ones whose wrong answer is
**silent**. `sessionCookieFor` and `safeSegment` are the two the audit named; take others if they meet
that bar.

## 3. Rules

- **Per-test teeth-check, reported.** For each test: what wrong implementation you tried and that it
  reddened. This is the standard T110 set and it is why T110's suite is trustworthy.
- **`safeSegment` is a sanitiser — test what it REJECTS.** A sanitiser test that only feeds it clean
  input passes against `return input`. If it guards path traversal or separators, the vector must be an
  input that is genuinely dangerous, and the expected output must differ from the naive-wrong one.
- **Pure functions only, for now.** No Compose UI tests here — that is a bigger rig and a separate task.
- **Read the test-results XML, not the exit code.** Two targets double the counts; the summary is what
  reconciles.

## 4. Acceptance criteria

- `:androidApp` has a test source set that runs in CI and fails the build on red.
- A named set of pure functions covered, with the teeth-check reported per test.
- `sessionCookieFor` and `safeSegment` are among them, and `safeSegment`'s vector is a rejection case.
- `:shared:check` still green; test-results XML counts reported, not inferred.

## 5. Out of scope

Compose UI tests. iOS test execution — that needs a macOS runner and is **not lane work** (see the note
in `reviews.md`, 2026-08-27). Coverage measurement. Visual regression.
