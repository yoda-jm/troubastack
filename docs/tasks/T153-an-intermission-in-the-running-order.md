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

## ⟨D1⟩ What the page shows — VLL, 2026-09-05

*"une page qui donne la marque genre TroubaStage qui dit pause (ou le terme consacré pour le entracte en
anglais) et aussi le nom du groupe."*

**Three elements, in this order of prominence:** the **label** (largest — it is what a musician reads across
a room), the **band name**, and the **TroubaStage mark** (smallest — it identifies the tool, it is not the
message).

### The label is CONTENT, not a hardcoded string

"The consecrated English term" is **Intermission** (theatre and concert programmes); **Interval** is British
usage; between two sets musicians usually say **Set break**. But VLL's band is French and would write
**Entracte** — so **do not hardcode any of them.** The `Label` field from the section above is authored by
whoever adds the break; the default is `Intermission` and it is freely editable. A French band types
`Entracte` and sees `Entracte`. **No translation layer, no term debate in code.**

An empty label renders the default rather than a blank card.

### The band name comes from the bundle, and may be absent

`band_name` exists since T143 (`bundle.proto:136`) — so it is already there for new bundles. **Absent ⇒ omit
the line entirely.** Do not print "Unknown band" on something a musician looks at mid-gig: that placeholder
is right for a library row and wrong for a performance page.

### ⚠ The mark is where this will break, and it is not obvious

The brand assets live under `docs/brand/dist/bricks/*.svg`. **`docs/` is excluded from the Docker build
context**, so a server-side render that reads the asset from there works in-tree and **fails in the
container image** — the exact failure shape recorded in this repo before (a build step reading outside its
package). So:

- **embed the mark in the package that renders it** (the `webassets` pattern), or copy it in at build time
  and COPY it in the Dockerfile;
- **verify by building the container**, not by running the test suite. "The Go test passes" is not the claim
  to check here.

Prefer the **outlined-path** wordmark (BRAND06 turned it into committed paths precisely so rendering does
not depend on a font being present).

### Render it through `chartpdf`, not beside it

The baker turns PDFs into rasters; giving it a second, ad-hoc drawing path would put a page in the bundle
that **T144's golden test cannot see**. Render the card as a minimal document through `chartpdf` so it goes
down the same PDF→raster pipeline, and **add it to the golden fixtures** — a separator whose layout silently
drifts is the same class of bug as the one T144 exists to catch.

### ⟨R1⟩ additions

- A break with no label renders `Intermission`; a break labelled `Entracte` renders `Entracte` — assert the
  drawn text, not the field.
- A bundle **without** `band_name` renders the card **without** a band line and without a placeholder.
- The card is in T144's goldens, and a metric change moves its value.
- **Container check**: the image builds and renders a separator. Red-first here means *building the image*,
  since that is the only place the asset path can fail.

## Settled by VLL, 2026-09-06 — it IS a landable position

He was asked directly whether "next" stops on the intermission or steps over it, and answered that
**"next" stops on it**. So it is a real position in Stage's navigation, not a separator drawn in a list:
the page fills the screen, and the performer moves off it with the same gesture as any other page.

Two consequences that follow, and they are requirements, not readings:

- **The current entry must tolerate having no song.** `SongID` becomes optional exactly as this spec
  already describes; every reader of "the current item" must handle the song-less case rather than
  assume one.
- **It carries no number.** This is the T158 rule — *a number belongs to a song in the running order* —
  and the on-call bench already obeys it (`StageScreen.kt:1095`). An intermission between songs 2 and 3
  must leave the next song reading **3**, not 4, so VLL's setlist landmarks do not all shift by one. The
  shared numbering vectors live in **T158**; this task's job is to make the intermission one of the rows
  they cover.

## Deliberately open, for VLL

- **What does the page look like?** Out of scope here beyond "legible at arm's length"; treat it as a design
  question once the plumbing exists.

## Out of scope

Timed intermissions (a countdown), and any link to the T147 chronometer. If VLL wants the break to drive the
clock, that is a separate task built on this one.
