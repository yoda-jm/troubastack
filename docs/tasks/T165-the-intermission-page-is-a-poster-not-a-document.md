# T165 — the intermission page is a poster; Stage reads it like a document

**Surface:** TroubaStage. **Lane:** mobile. **Kind:** bug (visual, on VLL's stage rig).
**Number claimed** in the same push as this file.

VLL, 2026-09-07: *"la page d'intermission marche bien en portrait (et encore il y a du noir en bas), mais
en paysage c'est pas top."*

## Measured, on his tablet — not inferred

Pulled the baked separator raster off the device (`s11-p0-raster.png`, 1241×1754, A4 portrait) and profiled
its ink by band:

| band of the page | ink |
|---|---|
| 0–30 % | none |
| **30–60 %** | the label and the band name |
| 60–80 % | none |
| **80–90 %** | **the TroubaStage wordmark** |
| 90–100 % | none |

His landscape screen is 1920×1200. Stage draws the page at column width, so an A4 page is **2716 px tall**
and the viewport shows its **top ~44 %**. He therefore sees: the empty top third, part of the title block,
and **no wordmark at all** — it sits at 85 % of a page whose bottom half is off-screen. In portrait the
whole page fits, so it looks right. That is exactly the difference he reported.

## The cause is a category error, and it is mine

The separator is **baked as a page** so it can ride the bundle like any other — that part is right, and
T153 slice 2 enforces "exactly one page" for good reasons. But Stage then **presents** it like a page:
width-fitted and scrollable, because that is what you do with a chart you read line by line.

**A break's page is not read. It is looked at.** It has no reading order, no second half, nothing to scroll
to. Presenting a poster as a document is what produces an empty top third, a cropped block and a mark
nobody sees.

## ⟨D1⟩ VLL's ruling, 2026-09-07 — and it supersedes what I first wrote

*"Il faut sans doute baker un truc qui rentre en paysage, et si c'est affiché en portrait alors il faut le
centrer en comblant avec la couleur de background en haut et en bas."*

**Bake the card LANDSCAPE, and letterbox it in portrait with the background colour.** This is better than
the rule I had written ("always fit an A4 portrait card"), for a reason that is about his rig rather than
about taste: **the tablet is in landscape on stage.** A landscape card fitted to a landscape screen fills
it; a portrait card fitted to the same screen is a narrow strip with two wide empty margins. Optimise for
the orientation the thing is actually used in, and let the other one letterbox.

So, in full:

> A song's page obeys the reading mode. **An intermission's page ignores it and always fits the viewport**,
> and the card itself is **baked landscape**. In portrait it is centred, with the surround above and below
> taken from the reading scheme's background.

This also disposes of the scroll-trim question for breaks: there is nothing to trim, because there is
nothing to scroll.

**A consequence to name rather than discover:** the printed concert PDF composes the bundle's pages, so it
will now contain **one landscape page among portrait ones**. That is acceptable — a break page is a divider
in a printed set too — but it must be a decision, not a surprise, and the composer must not stretch it to
portrait to "fix" it. The T158 running-order sheet is unaffected: there, a break is a text row.

**And the filler must not be black.** VLL: *"et encore il y a du noir en bas"* even in portrait. A fitted
portrait page on a landscape screen leaves large margins; they are currently **pure black** on a white
page. Take the surround from the scheme (A69's `chromeColors()` or its successor), never a literal — the
same trap T164 flags. On a light scheme it should read as paper or a quiet neutral, not as a hole.

## Not in scope, deliberately

**Do not re-compose the baked card to survive a landscape crop.** Moving the wordmark up would make the
poster look wrong on the printed sheet and in portrait, to compensate for a presentation bug. Fix the
presentation.

## ⟨R1⟩ Red first

- The baked separator raster is **landscape** (width > height). Red today: it is A4 portrait.
- A break's page renders **fitted** in SCROLL and in FIT_WIDTH, not only in FIT_PAGE. Red today.
- In a portrait viewport the card is **centred**, with the surround above and below from the scheme.
- **Teeth:** revert to "a break obeys the mode" and the SCROLL/FIT_WIDTH assertions must fail — a test that
  only checked FIT_PAGE would pass today and prove nothing.
- A break's page is **not scrollable**: the whole page is on screen, so there is no scroll extent to
  consume.
- A song's page is **unchanged** in all three modes — this must not leak into normal pages.
- The surround colour comes from the scheme, asserted different between a light and a dark scheme.

## Done means

VLL turns the tablet either way during the break and sees the same complete card — label, band name and the
mark — with a surround that belongs to the scheme he is reading in.
