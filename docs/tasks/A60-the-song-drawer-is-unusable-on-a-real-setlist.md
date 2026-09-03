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

## P5 — in scroll mode the arrow and the swipe disagree, and nobody wrote that down

**I first told VLL this was expected. That was wrong, and he was right to push back.** My answer
cited `StageViewModel.next()` — which **production never calls**; it appears only in
`StageViewModelTest`. The arrows, keys and pedal call `turnNext` in `StageScreen`, which branches on
the reading mode. Checking the wiring instead of the first plausible function gives a different
answer:

| input | page / width | **scroll** |
|---|---|---|
| horizontal swipe | turn a page (`turnNext`) | **cross to the next song** (`scrollSwipeNext`, N8) |
| ‹ › FABs, keys, pedal / volume | turn a page (`turnNext`) | **scroll one page inside the current song** |

So in **scroll mode only**, two inputs that a performer treats as the same command do different
things: a swipe advances a whole song, an arrow nudges the column. Page and width are consistent —
everything routes through `turnNext`'s else-branch. VLL narrowed it to scroll from use alone, which
is exactly where the code diverges.

**Is it intentional?** Partly. N8's comment argues the swipe case explicitly — in scroll the vertical
axis belongs to the column, so the horizontal one must mean songs, and it ends *"'Horizontal swipe
advances the unit' now holds in every mode."* It says **nothing about the arrows or the pedal**,
which keep page granularity in that mode. So one half was designed and written down; the other half
was left as whatever fell out.

### DECIDED by VLL, 2026-09-03 — and the split is touch vs hardware, not arrows vs volume

VLL: *"pour moi la fleche doit etre pareil qu'un swipe, par contre la pedale ca veut dire qu'on a pas
acces a l'ecran donc ca doit avancer dans la page et faire next song tout a la fin"*.

The intent, in his own follow-up: **an input should do what you cannot already do another way.** If
you are touching the screen you can already scroll by dragging, so a button that only scrolls is
redundant — *"c'est pas malin de click pour juste scroller"* — and every touch control should behave
the same. On a pedal your hands are busy, dragging is unavailable, and advancing **within** the
current song has no other input, so that is what the pedal must give you.

(An earlier draft of this section justified it as "you cannot correct a wrong jump". That reasoning
was mine, not VLL's, and it is weaker: the real argument is redundancy, and it also explains why the
buttons scrolling in Scroll mode is *pointless* rather than merely inconsistent.)

**⚠ Do not implement that as "keys behave like the swipe".** `StageKeys.kt` says it outright:
*"Bluetooth pedals present as keyboards sending PageUp/Down or arrows; Space is common; volume keys
are the phone stand-in."* **A BT pedal sends arrow keys.** Routing key events to the swipe behaviour
would make a real pedal skip whole songs — the precise outcome VLL is ruling out. The line he is
drawing is **touch vs hardware**, and it lands like this:

| input | scroll mode, after this change |
|---|---|
| horizontal swipe | cross to the next song *(unchanged)* |
| **on-screen ‹ › FABs** | **cross to the next song** — the change |
| BT pedal (arrows / PageUp / Space) and volume keys | advance within the column, cross at the end *(unchanged)* |

So the work is narrow: the FABs at `StageScreen.kt:585-586` stop sharing `turnNext` with the hardware
path and use the scroll-mode song-cross (`scrollSwipeNext` / `scrollSwipePrev`) when `scrollMode` is
on. Keys and the volume registrar keep `turnNext` exactly as they are. Page and width modes are
untouched — everything there already agrees.

**The accepted cost, stated so nobody re-opens it:** someone using a real keyboard with a tablet gets
the pedal behaviour, because a key event cannot be told apart from a pedal's. That is the right way
round — mistaking a keyboard for a pedal costs a fine-grained turn; mistaking a pedal for a keyboard
costs a skipped song on stage.

**And update the N8 comment.** It currently claims *"'Horizontal swipe advances the unit' now holds in
every mode"* and says nothing about the other inputs. After this it should state the rule VLL actually
gave, with the reason that makes it stick: **an input should do what you cannot already do another
way** — the screen already scrolls by dragging, the pedal has no other way into the current song.
See [`docs/design/11-stage-navigation.md`](../design/11-stage-navigation.md), which is now the
home for this contract.

**Minor, found on the way:** `StageViewModel.next()` and `.previous()` are **dead in production** —
only `StageViewModelTest` calls them. Either delete them or say in a comment that they exist for the
model's own tests, because they are a trap: they read like the app's navigation and they are not.

## Done when

- On the device, with a setlist longer than the screen, **every** song in the drawer can be reached
  and tapped. Check with the real running order, not a three-song fixture.
- The running order is numbered from 1, and the "On call" group's treatment is a stated decision.
- The two headers are styled the same way.
- `:shared:testDebugUnitTest` still reports **303** plus whatever you add. Match the count.
- P4 is filed separately, not half-done here.
- **P5:** the ‹ › FABs cross songs in scroll mode; keys and volume do **not**. Verify with a real
  hardware turn (or volume keys) that the pedal path still advances within the column — if a
  pedal press skips a song, the change went in on the wrong side of the line.
