# A100 — On Android 9, the app forgets its sign-in at every restart

Owner: **mobile**, after A99. Spec: Fable, 2026-11-06, from mobile's note while measuring A99. Size: S, once the
cause is measured.

## What was seen

On an API 28 emulator (A97_API28), the session doesn't survive a process restart. The secrets file is rewritten at
launch. That matches A54's heal path in `Storage.kt` (`openOrHeal`): when `EncryptedSharedPreferences.create` can't
open the store, it wipes the store and its master key and starts again. So each launch signs the person out and
resets their settings.

If it happens on real Android 9 devices too, a band member on an Android 9 tablet must sign in every time they open
the app. VLL's tablet runs Android 14.

## The work

1. **Measure first.**
   - On API 28, over 3 restarts: does the heal fire every time? Log the exception `create` throws, its class and
     message, before the wipe.
   - The same on API 29 and API 34, for comparison.
   - Is it the emulator only? Say what the evidence supports.
2. **Name the cause from the measurement.**
   - One unproven hypothesis: `MainActivity` opens two `Storage` instances (the `Storage.kt` KDoc says so), and both
     may create the master key or the keyset at first launch, leaving a store that the next launch can't decrypt.
   - Confirm or rule it out. Don't fix by guess.
3. **Fix it, so A54's heal stays a last resort, not a launch routine.**
   - One opener of the encrypted store per process is the obvious shape if the race is the cause. Your call.
   - The heal itself and its "settings were reset" signal stay as they are.

## Acceptance

- **API 28:** sign in, set a theme, then restart the app 3 times. The session and the theme survive, and the heal
  log stays silent. Before the fix, the same journey shows the heal.
- **API 34:** unchanged.
- **A unit or instrumented test** for whatever was racing, where the platform allows. **Teeth:** reintroduce the
  cause → red.
- **CI:** the usual set, green.
