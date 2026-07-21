# T60 — Chord transposition for text charts

**Lane:** web-core · **Size:** M · **Status:** SPEC'd 2026-07-21 (VLL green-lit roadmap item 3 with concrete rulings) · **Depends on:** nothing open (T19/T25/T39 chart stack is landed)

## What VLL asked for (verbatim rulings, 2026-07-21)

1. *"transposing a source, with an option that it also transpose the key of the song (checkbox before applying it)"* — a persistent transpose of a chart source in the song editor.
2. *"when we override the key in a playlist we can have a checkbox next to it that also say transpose the chords (grey deactivated when it is not a source document)"* — a per-setlist-item transpose riding the existing `KeyOverride`.
3. *"at bake time we save the transposed one of course"* — surface 2 is **burned into the bundle pages at bake time**, band-wide. NOT a view-time/per-member transform (the roadmap's per-member-offset idea is explicitly superseded by this ruling).
4. *"we need a way to preview it also from the playlist"* — a transposed-chart preview reachable from the setlist item.

(The setlist→song hyperlink from the same message is split out as **T61** — unrelated nav, lands independently.)

## Current facts the design builds on

- `Song.Key` is free text (`core/internal/app/app.go` Song struct); `SetlistItem.KeyOverride` is free text, baked into `BakedSong.key` as display metadata only.
- T19 charts: chords are NOT stored structure — a chord row is detected positionally at render time (`isChordRow`, `chordToken` regex in `core/internal/chartpdf/chart.go`); the client mirrors the same regex in `web/studio/src/pages/song-editor/chartHighlight.ts` for highlighting.
- Chart source lives per fileID (`GetChartSource`/`SaveChartSource`, LWW via `baseRevision`); the rendered PDF is a `SongFile{Generated:true}` re-rendered in place (same fileId, revision bump) — downstream (viewer, annotations, bake) never sees a new file identity.
- **Renderer never wraps**: `fpdf Cell(0,…)` clips; pagination is manual and purely line-count-driven. ⇒ *a transpose that preserves line count preserves pagination and page geometry exactly*, which is what keeps existing annotations valid. This is the spec's central invariant (see engine rule 5).

## Part A — the transpose engine (Go, `core/internal/chartpdf`)

One implementation, used by all three surfaces (editor apply, playlist preview, bake). No TS transpose implementation — the client only ever displays server output (the chord regex duplication for *highlighting* stays as-is; do not add a second *transpose* to drift).

New exported API (shape indicative; keep it in `chartpdf` next to the dialect it manipulates):

```go
// ParseKey parses a free-text musical key ("G", "F#m", "Bbm"). Strict:
// ^[A-G](#|b)?m?$ after TrimSpace. Returns ok=false otherwise.
func ParseKey(s string) (Key, bool)

// Transpose rewrites every chord row in source by the pitch-class interval
// from→to, leaving all other lines byte-identical.
func Transpose(source string, from, to Key) (string, error)
```

Rules:

1. **Only chord rows change.** Line classification uses the exact same `isChordRow` logic as rendering — the transposer and the renderer must agree by construction (same functions, not a copy). Title/section/lyric/blank lines byte-identical.
2. **Token rewrite:** for each chord token, transpose the root and the optional `/bass` by `(to.root − from.root) mod 12`; the quality/extension string (`m`, `maj7`, `sus4`, …) is preserved verbatim. `N.C.` unchanged. Mode (major/minor) is NOT changed — we shift, we don't re-mode.
3. **Spelling:** prefer flats when the target key is a flat key (F, B♭, E♭, A♭, D♭, G♭ majors; Dm, Gm, Cm, Fm, B♭m, E♭m minors), sharps otherwise. One table, unit-tested.
4. **Column alignment ("chords over words"):** each chord token stays anchored at its original start column; padding between tokens is re-computed. When a token grows (C→C#) and would collide with the next token's column, push the following tokens right just enough to keep ≥1 space — never merge or reorder tokens.
5. **Line-count invariant:** `Transpose` returns the same number of lines it was given, always. Add a test asserting rendered PDF **page count and per-line y-positions are identical** before/after transposition of a multi-page fixture. This is what keeps existing layer annotations correctly anchored — if this invariant ever has to break, that is a gate question, not a local call.
6. Round-trip test: transpose +n then −n is pitch-class-identical (string equality NOT required — spelling may normalize, e.g. A#→Bb; compare parsed pitch classes).

## Part B — surface 1: transpose a source (song editor, persistent)

- In the chart-source editor (SongDetails, `Generated` files only): a **Transpose…** control opening a small inline form: target-key picker prefilled from `song.key` (falls back to a ± semitone stepper when `ParseKey(song.key)` fails — interval-only transpose is still well-defined), a checkbox **"Also update the song key"** (default ON when the song key parses), Preview, Apply.
- Server endpoint (single atomic op — no client-side compose of two PATCHes):
  `POST /api/bands/{b}/songs/{s}/files/{f}/chart-source:transpose`
  body `{ targetKey?, semitones?, updateSongKey bool, baseRevision int, dryRun bool }`
  - `dryRun:true` → returns the transposed source, persists nothing (the editor feeds it to the existing `text-charts:preview` machinery for the visual preview).
  - `dryRun:false` → transposes + `SaveChartSource` (same fileId, revision bump, existing 409 LWW on `baseRevision`) + when `updateSongKey`, sets `song.key = targetKey` in the same service call. 409 semantics identical to a manual source save.
  - Reject (400) when the file is not `Generated`, or when neither a parseable `targetKey` nor `semitones` is given.

## Part C — surface 2: playlist key-override transpose (bake-time)

- New field `SetlistItem.TransposeChords bool` (json `transposeChords`), settable via the existing item PATCH alongside `keyOverride`.
- **Studio (SetlistDetail):** a checkbox **"transpose chords"** next to the key-override input in the item edit form. Enabled iff ALL of:
  1. the song has ≥1 `Generated` chart file,
  2. `ParseKey(song.key)` ok (we need a *from*),
  3. `ParseKey(keyOverride)` ok.
  Greyed otherwise with a title/tooltip stating the failing reason ("no text chart on this song" / "song key not set or not parseable" / "override key not parseable"). Per VLL: *"grey deactivated when it is not a source document"*.
- **Bake:** when `TransposeChords` is set and the three conditions hold at bake time, the baker renders that song's `Generated` chart files from `Transpose(source, songKey→overrideKey)` instead of the stored PDF bytes; non-generated files (uploaded PDFs) bake as-is. The transposed rendering is **burned into the bundle pages** — band-wide, everyone sees it (VLL ruling 3; consistent with the P205 declutter-not-privacy stance — no per-member rails involved).
  - If a condition no longer holds at bake time (key edited to garbage, chart deleted), the bake does NOT fail: it bakes the song untransposed and surfaces a per-song warning in the bake result/dialog (same pattern as existing bake warnings). Silent wrong-key pages on stage are worse than a visible warning; a failed bake the night before a gig is worse than both.
  - `BakedSong.key` continues to carry the override string for display — no proto change needed for MVP. (If we later want a "transposed ✓" badge in Stage, that's a new optional field — not in this task.)
  - Annotations: valid by the Part A line-count invariant (identical pagination/geometry). State this in the baker code comment.
- **Preview from the playlist** (VLL ruling 4): a preview affordance on the item (enabled under the same three conditions) →
  `GET /api/bands/{b}/setlists/{sl}/items/{it}/chart-preview` → resolves the song's first `Generated` chart, applies the item's transpose (identity transpose if `transposeChords` is off — still useful as "show me the chart from here"), returns the rendered PDF inline. No persistence.

## Acceptance

1. Go unit tests: ParseKey table; token rewrite incl. slash chords + N.C.; flat/sharp spelling table; alignment padding (grown token pushes, ≥1 space); **line-count + rendered-geometry invariance on a multi-page fixture**; round-trip pitch-class identity.
2. httpapi tests: `:transpose` dryRun vs persist (+409 on stale baseRevision, 400 on non-generated file), item PATCH with `transposeChords`, item `chart-preview` happy + greyed-condition paths.
3. Bake test: setlist with transposed item → bundle pages differ from untransposed bake ONLY on chord-row pixels of that song's chart pages (page count identical); degraded-condition bake produces the warning, not a failure.
4. e2e (studio): editor transpose G→A with "also update song key" → source rewritten, key field shows A, preview matches; setlist checkbox greys with the right tooltip on a PDF-only song and enables on a chart song.
5. Red-first where it bites: the geometry-invariance test and the greyed-checkbox conditions demonstrated failing before implementation.

## Out of scope

- Per-member view-time transpose in Stage (superseded by ruling 3 — bake burns it).
- Transposing uploaded PDFs (impossible), chord recognition beyond the existing `chordToken` grammar, capo notation semantics ("Capo 2" text lines are literal text and stay literal).
- Proto/bundle changes (display badge etc.) — gate-ask first if wanted later.
