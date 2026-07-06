# B05 — Regenerate `docs/demo` via the real bake pipeline (retire the hand-bake)

**Priority:** B-track, after B04 · **Size:** XS/S · **Area:** `docs/demo`, `core` (script only)

## Context

`docs/demo/demo-concert.tstage` was **hand-baked** before the pipeline existed (see its
README) and still carries the pre-T16 em-dash mojibake in its rendered titles ("â€""),
visible on both Android and the IOS02 simulator proof. B02 made the real pipeline work
end to end (seed → `POST …/bake` → `.tstage` → Kotlin loader, live-verified at review),
so the hand-baked artifact's days were declared numbered by B02's own spec.

## Status (2026-07-06) — BLOCKED on scoping; needs a Fable decision (not executed)

Scoped from the web-core lane; **not executed** — three conflicts between this spec and
the shipped B02/B04 pipeline mean regenerating the artifact now would silently change
what the demo shows and still miss the acceptance. Reporting rather than approximating:

1. **Wrong part.** Acceptance wants the Stage title "Wonderwall — Vocals". But the Baker
   (B02 v1) bakes each song's **default shared-pool file** = lowest DisplayOrder = the
   seed's Wonderwall **index 0 = "Wonderwall — Score"** (3 pages), not the Vocals part
   (index 1). Baking Vocals needs the **per-member/my-files bake** that B02 explicitly
   deferred ("Per-member my-files bakes are a product decision for later").
2. **No single demo setlist.** Today's `docs/demo/demo-concert.tstage` is a single
   Wonderwall bundle. The seed's only marie setlist is **"Sat @ The Anchor" (3 songs:**
   Wonderwall, Hallelujah, Black Hole Sun). Baking it yields a 3-song concert baking
   default parts — different musical content, which this spec lists as out of scope.
3. **Reproducibility.** "byte-reproducible modulo bakedAt" is not achievable: `concertId`
   is the setlist's server UUID and `songId`s are server UUIDs too — all random per seed
   run. Reproducible only modulo bakedAt **and** those IDs.

**Decision needed (Fable):** pick one, then this becomes executable — (a) add a dedicated
single-song demo setlist to the seed whose default pool file IS the Vocals part (so v1
bakes exactly the current demo); (b) accept the demo becoming the full "Sat @ The Anchor"
3-song concert (relax acceptance #2 + the "Wonderwall — Vocals" title); or (c) land the
per-member/my-files bake first (its own task) and bake marie's my-files view. The em-dash
fix (T16) is ready to prove itself either way once the source part is settled.

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

## Decision (2026-07-06, architect): option (b) — the demo becomes the real concert

The demo's job is to showcase the product; a **genuinely-baked 3-song concert**
("Sat @ The Anchor": Wonderwall, Hallelujah, Black Hole Sun — default parts) does that
strictly better than the hand-baked single song: it exercises the multi-song pager, the
setlist metadata (key/notes/tempo — which A08 will render), and it is the exact artifact
the real pipeline produces. Option (a) adds seed complexity to preserve a historical
artifact's shape; option (c) gates a demo refresh on a whole product feature —
disproportionate. The out-of-scope "don't change the demo's musical content" was drift
protection, waived here deliberately.

**Amended acceptance criteria (supersede the originals):**
- `docs/demo/demo-concert.tstage` = the bake of the seeded "Sat @ The Anchor" (3 songs,
  default parts), file name unchanged; loads with zero issues in the Kotlin loader.
- Title evidence: the Stage/PDF title shows **"Wonderwall — Score"** with a true em-dash
  (the default part; T16 proving itself in the shipped artifact).
- Reproducibility, honestly stated: identical modulo `bakedAt` **and server-assigned
  concert/song UUIDs**; the exact reproduction commands live in `docs/demo/README.md`
  (which drops the hand-bake narrative, keeps a one-line historical note).
- Check `README.md`'s quick-start still reads correctly (the import flow and filename
  are unchanged; update the one-song description if it exists).
- `make test` + app `:shared:check` green.
