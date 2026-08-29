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

## Teeth-check

For the wording map: an unmapped reason must still surface the server's own word — mutate the fall-through
to a generic string and a named test must redden. Report the count.

**Be honest that the contrast half has no automated guard.** It is a colour on a Compose surface; there is
no assertion that makes it provable. **Attach a device screenshot of the scanner with the fallback button
visible**, and say plainly in the submission that the visual half is eyeball-verified.

## Out of scope

App-wide theming · the scanner's layout · anything about decoding.
