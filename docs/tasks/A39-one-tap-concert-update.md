# A39 — One tap from Home to get the current concert onto the device

**Priority:** normal · **Size:** M · **Area:** `app/shared` (Home), `app/androidApp` (transport,
MainActivity), iOS host. Lane: Mobile. **After A38** — both land on Home and share its connection
state.

VLL, 2026-08-22: *"the bake seems far-fetched — if connected it should be one click from the Home page
or something like that, at least way easier than going into some Studio menu."*

## The ask is right; the verb is wrong — and the code says why

I went looking for how to put "bake" on Home and found three facts that change the answer:

1. **Baking is admin-only.** `bakeapi.go:90–98` — the band-wide bake *is* the bake (P205) and
   `role != app.RoleAdmin` → 403 (I11). A **Bake** button on Home would be dead for every ordinary
   band member, which is most people holding a phone at rehearsal.
2. **Nothing tells the app a bake is even needed.** There is no dirty/stale signal on a setlist
   anywhere in the API (grepped). So a Home "Bake" could only ever re-bake unconditionally — a fresh
   revision on every tap, whether or not anything changed, onto a code path with a **known concurrency
   race** (`TestBake_ConcurrentSameSetlist_distinctRevs` is a documented flake over a real race in
   `baker.go`). Putting a phone button on that is asking for it.
3. **"Is my copy out of date?" is already answerable, with no server change.**
   `GET /api/bands/{bandId}/concerts` returns `currentRev` per concert *and* a per-song `rev`
   (`bakeapi.go` `concertView`; mirrored in `AvailableConcert`). The app already downloads through
   `downloadBundle` (`HttpTransport.kt:174`). Everything needed is in place.

**So Home gets "Update", not "Bake".** That delivers exactly what VLL described — one tap, no Studio
menu — and it works for *everyone*, not just admins. What he is feeling is the effort of getting the
current concert onto the device, and re-baking is not what removes that effort; fetching is.

## Part A — the deliverable: "Update" on Home

A single Home action, next to the connection row A38 introduces.

- **Recognized and up to date** → a quiet "Up to date" state, no shouting.
- **Recognized and something is newer** → **"Update"** with what is waiting: *"Sat @ The Anchor —
  new version"*, or *"2 concerts to update"*. One tap downloads and installs.
- **In flight** → progress, and it must be **cancellable**; bundles are large and this may be a phone
  on venue wifi.
- Comparison is `currentRev` from `listConcerts` versus the rev of what is on disk. **Do not compare
  timestamps** — `updatedAt` is a display field; `currentRev` is the truth.
- After updating, Home's resume/concert count reflects the new state without needing a restart.

### How it degrades — the part that must not be hand-waved

| situation | what Home shows |
|---|---|
| **Offline** (A38's Offline) | no Update action; the existing "concerts on device still work" reassurance stands. **Never** an error — this is the normal gigging case (I12). |
| **Guest** (A38's Guest) | no Update action; the connection row already offers Sign in / Connect, which is the actual next step. Do not duplicate it. |
| **Recognized, no band membership / no concerts** | nothing to update; say so plainly once, don't render a dead button. |
| **Download fails mid-way** | the previously installed concert **must remain playable**. Never leave a half-written bundle where the old one was. |

That last row is the one that matters most: a failed update the night before a gig must not cost the
player the copy they already had.

## Part B — "Bake now", and why it is NOT in this task

If VLL still wants the app to *produce* a bake, it needs two things first, and both are server work:

1. **A dirty signal** — the setlist API saying "there are changes since the last bake". Without it the
   button either lies or re-bakes needlessly.
2. **A decision about the baker race** above, since a phone button raises the odds of concurrent bakes
   on one setlist.

It is also **admin-only**, so it belongs wherever admin actions live, not on the Home surface every
member sees. **File it separately if wanted; do not build it here.** If VLL's real complaint turns out
to be "publishing a setlist from the studio is buried", that is a *studio* task and I will spec it —
tell me and I will.

## Acceptance criteria

- One tap from Home downloads and installs a newer concert; assert the on-disk rev advances to the
  listed `currentRev`.
- **Up-to-date shows no action** — assert the button is absent, not disabled-and-dead.
- **Interrupted download leaves the previous bundle intact and playable** — kill the transfer mid-way
  and assert the old concert still opens in Stage. Test it; do not reason about it.
- All four degradation rows asserted, especially **Offline is not an error**.
- Cancel works and leaves no partial file.
- No new server endpoint; `listConcerts` + `downloadBundle` only. If you find yourself adding one,
  stop and come back to the gate.
- `:shared:check`, `:androidApp:assembleDebug`, iOS klib green. Device screenshots of: an available
  update, in-flight, up to date, and offline.

## Out of scope

- Triggering a server bake (Part B above).
- Automatic/background updating. P201's live-follow already exists for the connected case, and a
  surprise download on metered data is its own decision. **This is a button the player presses.**
- Granular per-song updates. `songs[].rev` makes them possible later; whole-bundle is the honest first
  step.
- Anything in the web studio.

## Note for whoever picks this up

Home now accumulates: the connection row (A38), the ⚙ Parameters hub (A36), and this. Three surfaces
landing on one screen within days of each other. **Decide the Home layout once, deliberately** — if
you find yourself squeezing a third control into a row that was designed for one, say so at the gate
and I will spec the layout rather than letting it accrete.
