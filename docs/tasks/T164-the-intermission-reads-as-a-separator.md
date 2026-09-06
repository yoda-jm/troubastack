# T164 — an intermission should look like a break, not like a song without a number

**Surface:** TroubaStage — the song drawer **and the top bar's position label**. **Lane:** mobile.
**Kind:** polish (visual).
**Number claimed** in the same push as this file.

VLL, 2026-09-07, seeing his first intermission in the drawer: *"ce serait bien si l'intermission était
centré dans la colonne des songs dans la présentation Stage avec des traits à gauche et à droite et
peut-être un peu en grey."*

## Why he is right, and it is not only taste

T153 made the break **correct**: it carries its label, takes no number, and does not shift the song after
it. But it is still emitted as `DrawerRow.Song` (`StageScreen.kt:1106`), so it renders as a song row with an
empty number column — **an absence rather than a thing.** Glanced at in a dark venue between two numbered
titles, a left-aligned untitled row reads as a song whose number failed to load, not as a pause.

**And the printed sheet already draws it his way.** `setlistpdf` renders a break as a centred
`"— label —"` (`setlistpdf.go:108`) while songs stay left-aligned and numbered. So this is not a new visual
idea: it is **making the drawer agree with the sheet the band reads from**. That consistency is the
argument, and it is the one to keep if anything else about the styling is debated.

## What to build

A **distinct row type** — `DrawerRow.Intermission` — rather than a `Song` row with special-case styling,
so nothing that iterates song rows (numbering, jump targets, cue chips) has to remember to skip it.

- **Centred** in the drawer's column.
- **A rule to the left and to the right** of the label, vertically centred on the text — the conventional
  section-break device, and it is what makes it read as an interruption rather than an entry.
- **Muted colour** for both the rule and the label (VLL's "un peu en grey"), so it recedes relative to the
  numbered titles it separates.
- **The label, or the default.** Empty ⇒ the presenter's `INTERMISSION_DEFAULT_LABEL`, never `"Song N"` —
  the T153 rule, unchanged.

**It stays selectable.** VLL settled that "next" stops on a break, so the row must still be tappable to
jump to it. Do not turn it into a decorative non-interactive divider — `DrawerRow.Divider` already exists
for that and is a different thing.

## The top bar says **"Song 12/27"** on a break — same rule, second surface

Found by mobile's on-device QA (2026-09-07) and reported there as *"minor polish (not a bug)"*. **It is the
defect VLL himself reported**, and he did not call it minor. On the Stage top bar, seeing the break:

> *"et dans l'app l'intermission dit «Song 12» (il est en 12e position), ça numérote après, ..... bref c'est
> pas top."*

**Half of that is already fixed and half is not**, which is worth stating precisely so nobody re-fixes the
fixed half:

- *"ça numérote après"* — **fixed.** Device QA confirms the song after the break is still #12; the break
  takes no running-order number and shifts nothing.
- *"l'intermission dit «Song 12»"* — **still live.** The top bar's position label is a plain entry counter,
  so a break is announced as a song.

It is the same rule as the drawer row — *a break is not a song* — on a second surface, so it belongs here
rather than in its own task. The position label for a break should read as a break (its label, or the
default word, with the *N/total* position kept if that is useful for orientation) and **never `Song N`**.

⟨R1⟩ **the top bar on a break renders no `Song N`** — red today. Teeth: the assertion must fail on the
current string; a test that only checked the drawer passes today and proves nothing about the bar.

## The trap, and it is A69's

**The drawer's colours must come from the scheme, not from a hardcoded grey.** A69 is landing exactly
because the drawer stays white in NIGHT/AMBER; a `Color.Gray` here would be invisible on a dark surface and
would have to be found and fixed a second time. Take the muted tone from the same
`chromeColors()`/scheme source A69 introduces — and if A69 has not landed yet, **say so at the gate and
sequence behind it** rather than hardcoding a value that A69 will then have to hunt down.

## ⟨R1⟩ Red first

- `drawerRows` emits an **`Intermission` row**, not a `Song` row, for a break — and the song rows around it
  keep their numbers `1, 2` with the break carrying none. Red today: it is a `Song`.
- **Teeth:** revert to emitting a `Song` row and the type assertion must fail; a test that only checked
  "no number" would still pass, because that is already true.
- An empty label renders the default word, not `"Song N"` and not an empty rule pair.
- The row is still **tappable** and jumps to the break's page (VLL's settled "next stops on it").
- The muted colour is read from the scheme, not a literal — assert it differs between the light and a dark
  scheme, which is what stops the A69 mistake recurring.

## Done means

Glancing at the drawer in a dark venue, VLL sees at once where the set breaks — the same way his printed
sheet shows it — and nothing about the numbering, the jump behaviour or the night schemes has moved.
