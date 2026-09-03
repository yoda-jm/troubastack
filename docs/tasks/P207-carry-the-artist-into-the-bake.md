# P207 — carry the artist into the bake, and show it in the song drawer

**Lanes:** web-core (stage 1: proto + baker), then mobile (stage 2: the drawer). **Size:** S each.
**Status:** spec, not started. **Sequencing: after the concert on 2026-09-05** — it changes the bundle
format, which is not a two-days-before-a-gig change.
**Asked by:** VLL, 2026-09-03: *"je veux qu'on travaille sur mettre le author dans le bake, le drawer
… le rajoutera avec tiret long et le nom, le tout en gris, et le overflow y sera autorisé (l'auteur
n'est pas si important)"*. This is A60's P4, split out as promised.

## The finding: everything exists except one field

The artist is already carried everywhere **except** the bundle:

- the song record has it — `Artist` (`core/internal/app/app.go:209`);
- the setlist detail already exposes it — `SongArtist` (`service.go:1810`), populated from
  `song.Artist` (`:1858`);
- **the baker already holds it at the exact line where it sets the title**:
  `baker.go:502` reads `Title: item.SongTitle`, and `item` is that same setlist item.

So this is not plumbing a new value across the system. It is adding one field to the container and
one line to the baker.

## Stage 1 — core: one proto field, one mirror, one assignment

`proto/troubastack/v1/bundle.proto` is the source of truth for the container shape; the Go and Kotlin
types are hand-mirrored from it (I1/P203 — there is no codegen in the build).

1. **`BakedSong` gains `string artist = 13;`** — fields 1–12 are taken, 13 is the next free one.
   Follow the comment style already there: the file documents, for every field from 5 onward, exactly
   why the addition is safe (proto3 default-empty ⇒ old bundles stay valid, old loaders ignore it).
   Say the same for this one, and say the T26 thing too: like the title, the artist is a **snapshot at
   bake time**, so a later rename does not propagate into an existing bundle.
2. **Mirror it in the Go type** and set it in the baker beside the title:
   `Artist: item.SongArtist`, on the line under `Title: item.SongTitle`.
3. **Empty is normal, not an error.** Plenty of songs have no artist; absent must behave exactly as
   today, everywhere.

## Stage 2 — the app: the drawer line, and what may be sacrificed

The app is **TroubaStage** (BRAND02 renamed it from TroubaShare).

1. **Mirror the field** in `BundleModel.kt`'s `BakedSong` as `val artist: String = ""`. Every field
   there is already defaulted, so a bundle without it deserialises unchanged — that is the
   compatibility property, and it comes free only if the default is kept.
2. **The drawer row**, per VLL: after the title, an **em dash** and the name — `Title — Artist` — with
   **the artist in grey** (the drawer already has a muted role in
   `MaterialTheme.colorScheme.onSurfaceVariant`; reuse it rather than inventing a colour).
3. **Overflow is allowed, and that is a deliberate ranking.** *"L'auteur n'est pas si important"* — so
   when the row is too narrow, **the artist is what gets clipped, never the title, and the row must
   not grow a second line.** Concretely: one line, the title keeps its space, the artist takes what is
   left and is ellipsised. A layout where a long artist pushes the title out, or wraps the row and
   changes the list's rhythm, has inverted the priority.
4. **No dash when there is no artist.** A trailing "—" on a song with no artist is worse than no
   artist at all.

## Explicitly out of scope

- Showing the artist anywhere else — the Stage header, the page chrome, the Studio. VLL asked for the
  drawer; adding it elsewhere is a separate decision with its own screen-space argument.
- Editing the artist from the app. Stage is read-only (I12).
- Re-baking existing concerts to acquire the field. They simply keep showing no artist, which is
  exactly what the additive design is for.

## Done when

- A freshly baked bundle carries the artist, and a song without one carries an empty string —
  check the actual `bundle.json` inside a `.tstage`, not the Go struct.
- **The compatibility arm, which is free here:** `app/shared/src/commonTest/resources/fixtures/baked/`
  is a **frozen real-baker bundle from before this field** (see [A59](A59-give-the-baked-fixture-a-regeneration-path.md)).
  It must still load with zero issues, and the drawer must render those songs with **no dash and no
  grey suffix**. Do not regenerate that fixture for this task — its being old is precisely what makes
  it the test.
- On the device, a long artist on a narrow row **clips the artist and leaves the title intact, on one
  line**. Check it with a deliberately long name; the failure mode is invisible with short ones.
- `:shared:testDebugUnitTest` and `go test ./...` green, `gofmt -l core` clean. Match the counts.
- The proto comment explains the additive-compatibility argument, as every field from 5 onward does.
