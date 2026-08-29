# A56 — Text over the camera has to be readable, and a dead link deserves a sentence

**Lane:** Mobile · **Origin:** VLL during the device pass — *"dark blue on black, not easy to read"* ·
**Verified against `c331f85`** · **Small.**
**Files:** `QrScanScreen.kt`, and the join sheet's Blocked branch in `JoinDialog.kt`.

## 1. The scanner's controls are the default primary colour on a black scrim

The "Paste a link instead" button sits over the live camera preview in the theme's primary indigo. On the
dark preview that is close to unreadable — and it is the **escape hatch**: the control someone reaches for
when the scan is not working is the one they cannot see.

Give the over-camera controls an explicit on-scrim treatment (light/white content, and enough of a
scrim behind them that it survives a bright or busy camera image). The "Point at the invite QR" chip
already reads fine because it sits on a surface — match that.

**Do not** solve this by theming the whole app. It is the over-camera layer only.

## 2. While you are there: `revoked` is not a sentence

The join sheet surfaces the server's machine-readable reason **raw**, so a dead link reads as the single
word `revoked` (likewise `expired`, `exhausted`). Honouring the server's words was deliberate — T124's
lesson, and A52 got it right — but the word is a *machine* token and the person is holding a QR that just
failed.

Wrap it: *"This invite was revoked."* / *"This invite has expired."* / *"This invite has already been
used."* — mapping the server's reason to a sentence, with an **unmatched reason falling through to the raw
word rather than a generic message**. Never invent a failure the server didn't report.

Put the mapping in the existing `shared/join` pure seam beside `acceptOutcome`/`previewOutcome` so it is
unit-testable, including the fall-through.

## 3. "Manage" is the wrong word, and the right one is already taken

VLL, looking at the account panel: *"Manage is a strange wording for what it contains."* He is right, and
the collision is with our own UI.

`HomeScreen.kt:742` renders a secondary **"Manage"** whose own comment (`:164`) says what it is —
*"server/account details, reached via the Connect modal"*. On device that modal holds: the server URL, the
username/password, the paste-invite and scan entries, and the discovered-servers list. **It is the
connection, not the content.**

Meanwhile the TroubaStudio tile's subtitle reads *"Author, import & **manage** concerts"*. So the same
word names both "the place your music is organised" and "the place you change which server you are talking
to" — and the first meaning is the one a person will assume, because it is printed one tile away.

**Rename it to "Server & account"** (or an equally literal phrase — the test is that the label names what
is behind it). `identityHasManage`'s *behaviour* is right and stays; this is the label only.

**Adjacent, and explicitly VLL's call — I am flagging, not prescribing:** the neighbouring
**"⚙ Parameters"** is a Gallicism where English UIs say **"Settings"** (A36 established "Parameters" as the
in-repo term, so this may be deliberate product voice). **Do not change it under this task** unless VLL
says so; it is noted here so the decision is made once rather than drifting.

## Teeth-check

For the wording map: an unmapped reason must still surface the server's own word — mutate the fall-through
to a generic string and a named test must redden. Report the count.

**Be honest that the contrast half has no automated guard.** It is a colour on a Compose surface; there is
no assertion that makes it provable. **Attach a device screenshot of the scanner with the fallback button
visible**, and say plainly in the submission that the visual half is eyeball-verified.

## Out of scope

App-wide theming · the scanner's layout · anything about decoding.
