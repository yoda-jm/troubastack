# T154 — Re-ticking a file in "my files" does not take the second time

**Lane:** web-core (studio). **Size:** S. **Status:** STILL CANNOT REPRODUCE after Fable's CORRECTED repro
2026-09-06 (web-core) — no fix shipped; likely the live VIEWER STRIP, not the checkbox (see the gate).
Fable corrected the sequence (my ⟨R1⟩ described a path VLL never took): two files both ticked, untick A
(→ `PUT [B]`, never empty), then IMMEDIATELY re-tick A. I drove that EXACT sequence three ways — two awaited
clicks, a synchronous double-click (React 18 flushes discrete clicks so no stale-closure batch), and the
same under a 1s-delayed PUT — all GREEN at checkbox + server + reload. The MyFilesEditor checkbox is local
optimistic state and robust; the strip refresh (`refreshMyFiles`) reads fresh server state sequentially
after each write, so by inspection it should also be correct. So the defect is not where the ticket points.
NEW hypothesis for the gate: "reselect/pin" = the file reappearing in the live VIEWER STRIP (the parent),
which my tests do not assert — that is the layer to target next. Guard `my-files-retick.spec.ts` (3 tests)
covers the checkbox+server+reload path.
Landed `e2e/my-files-retick.spec.ts` as a regression guard: the 3-step include→exclude→include **through the
saved-empty state** ([]→[A]→[]→[A]), asserting the live checkbox AND server persistence AND survival across
a reload — plus a rapid re-tick under a 1.2s-delayed PUT (the race window). Both pass. The current code
already carries the guards that counter the described race: `MyFilesEditor` is `memo`'d on `(bandId,songId)`
so a parent refresh can NOT re-render it (the checkbox is purely local `included` — a revert from stale
props is structurally impossible); the seed effect is one-shot on `[bandId,songId]`; and `drain` never
re-seeds local state on a successful write (only reconciles on failure). Those landed 2026-09-04 (T82/T82b),
before this filing. So the "re-seed from a stale read" mechanism the spec hypothesises does not fire. Either
recent work already closed it, or the live repro needs a condition I did not hit (a specific song/file+layer
shape, a tab remount, or VLL's browser/network) — I did not guess-fix. **The server is not the bug — Fable
proved it.**

## What VLL reported

*"selecting a song for 'me' pin it the first time, but then after deselecting it does not reselect it the
second time."*

Tick a file → it sticks. Untick it → fine. Tick it again → **it does not take.** A toggle that only works
once is worse than one that never works: the first success teaches you to trust it.

## The server round-trips correctly — reproduced on a scratch instance

I drove the exact sequence against a throwaway server (seeded demo, not VLL's data):

```
initial                       -> 1 file
PUT fileIds:[A]               -> 1 file
PUT fileIds:[]                -> 0 files      (an empty selection is stored, not deleted)
PUT fileIds:[A]               -> 1 file       ← the step VLL says fails
```

So `SetMyFileSelection` stores an empty list rather than deleting the row, `MyFileSelection` distinguishes
*saved-empty* from *never-set*, and re-selecting persists. **The defect is client-side, in Studio.**

## Where to look, and the reading that fits

`MyFilesEditor.toggleInclude` itself is correct (copies the Set, toggles, `setIncluded`, `schedule`). So
look at the interaction between the **optimistic local state** and the **reload effect**:

- `included` is seeded once from `selected`, and the load effect also calls `setIncluded(incSet)`.
- After a successful write `drain` calls `await onChanged()`, which refreshes the parent (the viewer strip).
- If that refresh re-runs the load effect, it **re-seeds `included` from the server** — and if the read is
  even slightly behind the write it has just made, the freshly ticked box is reverted.

That shape explains "works once": the first tick moves from *unset* to *custom*, the second from *saved
empty* back to *custom* — different starting states, different races.

**Do not fix by removing the reconcile-on-failure path** (that exists so a rejected write cannot leave the
UI lying). Fix the ordering so a success never re-seeds local state from a stale read.

## ⟨R1⟩ Red first

The e2e must drive the **full three-step sequence**, because two steps pass today:

1. tick a file, assert it is checked **and** persisted (reload, still checked);
2. untick it, assert unchecked and persisted;
3. **tick it again, assert checked and persisted.** Red today.

**Teeth-check:** the same test with only steps 1–2 must pass on the current code — if it fails, the fixture
is not reproducing VLL's path.

## Out of scope

The empty-selection confirmation and the default-file rule (T138) — both correct and unrelated.
