# First rehearsal with TroubaStack — field report, 2026-09-04

Studio (web) on a phone, Stage on a tablet, both on one self-hosted server. VLL reported twelve things.
This file tracks each from **symptom → proof → root cause → fix task**, so nothing is closed on a story.

**No band data in this file.** The repo is public. Songs are referred to by shape (`the 72-line chart`,
`#20`), never by title; the same applies to every task file these entries point at.

## Ground rule for every fix here: **RED FIRST**

VLL: *"red first les tests"*, and again: *"pour chaque bug un red first"*. **Every row in the board below
carries its own red-first assertion** — not one shared gesture at testing. A fix in this file is not done
until *its* test has been **seen failing** without *its* fix.

Two reasons it matters more than usual here:

- Several of these are **silent** — the wrong result looks like a normal one. A test that passes both
  before and after proves nothing about a silent bug.
- We already shipped a test that **agreed with a defect** (T138's e2e excluded a file and expected the
  count an all-ticked editor produces). It was green in the same run that was red elsewhere.

A red-first assertion also has to have **teeth**: its expected value must differ from what the buggy code
produces. "Renders without error" is not a red-first test for a layout bug.

## Status board

| # | Symptom (VLL's words) | Status | Evidence | RED FIRST — the assertion that must fail today | Fix |
|---|---|---|---|---|---|
| 1 | *"toutes mes chansons etaient reordonnées"* | ✅ **FIXED** (`22842291`) | bake timeline + scratch import | ≥10 imported items come back in **folder order**, not all at `Position 0` | **T140** |
| 2 | *"2 bake avec le meme nom, je ne sais pas quelle version, ni quel serveur, ni quel band"* | ✅ **CONFIRMED** — the row shows only a label; `concertRev` + `bakedAt` exist and are discarded | `ConcertRow` | two bundles differing only in rev/date render **two distinguishable rows** | **T143** |
| 3 | *"je ne peux pas avoir des infos sur un bake ni le supprimer du device"* | ✅ **CONFIRMED — and worse than filed**: on the perform row there is **no ⋮ at all** | `ConcertRow`, `lean = !manage` | a non-damaged bake exposes a **delete affordance**; asserted on the row, not on Manage | **T143** |
| 4 | *"en scrolling les annotations ne sont plus alignés"* | ✅ **ROOT-CAUSED + PROVEN** — not scrolling: **the render reflowed under the marks** | the tablet's 17:46 bundle vs tonight's 22:20 | same source + same annotation ⇒ the mark stays **on the same text**, across a renderer change | **T145** |
| 5 | *"quand il y a des annotation trop loin l'ensemble des paroles est plus petit"* | ✅ **ROOT-CAUSED** — same event as #4; the type shrank and pages reflowed. **Not caused by the annotation** | page-extent measurements below | a fixed source renders to a **pinned page count + layout hash** | **T144** |
| 6 | *"failed to fetch sur tous les morceaux"* (Studio) while the bake works | ✅ **ROOT-CAUSED + PROVEN** | `Content-Length` 1720 vs 3029-byte blob; fails on loopback too | `GET /api/files/{id}` returns **all** the blob's bytes when `Size` disagrees | **T141** |
| 7 | *"diminuer la marge a gauche des fichiers textes rendu"* (enables a future 2-column option) | 🟢 enhancement | — | margin is a **named constant** with a test asserting the rendered left edge | **T146** |
| 8 | *"dans Stage un chronometre (start/pause/reset) et une horloge"* | 🟢 enhancement | — | chrono state machine: start→pause→resume→reset returns to **00:00** | **T147** |
| 9 | reordering on a phone is painful (4 distinct faults) | ✅ **all four confirmed in code** | see below | an item can be dropped **after the last row**; container auto-scrolls; arrows keep focus | **T142** |
| 10 | auto-update bake + a manual bake from Studio → **no toast in Stage** | ✅ **CONFIRMED by absence** — `applyUpdate` emits nothing | `StageViewModel` | `applyUpdate` emits a message **and** leaves the page index unchanged | **T143** |
| 11 | *"il faut regrouper par groupe (accordion), le titre + version et date en gris"* | 🟢 design input, folded into **T143** | — | a two-band library renders **two groups**; each row shows rev + date | **T143** |
| 12 | *"j'ai jamais demandé d'auto adjustment"* | ✅ **ANSWERED — VLL is right**: auto-fit (T76) landed 08-23 and first reached his charts on 09-04 | `127519fd` vs the 08-22 blob timestamps | (not a defect — a **product choice** to confirm) | **T146** |

## The evidence that settled #4 and #5

VLL: *"le bake de cet apres midi a la bonne taille et l'annotation du trait est bien sur le Verse 5, donc
quelquechose a changé."* He was right, and there turned out to be **two** independent copies of that
afternoon state — so the finding does not rest on one artefact.

