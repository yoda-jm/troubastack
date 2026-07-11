# T37 — New text chart from pasted lyrics (paste-first import)

**Priority:** normal (VLL wish, 2026-07-12: "import lyrics from an azlyrics
link") · **Size:** S · **Area:** `web/studio` (chart creation flow) + a pure
normalizer + tests. **Depends on T36** (the Files section must be reachable).

## The decision (RULED): paste-first, NO scraper

The outcome VLL wants is "a song's lyrics in as a text chart, fast." An
azlyrics-specific fetcher is the wrong build: the site is Cloudflare-gated and
its ToS prohibits automated access (a server-side fetch will flake/403 and
breaks on DOM changes), and it's third-party copyrighted text — fetching it
server-side puts troubacore in the copying business, pasting keeps the human in
the loop. **Paste covers every source** (azlyrics via the user's own browser
copy, Word, notes apps) with zero external dependencies. A generic
readability-style URL fetch stays OUT unless VLL explicitly wants it later
(recorded option, his copyright call).

## Changes

1. **"＋ New chart from lyrics"** beside "＋ New text chart" in the Details
   panel's Files section (T36's home): opens a dialog with a name field
   (prefilled from the song title) + a big paste textarea → creates a text
   chart pre-filled with `normalizeLyrics(pasted)` and opens it in the existing
   T19 chart editor for cleanup. No new file type, no new API — it's the
   existing create-chart call with initial content.
2. **`normalizeLyrics` (pure, unit-tested), deliberately minimal:** normalize
   CRLF→LF; trim outer whitespace; collapse 3+ consecutive blank lines to one
   blank line (section break); strip trailing all-site-cruft lines ONLY when
   they match a tiny conservative blacklist (e.g. lines that are exactly
   "Submit Corrections" / "Writer(s):…" / "Thanks to … for these lyrics") —
   when in doubt, KEEP the line (the chart editor is right there for cleanup).
   Do NOT touch section labels, chords, brackets, or capitalization.
3. **Tests:** unit table for the normalizer (CRLF, blank-line collapse, cruft
   blacklist, keep-ambiguous); one e2e: open dialog → paste multi-verse text →
   chart created, editor shows the normalized content, bake-able like any T19
   chart (the existing chart pipeline needs no change — assert creation only).

## Acceptance criteria

- e2e red-first (the button doesn't exist pre-fix); unit table green; full
  suite green; `tsc -b studio` clean; pixels of the dialog at the gate.

## Out of scope

- URL fetching / scraping of any site (recorded as a possible later option —
  VLL's call); chord detection or ChordPro parsing; auto-formatting beyond the
  minimal normalizer; app-side changes (charts already flow through the bake).
