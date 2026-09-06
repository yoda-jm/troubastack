# T156 — The style bar on a narrow screen: reach it, and see the size before you draw

**Lane:** web-core (studio). **Size:** M. **Status:** spec, 2026-09-06, from VLL. Two parts on one surface,
and they interact — B adds width to the bar A is trying to make reachable.

## ⟨A⟩ The second toolbar cannot be panned when it overflows

VLL: *"sur un écran étroit la seconde barre d'outils avec taille couleur… n'est pas bougeable comme celle
du haut pour voir plus loin quand tout n'est pas affiché, c'est embêtant."*

**Do NOT add a third scroll mechanism before reproducing this.** I checked the CSS and both bars already
declare what they need:

```
.topbar-pill .tool-palette   overflow-x: auto; touch-action: pan-x;   (T65 C / T66 E)
.ctx-bar .style-controls     overflow-x: auto; touch-action: pan-x;   (line ~826)
```

Both also get the `.of-start`/`.of-end` fade masks. **So the mechanism is present and something else
defeats it in practice** — a clipping ancestor, a wrap instead of a scroll, a container that sizes to
content, or the row simply never becoming scrollable because it wraps first.

**Required first step:** reproduce at a narrow viewport in Playwright (a phone width), and report *why* the
existing scroll does not reach — measure `scrollWidth` vs `clientWidth` on `.style-controls`, and whether
the row wrapped. Fix the cause. Bolting a drag handler onto a row that was already meant to scroll would
hide the real defect and add a second thing to keep in sync.

I am deliberately not naming a cause: I read CSS, I did not observe the page. Last night I mis-routed a bug
three times by reasoning from artefacts instead of attaching a debugger.

## ⟨B⟩ You cannot see how big the tool is until you have used it

VLL: *"il faudrait aussi savoir quelle est la taille de l'outil avant de l'utiliser (un cercle grisé
pointillé pour le freehand/trait/…, et un texte pareil quand c'est du texte)."*

Today the size is a number or a slider — you learn what it means by drawing and undoing. His proposal is
right and it is the conventional answer: **show the size, do not describe it.**

- **Stroke tools** (freehand, line, arrow, highlight…): a **greyed dotted circle** whose diameter is the
  current stroke width, rendered at the canvas's current zoom so it means the same thing as the ink.
- **Text**: a short sample rendered **at the chosen size and font**. Use a neutral sample (`Abc`, or the
  localised equivalent) rather than a brand word — a brand string in a tool preview is a maintenance
  liability and a translation problem.
- The preview updates live as the size changes, and it is **not interactive** — it is a legend, not a
  control.

### The interaction with ⟨A⟩, which is the reason these are one task

The preview **adds width** to the very bar that already overflows. So it must be built as part of the
overflow solution, not before it: whatever makes the bar reachable has to account for one more item, and
the preview should be among the first things visible rather than the item that gets pushed off the edge.

## ⟨R1⟩ Red first

- **⟨A⟩** At a phone viewport with enough controls to overflow, assert the last control is **reachable**
  (scroll the container, then assert it is in view and clickable). Red today.
- **⟨A⟩ teeth:** the same assertion at a desktop width must pass **before** the fix — otherwise the test is
  measuring something other than the overflow.
- **⟨B⟩** With stroke size *s*, assert the preview circle's rendered diameter tracks *s* (two different
  sizes, two different diameters — a preview that never changes would pass a presence-only test).
- **⟨B⟩** With the text tool, assert the sample renders at the chosen size, and that **no brand string** is
  used.

## Device-QA

Bundle with the owed pass: whether a dotted circle at 2 px is legible at arm's length is not a question a
viewport test answers.
