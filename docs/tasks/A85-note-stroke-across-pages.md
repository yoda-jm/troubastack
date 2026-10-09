# A85 — A note stroke that leaves its page: split it, don't drop it; and show where the pages meet

**Lane:** mobile · **Status:** specced 2026-10-09, not started · **Origin:** VLL, 2026-10-09, after a device
session with Fable: *"en page partagée, un helper au milieu qui marque la délimitation devrait aider, on peut
aussi faire un nouveau down à l'entrée sur une nouvelle page et un up quand ça en sort"*.

## 1. What happens today (measured on VLL's tablet, 2026-10-09)

In two-up note mode each page has its own drawing surface (`NoteLayer`, one `pointerInput` per page). A stroke
belongs to the page where it went down. Every later point that is off that page (on the other page, in the
gutter, or in a FIT_PAGE letterbox bar) maps to `null` in `NoteGeometry.touchToNote` and is **dropped
silently** by `mapNotNull` (`NotePad.kt`, the commit and the eraser).

Two visible results, both seen in VLL's note `b5a486c3…` on 10-09:
- **A word written across the middle loses everything past the edge.** The left page kept one stroke, cut
  dead at x = 1600 (the note's right edge); the rest of the word was gone.
- **A stroke that leaves and comes back is joined by a straight chord** across the missing part, because the
  points either side of the gap become neighbours.

The capture showed the input itself was clean: ~100 Hz, 0.1 px resolution, no contact drop-outs. This is
purely how the app routes points.

## 2. The behaviour VLL chose

**⟨D1⟩ Leaving a page ends the stroke there; entering a page starts a new one.** One continuous pen movement
becomes one stroke **per page segment**, as if the pen had lifted at the edge and gone down again on the
other side:
- At the moment the pen crosses a page's edge, that page's stroke ends **at the edge**: the crossing point
  interpolated between the last inside sample and the first outside one. It is committed and saved like any
  pen-up.
- When the pen enters a page (the neighbour, or the same page coming back), a **new** stroke starts there,
  seeded with the entry point on the edge, and is committed at the next exit or the real pen-up.
- Points in the gutter or a letterbox bar belong to no page and draw nothing. That is right: there is no paper
  there.
- Both pages save. In two-up, both are editable, so a word across the middle ends up as two notes, one per
  page. Each is placed on its own page in Studio by T185/T186 as usual.

**The eraser follows the same rule.** It clears on whichever page it is over, and crossing the middle carries
on erasing on the other page instead of stopping.

**Single-page view:** the same rule against the letterbox bars and the page edges. Leaving the page ends the
stroke at the edge and coming back starts a new one, which removes the straight-chord artefact. No neighbour
exists, so nothing else changes.

**⟨D2⟩ Show where the pages meet, in note mode only.** In two-up note mode, draw a clear divider down the
seam between the two pages: a thin line in note-bar chrome (not ink), the full height of the pages. It shows
the writer where one note ends and the next begins, before the pen gets there. It is chrome: never in a note
bitmap, never baked, gone when note mode ends. It must read on all four colour schemes; check it on the tablet
in paper and in night.

## 3. Properties for the implementation (the mechanism is yours)

- **One owner for the gesture across the spread.** Today the per-page `pointerInput` that received the down
  keeps the pointer for the whole gesture, so the neighbour never sees it. Whatever you choose (one handler
  over the spread that routes points to pages, or another), **a single pointer must be able to produce
  strokes on both pages within one gesture.**
- **The edge point is interpolated, not snapped to the last sample.** At ~4 px between samples, a fast
  crossing would otherwise leave a visible gap before the edge.
- **The A71 rules still hold per segment:** no slop, the entry point is the first point, the first pointer owns
  the gesture, a dot is a dot.
- **The wet preview splits the same way**, so what the writer sees while drawing is what is committed. No wet
  ink in the gutter.
- **Auto-send (A77) and the ✎ index** see two committed notes, one per page, as two normal commits.
- Keep `StrokeReader` pure and unit-testable. The split belongs in pure code that can be tested off-device.

## 4. Not in this task

- Zooming the page to write small (VLL and Fable discussed it the same day: the touch panel itself loses the
  sub-millimetre detail of very small writing, measured as a collapsed 1 mm loop in the raw input). That
  would be its own task, if VLL wants it.
- Changing how Studio shows a note (T186).

## 5. Acceptance

- **Unit (pure):** a polyline that goes A → gutter → B yields exactly two segments. A's segment ends on its
  right edge at the interpolated crossing, and B's starts on its left edge. A polyline that goes A → bar → A
  yields two segments on A with **no** chord across the gap. A stroke entirely inside a page is unchanged. A
  stroke that starts in the gutter and moves onto B starts at B's edge.
- **Teeth:** restore the old drop-off-page behaviour, and the A → gutter → B test goes red. Print the swapped
  line.
- **Device (VLL's tablet, two-up):** write one word across the middle. Both halves stay on screen after pen-up
  and after closing and reopening the concert. Pull both note PNGs and attach them: each half is on its own
  page, ending and starting at the seam. Erase across the middle: both sides clear. Paste the divider in paper
  and night schemes.
- **Regression:** `:shared:testDebugUnitTest`, `:androidApp:assembleDebug`, the iOS compile, and the existing
  note tests (`StageNotesGuardTest`, `StageNoteModeTest`, the StrokeReader tests) are green.