**From the tablet**, pulled read-only without touching app state:

```
adb exec-out 'run-as com.troubastack.app tar cf - files/bundles/<id>' > afternoon.tar
```

Two bundles were on the device: `c90335ac…` (**17:46**, `concertRev 9`, 22 songs) and `9d8b293e…`
(**22:20**, `concertRev 10`, 23 songs). That is itself the proof for #2 — the two bakes **are**
distinguishable in the data, and only the display drops it.

**Correction to an earlier claim in this file.** I wrote that the re-seed had *destroyed* the afternoon
bakes server-side and that the device held the only copy. **It did not.** The web-core lane preserved the
whole pre-re-seed data directory at `troubastack-demo/data.preseed-20260904-191837`, and it holds **nine
bakes from 09:53 to 18:54**. That is a far better instrument than the single device bundle, and it is what
made the bisect below possible.

### The measurement — same song, same source, same annotation

`0.000` is the top of the page, `1.000` the bottom. "text →" is where ink stops.

| | page 2: text ends | the mark | verdict |
|---|---|---|---|
| **17:46 (afternoon)** | `0.409` | `0.328 → 0.424` | lands **exactly at the end of the text** — VLL: *"bien sur le Verse 5"* |
| **22:20 (tonight)** | `0.051` | `0.328 → 0.424` | **orphaned in blank space** |

The second page emptied — its content was pulled up onto page 1, whose ink now reaches `0.952` instead of
`0.944`. **The mark never moved. The words moved out from under it.**

And it is not one song. Every annotated song reflowed between the two bakes:

| song (by shape) | pages | text ends — afternoon → tonight | mark |
|---|---|---|---|
| 4-page chart | **4 → 3** | 0.944 → 0.953 | unchanged, now over different words |
| 2-page chart | **2 → 1** | 0.947 → 0.911 | unchanged |
| 2-page chart | **2 → 1** | 0.944 → *(overlay gone entirely)* | **lost** |
| 1-page chart | 1 → 1 | **0.754 → 0.949** | mark at 0.744 was at the end, now mid-text |
| the 72-line chart | 2 → 2 | p2: **0.409 → 0.051** | **orphaned** |
| 1-page chart | 1 → 1 | 0.734 → 0.924 | overlay extent also changed: `0.014–0.467` → `0.452–0.467` |

One song's overlay **disappeared from the bundle altogether**, and one overlay's own extent changed — so
the marks are not merely mis-positioned, some are being **re-rendered or dropped**.

### What changed, and what did not

Ruled out, each by measurement rather than reasoning:

- **The source did not change.** The chart source is **byte-identical** (same md5) across the current
  folder, the 16:21 backup, and the pre-v2 archive from 12:16.
- **The import did not introduce auto-fit** — but *something* did, and it was not "always there".
  **0 of 46** sources carry a `size:` directive in any of the three snapshots, so nothing was stripped;
  auto-fit applies because no size is given. **What changed is that auto-fit itself only landed on
  08-23** (`127519fd`, T76), and VLL's blobs were rendered **08-22**. See the correction below.
- **T138's default-file change is not the cause.** Five of the six affected songs have **exactly one**
  file, so which file is "default" cannot matter for them.

### The bisect, on nineteen real bakes

Same song, same source, page-2 ink extent (`1.000` = bottom of page):

| bakes | when | page 1 | page 2 | the mark |
|---|---|---|---|---|
| 9 preserved bakes | **09:53 → 18:54** | `0.944` | `0.409` | **correct, all nine** |
| 10 bakes, new instance | **20:42 → 22:20** | `0.952` | `0.051` | **orphaned, all ten** |

**It is a step change, and it lands exactly on the 19:18 redeploy + re-seed.** Nine hours of bakes on one
side, ten on the other, nothing in between and no drift within either group.

### ANSWERED — and it corrects two claims I made above

**It is not a regression at all.** The web-core lane ran T144's ⟨V1⟩ experiment and found the render
**byte-identical** across `3999abe0..8f662f60`. That falsified my window, and the reason is a **mistake in
my own probe**:

> I reported "the charts were already re-rendered twice inside the old instance (45 blobs at 16:36, 105 at
> 17:52) and the layout came out identical". **My `find -printf` printed `%TH:%TM` with no date.** Those
> blobs are from **08-22**, two weeks earlier — not the same afternoon. There was no same-day re-render,
> so the "re-rendering doesn't change output" bullet was never supported.

With the dates restored, the sequence is simple and fully explained:

