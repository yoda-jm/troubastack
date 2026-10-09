# A83 — Studio browse: concerts grouped by band, and a ⋯ menu with Bake and live mode

**Lane:** mobile · **Status:** specced 2026-10-09, not started. Mobile can start it today. Only the Bake
item's *dialog* needs **T189** (web-core); until T189 lands, Bake opens the concert page (§4). VLL took the
choices in §2 on 2026-10-09 · **Origin:** VLL, 2026-10-09: *"in the studio screen the concerts tab, we cannot
see what belongs to what band, this is not practical when you have multiple bands, also a ... for various
stuffs activatable on playlist could be nice (I am thinking bake and auto bake at first)"*.

**Touches:** `StudioBrowseScreen.kt` (`ConcertsTab`, `LauncherRow`) and `HttpTransport.kt`
(`fetchStudioConcerts`, `StudioConcert`, `SetlistRow`). Any ordering logic goes in a pure function with a
`commonTest` (§5).

## 1. What is wrong today

- **The band is invisible.** `StudioConcert` already carries `bandName` (and `bandId`), but the row shows only
  `name` plus `eventDate · venue`. With two bands, two concerts called "Rehearsal" cannot be told apart. A
  concert with no date and no venue gets **no meta line at all**, which is common for working playlists.
- **Undated concerts have no defined order.** `fetchStudioConcerts` sorts dated before undated, then by date.
  Undated ones keep the server's per-band order, interleaved band by band. Studio web sorts undated concerts by
  name (`setlistOrder.ts`: *"drafts, by name"*).
- **Nothing can be done from a row.** A65 made this screen *"launchers only… NOTHING else"* (a reviewer's
  ruling, recorded in the `StudioBrowseScreen` KDoc). **VLL now overrides that for the two actions in §4
  only.** Create, rename, delete and editing stay in Studio, as I10 requires. Update the KDoc so it states the
  new boundary. Do not just delete it.

## 2. VLL's choices (2026-10-09)

- **⟨D1⟩ Group the concerts under band headers.**
- **⟨D2⟩ Bake opens Studio's own bake dialog** (through T189's deep link). There is **no native bake dialog**:
  P205's rule, *"never capture [layer defaults] silently"*, is kept by having one dialog in one place.
- **⟨D3⟩ Everyone sees the ⋯ menu.** For a non-admin, the admin-only items are **disabled**, with the reason
  shown. They are not hidden.

## 3. ⟨D1⟩ Band headers

- **One section per band**, headed by the band name, using `stickyHeader` in the `LazyColumn`. Use the Studio
  accent for the header (`LocalBrandAccents.current.studio`), as the tabs do.
- **Band order:** band name, natural A–Z. Ignore case and accents, and compare numbers by value (`Band 2` before
  `Band 10`). This is the same rule as T188's tag order.
