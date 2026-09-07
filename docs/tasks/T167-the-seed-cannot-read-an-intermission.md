# T167 — a band folder cannot express an intermission, so a real setlist cannot be seeded

**Surface:** `cmd/seed` (and `cmd/seed/canonical.go`). **Lane:** web-core / core. **Kind:** gap (T153 leftover).
**Number claimed** in the same push as this file.

Found while doing what VLL asked — *"tu peux mettre à jour mes deux groupes depuis 8080 vers le band
folder?"* — i.e. backporting his live setlists into the local band library that `make band=<shortname>`
seeds from. One band went through cleanly. **The other could not, and the reason is a defect.**

## Three formats, and T153 only taught one of them

| format | file | knows `kind`/`label`? |
|---|---|---|
| band export/import v2 | `internal/app/bandio_v2.go` `v2SetlistItem` | **yes** — `Song` empty for a break, `Kind`, `Label` |
| the seed's **reader** | `cmd/seed/main.go` `loadSetlists` | **no** |
| the canonical **writer** | `cmd/seed/canonical.go` `canonSetlistIt` | **no** |

T153 added `kind`/`label` to the archive format and left the folder format behind. That is the same shape as
the three enumerations flagged on 2026-09-07: a list of fields mirroring a set defined elsewhere, which goes
stale silently.

## What it costs, concretely

`loadSetlists` maps `item.song` (a repertoire slug) to a title, and an unknown slug is a **hard error** —
deliberately, and its docstring is right: *"a gig list that quietly loses a song is the failure this task
exists to prevent."* A break carries **no** slug. So a `setlists.json` describing a real setlist that
contains an intermission makes `make band=<shortname>` **fail outright**:

```
setlist "…" references unknown song slug ""
```

Loud rather than silent, which is the good half. The bad half: **a band whose setlist has a break cannot be
represented in its own folder at all.** The folder is the source that recreates the server, so today that
band's running order is not reproducible — and the break is exactly the thing that recently changed.

I therefore **wrote one band's file and deliberately skipped the other**, rather than write a setlist with
the break silently dropped. Dropping it would have been the failure `loadSetlists` exists to prevent, just
committed one layer earlier.

## What to build

Teach both sides the two fields the archive already carries:

- **`loadSetlists`** — accept `kind` and `label`; an item with `kind: "intermission"` carries no `song` and
  must not be slug-resolved. Absent `kind` ⇒ `"song"`, so **every folder written before this keeps its
  meaning** (the T153 rule).
- **`canonSetlistIt`** — emit `kind`/`label`, or the canonical writer will strip a break the next time it
  runs, re-opening the hole from the other end.

## ⟨R1⟩ Red first

- A `setlists.json` whose items include `{"kind":"intermission","label":"Entracte"}` seeds a setlist with a
  break at that position, and the songs around it keep their running-order numbers. **Red today: it fails
  with `unknown song slug ""`.**
- **Teeth:** an item with **no** `kind` still seeds as a song — assert a break's `Kind` stays *empty* on a
  plain item rather than defaulting to the literal `"song"`, which is the trap T153 already recorded.
- An intermission with a **blank** label seeds as a break with a blank label; the default word is the
  renderer's business, not the seed's.
- **Round-trip:** folder → seed → `ExportBand` → folder produces the same items in the same order, break
  included. That is the assertion that keeps the three formats from drifting again, and its absence is why
  they drifted this time.

## Done means

VLL's second band's real running order — the one with the break he asked for — can be written into its
folder, and `make band=<shortname>` rebuilds the server with the break where he put it.
