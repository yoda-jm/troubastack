# A91 — One concert list in Stage: one row per concert, one state, one action (absorbs A90)

Owner: mobile. Spec: Fable, 2026-10-20. Status: specced. One **[VLL]** decision, implement on its default.
**Absorbs A90** (a deleted concert can never be downloaded again). Implement A90's red-first tests here. Do not
land a separate A90.

## Why (VLL, 2026-10-20)

> "toute cette liste dans le home c'est pas très facile, ça marche jamais comme on pense, les rebake les updates, les
> nouveaux"

That came after a real failure: a freshly re-baked concert was on the server and not on the tablet, and there was no
way to download it.

The root cause is **too many concepts, spread over two places**:
- **A31 split one list into two intents.** *TroubaStage* (Perform) lists only local bundles, with no server
  affordance. *TroubaStudio* (Manage) shows the same list plus Import, download/update offers and freeze/pin/delete.
  A third list (A65/A83 Studio-browse) carries Re-bake and Live. **A musician looks for a concert where they play it,
  and the downloads live elsewhere.**
- **State hidden in chips and flags.** `NewlyAvailable` / `UpdateOffered` chips, the policy book (FROZEN, pin) and
  the invisible `installedOnce` (A43-fix). With that last one, a deleted concert is never offered again. That is
  A90's defect, and its cause is in `UpdatesManager.diff()`.

Each rule is defensible alone. Together the screen can't be predicted.

## The model: one list, one row per concert

The list lives in **TroubaStage** (the Perform entry). It shows **every concert of my bands**, the union of:
- the server manifest (`GET /api/bands/{id}/concerts`);
- the bundles on the device, including local-only and imported ones.

Each row shows: the name, the band, the date or venue, and a **state line in plain words**. **One** primary action.

| State | Shown as (wording to go on the label list) | Primary action |
|---|---|---|
| **Up to date** | "Up to date · v7 · baked 2 h ago" | **Open** |
| **Update available** | "New version v7 (you have v5) · 2 songs changed, 1 added" | **Update** (Open stays one tap away, on the old version) |
| **Not on this tablet** | "On the server · v7" | **Download** |
| **Updating / Downloading / Baking** | a progress line | Cancel |
| **Offline / server unreachable** | "Last checked 3 h ago" (and the local rev, if any) | **Open** (local) / disabled if none |
| **Local only** (imported, not on the server) | "On this tablet only" | **Open** |
| **Damaged** | "Can't be opened, re-download" | **Download** |

**The rules:**
1. **No chips, no banners, no hidden flags.** The state is always written on the row.
   - "Don't nag about a deleted concert" now means **it is not announced**. It stays in the list as "Not on this
     tablet", with Download.
   - `installedOnce` stops gating availability. It may remain only to avoid *announcing*.
2. **Delete from tablet** never removes the row while the server still has the concert. The row becomes
   "Not on this tablet".
3. **Refresh** sits at the top of the list, and the list also refreshes when it opens. "Last checked …" is always
   visible.
4. **"What changed"** is computed from the data both sides already have: the manifest's per-song revs against the
   installed bundle's `songRev`s. It reads "N songs changed, M added, K removed". Tapping it shows the titles.
5. **The ⋯ menu** holds the secondary actions only:
   - **Re-bake** (admin). It shows progress inline, then the row turns "Update available" by itself. It reuses the
     A83 call and the bake-progress endpoint.
   - **Rehearsal live mode** (admin, A83/P201).
   - **Freeze this version.** No update offered: the row reads "Frozen at v5", and that replaces the FROZEN/pin
     jargon. **Unfreeze.**
   - **Delete from tablet.**
   - **Import a .tstage…** lives once, in the list's top-bar menu, not per row.
6. **Never during a performance.** Nothing downloads or swaps while a concert is open in Stage. That is I13, kept.
   P201 rehearsal auto-update inside an open concert is unchanged.

### [VLL] Automatic updates

- **Default (implement): "Keep concerts up to date" ON**, one switch at the top of the list.
  - When the list is shown, NOT during a performance, every concert in state "Update available" that is not frozen
    installs by itself, and its row shows the progress.
  - Why: VLL's pain is having to decide. Most rows then simply read "Up to date".
- **Cost to state on the switch:** an update re-bakes pages. A rehearsal note drawn on a page that changed
  **no longer shows on that page** (its key is the page's raster hash), and in Studio it reads
  "not in the current bake". Notes on unchanged pages are unaffected. The switch's caption says this in one line.
- **Alternative:** OFF by default, so "Update" stays a tap.

## Where the other things go

- **The TroubaStage tile → this list.** It is the only concert list in the app.
- **The TroubaStudio tile keeps** Bands, invites and QR (A65), Connect / server, Open Studio (web), Notes management
  (A70 Notes tab), and About / app update (A88).
- **It loses its concert list.** The Manage intent's offers and freeze/pin/delete move to the Stage list's ⋯ menu.
  The A65/A83 Studio-browse concert rows are replaced by the same unified list. One component, not two.
- **`ConcertIntent` Perform/Manage is retired.** Admin-only items in ⋯ are gated by the band role (`isAdmin`), not by
  which tile you came from.

## Implementation notes

- **Put the state machine in shared code, as a pure function:**
  `rowState(local: LocalBundle?, server: AvailableConcert?, reachable: Boolean, policy: PolicyRecord?, inFlight:
  Op?) → RowState + primaryAction`. All the rules above are table-tested there. Compose only renders it.
  This is what makes "it works the way you think" testable.
- **Merge key:** `concertId`. A server concert and a local bundle with the same id make ONE row.
- `UpdatesManager.diff()` keeps its update-suppression semantics (FROZEN, pin, `finalLocked`), but it no longer
  decides availability.
- The per-song diff uses the manifest `songs[].rev` and the bundle's `songs[].songRev`, both already on the wire.

## Tests (red-first, pure `commonTest`)

- **A90's case:** with `installedOnce=true`, not on disk and on the server, the row is "Not on this tablet" with
  Download, and no announcement. Red on today's `diff()`-only path.
- The full `rowState` table: one test per row of the state table, plus frozen (no update action, "Frozen at vN"),
  `finalLocked`, offline with and without a local copy, damaged, local-only, and in-flight.
- **What changed:** changed, added and removed counts against a fixture manifest and bundle.
- **Delete from tablet** with the server reachable gives "Not on this tablet", and the row is not removed.
- **Auto-update ON:** a non-frozen "Update available" concert gets an apply call while the list is shown, and NO
  apply while a concert is open (the I13 guard). Frozen: no apply.

## Device pass (VLL)

On the tablet:
- delete a concert → it is still listed as "On the server", Download → it opens;
- re-bake from ⋯ → the row shows progress, then "New version", then it updates (auto ON) → "Up to date";
- airplane mode → "Last checked …", and Open still works.

## Out of scope

- Server and Studio-web changes: the manifest already carries what is needed.
- The serverless architecture (docs/wip). This list is designed to work the same if concerts later come from a
  shared folder (a row state is source-agnostic).
