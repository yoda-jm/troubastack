# T155 — A shape you have just drawn should not stay selected

**Lane:** web-core (studio, annotation canvas). **Size:** S/M. **Status:** spec, 2026-09-06, from VLL.

## What VLL reported

*"apres avoir dessiné une forme elle est selectionnée, c'est pas terrible vu qu'on reste sur l'outil, on a
l'impression qu'on peut la bouger, et c'est encore pire pour les traits et freehand car ça nuit à la
lisibilité de ce qu'on a fait."*

Two distinct problems, and he named both precisely:

1. **The affordance lies.** You are still holding the drawing tool, but the object is selected — which
   says "you can move me". You cannot, not without switching tools. The interface offers a handle that
   does nothing.
2. **It hides the work.** Selection chrome — bounding box, handles — sits on top of what you just drew.
   For a rectangle that is tolerable; for a **line or a freehand stroke** the box is far larger than the
   ink and obscures the very thing you were checking.

The second is the one that matters on stage: freehand over lyrics is the showcase gesture of this product,
and the moment you finish a stroke is exactly when you want to see it cleanly.

## The rule

**Selection is a state of the SELECT tool.** While a drawing tool is active, nothing is selected. Finishing
a stroke leaves the canvas showing the stroke and nothing else; the tool stays armed for the next one,
which is what a musician marking up a chart actually does — several marks in a row, not one then a move.

Switching to the select tool is what makes objects selectable, and *that* is when handles belong on screen.

## Required

- On completing a draw, **do not select the new object**, whatever the tool.
- The tool remains active (unchanged).
- Undo still targets the last drawn object — do not implement "deselect" by losing the reference.
- If some flow genuinely needs the new object selected (e.g. text, where you may want to type straight
  away), state that exception explicitly rather than leaving it implicit.

## ⟨R1⟩ Red first

- Draw a freehand stroke: assert **no object is selected** afterwards, and that no selection chrome is
  rendered. Red today.
- Same for a line and for a rectangle — VLL named lines and freehand as the worst, but the rule is uniform.
- Draw two strokes in a row without touching the toolbar: **both exist**, neither is selected. This is the
  real usage and it guards against "fixing" the problem by clearing the canvas state too aggressively.
- Switch to the select tool and click the stroke: it **is** selectable, with handles. The rule removes
  selection from drawing, not from the product.

## Device-QA

Bundle with the existing owed pass. Legibility of a fresh stroke is exactly the kind of thing that reads
differently on a tablet under a hand than in a desktop browser.
