# T140 — Importing a band SILENTLY SCRAMBLES every setlist's order

**Lane:** web-core (core). **Size:** S. **Status:** fixed 2026-09-05 (web-core) — `parseV2` now sets
`Position: idx` from the folder's array order, so imported setlist items no longer collapse to Position 0.
Teeth-checked: `TestImport_PreservesSetlistOrder` (12 items in a non-monotonic order) goes RED when the
assignment is reverted. Awaiting reviewer re-verify. **Severity: HIGH — silent data loss, already hit VLL
in performance conditions.**

## What happened, in the field

VLL's first rehearsal with TroubaStack. Before it, his concert's songs were **in the wrong order**;
mid-afternoon they had been right, and he had a bake proving it.

Reconstructed from the surviving bakes (each `.tstage` is a timestamped snapshot of the order):

| when | order starts | |
|---|---|---|
| folder @ 16:21 | `song A \| song B \| song C` | correct |
| bake 1 @ 20:42 | `song A \| song D \| song E` | **scrambled** |
| bakes @ 20:52+ | `song A \| song B \| song C` | he re-ordered by hand |

Same songs, different order. Between the two, the band was **re-imported** — the band id changed
(`0cf20569` → `fa8eb007`), the setlist id changed, and `bakes/` was recreated at 20:42.

## Root cause — one missing field

`bandio_v2.go`, reading `setlists.json`:

```go
msl.Items = append(msl.Items, manifestItem{
    SongRef: songID, KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride,
    Notes: it.Notes, OnCall: it.OnCall, TransposeChords: it.TransposeChords,
})   // ← Position is never set ⇒ 0 for EVERY item
```

`v2SetlistItem` has no `position` field **by design** — the folder expresses order as **array order**, which
is right for a hand-written file. But the reader must then *materialise* that order into `Position`, and it
does not. `ImportBand` faithfully writes `Position: it.Position` — all zeros — and the retrieval order
becomes whatever the store yields.

## Reproduced on the real library

Seeded a scratch server from the real folder:

```
23 items → ALL at position 0
stored order : song G | song C | song F | …
folder says  : song A | song B | song C | …
ORDER PRESERVED? false
```

## Fix

Set `Position: idx` from the array index in that loop. That is the whole change; the folder's array order
becomes the stored order, which is what every other layer already assumes.

## Acceptance

- **Round-trip order**: import a folder whose setlist has a deliberately non-alphabetical order, and the
  stored items come back in **exactly** that order. **Teeth-check it** — drop the `Position` assignment and
  confirm the test goes red, because an all-zero set can still *happen* to come back in order on a small
  fixture. Use ≥10 items and an order that no sort would produce.
- **Positions are distinct** after import (no two items share one).
- A bake taken straight after an import has the folder's order — the assertion closest to what failed.
- `-band` seeding of the real library preserves order (it goes through this path).

## Why this one is worth a post-mortem line

The data was correct in the folder, correct on the way in, and correct in every layer that *reads* it. One
unset integer between them, and a concert's running order was silently rewritten — discovered not by a
test but by a musician in front of his band.