- **Within a band:** dated concerts newest first (today's rule), then undated ones by name, natural A–Z
  (`Session 2` before `Session 10`).
- **Exactly one band:** no header, so the list looks as it does today. A header that names the only band adds
  nothing.
- **A band with no concerts gets no section.** The Bands tab already lists every band.
- The meta line is unchanged: `eventDate · venue`, either part omitted when blank. The header now carries the
  band, so the row does not repeat it.

## 4. ⟨D2⟩⟨D3⟩ The ⋯ menu on every concert row

A trailing `IconButton` (⋯) on each concert row opens a `DropdownMenu` anchored to that row. The row tap still
opens the concert in Studio, as today. Items, in this order:

1. **Bake** (or **Re-bake** when `lastBakedAt` is present): opens Studio, the same way a row tap does, at
   `/bands/{bandId}/setlists/{setlistId}?bake=1`, with the band name as the frame title. **T189** makes Studio
   open its bake dialog on that parameter. Until T189 lands, the same link opens the concert page, where the
   Bake button is. **Do not build anything native for baking.**
   - Disabled when `songCount == 0`, with the supporting text *"No songs yet"* (Studio's empty-concert guard,
     `bakeSetlistDisabled`).
2. **Arm live mode · auto-bakes for 3 h** / **Disarm live mode**, the same labels as Studio (T132). The 3-hour
   consequence is in the label, so there is **no confirmation**.
   - The action is `POST /api/bands/{bandId}/setlists/{setlistId}/live` with `{"live": <!current>}`, the
     same endpoint Studio uses. Then refetch the list.
   - While the call is running, the item cannot be pressed twice.
   - On failure, show a short message (`Snackbar`), *"Couldn't change live mode"*, and leave the row as it
     was.

**Non-admins (⟨D3⟩):** both items are present and **disabled**, each with *"Admins only"* as supporting text.
Bake and live mode are admin-only on the server (`bakeapi.go`, `SetSetlistLive`). The disabled state must come
from the band role, **not** from a failed call. **Server authorisation is unchanged.** The menu only reflects
it.

**A "Live" chip** on the row while live mode is on (`liveUntil` is in the future on the device clock),
matching Studio's `live-chip`. Use the Studio accent, small, after the title.

**Data the rows need**, all already in `GET /api/bands/{id}/setlists`: extend `SetlistRow` with `songCount`,
`lastBakedAt` (nullable) and `liveUntil` (nullable; treat Go's zero time `0001-01-01T00:00:00Z` as off). Add
`isAdmin` per concert from the band's role. `isAdminOfBand` already exists. **Call it once per band, not once
per concert.**

## 5. Not in this task

- PDF or bundle download, duplicate, delete: Studio's other menu items. VLL asked for bake and auto-bake
  "at first". Leave the menu built so that more items can be added later.
- Any native bake UI or bake progress.
- Grouping or ⋯ on the Stage concert list (`MainActivity`). That list is a different screen.
- Changing who may bake or arm live mode.

## 6. Acceptance

**Pure logic (`commonTest`):** a function that turns the flat concert list into sections, for example
`groupConcerts(concerts) -> List<Section>`.
- Two bands named `Band 10` and `band 2`: the `band 2` section comes first (natural order, case ignored).
- Within one band, dated `2026-09-12` comes before `2026-03-01`, both come before undated, and undated
  `Session 2` comes before `Session 10`.
- **Feed the input interleaved across bands**, in the order the server would return it, so that a function
  which only sorts and never groups fails.
- With exactly one band, there is one section, flagged "no header".
- **Teeth.** Replace the undated comparison with a plain `String.compareTo`. Confirm the
  `Session 2`/`Session 10` case goes **red**, and print the replaced line. Restore it.

**Device pass (emulator or tablet, synthetic demo data only: `make demo`, the seeded cast).** This is a gate,
not optional. A pure test cannot show occlusion, a missing anchor or a dead tap.
- **The seed puts no user in two bands.** The two demo groups have separate members (`marie` administers one,
  `maestro` the other). So, on an isolated demo server (not :8080), first have `maestro` invite `marie` into
  the orchestra as a **member**, and accept. `marie` is then admin of one band and a plain member of the other,
  which covers every case below with one account.
- Sign in as `marie`.
  - Both band headers show, and each concert sits under its own band.
  - Scrolling keeps the current band's header pinned.
- **As admin:**
  - ⋯ then *Arm live mode* shows the Live chip after the refetch. ⋯ again reads *Disarm live mode*, and that
    removes the chip.
  - Confirm the change in **Studio web** too: the same concert shows Live.
- **As admin, Bake:**
  - ⋯ then *Bake* opens Studio on that concert. Before T189, it lands on the concert page; after T189, the bake
    dialog is open.
  - On an empty concert, Bake is disabled and reads *No songs yet*.
- **As a non-admin member** of a band:
  - Both items show **disabled** with *Admins only*.
  - A tap does nothing. No request is sent: check the server log or a request counter.
- **Exactly one band:** no header.
- Attach screenshots to the gate entry: two-band grouped list, the admin menu, the non-admin menu, and the
  Live chip. **Demo data only. No real band names in any committed or posted frame.**

**Regression.** `./gradlew` unit tests (from `app/`) and the iOS compile (CI compiles iOS separately) are
green. The Bands tab and its QR button are unchanged.
