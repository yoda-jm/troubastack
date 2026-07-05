# T18 — De-dup the Go `ConcertBundle` mirrors (bake ↔ mkbundle)

**Priority:** anytime, XS filler · **Size:** XS · **Area:** `core/internal/bake`, `core/cmd/mkbundle`

## Context

B02 introduced `core/internal/bake/bundle.go` — the REAL producer's Go mirror of proto
`ConcertBundle` (AUTHORITY comment, canonical JSON, deterministic `.tstage` writer). Its
own NOTE records that `core/cmd/mkbundle` (the A03 dev-only fixture generator) carries a
**parallel copy** of the same structs, kept separate at the time "to avoid perturbing
the committed fixtures". Two hand-written mirrors of the same proto in the same language
is exactly the I1 drift class we keep paying for — and this pair is trivially unifiable
because `cmd/` may import `internal/`.

## Changes

1. Make `core/cmd/mkbundle` import and use `internal/bake`'s `ConcertBundle` (+
   `MarshalCanonical`, `WriteTstage`, `Sha256Hex`) and delete its private copies.
2. Regenerate the committed fixtures (`make fixtures`) in the same commit. Expect
   byte-identical output; if anything differs, STOP and report the diff instead of
   committing it — a difference means the two mirrors had already drifted, which is a
   finding in itself.
3. Keep mkbundle's fixture-specific logic (synthetic content, torture cases) local to
   mkbundle — only the container/manifest types and writers unify.

## Acceptance criteria

- Exactly one Go mirror of proto `ConcertBundle` remains (grep: `AUTHORITY.*bundle.proto`
  matches only `internal/bake/bundle.go`).
- `make fixtures` produces no diff in committed fixtures (or the drift is reported, not
  committed).
- `go build/vet/test ./...` green (incl. mkbundle + A02 loader tests via CI's android job).

## Out of scope

- The TS/Kotlin mirrors (that unification is P203's codegen decision); any bundle
  format change.
