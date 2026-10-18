# T199 — Straighten a crooked scan (per-page deskew), on top of T198's crop

Owner: web-core (core bake + Studio). Depends on: T198 (crop model, cropped render, overlay transform, crop API).
Spec: Fable. Status: specced; one decision marked **[VLL]**, implement on its default.

## Requirement (VLL, 2026-10-18)

> "surtout b, tu peux le spec comme un nouveau requirement, pour a on va dire que c'est au musicien de faire ça"

- **In scope (b):** a page scanned a few degrees crooked bakes **straight**. That means a small angle per page,
  set by hand or detected automatically.
- **Out of scope (a):** quarter turns (90°/180°/270°) of a page scanned sideways or upside down. The musician
  fixes the PDF before uploading. Do not add a quarter-turn control, and do not auto-detect orientation.

## Principles (inherit T198's P1–P4)

- **Non-destructive.** The PDF bytes are never modified. Angle 0 (or no angle) bakes byte-identically to
  T198's output for that page.
- **Sharp.** The straightened page is rotated from a render at no less than T198's `dpi_eff` for that page,
  never from the 150-dpi whole-page raster. A straight edge in the source must stay as sharp as in an
  unrotated crop (§Quality).
- **Marks don't move in storage.** Annotations keep their stored source-frame coordinates. Deskew is part of
  the bake-time VIEW transform, exactly like the crop.
- **Uploaded PDFs only** (generated charts are already straight).

## Data model

Extend T198's per-page entry. No new top-level field:

```
crop.pages["<i>"] = { x, y, w, h, source, angle? }   // angle in degrees, + = counter-clockwise correction
```