| when | what |
|---|---|
| **08-22 16:36** | the live blobs were rendered — this is the layout VLL had all along |
| **08-23 14:24** | `127519fd` lands **T76 auto-fit**: *"largest body size that keeps a chart on one page"* |
| 08-23 → 09-04 | the charts are **never re-rendered**, so they keep their pre-T76 layout |
| **09-04 19:19** | the re-seed re-renders all 157 blobs — **T76 applies to them for the first time** |

So the reflow is the **intended** compaction finally reaching two-week-old blobs. Nothing to revert;
T144's guard is the right fix, and it landed at `e5750074`.

### Which makes #12 my error too, and VLL right

I wrote that auto-fit *"has always applied"* and that the import did not introduce it. The `0/46
size:`-directive measurement was correct, but **the conclusion was not**: auto-fit did not exist before
**08-23**, and VLL's charts did not receive it until **09-04**. From where he sits, *"j'ai jamais demandé
d'auto adjustment"* describes exactly what happened — a behaviour appeared on his charts that he never
asked for, two weeks after it was written.

**That turns #12 from "not a bug" into a product question for VLL** (see T146): auto-fit shrinks type to
keep a chart on one page. He can have a fixed size via an explicit `size:` directive, or the two-column
route which trades width for type size — but the default that shrinks a 72-line chart is a **choice**, and
it is his to make, not something to defend as documented behaviour.

### And it makes T145 more important, not less

The renderer did not break — **it improved**, and every annotation in the library broke anyway. A mark
anchored to `(page, fraction)` of a regenerated render cannot survive any layout improvement, ever. That
is the whole argument for T145, and it is now backed by a case where nothing was wrong except the anchor.

## 1 — The setlist order was silently scrambled (T140 — fixed, `22842291`)

**Proven.** Each `.tstage` is a timestamped snapshot of the running order, so the bakes are a timeline.

**Root cause: one unset integer.** The v2 reader built each imported setlist item without setting
`Position`, so every item was `0`. The folder expresses order as **array order** — correct for a
hand-written file — but nothing materialised it. Reproduced on the real library: 23 items, all at
position 0, order no one chose.

**Note on the two recovered bundles.** The 17:46 and 22:20 running orders differ, but the difference is
fully explained by **two songs inserted** in the evening, not by a scramble — so this pair is *not*
additional evidence for T140. Recorded so nobody re-reads it as such.

## 2 & 3 & 10 & 11 — Stage: which bake is this, and let me manage it (T143)

`bundle.json` carries `concertId`, `name`, `concertRev`, `bakedAt`, `bakedBy`, `roster` — and **no band
name, no server URL, no app version**. But `concertRev` increments and `bakedAt` is distinct, so the two
bakes *are* distinguishable from data Stage already holds. **A display gap, not a format gap** — much
cheaper. Adding band + server identity to the bundle is a separate, larger question.

**Correction to the original filing (VLL, verified):** *"tu parlais du '...' sur le bake dans TroubaStage ?
il n'y est pas actuellement."* He is right. `lean = !manage`, and the perform row renders only
`entry.label` — **there is no ⋮ on it at all**; the menu exists solely in Manage, and even there it has no
Delete (deletion is gated on `damaged`). So bake identity must appear **on the perform row**, and the
delete affordance has to be added, not merely surfaced.

