# A10 — Stage night mode: don't blind the performer in a dark venue

**Priority:** A-track, unblocked · **Size:** S/M · **Area:** `app/shared` (stage)

## Context

Baked pages are white PDF rasters. On a dark stage a full-brightness white page is a
floodlight in the performer's face (and everyone behind them). Every serious sheet-music
app has an inverted/night rendering mode; Stage has none.

**Design decisions (resolved):**
1. Two modes in v1, toggled from the existing Stage chrome (one button, cycles):
   **Normal** and **Night** (inverted: page raster color-inverted so paper is near-black
   and ink is light). Implement via a `ColorFilter`/`ColorMatrix` on the DRAWING of the
   already-decoded bitmaps — never re-encode or mutate the cached bitmaps, and never
   touch the bake (the bundle stays untouched; this is a pure view transform, I12).
2. **Annotation overlays get the same transform as the page** so composites stay
   coherent (a red cue on an inverted page becomes its inverse — acceptable and
   standard; do NOT try to keep overlay colors "true" on an inverted page in v1).
3. The choice is a **local reading preference** (like role/fit): persisted via the
   Storage KV, restored on next Stage open. Reading behavior only — squarely inside
   A04's read-only contract.
4. Simple inversion first (matrix `-1` diag + offset). If plain inversion looks bad on
   the demo scans, a "soft" variant (invert + slight warm tint) may be tuned — state
   what shipped with a screenshot pair.

## Changes

1. `StageScreen`: apply the mode's `ColorFilter` at draw time to page + overlays; add
   the chrome toggle; persist/restore via the injected Storage KV (same pattern as the
   B03 policy book — app DI, no new seam).
2. commonTest: mode cycling + persistence round-trip (pure logic); the matrix constant
   documented.

## Acceptance criteria

- Screenshot pair on the real-baked demo: Normal vs Night (page near-black, notation
  light, overlays inverted consistently), same page geometry (fit modes unaffected).
- Toggle state survives leaving/reopening Stage; `:shared:check` + iOS klibs +
  `assembleDebug` green.
- No bitmap re-encoding (verify: the LRU cache holds the same decoded bitmaps in both
  modes — the filter is draw-time only).

## Out of scope

- Brightness control (OS owns it); per-layer color remapping; sepia/e-ink themes;
  Studio dark mode (T03 did the web).
