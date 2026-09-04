# T138 — "My files" is a BAKE selection, and the default file is ONE shared rule

**Lane:** web-core (studio + a small core change). **Size:** M. **Status:** spec, redesigned by VLL
2026-09-04 after he hit the original defect on the live demo.

## How this started, and why the first spec was wrong

VLL rebaked a two-file song and got only the lyrics, while Studio's "my files" showed **both files
checked**. He asked: *"le default du bake n'est pas le même default de cet affichage ?"* No — Studio
resolved an unset selection to **all pool files**, the baker to **one**.

My first fix was to *explain* the mismatch: leave the display alone and add a notice naming the file the
stage would take. **VLL's redesign removes the mismatch instead**, and it is better — there is nothing
left to misread, because the two surfaces stop sharing a meaning they never agreed on.

> *"the bottom should always have all files, and the my files should have the first one created selected
> and the others not by default and just serve what I want in my bake and what order, this does not
> change any display or order in the bottom menu of the studio"* — and *"sometimes I just want none and
> it is ok if I don't play the track."*

## The model: two surfaces, two jobs

| | what it is | what drives it |
|---|---|---|
| **Studio's bottom strip** | a **browser** over the song's files | the **pool**, always all files, in pool order |
| **"My files"** | a **bake selection**: what I read on stage, in what order | the member's ordered list |

**The strip stops reflecting the selection at all.** That is the point: the display a member reads while
editing no longer pretends to describe what they will read on stage.

**Empty is a legitimate, meaningful state** — *"I don't play this song"* — not an accident. Keep the
confirmation on saving an empty selection, but its job changes: it no longer guards a mis-click, it
records a decision.

## ⟨R1⟩ The default file is ONE rule, expressed once — the requirement everything else rests on

VLL: *"une seule règle partagée."* The default must **not** be re-described per surface. Writing "the
first one created" in Studio while the baker says "lowest DisplayOrder that is a viewable PDF" would
create a **third** definition and rebuild the exact bug this task removes.

### It already diverges today — found while specifying this

```
core/internal/bake/baker.go  defaultFile : ContentType == "application/pdf"           → PDF only
web/studio/.../Viewer.tsx    isViewable  : "application/pdf" || startsWith("image/")  → PDF or image
```

`defaultFile`'s own comment claims it picks *"the same one Studio opens by default"*. **For a song whose
only viewable file is an image, that is false**: Studio opens it, the baker finds no default at all. Two
definitions, one of them documented as agreeing with the other.

### How to make it one rule, using the repo's own mechanism

This project already solves "two languages must not drift" with **committed vectors mirrored across
lanes and diffed in CI** (`docs/contracts/beat-phase.vectors.json`, and the P205 view-resolution vectors
whose sync `ci.yml` enforces with a `diff -u`).

Do the same here: **`docs/contracts/default-file.vectors.json`** — cases of `(files[], expected default
filename or none)` covering PDF-only, image-only, mixed, DisplayOrder ties, and an empty pool. Go and
TypeScript each run the whole set. **Settle the PDF-vs-image question while writing the vectors**; either
answer is fine, but one of the two implementations changes and the vectors are what stop it drifting back.

## What changes

- **Viewer's strip** reads the **pool** (`listFiles`), not `getMyFiles`. It no longer honours the personal
  order — **a real behaviour change**: today `getMyFiles` returns the member's order and the Viewer is
  told to honour it. Losing it is deliberate, per VLL; say so in the commit rather than letting someone
  rediscover it as a regression.
- **`MyFileSelection`'s unset default** becomes **the shared default-file rule**, not "all pool files".
- **`MyFilesEditor`** seeds from the selection, so an unset song shows exactly the default file ticked —
  which is also what the stage will take. **The two now agree by construction**, which is the whole point.
- The `custom` pill still marks a saved selection; it is no longer the only signal, because the
  checkboxes finally mean what they appear to mean.

## The strip marks what is MINE (VLL, 2026-09-04)

VLL: *"peut etre qu'on peut avoir une puce ou une couleur de tab en bas qui dit que le fichier est choisi
pour moi ? on a pas l'ordre mais une indication, ou grisé ceux que je n'ai pas ?"*

**Yes — and it repays a real cost of the redesign.** Making the strip a pure pool loses the at-a-glance
*"which of these do I actually take on stage?"*. A marker gives it back **without** rebuilding the
original bug, because content and annotation become separate things: the strip shows **everything**, and
the marker says **which are mine**. The old defect was the strip's *content* silently claiming to be the
selection.

**Mark the selected; do not grey the rest.** In an interface, greyed means *unavailable* — but those files
open perfectly well in Studio, so dimming them is a false signal. An additive marker on the member's own
files says the true thing and implies nothing about the others. The marker carries a label/tooltip stating
what it means ("in my stage selection"), because an unexplained dot is another thing to misread.

**And it makes the default legible, which is the original bug's cure.** With ⟨R1⟩'s shared rule, a member
who has never chosen sees **exactly one** file marked — so the strip teaches the model on sight: *one is
marked, that is what I will get.* That is precisely the understanding that was missing when this task was
filed.

**Order stays out of the strip.** VLL: *"on a pas l'ordre mais une indication."* Order is a bake property
and belongs to my-files; showing it in a browser would re-blur the line this task draws.

## Acceptance

- **The vectors are the guard**: Go and TS both run `default-file.vectors.json`, and CI diffs the mirrored
  copies. **Teeth-check it** — change one implementation's predicate and confirm the other lane goes red.
- An unset song shows **exactly one** box ticked (the default file) in my-files, and the bottom strip shows
  **every** file regardless.
- Saving an empty selection is confirmed, and the member then gets **no pages** for that song in a bake —
  asserted on a bundle, not on the UI.
- Reordering a personal selection changes the **baked** sequence and **not** the strip.
- The strip **marks** the member's selected files and leaves the others fully enabled (not dimmed); an
  unset member sees exactly one marked, and it is the file the bake takes.
- A song whose only viewable file is an **image**: Studio and the baker agree on the default (this is the
  case that fails today).

## Out of scope

The baker's union-pool machinery (T137) — unchanged. This task only settles what "default" means and which
surface answers which question.
