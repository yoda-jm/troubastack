# A08 — Stage shows the setlist metadata (notes / key / tempo)

**Priority:** A-track, unblocked, high value/effort ratio · **Size:** XS/S · **Area:** `app/shared` (stage)

## Context

B02's design decision 1 put setlist overrides into the bundle manifest as METADATA
(`BakedSong.displayNotes/key/tempo` — proto fields 5–7, Kotlin mirror already parses
them) precisely so "the presenter may display them in a later task". This is that task.
Today Stage renders pages and ignores the metadata entirely — but "Acoustic intro,
capo 2. · Em · ♩=98" is exactly what a performer wants at the top of a song.

**Design decisions (resolved):**
1. Display on the **first page of each song only**, as one thin single-line strip
   (notes · key · tempo, omitting empty fields entirely — most songs will have none, in
   which case NO strip renders and the layout is identical to today).
2. The strip must be **footprint-stable**: reserve nothing when there's no metadata;
   when present, it's a fixed-height overlay bar (top edge, semi-transparent scrim over
   the page margin), never an in-flow element that resizes the page — Stage's fit
   modes and page geometry stay untouched (I12: pure compositor + pager).
3. Read-only, offline (values come from the loaded bundle — no network, I12).

## Changes

1. `StageScreen`/`StageViewModel`: expose the current song's `displayNotes/key/tempo`
   (already on the loaded `BakedSong`); render the strip per decisions 1–2. Tempo
   renders as `♩=N` (0 = omitted), key as-is, notes truncated with ellipsis at one line.
2. commonTest: strip-content formatting (all-empty → null, partial combos, truncation) —
   plain function, easily unit-tested.

## Acceptance criteria

- Loading the real-baked demo (`Sat @ The Anchor`) shows "Acoustic intro, capo 2. · Em"
  on song 1 page 1 and "♩=98" on song 3 page 1; songs/pages without metadata render
  pixel-identical to today (screenshot pair).
- `:shared:check` green (incl. the new formatting tests); iOS klib compiles green;
  Android unaffected otherwise.

## Out of scope

- Editing metadata in the app (Studio owns setlists); a metronome (see the idea list —
  a tempo *display* is not a click track); per-page metadata.
