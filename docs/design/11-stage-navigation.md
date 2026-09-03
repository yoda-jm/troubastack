# Stage navigation — reading modes, and what every input does

**Status:** the contract, as decided. Written 2026-09-03 because it was not written anywhere: the
rules lived in code comments (`N1`–`N8`, `A09`, `A12`, `A13`, `A14`) and in task specs, so the only
way to answer "what should the pedal do?" was to read `StageScreen.kt`. VLL asked for it after being
surprised by his own app mid-rehearsal.

Scope: navigating a baked concert in **Stage** (the performance view). Editing is out of scope —
Stage is read-only by I12.

## The two things Stage navigates

A concert bundle is **one flat list of pages** spanning every song, in setlist order. A song is a
**range** within that list, not a separate container. Two consequences that explain most of the
behaviour below:

- advancing "one page" at the end of a song lands on the **next song's first page** — crossing is
  continuous, never blocked by a song boundary (a performer with a pedal cannot stop at every song
  end);
- **crossing must still read as crossing**, so it fires a transient boundary cue (`N1`).

Jumping directly to a song is a **separate** action, not a fast page turn: the song drawer.

## Reading modes

Cycled with the ⚙ sheet or by cycling (`A14`). They change the unit under your finger:

| mode | what a page is | vertical axis |
|---|---|---|
| **Page** | one whole page, fitted | nothing to scroll |
| **Width** | page fitted to width, taller than the screen | scrolls within the page |
| **Scroll** | a continuous column of the song's pages | **the column itself** |

In **two-up** (landscape, `A12`) a turn moves by a whole **spread**, not a page.

## What each input does — the rule

**An input should do what you cannot already do another way.** Decided by VLL, 2026-09-03:

> *"si j'utilise le tactile c'est que c'est possible, donc cohérence simplicité (tout marche pareil),
> c'est pas malin de click pour juste scroller. En revanche si j'utilise une pedale ca veut sans doute
> dire que j'ai les mains prises, donc il faut que je puisse avancer dans mon morceau courant"*

Read it as a redundancy argument, not a caution argument:

- **If you are touching the screen, scrolling is already free** — you drag the column with your
  finger. A button that only scrolls duplicates the gesture you just chose not to use, so it should
  do the thing a finger cannot do as conveniently: move to the next song. And since the swipe and the
  buttons are then the same command, **everything on the touch surface behaves the same** —
  consistency and simplicity, one rule to learn.
- **If you are on a pedal, your hands are busy** and dragging is not available. Advancing *within*
  the current song has no other input. So that is exactly what the pedal must provide, crossing to
  the next song only when the column runs out.

Each surface is given the capability the other one leaves missing. That is why the split falls where
it does, and why the on-screen buttons scrolling in Scroll mode is not merely inconsistent — it is
**pointless**, spending a tap on something the finger beside it already does better.

| input | Page / Width | Scroll |
|---|---|---|
| horizontal swipe | turn a page (or spread) | **cross to the next/previous song** (`N8`) |
| on-screen **‹ ›** buttons | turn a page (or spread) | **cross to the next/previous song** |
| **pedal** (Bluetooth) and volume keys | turn a page (or spread) | advance **within the column**, crossing only at its end |
| song drawer | jump to any song | jump to any song |

Why the swipe is coarse in Scroll and not elsewhere (`N8`): in Scroll the **vertical** axis belongs to
the column, so the horizontal axis is the only free one — and "a horizontal swipe advances the unit"
should hold in every mode. In Page and Width the horizontal axis *is* the page turn, so there is
nothing to reassign.

> ⚠ **Not yet true in the code:** the **‹ ›** buttons still share the hardware path in Scroll mode and
> advance within the column. Closing that is [A60](../tasks/A60-the-song-drawer-is-unusable-on-a-real-setlist.md)
> P5. Every other row above is current behaviour.

### The trap: a Bluetooth pedal is a keyboard

`StageKeys.kt` maps `PageUp`/`PageDown`, the arrow keys, `Space` and the volume keys to a page turn,
because **Bluetooth page-turner pedals present themselves as keyboards** sending exactly those keys.
Volume keys are the phone stand-in for people without a pedal.

So the touch/hardware split **cannot** be implemented as "keyboard events behave like the swipe" —
that would route a real pedal to the coarse behaviour and make it skip whole songs. The split is by
**surface**: on-screen controls versus key events, whatever sends them.

**The accepted cost:** someone using a real keyboard gets the pedal behaviour, because a key event
carries no evidence of what produced it. That is the right way round — mistaking a keyboard for a
pedal costs one fine-grained turn; mistaking a pedal for a keyboard costs a skipped song on stage.

## Edges and feedback

- **Blocked turn** (`N7`): in Page/Width, a turn at the very first or last page is a true no-op, so an
  end/start glyph flashes — otherwise a dead swipe reads as a dropped input. In Scroll the native
  rubber-band already says "end of column", so the glyph is used only for a blocked **song** cross.
- **Boundary cue** (`N1`): crossing into another song flashes a transient marker.
- **Any tap toggles the chrome** in every mode; it never navigates. Navigation is swipe, buttons,
  keys, or the drawer — a bare tap on a page must never move the music (`N3`).

## The song drawer

The only way to jump to an arbitrary song. It lists the running order with a meta line
(`notes · key · ♩=tempo`) and the member's cue glyphs, highlights the current song, and groups
**On call** (bench/encore, `T23`) below the running order — those keep their original indices so a
jump still lands correctly.

The drawer is `A15`. It is opened from the **Songs** button and closes on the scrim or back; swipe
gestures are live only while it is already open, so a left-edge swipe can never open it
mid-performance and fight the page-turn gesture (`A04`/`A12`).

## Why this document exists

Navigation is the part of the product a musician touches while playing, and it is the part with the
most invisible state: three reading modes, two page layouts, four input surfaces. Before this file the
rules were correct but scattered, and the only reliable answer to "is this expected?" was to read the
composable. When VLL asked exactly that question about his own pedal, the honest answer took a code
review — that is the gap this file closes.
