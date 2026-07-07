# T26 — Carry song titles in the baked bundle (kill the "Song N" fallback)

**Priority:** medium (UX, surfaced by A15) · **Size:** S ·
**Area:** `proto`, `core` (bake + mkbundle), `app/shared` (loader + drawer/labels)

## Context

The `.tstage` bundle's `BakedSong` carries `song_id`, revisions, pages, and the B02
setlist metadata (`display_notes`/`key`/`tempo`) — but **no song title**
(`proto/troubastack/v1/bundle.proto:28`, fields 1–7). The Stage therefore names
songs with a client-side fallback — `"Song ${songIdx + 1}"`
(`app/shared/…/stage/StageModel.kt:84`) — which A15's song drawer made prominently
visible: the 2026-07-07 evidence screenshot lists "Song 1…Song 4" for a concert
whose songs are Wonderwall / Hallelujah / Black Hole Sun / The Open Road. The
titles exist in the rasters, but a performer choosing an encore in the drawer
shouldn't have to remember set positions. (Verified against the shipped
`docs/demo/demo-concert.tstage`: song objects have keys `songId, songRev,
sourceRevision, pages, displayNotes, key` only.)

**Design decisions (resolved):**
1. **Additive proto field:** `string title = 9;` on `BakedSong` (field 8 is
   T23's `on_call` — ruled 2026-07-07) — proto3
   default-empty, so old bundles stay valid and old loaders ignore it (the same
   compatibility argument as B02's fields 5–7, documented right above it).
2. **Baker writes the song's current Title** at bake time (a bundle is a snapshot;
   no rename-propagation machinery). The T18 unified writer means ONE Go site.
3. **Client fallback stays:** empty/absent title → the existing "Song N" — never a
   blank drawer row.
4. **Sequencing:** T23 (encore/bench) also adds a bundle-proto field; land the two
   proto changes in whichever order the lanes reach them, but rebase early — the
   file is a collision hotspot the moment both are in flight.

## Changes

1. `proto/troubastack/v1/bundle.proto`: `string title = 9;` on `BakedSong` with a
   comment mirroring the fields-5–7 compatibility note; `buf lint` clean.
2. Core: the bake writer (`internal/bake`, T18-unified with mkbundle) populates it
   from the song record; `make fixtures` regenerated if the fixture writer gains
   the field (intended diff only).
3. App (A-track): `BakedSong` Kotlin mirror + loader map `title` (tolerant of
   absence); `StageModel` uses it with the "Song N" fallback; the drawer, and any
   label that renders a song name, shows the real title.
4. Tests: Go — bake output carries titles (extend the existing bundle-shape test);
   Kotlin — loader maps title, fallback on empty (extend the loader test).

## Acceptance criteria

- Fresh bake of the demo setlist → the A15 drawer lists real titles ("Wonderwall",
  …), verified by an emulator screenshot.
- The SHIPPED old `docs/demo/demo-concert.tstage` still loads with zero issues and
  shows "Song N" (fallback intact) — no demo regen required by this task (it may
  ride the next T24-style regen).
- `make test`, `buf lint`, `:shared:check` + iOS klibs green; `make fixtures`
  zero-diff except the intended title field.

## Out of scope

- Renaming propagation into existing bundles; artist/subtitle fields; any drawer
  redesign (A15 shipped); the T23 `on_call` field (its own task — coordinate the
  proto file merge only).
