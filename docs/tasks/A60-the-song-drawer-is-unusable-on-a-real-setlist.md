# A60 — the song drawer does not scroll, and three smaller things from real use

**Lane:** mobile. **Status:** spec, not started.
**Raised by:** VLL on 2026-09-03, **while rehearsing on the tablet**, two days before the concert.
Everything below was confirmed in the code before being written down.

## P1 — the drawer cannot scroll, so most of the setlist is unreachable ⚠ do this first

`SongDrawerSheet` (`StageScreen.kt`) puts its items straight into `ModalDrawerSheet`'s content slot:

```kotlin
ModalDrawerSheet {
    Text("Songs", …)
    main.forEach { (i, s) -> SongDrawerItem(state, i, s, onJump) }
    …
}
```

That slot is a plain `ColumnScope` — **no `verticalScroll`, no `LazyColumn`**. Every item is laid out
whether or not it fits, and anything past the bottom of the screen simply cannot be reached.

**This is not cosmetic and it is not theoretical.** The concert being rehearsed has **22 songs**. A
drawer showing perhaps eight of them, with no way to reach the rest, means the app's only
song-jump affordance covers the first third of the gig. On stage that is the difference between
finding the next number and not.

**Fix:** make the content scrollable — `Modifier.verticalScroll(rememberScrollState())` on a wrapping
column, or convert to `LazyColumn`. Prefer `LazyColumn` if the item count can grow; a setlist is
small, so either is defensible — say which and why.

**Verify on the device with a setlist longer than the screen**, not in a preview. The bug only exists
past the fold, so a short fixture cannot see it. This is also a regression test worth having: assert
the drawer's content is scrollable, or that item N of a long list is reachable.

## P2 — number the songs

The running order has no numbers. On stage a band calls "number seven", not "the one after the
ballad". `SongDrawerItem` already builds a `Column` for the label, and the index `i` is already in
hand — the number is available at the call site.

**Two details worth deciding rather than stumbling into:** number the **main running order** from 1,
and decide explicitly what the **"On call"** group shows — continuing the numbering implies those
songs are in the set, which is the opposite of what "on call" means. Suggest: no numbers on the bench,
or a separate marker. Say which, and why, in the code.

## P3 — "Songs" does not read as a header, and the drawer contradicts itself

Confirmed in the same function: **"On call" is preceded by a `HorizontalDivider`; "Songs" is not.**
Two headers in one drawer, one of which is visually separated and one of which floats above the list
looking like an item. VLL read the second as a header and the first as noise, which is exactly what
the markup says.

**Fix:** treat both the same — divider, or background, or a shared style — and make it one decision
applied twice rather than two ad-hoc choices.

## P4 — the artist is not in the app, and cannot be shown without a bake change

VLL asked for the artist after the title, or small underneath. **The data is not there.** `artist`
appears nowhere in `app/shared/src/commonMain`, and nowhere in `core/internal/bake` — the setlist API
carries `songArtist`, but the **`.tstage` bundle does not**. The drawer's meta line is
`notes · key · ♩=tempo` from the song's first page; there is no artist to add to it.

So this is **not a UI change**: it needs the bundle schema and the baker to carry the artist, then the
app to display it — a core change and an app change, in that order, with the usual bundle-compat
question for already-baked concerts (an older bundle has no artist; the field must be optional and the
UI must render fine without it).

**Recommendation: split P4 into its own task and do it after the concert.** P1–P3 are self-contained
in one file and can land today; P4 crosses lanes and touches the bundle format, which is not what to
change two days before a gig. Filing them together here only because they came from one session of
real use.

## Not a bug: the arrow advances a page, not a song

VLL asked whether it is expected that the arrow "does not make next song but scrolls a little
further" on a landscape tablet. **It is expected**, and the answer is in
`StageViewModel.next()`:

```kotlin
fun next() = goToPage(_state.value.current + 1)
```

Pages are **one flat list across the whole concert**, and a song is just a range within it. So the
arrow always moves exactly one page; it crosses into the next song only when the current song's last
page is passed. A three-page song takes three presses. In Width and Scroll reading modes the page is
taller than the viewport, so one page-advance looks like "scrolled a little further" rather than a
discrete turn — the `StageScreen` comment states the intent: *"page/width turns pages, scroll crosses
songs (N8)"*.

**Recorded here so it is not re-raised as a bug.** But note the interaction, which is the real
insight: the arrow felt wrong *because* the drawer — the affordance that does jump songs — is
broken by P1. Fix P1 and the arrow stops being the only thing to reach for.

## Done when

- On the device, with a setlist longer than the screen, **every** song in the drawer can be reached
  and tapped. Check with the real running order, not a three-song fixture.
- The running order is numbered from 1, and the "On call" group's treatment is a stated decision.
- The two headers are styled the same way.
- `:shared:testDebugUnitTest` still reports **303** plus whatever you add. Match the count.
- P4 is filed separately, not half-done here.
