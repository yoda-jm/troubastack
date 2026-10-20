# A90 — A concert on the server can always be downloaded on demand (defect) — ABSORBED INTO A91

> **Superseded 2026-10-20 (VLL): absorbed into `A91-one-concert-list.md`. Do not implement separately. A91 carries
> this cause, this fix and these red-first tests.**

Owner: mobile. Spec: Fable, 2026-10-20. Type: **defect** (VLL: "this is super strange, and I have no way to force a
download… this is a defect").

## What happened

A concert existed on the server, freshly re-baked: rev 7, listed by `GET /api/bands/{id}/concerts`. It was NOT on the
tablet, and the tablet offered no way to get it.
- Another concert, never installed on that tablet, was offered and downloaded fine the same evening. So the manifest
  fetch works.

## Cause (read in code)

`UpdatesManager.diff()` (`app/shared/.../distribution/Updates.kt`), from the A43-fix:

```kotlin
if (local == null) {
    if (book[c.concertId]?.installedOnce != true) out += Availability.NewlyAvailable(c.concertId)
    continue
}
```

- A concert that was installed once and then removed from the device is **never surfaced again**.
- `diff()` is the only path to a download. So the anti-nag rule ("don't re-offer what the user deleted") became
  **"can never be downloaded again"**.
- The rule was right about *nagging* (no chip, no banner) and wrong about *availability*.

The policy book is stored encrypted (`troubastage.secrets.enc.xml`), so this cannot be confirmed on the device by
reading. The behaviour matches only this path.

## Required behaviour

1. **Every concert in the manifest that is not on the device is downloadable, explicitly, from the concert list.**
   - Show such concerts in a quiet "On the server" (or "Not on this tablet") section, or as rows with a **Download**
     action. Use whichever fits the A83 concert list.
   - Never as a chip or banner. The anti-nag stays: a deleted concert does not *announce* itself.
   - Downloading one clears its "deleted" quietness: it is installed again, and updates are offered as usual.
2. **A manual "Check for updates / Refresh" on the concert list** re-fetches the manifest on demand, and offers
   updates for installed concerts even if the periodic check hasn't run. If such a control exists already, make sure
   it reaches this path.
3. **Keep:**
   - FROZEN, LocalPin and `finalLocked` still suppress UPDATE offers for an installed concert;
   - an explicit Download of a not-installed concert is not an update, and is never suppressed.

## Tests (pure, `commonTest`, red-first)

- A concert with `installedOnce=true` that is not on disk is still returned as **downloadable** by the new
  listing/action path, and is NOT returned as a `NewlyAvailable` chip. Show that this goes red on today's code.
- A never-installed concert: still `NewlyAvailable` (no regression).
- An explicit download of a previously deleted concert imports it, then `diff()` offers its next rev as an update.
- FROZEN or pinned installed concert: no update offer (no regression).

## Device pass (VLL)

On the tablet: delete an installed concert → it is no longer announced. It is listed under "On the server" with
Download → download it → it opens.

## Out of scope

Studio-side changes. The server already lists every concert.
