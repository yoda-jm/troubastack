# T143 — Stage: say WHICH bake this is, let me delete one, and tell me when one updates

**Lane:** mobile. **Size:** S/M. **Status:** spec — three field reports from VLL's first rehearsal,
2026-09-05, all three confirmed in the source.

## The three symptoms, and what the code says

### 1. *"2 bake avec le meme nom … je ne sais pas quelle version, ni quel serveur, ni quel band"*

`ConcertRow` renders **`entry.label` and nothing else**. Meanwhile `bundle.json` carries `concertRev`
(observed incrementing 1→10 across a session), a distinct `bakedAt` per bake, plus `concertId` and
`roster`.

**The data can already tell two bakes apart — the row throws it away.** Show the rev and the bake time on
the row; that alone answers "which one is this?".

*What is genuinely missing from the format*: **band name and server origin**. Those are not in the
bundle at all, so "which server / which band" needs a format addition — a separate, larger decision.
**Do the display first**: it costs nothing and may well be enough.

### 2. *"je ne peux pas … le supprimer du device (manque un … ?)"*

The **⋮ menu exists** — it offers Freeze / Unfreeze / Pin this version / Unpin. **Delete is not in it.**
Deletion is implemented (`File(entry.dir).deleteRecursively()`) but reachable **only when
`entry.damaged`**, where it *replaces* the ⋮ instead of joining it.

So a healthy duplicate — exactly VLL's case — cannot be removed. **Put Delete in the ⋮**, with a
confirmation naming what is being removed.

**One caution the investigation turned up:** a re-import **destroys the server-side bakes** of the old
setlist (its directory is gone). The device may therefore hold the **only** copy of a bundle. The
confirmation should say so rather than treat a bundle as a cache.

**And respect `lean`:** in perform intent the row deliberately has *no* trailing controls. Managing
bundles belongs outside performance; do not add a delete affordance to the performing surface.

### 3. *"j'ai passé mon bake en auto update … j'ai pas vu de toast dans stage pour me dire que c'était mis a jour (j'étais dans le bake)"*

Confirmed **by absence**: `StageViewModel.applyUpdate` swaps the bundle in and emits nothing — there is
no toast, snackbar or message channel in the Stage view model at all.

The silence is deliberate *for the page*: the docstring's whole point is to swap "WITHOUT moving the page
the performer is on", and it carefully remaps position and per-song layer choices. **But nobody decided
the performer should not be told** — non-disruption of the page got conflated with saying nothing.

**The sheet changed under a musician mid-rehearsal. That warrants a word**, as long as the word cannot
steal the page: a brief, non-modal, self-dismissing indication naming what arrived (e.g. the new rev),
never a dialog, never anything that takes focus or covers music.

## Acceptance — RED FIRST (VLL)

- A row for two bundles with the same name renders **distinguishable** text (rev + bake time). Assert on
  two same-named fixtures; today they render identically, so the test fails first.
- Delete is reachable from ⋮ on a **healthy** bundle, is confirmed, and removes it; the list refreshes.
- Delete is still **absent** in `lean` (perform) mode — assert the negative, or the next change will
  quietly add it back.
- An applied auto-update produces a user-visible, self-dismissing notice, and the current page does not
  move (the existing remap behaviour must keep passing).

## Out of scope

Adding band + server identity to `bundle.json` — a format change, decided separately once the display
fix shows whether it is still needed.
