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
| 6 | *"failed to fetch sur tous les morceaux GVO"* (Studio) while the bake works | 🟡 **under investigation** — blobs intact, no server-side error | see below | — |
| 7 | *"diminuer la marge a gauche des fichiers textes rendu"* (enables a future 2-column option) | 🟢 enhancement | — | — |
| 8 | *"dans Stage un chronometre (start/pause/reset) et une horloge"* | 🟢 enhancement | — | — |

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

## 6 — Studio: "failed to fetch" on every song, while the bake works

Established so far: **all 51 GVO file blobs are present on disk**, and the server log shows no error. So
this is not missing data. "Failed to fetch" is a *transport-level* failure (the request never completed),
which points at host/origin rather than at content — consistent with VLL's note that the host had been
the **external IP**. Investigation continues.

## 7 & 8 — Enhancements

Left margin of rendered text charts is currently sized for a single column; reducing it is the
prerequisite for a future two-column layout. Stage wants a chronometer (start/pause/reset) and a clock —
HUD, with an option to keep the clock visible bottom-right.
