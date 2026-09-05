# T148 — The bake's date must read in the musician's own time, not UTC

**Lane:** mobile. **Size:** S. **Status:** spec, 2026-09-05, from VLL on the tablet.

## How this was found, which is the whole argument

VLL, looking at the device an hour after I baked at **11:04** local: *"tu peux regarder le bake de 9h04
que j'ai sur la tablette"*. He named the bake by a time that does not exist for him. `concertRowSubtitle`
formats `bakedAt` in **UTC**, and he is on UTC+2.

Then, plainly: *"faut corriger pour afficher l'heure locale."*

**This is the failure mode a unit test cannot catch.** `BundleRowTest` asserts
`"rev 7 · 2023-11-14 22:13"` and is perfectly correct — it pins the formatter against a fixed instant. What
it cannot know is that the string reaches a human who reads it as *his* clock. The row exists to answer
*"which bake is this?"*, and a two-hour lie makes it answer wrongly at exactly the moment it matters — two
bakes made the same afternoon.

## Required

- Format `bakedAt` in the **device's local time zone**, not UTC.
- The choice was deliberate: T143's commit says *"UTC-formatted with no datetime dep"*. So say **how** the
  zone is obtained — a `expect/actual` platform seam is the honest route in KMP; adding a datetime
  dependency is a decision, not a detail. **Do not smuggle in a library without saying so.**
- If a zone genuinely cannot be resolved, fall back to UTC **and label it** (`… UTC`). A timestamp with no
  zone that is silently not local is what produced this report.

## ⟨R1⟩ Red first, with teeth

The existing test passes today and will pass after the fix if you only change the input — so it guards
nothing here. The new assertion must **fail on the current code**:

- fix an instant and a zone at **UTC+2**, assert the rendered string shows the **local** hour, not the UTC
  one. On today's code this is red by exactly the two-hour gap VLL hit.
- one zone **behind** UTC as well, so a sign error cannot pass both.
- a date-boundary case: an instant that is *the previous day* in UTC and *today* locally. Renders the local
  date — this is where a naive `+offset` on the time alone breaks.

**Teeth-check:** revert to UTC formatting and confirm all three go red.

## Device-QA

Bundled with the T143 / T147 device pass, now unblocked: bake a bundle, look at the row, and read the time
back against the tablet's own clock. That is the only check that would have caught this in the first place.

## Out of scope

Showing a relative time ("2 hours ago"), and any other timestamp in the app. This task fixes the bake row,
which is where the confusion was reported.
