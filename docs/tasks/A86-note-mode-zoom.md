# A86 — Zoom in note mode: write big under the pen, land small on the page

**Lane:** mobile · **Status:** specced 2026-10-09, not started · **Depends on:** A85 (a stroke that leaves its
page splits). Build on its single touch → page routing; do not add a second mapping. · **Origin:** VLL,
2026-10-09: *"le plus pratique c'est des contrôles zoom +/− 1:1 et une sorte d'overview dans un coin pour
bouger la fenêtre de zoom"*.

## 1. Why (measured, 2026-10-09)

Small writing comes out without its detail. Fable captured the raw touch input on VLL's tablet (`getevent`,
the `fts_ts` panel) while he wrote "test" small and then large, with his disc stylus:
- **The input is dense and clean:** ~100 reports/s, 0.1 px resolution, no drop-outs.
- **The app is faithful:** the stored note matches the raw path stroke for stroke.
- **The panel loses sub-millimetre shape:** the small word was ~9 × 4.5 mm on screen, and the ~1 mm loop of
  its "s" is already a plain cusp in the raw data. The large word keeps the loop.

No app change recovers detail the panel never reported. Zooming does: the hand makes large, well-captured
movements, and the note receives them scaled down.

## 2. VLL's design

**⟨D1⟩ Controls in the note bar: `−`, `+` and `1:1`.**
- Steps **1×, 1.5×, 2×, 3×** (`+` and `−` walk them; `1:1` returns to 1× in one tap). The current factor shows
  between the buttons (`2×`).
- The zoom centres on the **current view's centre** the first time, then keeps the window where the overview
  puts it.
- The zoom is part of note mode: it **resets to 1× when note mode ends**, and is not persisted. Reading on
  stage is never zoomed by this.

**⟨D2⟩ An overview in a corner moves the zoom window.**
- Shown only while zoomed (> 1×): a small thumbnail of the whole visible surface (the page, or the two-up
  spread), with its notes, and a rectangle for the zoomed window.
- **Drag the rectangle** to move the window, or **tap** a point in the thumbnail to centre the window there. The
  window is clamped to the page(s) and never shows beyond the paper.
- It sits in the **top-right corner** by default, clear of the note bar. A small button collapses it to an icon
  (and back), for when it covers what you want to write on. Collapsed or not is remembered for the note-mode
  session only.
- The overview is chrome: it takes no ink. A touch on it never draws, and it does not belong to any page
  (A85: it is "no paper").
- **Writing never moves the window.** No auto-advance. The overview is the only way to pan in this task,
  because two-finger pan would fight A71's "the first pointer owns the stroke".

## 3. What a stroke means while zoomed

- **Ink lands where the pen is on the page.** The touch → note mapping includes the zoom window, so writing at
  3× puts a stroke a third of the size, at the right place on the paper. One mapping function serves the
  pencil, the eraser, A85's edge splitting and the wet preview. A second copy is how they drift apart.
- **Pen and eraser widths stay in page units** (what lands on paper is what the width picker says), so at 3×
  the line looks three times as thick on screen. That is correct: it is the true ink seen through a magnifier.
- **A85 holds while zoomed.** The window can straddle the seam in two-up, and a stroke crossing it splits as in
  A85. The A85 divider is drawn in the zoomed view too.
- **"Clear page" and send** act on pages, as today; zoom does not change which page is which.

## 4. ⟨D3⟩ Resolution: what you see at pen-up must be what was drawn (measure first)

The note bitmap is `NOTE_W = 1600` px across a page. At 1× in two-up landscape, a page is ~960 screen px wide,
so the note has spare resolution. At 3× the same page is ~2900 screen px wide, so the stored note becomes
**coarser than the screen**. The wet preview (drawn as a vector) would then be crisp, and the committed stroke
visibly softer at pen-up, which looks like the app "changing" the writing.

**Measure before changing anything.** At 2× and 3× on VLL's tablet, write the same small word. Screenshot the
wet stroke just before pen-up and the committed one just after, at the same crop, and attach both pairs.
- **If the committed ink is visibly softer at 3×**, the fix is a higher note resolution. That changes the
  stored format: the PNG size, the upload, and T186's viewer, which composites the PNG over the page. **Bring
  it to the gate as a proposal before implementing it** (a number, its file-size cost on a real page, and the
  Studio side's check). Do not raise `NOTE_W` inside this task on your own.
- **If it is acceptable**, say so with the pair as evidence, and cap the steps at the largest factor that
  passes.

## 5. Not in this task

- Two-finger pan or pinch-to-zoom.
- Auto-advancing the window while writing (a "writing line" that shifts on its own).
- Zoom outside note mode, for reading.
- Changing `NOTE_W` (see §4: proposal first).

## 6. Acceptance

- **Unit (pure):** the zoomed touch → note mapping. At 1× it equals today's mapping, for both fill modes. At
  2× with the window at a known offset, a touch maps to the expected note point. Window clamping keeps the
  window inside the page(s) at every step. A85's split still yields two segments while zoomed in two-up.
- **Teeth:** drop the zoom term from the mapping, and the 2× test goes red. Print the swapped line.
- **Device (VLL's tablet):**
  - Write the same short word at 1× and at 3×, the same physical size under the pen. At 3× the stored word is
    a third of the size, and its small loops survive. Attach the two note PNG crops side by side.
  - Move the window through the overview by drag and by tap. Collapse and expand the overview.
  - Leave note mode: the view is back at 1×.
  - The §4 wet/committed pairs.
- **Regression:** `:shared:testDebugUnitTest`, `:androidApp:assembleDebug`, the iOS compile, and the existing
  note tests are green.
