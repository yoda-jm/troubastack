# A62 — in scroll mode, swiping back should land at the start of the previous song

**Lane:** mobile. **Size:** XS. **Status:** spec, not started.
**Raised by:** VLL, 2026-09-03, **while playing**: *"in Stage, in scroll mode, swiping previous should
put you back on the top of the first page (in the 2 other mode it is correct to go to the last page of
the previous song)"*.

**This is the exception to the pre-gig app freeze**, and it qualifies for the reason the freeze names:
it came from VLL using the app, not from us looking for something to harden.

## The finding: forward and back are not symmetric

In `StageScreen.kt`, the scroll-mode song cross:

```kotlin
scrollSwipeNext → vm.goToPage(songRange.last + 1)   // = the NEXT song's FIRST page   ✔
scrollSwipePrev → vm.goToPage(songRange.first - 1)  // = the PREVIOUS song's LAST page ✘
```

Forward lands at a song's **start**; back lands at a song's **end**. Swipe back and you arrive at the
bottom of the previous song and have to scroll up through it — mid-performance.

## The fix, and the line not to cross while making it

`StageViewModel.goToSong(i)` already means *"jump to the first page of song i"*. So:

```kotlin
scrollSwipePrev → vm.goToSong(state.currentSong - 1)
```

keeping the existing `isBlockedSongCross` guard and its N7 flash at the first song.

**⚠ Do NOT make the same change in `turnPrev`.** Its scroll branch also ends with
`vm.goToPage(songRange.first - 1)` (*"column top → previous song's last page"*) and **that one is
correct**. It is the *fine-grained* path — pedal, keys, volume — where you are stepping backwards page
by page; continuing onto the previous song's **last** page is the continuation of that traversal.
Landing on its first page would skip the whole song backwards on a pedal press.

This is the same boundary VLL set earlier today and it decides the question cleanly:

| control | unit | where "back" lands |
|---|---|---|
| swipe and the on-screen **‹** | a **song** | the previous song's **first** page — *this task* |
| pedal, keys, volume | a **page** | the previous song's **last** page — *unchanged* |

The FAB inherits the fix for free: `StageFab("‹")` already routes to `scrollSwipePrev` in scroll mode.

## The half that is not a page index

VLL asked for the **top of the first page**, and that is two things: which page is current, and where
the column is scrolled. Setting `current` fixes the first. The second depends on whether the
`LazyColumn` resets when the song changes — if it keeps its offset, you land on the right page at the
wrong scroll position, which looks like the bug is only half fixed.

**Verify the scroll offset on the device, not the page index in a test.** A unit test can prove
`goToSong` picked the right page; only the device shows whether the column is at its top.

## Done when

- In **scroll** mode, swiping back — and pressing **‹** — lands at the **top of the previous song's
  first page**. Checked on the device with a multi-page song, since a one-page song hides the bug.
- In **page** and **width** modes nothing changed at all.
- On a **pedal/key** press at the column top, you still land on the previous song's **last** page.
  This is the regression the fix could easily cause; test it explicitly.
- At the first song, back is still a blocked cross with the N7 flash, not a silent no-op.
- `:shared:testDebugUnitTest` green; match the count.
