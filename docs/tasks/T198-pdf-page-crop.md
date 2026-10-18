# T198 — Per-page bake area (crop) for uploaded PDFs, with autodetect

Owner: web-core (Studio + core bake). No mobile half (amended — see §Annotations 3; A89 is withdrawn).
Spec: Fable. Status: specced — three decisions for VLL marked **[VLL]**; implement the rest, those default as stated.

## Problem

An uploaded PDF is often a scan that does not fill its page: wide white margins, a skewed sheet sitting in
the middle, a phone photo with the table edge around it. The bake rasterizes the WHOLE page
(`core/internal/bake/render.go`, `pdftoppm -png -r <dpi>`), so on the tablet the music is small and the
margins waste the screen. VLL wants, **per page**, in Studio, a way to set the area that gets baked — with an
autodetect when enabled, keeping the aspect ratio, and **keeping the original PDF's precision**.

## Principles (non-negotiable)

- **P1 — Non-destructive.** The uploaded PDF bytes are never modified. A crop is metadata on the file; Reset
  returns the exact uncropped bake.
- **P2 — Precision is kept, not resampled away.** A cropped page is rendered FROM THE PDF at a higher
  effective DPI over the cropped region (`pdftoppm -x -y -W -H` at the scaled resolution), never by cropping
  and upscaling the 150-dpi raster. See §Precision.
- **P3 — Annotations never move because of a crop.** Marks keep living in the FULL-page frame they are
  stored in today (`points` page-relative [0,1], stroke width a fraction of page width — `web/ink/src/index.ts`).
  The crop is a VIEW transform applied at bake and in Studio's cropped preview; changing or resetting a crop
  changes no stored coordinate. See §Annotations.
- **P4 — Uploaded PDFs only.** Generated charts (chartpdf) already lay out to the page; no crop UI for them.

## Data model

On the song file (uploaded PDF), additive and optional:

```
crop: {
  auto: bool                     // RESERVED, always false (ruling A, 4b): no persisted autodetect toggle
  pages: { "<pageIndex>": { x, y, w, h, source: "auto" | "manual" } }   // source-page normalized [0,1]
}
```

- Rects are in the SOURCE page frame, normalized [0,1], origin top-left — the same frame as annotations, so
  one transform serves both.
- A page with no entry bakes uncropped. **Amended (Fable, stage-4b ruling):** detection never runs at bake, and a
  bake never writes to the file record. Autodetect is a save-path operation: a read-only detect call proposes
  rects (with the hiddenMarks for the proposal), Studio shows them, and the user commits them through the normal
  crop save with `source: "auto"`. **Ruling A (4b):** there is no persisted autodetect toggle. "Auto (all pages)" is the action; it fills pages
  with no rect or with an `auto` rect, and the per-rect `source` drives the "auto" badge. A `manual` rect is never overwritten.
  The bake only reads stored rects, so what Studio shows is what bakes. Auto-on-upload is out of scope.
- Validation server-side: 0 ≤ x, y; x+w ≤ 1; y+h ≤ 1; w, h ≥ 0.2 (a smaller rect is almost surely a mistake
  — refuse with a message). Unknown page indexes are refused.
- The crop is part of the file's bake input: it enters the raster cache key and the bake revision hash, so
  changing a crop marks the song dirty and (live mode) autobakes like any other edit.
- Export/import (.zip backup, zzexport) carries `crop` — add it to the round-trip test.

## Aspect ratio **[VLL]**

VLL said "keeping AR". Two readings; I do not rule:

- **A (default, recommended): the rect is locked to the source page's aspect ratio.** A crop is then exactly a
  zoom: the tablet page shape never changes, page turns and two-up layout behave as today, and no page gets
  letterboxed. Autodetect computes the content bbox, then grows it (never shrinks it) to the page AR, centred
  on the content, clamped inside the page.
- **B: free rect, the CONTENT's AR is kept (no stretching).** Tighter on landscape-ish content, but pages of
  one song get different shapes; Stage's fit/scroll logic (T149 scroll trim, two-up) must handle mixed shapes.

Implement A. Keep the AR policy one field (`crop.lockAR`, default true) so B is a later toggle and not a rewrite;
do not build B's Stage consequences in this task.

## Autodetect

- Pure Go, on the page raster the bake already produces (no new binary): find the bbox of "ink" pixels using
  the same `isDarkInk` predicate T149 uses (`core/internal/bake/content_bottom.go`), with noise rejection:
  ignore connected specks below a small area, and ignore a dark band touching the page edge (scanner bed /
  table edge / fold shadow) — the two failure modes of a naive bbox on real scans.
- Add a margin (default 2% of page width each side), then apply the AR policy, then clamp.
- If detection finds nothing, or the result covers > 95% of the page, store no rect (bake uncropped).
- Detection is per page. Studio offers: **Auto (this page)**, **Auto (all pages)** (does not touch manual
  rects), **Reset (this page)**. There is no persisted Autodetect toggle (ruling A, see §Data model).
