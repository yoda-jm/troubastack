# A66 — "where you are" on Home and Stage: drop the duplicate, let the product name carry it

**Lane:** mobile. **Size:** S–M. **Status:** spec, ruled from the lane's proposal. **Takeable now.**
**FREEZE LIFTED — the concert was cancelled (VLL, 2026-09-03).** The pre-gig app freeze is over and this is takeable now. The sequencing note below is kept only where it still carries a real dependency; "after the gig" no longer applies to anything.


**Raised by:** VLL, 2026-09-03: *"we can probably remove the TroubaStage at the very top left (same line
as Guest/2 bands), we should know where we are, but then instead of Perform in the Stage submenu, maybe
TroubaStage (with right colors) and make subsections and color as brand required. same for the studio
page."*
Proposed by the mobile lane and **routed for review before implementation** — the right order, and the
reason these rulings exist before any code.

## Ruling 1 — drop the top-left masthead. Yes.

`HomeScreen.kt:419` renders a muted `"TroubaStage"` (`labelLarge`, `onSurfaceVariant`) on the account
row. Remove it.

**The reason matters more than the removal**, because VLL also said *"we should know where we are"* and
those look contradictory. They are not: **Home is the root — you cannot be lost on it.** A location cue
earns its place on screens you can arrive at from several directions, not on the one screen every path
returns to. And the label was wrong anyway: it said `TroubaStage` while Home is the launcher for
*both* products.

**So the "where am I" budget moves to where you can actually be lost** — the Studio WebView frame,
whose title is literally "Edit" today. That is [A65](A65-studio-screen-entry-points-and-a-showable-QR.md),
and it is where the effort belongs.

## Ruling 2 — the Stage tile leads with the wordmark, but keeps its explanation

Replace the leading `"Perform"` with the two-tone `▶ TroubaStage` wordmark (already built, `87e1460b`),
per BRAND10's rule and the accent/contrast constraints in
[BRAND09](BRAND09-home-wears-the-product-colours.md).

**But do not delete the sentence.** Read the current subtitle before changing it
(`HomeScreen.kt:468-473`): it is already dynamic — it shows `lastConcertName` when there is one, and
falls back to *"Perform · open a concert"* / *"Perform · import or download a concert"* when there is
not. **That fallback is the only place in the app that tells a newcomer what TroubaStage is for.** A
name is not an explanation — VLL made exactly this point about the `Role` chip
([A63](A63-the-parameters-chips-say-what-they-are.md)): *"I don't know exactly what it does."*

So: wordmark on the title line, and the existing dynamic subtitle **unchanged**.

## Ruling 3 — NO branding on the perform screen. This one is not a preference.

The lane asked whether the Stage *perform* screen gets a branded header. **No**, and the reason is
measured rather than aesthetic:

- The perform surface is deliberately spare — it shows song and position and nothing else, because it
  is read under pressure, on a stand, mid-piece.
- **The colour schemes exist to protect dark-adapted vision.** `NIGHT` and `AMBER` are used on a dark
  stage; `AMBER` is built specifically to preserve night vision in a pit or blackout
  ([12-annotation-colour](../design/12-annotation-colour.md)).
- A brand accent painted there would be **UI chrome, which the scheme filter does not govern** — the
  same class of inconsistency [A64](A64-night-mode-inverts-annotation-colours.md) documents for cue
  glyphs, where a red glyph survives a filter that turns the page's red ink cyan. Adding a magenta or
  gold wordmark to the performance screen means adding a colour that stays bright in Amber, in a
  blackout, next to the music.

**Brand the launcher, never the instrument.**

## Ruling 4 — subsections must earn their place

Accent on **section headers only** (large text, judged at 3:1), body text on the neutral scheme — that
is BRAND09's constraint and it stands.

But the count matters: a header above a single item is noise, not structure. **Only introduce a section
where there are genuinely several things of the same kind**, and say in the submission how many items
each section holds. Home is a short screen; sectioning it into four one-item groups would add chrome
and remove nothing.

## Ruling 5 — the Studio half folds into A65

The lane proposed this and is right. [A65](A65-studio-screen-entry-points-and-a-showable-QR.md) already
reworks that frame (title → band name, two entry points, the QR item). Doing the Studio branding
anywhere else would double-spec the same screen. **A66 is Home + Stage only.**

## Done when

- The top-left masthead is gone, and nothing replaces it on Home.
- The Stage tile leads with the two-tone wordmark **and still explains itself** when there is no last
  concert — check the empty state specifically, since that is the one that carries the explanation.
- **The perform screen is untouched.**
- Any section introduced holds more than one item, and the submission says how many.
- Accent use obeys BRAND09: headers/large text only, never small text on `--background`.
- `:shared:testDebugUnitTest` green, count matched; device-checked in both themes.
