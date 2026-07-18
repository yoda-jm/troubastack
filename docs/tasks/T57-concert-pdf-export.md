# T57 — Print a baked concert to PDF (paper fallback)

**Priority:** normal (VLL 2026-07-18: "download a PDF of my bake — a physical print in
case of tablet malfunction") · **Size:** M · **Area:** core (compositing + PDF
assembly + endpoint) + a studio download button. Ruling: reviews.md 2026-07-18.

## Design (ruled)

- **RESEQUENCED 2026-07-18 (ordering ruling): builds AFTER P205 Stage 2, on the
  band-wide bundle ONLY** — `scope=mine` is not part of T57. "My print" = band-wide
  bundle + identity filter (the authenticated caller; `?member=` admin-only for
  printing on someone's behalf). No new render pipeline.
- `GET /api/bands/{b}/concerts/{id}/pdf[&role=X][&member=Y admin-only]` — same
  gating as the bundle download. Composite per baked page via the **P205
  view-resolution rule** (mandatory > identity's personal layers > default_on ∧
  role_tag; never session toggles). **The rule's cases live in a shared test-vector
  JSON run by BOTH the Go test and commonTest** so print == screen by construction
  (the glyphs.json pattern applied to semantics). Bench/on-call songs print LAST,
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