- Test vectors (red-first, synthetic images generated in the test — no real scan committed): centred content
  with white margins; content + black scanner-edge band on one side; content + a single isolated dust speck
  far away; blank page; full-bleed page. The edge-band and speck vectors must fail on a naive bbox (prove it
  in the commit: run them against the naive version first).

## Precision

- Today: whole page at `dpi` (default 150). Target width of a baked page = W pixels.
- Cropped page: render only the rect, at `dpi_eff = dpi / w`, so the cropped region comes out ≈ W pixels
  wide — the music fills the tablet at the same pixel density the screen gets today, with real detail from
  the PDF rather than an upscale.
- **Cap at the source's native resolution:** for a scanned page (single full-page image), rendering above the
  image's own ppi adds bytes, not detail. Read the native ppi with poppler's `pdfimages -list` (same package
  as pdftoppm; already in the image? — verify in the container, memory: build-reads-outside-package) and use
  `min(dpi_eff, native_ppi)`; for vector pages there is no cap other than a pixel budget (longest side ≤ 4096).
- **[VLL] Should UNCROPPED scanned pages also bake at native ppi?** VLL's "keep original pdf precision" may
  mean the 150-dpi bake already loses detail he can see. This costs bundle size (a 300-ppi scan is 4× the
  pixels). Default in this task: NO — only cropped pages get the higher DPI; measure and report the bundle
  size delta for the three bands' real files (locally, not committed) so VLL can decide with numbers.
- The T169 greyscale re-encode applies unchanged.

## Annotations

Existing marks are stored in the full-page frame and must not move (P3). Consequences, all in scope:

1. **Bake overlay.** The overlay worker renders marks into the CROPPED frame: `x' = (x − cx)/cw`,
   `y' = (y − cy)/ch`, stroke width and text size scaled by `1/cw` (they are fractions of page width), so a
   mark lands on the same note of music it was drawn on. Do this in ONE place (the bake's overlay input), not
   in ink: ink keeps drawing page-relative points into whatever page it is given.
2. **Marks outside the crop.** A mark fully outside the rect is not drawn on the tablet; a mark straddling the
   edge is clipped. Studio must warn BEFORE the crop is saved: "2 annotations fall outside this area (layer
   X, Y) — they won't show on the tablet." with Keep crop / Grow crop to include them. Never silently drop:
   the mark stays stored and reappears on Reset.
