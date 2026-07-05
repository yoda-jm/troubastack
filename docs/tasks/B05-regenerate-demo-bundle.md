# B05 — Regenerate `docs/demo` via the real bake pipeline (retire the hand-bake)

**Priority:** B-track, after B04 · **Size:** XS/S · **Area:** `docs/demo`, `core` (script only)

## Context

`docs/demo/demo-concert.tstage` was **hand-baked** before the pipeline existed (see its
README) and still carries the pre-T16 em-dash mojibake in its rendered titles ("â€""),
visible on both Android and the IOS02 simulator proof. B02 made the real pipeline work
end to end (seed → `POST …/bake` → `.tstage` → Kotlin loader, live-verified at review),
so the hand-baked artifact's days were declared numbered by B02's own spec.

## Changes

1. Regenerate the committed demo bundle through the REAL flow: fresh seeded server →
   admin (`marie`) bakes the demo setlist → download the `.tstage` → replace
   `docs/demo/demo-concert.tstage`. Prefer doing this AFTER B04 lands (atomic bake
   publication) so the artifact comes from the hardened path; not a hard dependency.
2. Update `docs/demo/README.md`: it now documents provenance as "produced by the B02
   pipeline from the seed data" with the exact reproduction commands, replacing the
   hand-bake narrative (keep a one-line historical note).
3. Verify before committing: unzip → the Kotlin loader accepts it (the A02
   `FixtureBundleTest` pattern or the in-repo demo-bundle test if one exists), and the
   rendered titles show a real em-dash (pdftotext or a pixel check) — the T16 fix
   proving itself in the shipped artifact.
4. If any committed test/fixture pins the old bundle's hashes, regenerate those in the
   same commit (state which).

## Acceptance criteria

- `docs/demo/demo-concert.tstage` is byte-reproducible from the documented commands
  (modulo `bakedAt`; note the caveat) and loads with zero issues in the Kotlin loader.
- The Stage title renders "Wonderwall — Vocals" with a true em-dash (screenshot or
  pdftotext evidence in the PR).
- `make test` + app `:shared:check` green; README provenance updated.

## Out of scope

- Changing the demo's musical content; distribution (B03); bake GC (P202).
