# T153 — An intermission between two songs

**Lane:** first stage core (domain + tband + proto/baker), then studio and mobile. **Size:** L.
**Status:** spec, 2026-09-05, from VLL.

## What VLL asked

*"pouvoir rajouter un 'entracte' dans une playlist qui afficherait joliment un petit separateur entre 2
chansons, ca change tband tstage core stage studio je suppose ?"*

**Yes, all five** — and the scope is honest work, not a display tweak. The reason is one line of the domain:
`SetlistItem` is `{ID, SetlistID, SongID, Position, KeyOverride, TempoOverride, Notes}`. **A setlist entry
IS a song today.** An intermission is the first entry that is not one.

## The design: a kind on the entry, and ONE rendered page in the bundle

**Bake the intermission as a normal entry carrying a single rendered page**, not as page-less metadata.

Everything performance-side already works in pages: scroll, page turns, two-up, fit modes, the picker, the
position remap on a live update. A page-less entry would need new handling in **every one of them** — and it
would walk straight into the guard landed at `28a51f8a`, which fails a bake when an entry's overlay has no
page to live on. With one page, **Stage needs no new rendering path at all**: it draws the page like any
other and suppresses the musical chrome.

There is a precedent in the format worth following: `on_call` (T23) already marks an entry that is in the
bundle but outside the running order. `kind` is the same shape of idea, and like `band_id` (T143) it must be
**additive — absent ⇒ song**, so bundles already on a device keep working.

## What changes, layer by layer

| layer | change |
|---|---|
| **core / domain** | `SetlistItem` gets `Kind` (song \| intermission) and a `Label`; **`SongID` becomes optional** |
| **tband** | `setlists.json` items can declare a break — same vocabulary as a song entry, with its own stable id (see T150) |
| **proto / tstage** | `BakedSong` gets `kind` (additive) and the label; the entry carries exactly one page |
| **baker** | renders the separator page; **skips** annotation overlays, member sequences and file selection for it |
| **Stage** | shows the separator, suppresses key/tempo/beat/cues; decide whether it is a landable position |
| **Studio** | add / label / remove / reorder an intermission in the setlist editor |

## ⚠ The risk is not the feature. It is `SongID` becoming optional

Every consumer that assumes a setlist entry has a song will now meet one that does not — silently, because
an empty string is a valid string. **This is T140's shape exactly**: an unset field flowing through code
that never questioned it, producing a plausible wrong answer instead of an error.

**Required, and non-negotiable:** enumerate the consumers of `SetlistItem.SongID` **before** writing the
field, and state in the commit what each does with an intermission. `git grep SongID` is the start of that
list, not the end of it — the bake, my-files, member pages, the annotation engine and the setlist views all
read it.

Three that must be decided explicitly, not discovered:

- **Annotations**: an intermission has no source, so it has no T145 anchor and no overlay. Exclude it
  deliberately; do not let it fall out of a nil check.
- **Member pages / my-files**: it has no files. It must appear for every member regardless of selection.
- **Bake identity/order**: it occupies a `Position` like any entry, so T140's ordering must hold for it.

## ⟨R1⟩ Red first

- A setlist of song–intermission–song **bakes to three entries**, the middle one with `kind=intermission`,
  one page and **no overlays**. Red today: the domain cannot express it.
- **The running order survives**: positions 0,1,2 in that order, through the client-facing view (T140's
  assertion extended).
- A **pre-T153 bundle** still loads and every entry reads as a song — the `band_id` lesson from T143.
- An intermission is **skipped by the annotation path** and by file selection: no overlay is requested for
  it, and no member's selection can add or remove it.
- **Teeth-check:** make `kind` default to song on a *new* bundle too, and confirm a test fails — otherwise
  "absent ⇒ song" is untested and the additive claim is a hope.

## Deliberately open, for VLL

- **Is an intermission a landable position on stage?** I think yes — a musician wants to see "Entracte" on
  the sheet, and jumping past it would make the running order lie. But it is his call, and it changes
  Stage's navigation.
- **What does the page look like?** Out of scope here beyond "legible at arm's length"; treat it as a design
  question once the plumbing exists.

## Out of scope

Timed intermissions (a countdown), and any link to the T147 chronometer. If VLL wants the break to drive the
clock, that is a separate task built on this one.