3. **Tablet rehearsal notes (amended after web-core's code read — the first version of this item was wrong).**
   A rehearsal note is NOT vector points: it is a transparent PNG pinned to the exact baked page it was drawn
   on (`RasterHash` + `ConcertRev` + `PageInSong`, `core/internal/app/rehearsalnotes.go`), shown in Studio over
   its OWN captured raster (`PageRasterByHash`), and keyed on the tablet by `(songId, rasterHash)`
   (`NotePad.kt`). Nothing is ever re-placed — by design (T145). So there is no coordinate to convert, and the
   earlier "convert cropped→source points + round-trip" requirement is withdrawn. The real consequence of a
   crop is different and must be handled:
   - Cropping (or re-cropping, or resetting) a page gives it a new raster hash. On the tablet, that page's
     notes **stop showing on the page** (key mismatch); in Studio they read **"page changed"**.
     That is the honest existing behaviour — keep it — but VLL must not discover it by surprise.
   - Studio's crop warning (item 2) therefore also counts the **rehearsal notes on that page**:
     "This page has 3 rehearsal notes. After this change they'll show as "page changed" and no longer
     appear on this page on the tablet." Same Keep / Cancel choice.
   - Pin it with a test: a crop change on a page with a note → `pageChanged == true`; the note's
     `PageRasterByHash` background is still served (the old raster) so the T186 viewer keeps working.
   - The crop is recorded server-side in the bake's page-sources record (per rev, per page), NOT in the bundle:
     no tablet consumer needs it today. Should a vector tablet-mark ingest ever exist, the conversion uses that
     record, in ONE function, and an ingest must be able to tell which crop a mark was drawn against.
4. **Anchoring (T194–T197).** Uploaded files have no line source, so marks there are not linked; nothing to
   change. State it in a test name so a future change can't assume otherwise.
5. **Studio editing.** Studio keeps editing in the full-page frame (the canvas shows the full page with the
   cropped-out area dimmed). A "Preview as tablet" toggle shows the cropped page with the transformed overlay
   — it must use the same transform function as the bake (shared TS helper, or the server-rendered preview).

## Studio UI — VLL's design (2026-10-18; supersedes the earlier sketch)

> "a transparent area that you can move/rotate/scale that has always the aspect ratio of a page and cannot go
> out of the real pdf page, the inside is transparent … the outside area is light transparent grey with maybe
> dashed 45° grid in darker grey … the greyed area can stay even when the tool is not selected … a display as
> in tablet … should be a simple switch"

- **The window.** A transparent rectangle, ALWAYS the page's aspect ratio (policy A: w == h in page fractions).
  - **Move:** drag inside it.
  - **Scale:** drag a corner, about the opposite corner or the centre (pick one and keep it).
  - **Rotate:** a handle above the top edge (arrives with T199; build U1 so the handle slots in).
  - It **never leaves the real PDF page**. Every corner, rotated ones included, stays inside [0,1]². A drag that
    would push it out is clamped, never refused with an error.
  - Minimum side 0.2.
- **Affordances.** Inside the window: a faint grey move cross. Near the corners: a scale sign. On the rotate
  handle: a rotate sign. Static by default. A subtle animation on hover/focus is allowed, and must be OFF
  under `prefers-reduced-motion`.
- **Alignment grid (VLL, 2026-10-18):** "a very large grid in the middle, it helps seeing how it aligns and
  moves, so the next rotate will be easier to align".
  - While the crop tool is active, the window shows a large-cell grid INSIDE it: about 4×4 cells, thin and
    semi-transparent, readable over both white paper and dark ink. A centre cross may be slightly stronger.
  - The grid belongs to the WINDOW: it moves and scales with it, and with T199 it rotates with it. Aligning
    the grid's horizontals on the staves is how the user judges the rotation.
  - Hidden when the tool is off; the veil alone stays.
  - Pointer events pass through to the window's own move gesture.
- **Outside the window:** a light, translucent grey veil with a dashed 45° hatch in a darker grey.
  - The veil **stays visible when the tool is not selected**, so the baked area is always in view while
    annotating.
  - It must not block drawing: pointer events pass through when the tool is off.
- **"Tablet view" switch** (one simple toggle, Fable's call on VLL's "I let you decide"):
  - It swaps each VISIBLE page of the scroll view for its server-rendered bake preview (U1b: the same bake code,
    the draft crop, no write), lazily and cached per (page, crop).
  - Annotation editing is disabled while it is on.
  - No client-side transform, ever.
- The buttons Auto (page / all; never overwriting a manual rect), Reset (page / all, in-page confirm) and the keyboard
  (arrows move, Shift = bigger step, Escape cancels the current gesture) are as before. The labels are product
  words; bring them to the gate as one list.

## Documented feature (last stage, VLL)

VLL wants this to ship as a **documented feature**, not a hidden tool. Once the Studio UI lands, in the same
landing or right after it:

- **Public site** (`web/site/index.html`, the "Bring a PDF" `feat-list`), one bullet in the page's voice.
  Starting copy, which may be tuned at the gate:
  `<li><strong>Scanned crooked, with big margins? No problem</strong> — TroubaStack finds the music on each
  page and fills the screen with it, rendered sharp from your PDF. Your marks stay on their notes.</li>`
  "Crooked" is only true once deskew exists. Until then, use "Scanned with big margins?".
- **User docs** (`docs/USER-JOURNEY.md`, the upload step): a short paragraph covering what Auto does, how to
  adjust or reset one page, and that rehearsal notes on a re-cropped page show as "page changed".
- **No band material** in screenshots or examples: use the demo charts only.
- The wording is a product word (memory: a label is a product word). Bring the final copy to the gate, and
  keep the UI labels (Auto, Reset, Bake area…) identical between Studio and the docs.

## Out of scope

- Deskew / rotation of crooked scans (**[VLL]** — likely the next ask after this one; say so in the report).
- Non-rectangular areas, splitting one page into two, crop for generated charts.

## Acceptance

- Unit (Go): rect validation; autodetect vectors above; `dpi_eff` and the native-ppi cap; cache key includes
  the crop; overlay transform maps a known point to the known cropped pixel (and a mark outside → absent).
- Integration (Go): upload a synthetic scanned PDF (generated in the test), set a crop, bake → page raster
  width ≈ W and its pixels match a direct `pdftoppm -x -y -W -H` render; Reset → byte-identical to the
  pre-crop bake.
- Studio (vitest + one Playwright): drag handles respects AR; out-of-crop warning lists the right layers;
  preview overlay equals the bake overlay transform.
- Annotations §3: crop change ⇒ the page's rehearsal notes read "page changed", their T186 background still
  serves, and the Studio warning counts them.
- Export/import round-trip carries `crop`.
- Device pass (VLL): one real scan cropped, baked, checked on the tablet; before cropping a page that has a
  rehearsal note, the warning names it.

## Mobile follow-up — withdrawn

A89 is withdrawn. Rehearsal notes are rasters keyed by page hash, the tablet displays the baked page as it
is, and nothing on the tablet needs the crop rect. If a bundle consumer ever needs it, it is permille ints
(0..1000), like T149's `content_bottom_permille` — the mirror codegen has no float kind.