- `angle` is optional. 0 or absent means no rotation (P1 byte-identity).
- Validation: finite, `|angle| ≤ 10`, rounded to 0.05° on save, so tiny float noise can't mint new raster hashes.
- **Frame (amended 2026-10-18, follows VLL's UI: the user rotates a WINDOW over the uploaded page).**
  `x,y,w,h` describe the window's unrotated box in the SOURCE page frame. `angle` rotates that box about its own
  centre. The bake straightens the content so the window becomes upright, then cuts it. With angle 0 this is
  T198's rect, unchanged.
  - Validation: all four rotated corners lie in [0,1]², computed in PHYSICAL units with the page's W/H.
  - Studio clamps to the same rule.
  - The angle's sign convention is pinned by one test: a window rotated +θ to match staves crooked by +θ yields
    level staves.
- A page can carry an angle with a full rect (`0,0,1,1`) when only straightening is wanted.
- The angle enters the raster cache key and the bake input exactly as the crop does. It goes through the same
  crop API (`PUT …/files/{fileId}/crop`, dryRun included) and the same `.tband` export/import. Extend the
  round-trip and sanitizer tests (`importCrop` drops an invalid angle).

## Bake

For a page with an angle:
1. Take the axis-aligned bounding box of the rotated window in the source frame.
2. Render only that box from the PDF at `dpi_eff` (`pdftoppm -x -y -W -H`, T198's floor and native cap
   included), plus a few pixels of margin for the interpolation kernel.
3. Rotate by the angle with a proper interpolating resampler (bicubic / Catmull-Rom class; never
   nearest-neighbour), on a white background.
4. Cut the exact crop window. Apply the T169 greyscale re-encode as today.

The core is stdlib-first (`core/go.mod` header). Either write the resampler in-tree (small, pure, tested) or add
`golang.org/x/image` deliberately and say so at the gate. Both are acceptable; a shell-out to a new binary is not.

## Autodetect angle

- Pure Go, on a downscaled render of the page.
- Choose the angle in [-10°, +10°] that maximises the sharpness of the horizontal projection profile (the
  variance of row ink sums). Staff lines and text lines make this peak sharply when level.
- Coarse-to-fine: 0.5° steps, then 0.05° around the best.
- Report a confidence (the peak's height relative to the mean). Under a threshold, propose **0**: never tilt a
  page on a weak signal.
- Run it **before** crop autodetect, so the bbox is measured on the straightened page.
- **Test vectors** (synthetic, generated in the test; no real scan committed):
  - a page of staff lines rotated +2.3° → detected within ±0.1°;
  - the same at −4°;
  - a straight page → 0;
  - a blank page → 0 with low confidence;
  - a page with one long diagonal line and little else → 0 (low confidence), never the diagonal's angle.

  The last vector must fail on a naive "strongest line" detector. Prove it in the submission.

## Annotations

The view transform becomes **rotate about the page centre, then crop**. It must be applied in PHYSICAL units:
normalized x and y have different scales on a non-square page, so convert with the page's W/H before rotating
and back after.

- Freehand and line points rotate exactly. Stroke width and text size are unchanged by the rotation; the crop
  still scales them, as in T198.
- Text marks: the anchor rotates, and the text is still drawn horizontal, which is what a musician wants on a
  straightened page.
- Two-point shapes (rect, ellipse, highlight box): rotate both corners and re-take the axis-aligned box. At
  ≤10° this is the intended "it now sits square on the straightened text"; document it in a test.
- **One function.** T198's `CropHidesMark` and the bake drop must go through the SAME page-view transform
  (rotate then crop). Do not add a second copy for rotation. The dryRun's hidden-marks list must stay equal to
  what the bake drops.

### [VLL] Where Studio edits

- **Default (implement):** Studio keeps showing and editing the page as uploaded (crooked), with stored marks in
  that frame. "Preview as tablet" shows it straightened, with the transformed overlay.
- **Alternative:** Studio shows and edits the straightened page, inverse-transforming new marks into the source
  frame on save. That is nicer to annotate, and a larger change to Studio's canvas.

Build the default. Keep the transform pure and invertible so the alternative is a later step, not a rewrite.

## Rehearsal notes

Same as T198 §Annotations 3: an angle change changes the page's raster hash. The page's notes read "page
changed" and stop showing on that page on the tablet. They count in the pre-save warning (computed server-side
against the latest bake's sidecar), and the T186 background is still served.

## Studio UI

This goes into T198's crop tool, not a second tool.
- An angle control per page: fine steps (±0.05° / ±0.5°) plus a numeric field.
- **Auto angle (this page)** and **Auto angle (all pages)**.
- Alignment: use T198's window grid (it rotates WITH the window). The user rotates until the grid's horizontals
  run along the staves. No separate page-level guide lines.
- Reset clears the angle together with the crop, or offer "Reset angle" separately if the UI makes it natural.
- The preview shows the straightened, cropped page.

## Quality (acceptance)

- On a synthetic page with a thin horizontal black line rotated by +3°:
  - deskewing by +3° gives a line whose vertical thickness (rows above 50% ink) is ≤ the thickness of the same
    line rendered unrotated at the same `dpi_eff` + 1 px;
  - its row profile has a single peak.
  - A nearest-neighbour or 150-dpi-then-rotate implementation must fail this. Show it.
- Angle 0 is byte-identical to T198's output (P1).
- An overlay mark drawn on a source-frame note head lands on the same note head in the straightened bake. Use
  a synthetic page with a dot, and check that the overlay's dot centre is within 1 px of the raster dot centre.
- Export/import round-trip carries the angle. A hand-edited invalid angle is dropped on import.
- **Device pass (VLL):** a real crooked scan, auto angle, bake, read on the tablet.

## Documentation

Update T198's documented-feature copy once this lands: the site bullet may then say **"Scanned crooked, with
big margins? No problem"**, and USER-JOURNEY gets one sentence on the angle control. Use the same labels as the UI.

## Out of scope

- Quarter turns (VLL: the musician's job).
- Perspective or keystone correction (phone photos taken at an angle), and page curl.
- Per-region angles within one page.
