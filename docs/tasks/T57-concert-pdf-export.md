# T57 — Print a baked concert to PDF (paper fallback)

**Priority:** normal (VLL 2026-07-18: "download a PDF of my bake — a physical print in
case of tablet malfunction") · **Size:** M · **Area:** core (compositing + PDF
assembly + endpoint) + a studio download button. Ruling: reviews.md 2026-07-18.

## Design (ruled)

- Input: an existing bundle's blobs (band bake; `scope=mine` while it exists — P205
  slots in later as "band bundle + identity filter"). No new render pipeline.
- `GET /api/bands/{b}/concerts/{id}/pdf[?scope=mine][&role=X]` — same gating as the
  bundle download. Composite per baked page: raster + the DEFAULT-visible overlays
  (mandatory + untagged; `role=X` adds that role's tagged layers; never session
  toggles — a printed backup must be reproducible). Bench/on-call songs print LAST,
  marked in the header.
- Pure Go: `image/draw` + `x/image/webp` (decode) → JPEG q~85 → DCTDecode embed via
  a minimal PDF writer (or pdfcpu — lane's pick; no CGo, no shell-outs).
- Page: A4 portrait, image fit with margins; header "«Song» — page n/m [· On call]";
  footer "«Concert» · p/P".
- Studio: "Download PDF" beside the bundle download (testid).

## Acceptance

- Unit: page count == baked page count; valid PDF (header/xref parse); deterministic
  modulo timestamps; compositing red-first (raster-only vs raster+overlay differ
  where an annotation exists).
- e2e: button downloads a PDF; content-type/size sane (< bundle × ~1.5).
- gofmt/vet/tests; no new system deps.

## Out of scope

- 2-up/cut marks (later ask); client-side generation; P205 identity filtering
  (arrives with P205 stage 2/3); printing from the app.
