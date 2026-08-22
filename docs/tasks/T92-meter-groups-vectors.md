# T92 — Pin the metre parser with shared vectors, before a third runtime copies it

**Priority:** high — a latent cross-runtime drift, and A35 is about to widen it · **Size:** S ·
**Area:** `docs/contracts`, `core` (a Go test), `web/studio` (a TS test), CI drift-guard. Lane:
Web & Core. Filed by Web-Core from Fable's T86-studio review finding (2026-08-22).

## Why

The visual beat is safe from cross-runtime drift because `beatPhase` is pinned to
`docs/contracts/beat-phase.vectors.json`, run by both TS and Kotlin — the two runtimes cannot
silently disagree about *when a beat is*. But the metre parser that decides the **groups feeding
beatPhase** has **no shared vectors at all**. It exists twice today —
`app.ParseMeter` (Go, T86 core) and `meterGroups` (TS, T86 studio) — and they agree (Fable ran a
31-case probe against both). **A35 adds a third copy in Kotlin.**

Three independent implementations of one deliberately **lenient** parser, with nothing pinning them,
is exactly the drift the beat-phase vectors exist to prevent — and leniency makes it *worse*: a
disagreement does not throw, it silently returns 4/4, so the beat is quietly wrong in one runtime
only, with no error anywhere. The moment to close this is **before A35 lands**, not after.

## The fix

Add `docs/contracts/meter-groups.vectors.json` — the metre-parser analogue of the beat-phase
vectors — and run it from **every** runtime that parses a metre, on the same mirror + CI-drift-guard
pattern A34 established for `beat-phase.vectors.json` (`docs/contracts` == the runtime that consumes
it, checked in CI).

Shape (a flat table; `groups: null` means unset → 4/4, since the parser never throws):

```json
{ "cases": [
  { "meter": "4/4",   "groups": [1,1,1,1] },
  { "meter": "6/8",   "groups": [3,3] },
  { "meter": "3+4/8", "groups": [3,4] },
  { "meter": "x/y",   "groups": null },
  …
] }
```

- **Seed set:** Fable's probe table, already run against Go and TS — **13 valid** (`4/4`, `3/4`, `2/2`,
  `6/8`, `9/8`, `12/8`, `5/4`, `3/8→[1,1,1]` (3 is not > 3), `3+2/8`, `3+4/8`, `2+2+3/8`, `3+3+1/4`,
  whitespace `" 6 / 8 "`) and **18 malformed** (`""`, `"x/y"`, `"4/5"`, `"0/4"`, `"33/4"`, `"3+0/8"`,
  `"-3/4"`, `"-3+4/8"`, `"+4/8"`, `"3+/8"`, `"4/4/4"`, `"4.0/4"`, `"٤/٨"` non-ASCII digits, a 17-group
  additive, and the rest) — each malformed case `groups: null`.
- **Go:** replace/extend `meter_test.go` to load the JSON and assert `ParseMeter` against it (keep the
  in-code table if you like, but the shared file is the source of truth).
- **TS:** the studio already has `beat.spec.ts` reading `beat-phase.vectors.json`; add a sibling
  reader for this file asserting `meterGroups` (via the existing `tsconfig.contract` compile unit).
- **A35 (Kotlin):** consumes the same file in `commonTest` — that is the whole point; note it as the
  cross-lane dependency but the Kotlin run is A35's to add.

## Acceptance criteria

- `docs/contracts/meter-groups.vectors.json` exists with ≥ 13 valid + ≥ 18 malformed cases.
- Go `ParseMeter` and TS `meterGroups` **both** run it and pass; the count is asserted so a truncated
  file fails loudly.
- **Red-first:** a deliberate off-by-one in either parser (e.g. compound `n/3` → `n/2`) fails that
  runtime's run against the shared file; record it.
- CI drift-guards the file the same way it guards `beat-phase.vectors.json` (the mirror-consistency
  check), so a hand-edit that desyncs a runtime is caught.
- `gofmt`/`go vet`/`make test` and `tsc -b studio` green.

## Out of scope

- Changing the parser's behaviour — this only *pins* the existing lenient contract; any behaviour
  change is a separate task with its own vector update.
- The Kotlin run (A35 adds it against this same file).
