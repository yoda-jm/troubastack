# T84 — Stroke width: usable steps, and headroom above chart-text size

**Priority:** normal · **Size:** S/M · **Area:** `web/studio` (Toolbar width control + default/state).
**No wire or bake change** — width stays a fraction of page width. VLL 2026-08-21: *"the size of the
draw tool (at least the freehand) is a little bit not perfect — we have a lot of small sizes, and it
stops at a size that is just the text size in the chart, so no room if we have PDFs with bigger
fonts."*

## The complaint is literally true — measured

The control is `min 0.001 / max 0.02 / step 0.001` (page-width fractions), 20 linear notches. On A4:

| slider | width | vs chart body text (11 pt = 3.88 mm) |
|---|---|---|
| min 0.001 | 0.21 mm (0.6 pt) | hairline |
| 0.005 | 1.05 mm (3.0 pt) | ¼ |
| **max 0.02** | **4.20 mm (11.9 pt)** | **1.08× — one line of body text** |

1. **The ceiling really is text-sized.** The widest stroke available is 1.08× the height of the text
   it annotates. On a PDF with larger print — an engraved title, a large-print part, a scan at bigger
   scale — there is no marker weight left to reach for.
2. **The steps are badly distributed.** With a constant *delta* of 0.001, the bottom half of the
   travel (0.001→0.010) spans a **10×** range while the top half (0.010→0.020) spans only **2×**. So
   every perceptible change is crammed into the first few notches and the top half feels inert —
   which is exactly "a lot of small sizes".
3. **Our own data agrees.** Every pen width in the seeded showcase is **0.003–0.005** — notches 3 to 5
   of 20. Fifteen of the twenty notches are effectively dead for line work.

## Design (ruled)

1. **Geometric steps, not linear.** Each notch is a constant *ratio* (≈1.25–1.3), so a step feels the
   same at every point on the slider. Roughly 16 stops from **0.0008** (≈0.17 mm — a true hairline for
   dense engravings) to **≈0.05** (≈10.5 mm — a genuine marker). Implement as an index→width mapping
   function, not by fiddling `step` on the raw input; the slider's position is an index into the
   table.
2. **Ceiling ≈0.05–0.06.** That is ~2.5–3× the current max and gives real headroom over big print.
   Do not go much beyond: past ~12 mm a "stroke" stops behaving like ink and should be a highlight
   rect instead.
3. **Label it in physical units.** `4.0` means nothing to someone marking a printed page — show **mm**
   (a `pt` suffix is fine too). It also makes "about as thick as the text" a judgement the user can
   actually make.
4. **Never clamp a stored width.** An existing object whose width falls outside the new stop table
   must render **unchanged**, and must not be silently rewritten when the user edits some *other*
   property of that object. Snap-to-nearest-stop on load would retroactively re-weight annotations in
   every band's charts — the kind of silent data edit we have refused elsewhere (T72's create-time
   default, T79's no-migration). The slider may show the nearest stop as its *position*; the stored
   value only changes when the user actually moves it.
5. **Remember the width per tool.** A marker swipe and a box outline want different weights, and
   VLL's "at least the freehand" hints the freehand is where it bites. Keep the last width per tool
   kind (freehand / line / rect+ellipse) for the session rather than one global value.

## Deliberately NOT in this task

**Content-adaptive default width.** The idea is sound and we already do it server-side — B13's
highlighter derives its width from the target's height (`Width: t.h() * 1.15`, `cmd/seed/anchors.go`)
rather than a fixed fraction. The editor equivalent would measure the page's dominant text height via
pdfjs `getTextContent` and set the *initial* width from it. But the song editor does not read text
content at all today, so this means new per-file plumbing and state — and once §1–§2 land, reaching
any weight takes a couple of notches anyway. **Defer it**; revisit only if people are still fiddling
with the slider after this. If it comes back, it changes only the *default*, never the stored value.

## Acceptance criteria

- The stop table is geometric: a unit test asserts the ratio between consecutive stops is constant
  within tolerance, that the first stop ≤0.001 and the last ≥0.05.
- **Round-trip guard, red-first-able:** an object stored with a width that is not on the table (e.g.
  0.0037, as the seeded charts use) keeps that exact value after the user edits an unrelated property
  (colour/opacity) on that object. Assert the persisted value, not just the rendering.
- The displayed label is in mm and matches the rendered stroke — spot-check at three stops (min, a
  middle stop, max) against measured ink in a render.
- Per-tool width memory: set a wide freehand, switch to rect, switch back — the freehand width is
  still wide.
- Existing `style-width` / `style-width-value` testids stay attached (T78/T80 guard); run the
  **dangling-testid sweep** and the **full** e2e suite.
- Handoff shows a marker swipe at the new maximum over a large-print page beside one at the old
  0.02 maximum — the headroom is the point, so make it visible.
- `tsc -b studio` clean.