**VLL's design input, 2026-09-05** — *"regrouper par groupe (accordion ?) et en plus du titre avoir la
version et la date en gris (et peut etre dans les ... l'id de la playlist)"*:

- **Group the library by band**, collapsible.
- Each row: **title**, with **revision and date in grey** as secondary text.
- The **setlist id** belongs in the ⋮, not on the row — it is a support detail, not a performance one.

**#10, confirmed by absence:** `StageViewModel.applyUpdate` swaps the bundle and emits nothing; there is
no toast, snackbar or message channel in the view model at all. The silence is deliberate *for the page* —
the function exists to swap "WITHOUT moving the page the performer is on" — but **non-disruption of the
page got conflated with saying nothing at all**. The sheet changed under a musician mid-rehearsal; that
deserves a word, provided the word cannot steal the page.

## 4 & 5 — Annotations are anchored to a render that is regenerated (T145), and nothing pins the render (T144)

Measured above. The two faults are separable, and both need fixing:

**T144 — the render is not pinned.** The same source rendered differently after a deploy, and no test
noticed. A layout change is invisible in a diff and invisible in CI. **Red first:** a golden test that
renders a fixed, committed chart source and asserts **page count plus a layout hash**; break it by nudging
any metric and watch it go red.

**T145 — the anchor is the real bug.** A mark stores `(page index, fractional x/y)` **of a particular
render**. The render is regenerated on every bake, so any reflow silently re-points every mark on the
page — and here it pushed one onto a page that had emptied. This is the fault that turns a good rehearsal
into a bad one, because **it looks like nothing is wrong**.

Options, cheapest first — T145 should pick one and say why:
1. **Anchor to the source**, not the render (line/character offset), and project to page coordinates at
   render time. Survives reflow by construction.
2. **Re-anchor on re-render**: keep page coordinates but record the render's identity, and remap when it
   changes.
3. **Detect and warn**: store the render identity with the mark and flag marks whose render no longer
   matches. Does not fix anything, but stops a silent wrong answer — acceptable only as a stopgap.

**A hypothesis raised earlier and DISPROVEN — recorded so nobody re-runs it.** That a far-away annotation
widened the baked overlay's canvas, giving it a different aspect ratio from its page raster. Measured on
the real bake: **70 (raster, overlay) pairs across 10 bundles, 0 with differing dimensions, 0 pages taller
than 1:3.** The overlays are exactly page-sized. Not the fault.

## 6 — Studio: "failed to fetch" on every song, while the bake works (T141)

**Root-caused and proven.** `GET /api/files/{id}` declares `Content-Length: 1720` and then writes a
**3029-byte** blob; Go refuses to write past the declared length, the response is malformed, and the
browser reports the generic `TypeError: Failed to fetch`.

```
curl: (18) end of response with 1720 bytes missing     — identical on 127.0.0.1, the LAN IP and the public host
file record size : 1720  ← the chart SOURCE's length
stored blob      : 3029  ← the rendered PDF
```

**It fails on loopback**, so the external IP was a red herring — VLL's hypothesis and mine were both
wrong, and one command settled it. **87 of 158 files are affected**: every generated chart, both bands.

The bake works because it reads the blob directly and never consults `Size`.

**Two things had to be true for a musician to see this**: the importer wrote a size describing a
different object than the blob, *and* the handler trusted that stored field over the payload it was
holding. T141 fixes both, and must also repair the 87 live rows.

## 7 & 8 — Enhancements (T146, T147)

- **T146 — left margin.** The rendered chart's left margin is sized for a single column; reducing it is
  the prerequisite for a future **two-column** layout. It should become a named constant, so that T144's
  golden test pins it and a later two-column mode has one value to change.
- **T147 — Stage HUD.** A **chronometer** (start / pause / reset) and a **clock**, with an option to keep
  the clock visible bottom-right during performance.

## 9 — Reordering on a phone is painful (T142)

All four of VLL's observations are confirmed **in the code**, and they share one cause: `SortableList` is
built on **HTML5 drag-and-drop**, which is a desktop input model.

| VLL's words | what the code says |
|---|---|
| *"on ne peut pas deplacer un morceau en dernier"* | `reorder` lands an item **above** the target row. There is no row after the last, so **no drop position exists at the end**. The arrow can reach it; the drag cannot. |
| *"la fenetre ne scroll pas quand on est tout en haut ou tout en bas"* | no `scrollBy`/`scrollTop` anywhere — HTML5 DnD does not auto-scroll a container |
| *"les fleches repositionne ou on se trouve dans la page"* | nothing calls `focus()`; the moved row re-renders, focus is lost to `<body>`, and the browser scrolls |
| *"en tactile une fois sur deux ca faisait une selection du texte du titre"* | no `user-select: none` / `touch-action` on the grip or row: a touch not recognised as a drag becomes a text selection |

**The good news is VLL's own**: it is a single shared component (`SortableList.tsx`, 150 lines) used by
the setlist, the song files and my-files — so one rewrite fixes every reorder surface at once. He asked
for the established approach to be researched rather than invented; the modern answer is a
**pointer-events based** implementation (one code path for mouse, touch and pen) with an explicit
drop-zone model that includes an **end position**, container auto-scroll near the edges, `touch-action:
none` on the handle, and keyboard moves that **restore focus to the moved row**. VLL also asked for the
**semi-transparent moved row** many components use, and for the UX research to be written down.

## 12 — "I never asked for auto-adjustment"

**Answered, and the import is not responsible.** Measured across three time-separated snapshots:

```
current folder       : 0/46 sources with a size: directive
backup 16:21         : 0/46
pre-v2 archive 12:16 : 0/46
```

No source has ever carried an explicit size, so nothing was stripped by the migration. **But "auto-fit was
always there" is false**, and it was my error: `127519fd` (T76) introduced it on **08-23**, while these
blobs were rendered **08-22** and were not re-rendered until **09-04**. VLL saw the behaviour appear on
his charts because, for him, it genuinely did. A long chart therefore renders smaller than a short one **by
design**. If VLL wants a fixed size, the remedy is an explicit `size:` directive; if he wants long charts
to stay readable instead of shrinking, that is **T146**'s two-column direction, not a bug fix.

This is separate from #5: the *shrink between the two bakes* was real and is T144.
