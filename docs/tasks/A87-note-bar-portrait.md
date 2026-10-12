# A87 — The note bar fits a portrait tablet again, and the zoom overview frames what is actually visible

**Lane:** mobile · **Status:** specced 2026-10-12, not started. Small, and urgent: it is on VLL's tablet now.
**Origin:** VLL, 2026-10-12, after installing main (`0.1.0+ae0bd38c`) on his tablet: *"la barre en bas est
trop épaisse"*, and earlier *"l'aperçu dans le coin ne marche pas du tout"*. That second one turned out to be
"I hadn't zoomed", but it also exposed problem 2 below. Fable reproduced both on the tablet; screenshots are
local only, real chart.

## 1. The note bar overflows in portrait

On the 1200×1920 portrait panel the note bar is laid out in one row:
- the pencil and the eraser;
- 3 pen sizes and 4 colours;
- A86's `−` `1×` `+` `1:1`;
- then **Clear page** and the **✓** that ends note mode.

That row is wider than the screen. **"Clear page" is squeezed into a one-letter-wide column, its letters stacked
vertically down the right edge.** That stacked column is what makes the bar about 3× taller than it should be.
**The ✓ is pushed off screen**, so the way out of note mode has disappeared.

The bar must:
- **keep every control visible and tappable in portrait**, Clear page and ✓ included, with ✓ always visible.
  ✓ is the exit (A73);
- **stay one row tall** (its landscape height) whenever it can, with no text ever wrapping letter by letter:
  a `Text` in a squeezed slot needs `maxLines = 1` and must not shrink below its intrinsic width;
- choose the fallback when one row can't hold everything (layout is yours, but state it): a second row for
  the zoom controls, or the zoom controls collapsed into one `1×` chip that opens a small popover with `−`/`+`/`1:1`.
  **VLL's ask is a thin bar.** Prefer the chip if it keeps one row in portrait.

## 2. The overview's window rectangle ignores the bar

In fit-width portrait at 1.5×, the overview's rectangle covers about 30% more page than the screen shows. The
window is computed on the full surface height, but the note bar covers the bottom of the surface. The
rectangle must frame **what the writer can actually see and write on**:
- use the surface **minus the area under the note bar** when computing and drawing the window;
- clamp against that area too, so the bottom of the page can be brought above the bar.

## 3. Acceptance

- **Device, VLL's tablet, portrait.** The bar in note mode is one row at about its landscape height (or the
  documented fallback). Clear page and ✓ are visible and work. Attach a screenshot. Synthetic data on the
  emulator is fine for the committed screenshot; on the tablet, check by eye only.
- **Landscape two-up is unchanged.** Screenshot it.
- **Overview.** At 1.5× and 2× in portrait, the rectangle's bottom edge matches the last line visible above the
  bar, to within a line. Test it with the pure `NoteZoom.window` plus a bar inset: the window's height is
  (surface − bar) / zoom.
- **A unit test** pins that ✓ (`note-done`, or whatever it is called) is placed inside the screen width at
  1200 px portrait. A Compose layout test is fine. Teeth: re-add the overflow, and the test goes red.
- **Regression:** the CI set is green.
