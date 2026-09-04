# First rehearsal with TroubaStack — field report, 2026-09-04

Studio (web) on a phone, Stage on a tablet, both on one self-hosted server. VLL reported eight things.
This file tracks each from **symptom → proof → root cause → fix task**, so nothing is closed on a story.

## Ground rule for every fix here: **RED FIRST**

VLL: *"red first les tests."* A fix in this file is not done until its test has been **seen failing**
without the fix. Two reasons it matters more than usual here:

- Several of these are **silent** — the wrong result looks like a normal one. A test that passes both
  before and after proves nothing about a silent bug.
- We already shipped a test that **agreed with a defect** (T138's e2e excluded a file and expected the
  count an all-ticked editor produces). It was green in the same run that was red elsewhere.

So: write the assertion, watch it fail on today's code, then fix.

## Status board

| # | Symptom (VLL's words) | Status | Evidence | Fix |
|---|---|---|---|---|
| 1 | *"toutes mes chansons etaient reordonnées"* | ✅ **ROOT-CAUSED + REPRODUCED** | bake timeline + scratch import | **T140** |
| 2 | *"2 bake avec le meme nom, je ne sais pas quelle version, ni quel serveur, ni quel band"* | 🟡 **ESTABLISHED** — data can tell them apart, Stage doesn't show it | `bundle.json` fields | T141 |
| 3 | *"je ne peux pas avoir des infos sur un bake ni le supprimer du device (manque un … ?)"* | ⬜ not yet investigated (Stage UI) | — | T141 |
| 4 | *"en scrolling les annotations ne sont plus alignés"* | ⬜ not yet investigated | — | — |
| 5 | *"quand il y a des annotation trop loin l'ensemble des paroles est plus petit"* | ⬜ not yet investigated — likely same root as #4 | — | — |
| 6 | *"failed to fetch sur tous les morceaux GVO"* (Studio) while the bake works | ✅ **ROOT-CAUSED + PROVEN** | `Content-Length` 1720 vs 3029-byte blob; fails on loopback too | **T141** |
| 7 | *"diminuer la marge a gauche des fichiers textes rendu"* (enables a future 2-column option) | 🟢 enhancement | — | — |
| 8 | *"dans Stage un chronometre (start/pause/reset) et une horloge"* | 🟢 enhancement | — | — |
| 9 | reordering on a phone: arrows jump the scroll, drag won't auto-scroll, can't drop at the end, touch selects the title text | ✅ **all four confirmed in code** | see below | T142 |
| 10 | auto-update bake + a manual bake from Studio → **no toast in Stage** while sitting in the bake | ⬜ not yet investigated | — | — |

## 1 — The setlist order was silently scrambled (T140)

**Proven.** Each `.tstage` is a timestamped snapshot of the running order, so the bakes are a timeline:
the folder's order was correct, the first evening bake was scrambled, later bakes are correct again
because VLL re-ordered by hand. Between them the band was **re-imported** (new band id, new setlist id,
`bakes/` recreated).

**Root cause: one unset integer.** The v2 reader builds each imported setlist item without setting
`Position`, so every item is `0`. The folder expresses order as **array order** — correct for a
hand-written file — but nothing materialises it. Reproduced on the real library: 23 items, all at
position 0, order no one chose.

**Red-first note:** an all-zero set can *accidentally* come back in order on a small fixture. The test
needs ≥10 items in an order no sort would produce, and must be seen red before the fix.

## 2 — Two bakes, same name (T141)

`bundle.json` carries `concertId`, `name`, `concertRev`, `bakedAt`, `bakedBy`, `roster` — and **no band
name, no server URL, no app version**.

**But `concertRev` increments (1→10) and `bakedAt` is distinct per bake.** So the two bakes on the device
*are* distinguishable from the data; **Stage simply doesn't show it**. That makes this a display gap, not
a format gap — much cheaper. Adding band + server identity to the bundle is a separate, larger question
(it changes the format), and worth doing only if the display alone proves insufficient.

## 3 — No bake info, no way to delete one on the device (T141)

Not yet investigated. Pairs naturally with #2: the same surface that shows *which* bake this is should
let you remove one. Note the re-import **destroyed the afternoon bakes server-side**, so the device may
hold the only copy of a bundle — deletion should say what it is deleting.

## 4 & 5 — Annotations drift while scrolling; lyrics shrink when a mark is far away

Not yet investigated. Reported together and probably one root cause: if a far-away annotation widens the
overlay's bounding box, a fit-to-width step would scale the whole page down — which is exactly "the
lyrics get smaller". Treat #5 as a symptom of #4 until proven otherwise.

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
wrong, and one command settled it. **87 of 158 files are affected**, and of the 87 with a chart source,
**87 have `size` exactly equal to the source length**: every generated chart, both bands.

The bake works because it reads the blob directly and never consults `Size`.

**Two things had to be true for a musician to see this**: the importer wrote a size describing a
different object than the blob, *and* the handler trusted that stored field over the payload it was
holding. T141 fixes both.

## 7 & 8 — Enhancements

Left margin of rendered text charts is currently sized for a single column; reducing it is the
prerequisite for a future two-column layout. Stage wants a chronometer (start/pause/reset) and a clock —
HUD, with an option to keep the clock visible bottom-right.

## 9 — Reordering on a phone is painful (T142)

All four of VLL's observations are confirmed **in the code**, and they share one cause: `SortableList`
is built on **HTML5 drag-and-drop**, which is a desktop input model.

| VLL's words | what the code says |
|---|---|
| *"on ne peut pas deplacer un morceau en dernier"* | `reorder` lands an item **above** the target row — "the drop hint is the row's top border". There is no row after the last, so **no drop position exists at the end**. The arrow can reach it (`canMoveDown` allows `length-1`); the drag cannot. |
| *"la fenetre ne scroll pas quand on est tout en haut ou tout en bas"* | no `scrollBy`/`scrollTop` anywhere in the component — HTML5 DnD does not auto-scroll a container, it must be implemented |
| *"les fleches repositionne ou on se trouve dans la page"* | nothing calls `focus()`; the moved row re-renders, focus is lost to `<body>`, and the browser scrolls |
| *"en tactile une fois sur deux ca faisait une selection du texte du titre"* | no `user-select: none` / `touch-action` on the grip or row: a touch that isn't recognised as a drag becomes a text selection |

**The good news is VLL's own**: it is a single shared component (`SortableList.tsx`, 150 lines) used by
the setlist, the song files and my-files — so one rewrite fixes every reorder surface at once. He asked
for the established approach to be researched rather than invented; the modern answer is a
**pointer-events based** implementation (one code path for mouse, touch and pen) with an explicit
drop-zone model that includes an **end position**, container auto-scroll near the edges, `touch-action:
none` on the handle, and keyboard moves that **restore focus to the moved row**.

## 10 — No toast in Stage when a bake it is showing gets updated

Not yet investigated. Reported with auto-update enabled, a manual bake fired from Studio, and Stage
sitting *inside* the bake at the time.
