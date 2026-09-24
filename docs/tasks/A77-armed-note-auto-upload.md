# A77 — Armed auto-upload: a time-limited window where the tablet mirrors its notes to Studio

**Lane:** mobile · **Status:** specced, not started · **Origin:** VLL, 2026-09-23, after we talked an
unconditional version out of existence: *"une option/paramètre qui s'arme pour un temps limité sur la
tablette, upload automatique des notes, que l'utilisateur active quand il sait que le fond de page ne va pas
bouger."*

## 1. Why this shape and not the one we started with

The first idea was auto-send, always. It kept colliding with the same wall: a rehearsal note is keyed on the
page's **raster hash**, live mode **re-bakes**, and a re-bake detaches the note you are drawing on. Every
version of the feature turned into machinery for handling a background that moves.

**VLL's version does not solve that problem — it makes it not apply.** The user asserts the precondition
("the page behind me will not move"), so nothing has to be engineered for the case where it does. It also
leaves his other reason for taking notes untouched: he often writes precisely because there is **no
network**, and an unarmed tablet behaves exactly as today.

**The friction is deliberately kept** (VLL): the note stays a bitmap, it never travels back to the tablet,
and it is still recopied by hand in Studio. **Only the send is automated.** This is not a step toward notes
becoming annotations.

## 2. The precedent to copy — the shape, not the prose

Live mode (P201/I11) is already **opt-in, prominently indicated, and self-expiring** ("a forgotten live mode
must not survive to the next rehearsal"; "leave Stage ⇒ OFF"). A77 is its tablet-side twin and takes all
three properties. Do not invent a new idiom for arming a windowed mode.

## 3. ⟨D1⟩ Duration: the SAME window live mode uses — **3 hours** — and leaving Stage disarms it

**CORRECTED 2026-09-23 (Fable).** This first said 2 hours, citing a *sentence* in P201 ("a 2-hour
rehearsal"). The product ships `app.LiveModeWindow = 3 * time.Hour`, and the Studio row reads *"Arm live
mode · auto-bakes for 3 h"*. I confirmed a precedent by its prose instead of its constant — the same error
as T175, where I read the chart editor's comment instead of its route.

**One number, one concept:** both windows answer "how long is a rehearsal", so they must not disagree.
Whoever changes one changes the other; say so where the value is written on the tablet side.

No picker — a chooser would ask VLL to make the same decision every time for no gain. Two independent
expiries, so "I forgot" is nearly unreachable:

- the window ends on its own after 2 hours — **test it on an injected clock**, as P201 does;
- **leaving Stage disarms it**, matching P201's app-side transiency.

## 3c. ⟨D6⟩ The UI says no verb at all — "Auto-upload ON · 3 h" (VLL, 2026-09-24)

Studio owns **"Arm live mode"**, and the two modes are **mutually exclusive by construction**: live mode
arms edits to re-bake (the background moves); this arms *because the background will not move*. One verb for
two states that can never be on together is the worst case for a shared word, so VLL removed the verb rather
than picking a second one.

**The strings:**

- ⚙ row — **"Auto-upload notes to Studio"**, subtitle **"ON for 3 h · notes send as you draw"**
- banner — **"AUTO-UPLOAD ON — your notes are sending to Studio"**
- delete while on — **"Removed here and in Studio"**, against the plain **"Removed"** (§5's requirement: the
  same gesture must not silently mean two things)

**The internal names stay** (`isArmed`, `armedUntil`, `ARMED_UPLOAD_WINDOW_MS`). "Armed" is a precise
internal concept and renaming tested code buys the reader nothing; the UI avoids the verb for a collision
reason that does not apply inside the codebase. **Record it in glossary §10 as a deliberate divergence**, so
the next vocabulary sweep does not "fix" it — the same disposition D14 got.

## 4. ⟨D2⟩ Armed from the ⚙ Stage settings sheet, not the note bar

The ⚙ sheet is where per-session Stage state already lives (reading mode, colour mode, layers), and arming
is a deliberate act rather than a drawing act. **Keep it out of the note bar**: A75 measured his panel at
**686 dp** and spent that whole task getting buttons *off* the note rows; putting a mode switch back on the
strip would undo it.

**While armed, say so prominently** — the same reason P201's autobake carries a banner. His notes are
leaving the tablet; that is not a state to infer from a settings screen he closed.

## 5. ⟨D3⟩ While armed, a cleared page clears in Studio too — and only while armed

VLL's rule: *"« Studio reflète ma tablette » pendant ma séquence live, ça doit être la règle."* So inside the
window, deleting a note deletes the server copy.

**Two bounds, both necessary:**

- **Only while armed.** Outside the window, today's behaviour stands: clearing is local and the Studio copy
  survives (T170 §7's two lifetimes). The mirror is the armed state's promise, not a new global rule.
- **Only live pages.** An orphaned note is not on a live page, so it is outside the mirror: it stops
  auto-sending and clearing it locally does not reach the server. Without this sentence, "Studio mirrors my
  tablet" will later be read as "including my orphans", which reopens everything §1 avoided.

**And say it in the UI when it bites:** a delete that also removed the Studio copy must be visibly different
from one that did not, or the same action will mean two things depending on a mode the reader has forgotten.

## 6. ⟨D4⟩ A bake arriving while armed DISARMS it, visibly

The precondition is VLL's assertion, and someone else can violate it: the conductor bakes, the raster
changes, his note detaches. Continuing would push pixels tied to a background that no longer exists.

So: **an incoming bake ends the window and says so.** Not a silent stop and not a silent continue — he armed
this on a belief that has just become false, and he is the one who can decide whether to re-arm. Same
principle as A76's pedal and the hidden-note badge: a visible, recoverable state beats a silent one.

## 7. Sending, while armed

- **Coalesce, do not send per stroke.** A note grows stroke by stroke; debounce after the last stroke and
  **flush on page turn, on leaving note mode, on disarm and on expiry.**
- **Each auto-send carries `overwrite`** — and the spec says why, so nobody later "fixes" it into a prompt:
  the tablet is replacing **its own earlier auto-send of the same note**, not another member's work. That is
  precisely the distinction that made the unconditional overwrite wrong in the bulk-send path
  (`bulkNoteOutcome`), and it is why it is right here.
- A 409 that is *not* from the tablet's own prior auto-send still means a conflict: do not clobber, disarm
  and report.

## 8. Acceptance

- Armed, drawing on a stable page: the note reaches Studio with no Send press; one upload per pause, not one
  per stroke.
- **Clock-injected expiry**: the window closes on its own and the banner goes.
- Leaving Stage disarms.
- A bake arriving while armed disarms and is reported on screen.
- Unarmed behaviour is byte-for-byte what it is today — including with no network.
- Clearing a live page while armed removes the Studio copy; clearing an orphan does not; clearing anything
  while unarmed does not.
