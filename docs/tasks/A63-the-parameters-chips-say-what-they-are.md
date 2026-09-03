# A63 — the Parameters chips should say what they are, and look like what they do

**Lane:** mobile. **Size:** S. **Status:** spec, not started. **After the gig** — see the sequencing
note at the end.
**Raised by:** VLL, 2026-09-03: *"the 3 chips Role, Night and Layers — it is not easy to see that Night
can be switched … maybe labels for Night and Role could be nice? … same for Role, I don't know exactly
what it does."*

## The finding: three different controls, drawn identically, none labelled

The ⚙ sheet renders three `OutlinedButton`s side by side. They are not three of a kind:

| chip | what it actually is | what it looks like |
|---|---|---|
| `Role` / `Role: <x>` | opens a **picker** (`showRole = true`) | a button whose text is its own label |
| `Night` | the current value of a **four-state cycle** (Normal · Warm · Night · Amber) | a noun |
| `Layers…` | opens a **panel** | a button that opens something — the `…` says so |

**`Layers…` is the only one carrying an affordance marker, and it is the only one VLL did not
complain about.** That is about as clean a confirmation of the diagnosis as one gets.

The colour chip is the worst of the three, and worse than "a toggle you can't see": it shows one value
out of **four**, gives no hint that the other three exist, and offers no way back. And `Role` uses the
same word as both label and value, so when it is unset it reads as a noun, not a setting.

**The sheet already contains the answer, one row above.** "Reading mode" is a **labelled** control
whose `SingleChoiceSegmentedButtonRow` shows all three options at once. And "Auto-update" is labelled
*and* explained ("Apply new bakes as they arrive").

**VLL has already ruled on this once**, and the code records it — the auto-update indicator *"read as
a mystery dot next to the metronome (VLL). It lives only in the ⚙ sheet, clearly labeled."* Same
complaint, same person, same remedy. The three chips are simply where it was not applied.

## Work

1. **Label each control**, in the sheet's own existing style (`titleSmall`, as "Reading mode" and
   "Auto-update" use). The button then carries the **value**, not the label: `Guitar` under a "Role"
   label, not `Role: Guitar`.

2. **Colour: use the same segmented row as Reading mode.** VLL first suggested arrows either side,
   then found the better reason himself: *"I missed the Night because I misinterpreted them for
   something similar to the reading mode — maybe the color scheme could be the same as the reading
   mode?"*

   That is the sharper diagnosis, and it decides the widget. The problem is not only that the chip
   lacks an affordance; it is that sitting **directly under a segmented row** made him read the chips
   as *the same kind of object*, so "Night" looked like a label inside a group rather than a control.
   A false analogy created by proximity is not fixed by adding arrows — arrows would make a **third**
   idiom in one sheet. Making colour genuinely the same widget as reading mode removes the
   misreading at its source, and both controls are "pick one of N" anyway: three modes, four schemes.
   **If four segments prove too wide** on a phone, fall back to a dropdown showing the four labels —
   still a chooser, still showing the options. Arrows are the last resort, not the first.

3. **Role: a chooser affordance, never arrows.** Roles are an unordered set; stepping through them
   with ‹ › implies a sequence that does not exist. Give it the `…` convention `Layers…` already uses,
   or a dropdown chevron.

4. **Answer "I don't know what Role does" — this is the real defect.** No affordance fixes it. One
   line under the label, in the style of Auto-update's subtitle, saying what choosing a role *does*:
   it selects which annotation layers you see **for the whole concert**, which is why most people never
   open Layers. `Layers…` deserves its own line too — it overrides the choice **for the current song
   only**. Those two sentences are the difference between three mystery chips and a settings screen.

## Do not touch

**The on-stage ⚙ cycle stays a cycle.** Mid-performance, one tap with eyes on the music is right, and
`SchemeCycleDirection` exists for exactly that walk. This task changes **Parameters**, where you have
time to choose; it must not turn the performance control into a picker.

## Done when

- Each of the three carries a label in the sheet's existing style, and the button shows the value.
- All four colour schemes are visible as choices, and picking one applies it — checked on the device
  at phone width, which is where a four-segment row would break.
- Role and Layers each carry one line saying what they affect, and the two lines make the
  whole-concert vs current-song distinction obvious without reading the code.
- The performance-mode colour cycle is unchanged.
- `:shared:testDebugUnitTest` green; match the count.

## Sequencing

**After the concert.** This is discoverability, not a defect on the stand: every control works today
and VLL now knows what they do. [A62](A62-scroll-mode-back-lands-at-the-song-start.md) — where a swipe
lands you in the wrong place mid-performance — is the one that earns an exception to the freeze; this
one does not.
