# T189 — `?bake=1` opens a concert's bake dialog

**Lane:** web-core · **Status:** specced 2026-10-09, not started. Small, with no dependency. **A83** (mobile)
links to it · **Origin:** VLL, 2026-10-09, on A83: the app's concert ⋯ menu gets *Bake*. He chose that it opens
**Studio's own bake dialog** instead of a native copy, so P205's rule (*never capture layer defaults
silently*) stays in one dialog in one place.

## 1. What to build

`SetlistDetail` (route `/bands/{bandId}/setlists/{setlistId}`) honours a query parameter `bake=1`:

- Once the page has loaded the setlist and the caller's role, **open the existing `BakeDialog`**, exactly as
  the page's own Bake button does, with the same `songIds`, `onBake` and remembered layer defaults. **Do not
  add a second bake path.**
- **Open it once.** Then remove `bake` from the URL with a *replace* navigation (keeping any other parameters,
  notably the embedded-mode one the app adds, `embeddedUrl` in `EditorUrl.kt`). A reload, or a back and
  forward, must not reopen the dialog.
- **Ignored, silently, when the dialog would not be allowed:** the caller is not an admin, or the concert has
  no songs (`bakeSetlistDisabled`). The page shows as normal, with no error banner. The parameter is a
  convenience, not a command. Still strip it.
- Any other value (`bake=0`, `bake=yes`) is ignored and stripped.

**Server: no change.** Authorisation stays where it is. The bake endpoint is admin-only whatever the URL
says.

## 2. Not in this task

- Opening the dialog from the concert *list* page (`Setlists.tsx`). Only the detail route.
- Auto-confirming the bake. The dialog always waits for the user's **Bake**.

## 3. Acceptance (e2e, studio)

- **As admin:** `/bands/{b}/setlists/{s}?bake=1` shows the bake dialog. The URL no longer contains `bake=`.
  Reloading does **not** reopen the dialog.
- **Embedded mode:** with the embedded parameter also present, the dialog opens, and the embedded parameter is
  **still in the URL** after the strip.
- **As a non-admin member:** the page renders, there is no dialog and no error banner, and `bake=` is stripped.
- **Empty concert, as admin:** no dialog, and the parameter is stripped.
- Cancelling the dialog leaves the user on the concert page. Completing a bake behaves as the Bake button
  does today.
- **Teeth.** Remove the strip. Confirm the reload assertion goes **red**, and print the removed line.
  Restore it.
