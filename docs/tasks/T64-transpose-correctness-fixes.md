# T64 — Chord-transposition correctness fixes (T60 deep-audit follow-ups)

**Lane:** web-core · **Size:** M · **Status:** SPEC'd 2026-07-26 (from the T60 deep audit, reviews.md 2026-07-26) · **Depends on:** T60 (landed)

T60 shipped and its load-bearing line-count/pagination invariant is sound, but a
deep audit found correctness defects that produce **silently wrong output**. None is a
security issue; the worst (D1) is a wrong gig bundle with no warning. Fix as a batch.

## Fixes (severity order)

### D1 — HIGH: eligibility must be resolved against the file the baker actually bakes
Today the baker checks `defaultFile.Generated` (first `application/pdf` by DisplayOrder,
`baker.go:304/408`) while the bake-warning, the Studio checkbox, and the playlist preview
all check "ANY generated file exists" (`bakeapi.go:139`, `service.go:1711`, `service.go:1300`).
A song with an uploaded PDF at DisplayOrder 0 + a generated chart at 1, transpose on →
baker bakes the untransposed uploaded PDF, NO warning is emitted, and the preview shows the
transposed chart. Fix: define ONE resolver "the generated chart this song will bake/preview"
and use it in all four places — the baker, the warning, the preview, and the checkbox
enablement must agree on the same file. If a song has a generated chart but it isn't the
default-baked file, either bake the generated chart when transposing, or (if product wants
the default PDF to win) emit the warning and disable the checkbox. Decide + make all four
consistent. Test: the two-file ordering above bakes the transposed chart OR warns — never
silently bakes the wrong document.

### D2 — HIGH: unify the whitespace tokenizer with isChordRow
`isChordRow` uses `strings.Fields` (all Unicode whitespace incl. NBSP U+00A0, `\v`, `\f`);
`transposeChordRow` splits only on `' '`/`'\t'` (`transpose.go:139,144`). NBSP-separated
chord rows (common from web copy-paste, and valid per `validateChars`) transpose only the
first token; a leading NBSP hard-errors in `shiftRoot` (`transpose.go:206` reads byte `s[0]`
of a multi-byte rune). Fix: tokenize chord rows with the SAME rule `isChordRow` uses
(`strings.Fields` / `unicode.IsSpace`), preserving column positions for the anchor logic.
Test: NBSP/`\v`/`\f`-separated rows transpose EVERY chord; a leading-NBSP row transposes
cleanly (no error).

### D3 — MED: warn on runtime transpose/render failure at bake, not just eligibility
`transposeWarnings` (`bakeapi.go:124`) only re-runs `TransposeEligible`. If eligibility
passes but `Transpose`/`Render` errors inside the bake block (`baker.go:305-313`), the song
bakes untransposed with ZERO warning. Fix: the bake path should record per-song "asked to
transpose, eligible, but the transform failed → baked untransposed" and surface it in
`warnings`. (Keep the never-fail-the-bake contract; just make the fallthrough visible.)

### D4 — MED: disable the chart textarea during an in-flight transpose Apply
`SongDetails.tsx`: `dirty` gates the toggle + Apply at click time, but the textarea
(`HighlightedSource`) has no `disabled={transposing}`, so typing during the Apply
round-trip is clobbered by `setSource`/`setSavedSource` and `dirty` resets false. Fix:
disable the source textarea (and mark it busy) while `transposing`.

### D5 — LOW: preserve the user's key spelling / stop the F#→Gb contradiction
Targeting F# major spells the chart Gb/Db/Abm while `TransposeChartSource` stores the raw
"F#" as the song key — chart and key field disagree, and a nominal no-op transpose isn't a
no-op. Decide the spelling policy for enharmonic target keys (respect the user's typed
accidental for the tonic at minimum) and make the stored key consistent with the printed
chords. Keep the existing flat/sharp table for non-tonic notes unless you extend it.

### D6 — LOW: semitone path shouldn't force flats→sharps (and 0 should be a no-op)
`TransposeSemitones` always passes `flat=false` (`transpose.go:100`), so an Apply of a
flat-key chart respells to sharps and `+1 then −1` doesn't round-trip; semitones=0 (allowed
by the UI) rewrites spelling. Fix: preserve the source's existing accidental style on the
semitone path (or at least make 0 a true no-op), and consider blocking/ignoring a 0 Apply.

### D7 — LOW: keep tab-separated chord alignment stable
Tabs in a chord row collapse to single spaces (`transpose.go:139-146`) while the lyric line
keeps its tabs, so chord-over-lyric x-alignment drifts. Fix: preserve tab runs (or normalize
both rows consistently) so a grown/shrunk token doesn't shift chords off their syllables.
(y/pagination is already invariant — this is x only.)

## Also fold in (test-coverage gaps from the same audit)
- Negative/authz regression tests for the two T60 endpoints (`:transpose`, item
  `chart-preview`): non-member → 403/404, cross-band `fileId`/`itemId` → 404. Enforcement
  already exists; only the guard tests are missing.

## Out of scope
- The T62/import security work (that's T63).
- Any change to the line-count/pagination invariant (it's correct; keep it).

## Acceptance
1. Go unit/httpapi tests for D1 (two-file ordering), D2 (NBSP/`\v` rows + leading NBSP),
   D3 (forced transform failure → warning present), D5/D6 spelling/round-trip, D7 alignment.
2. e2e for D4 (type during Apply → edits not lost) and the D1 UI/preview/bake agreement.
3. The added negative endpoint tests pass; `go test ./...` + gofmt + vet green; tsc +
   studio build clean; no dist churn.
4. Red-first on D1 and D2 (demonstrate the wrong output before the fix).

## Notes
Present at the gate; cite reviews.md 2026-07-26 (audit) as the source. Batch is fine, but
D1 and D2 are the ones that produce wrong musical output — prioritize them.
