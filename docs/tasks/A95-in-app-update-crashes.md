# A95 — "Update" crashes the app: the metered-network check needs ACCESS_NETWORK_STATE

Owner: **mobile**. **Urgent, small: do it before continuing WIN03.** Spec: Fable, 2026-11-01. Reported by VLL on
his 11" tablet: "cannot update from the app, it crash the app, nothing happen".

## What happens (read from the tablet's crash log, 2026-11-01 22:12)

The installed build is 89c8e80 (2026-10-21), which has A88. Tapping Update crashes every time:

```
java.lang.SecurityException: ConnectivityService: Neither user 10356 nor current process has
  android.permission.ACCESS_NETWORK_STATE.
  at com.troubastack.app.AppUpdater.onMeteredNetwork(AppUpdater.kt:74)
  at com.troubastack.app.AppUpdateController.update(AppUpdateController.kt:83)
```

`AppUpdater.onMeteredNetwork` reads `ConnectivityManager.isActiveNetworkMetered`, and the manifest declares
INTERNET, multicast, camera, Bluetooth and REQUEST_INSTALL_PACKAGES, but **not ACCESS_NETWORK_STATE**. Main is
unchanged on both points, so every installed build since A88 crashes on Update.

A88's device pass could not catch this: dev builds stay quiet, so the Update button was never pressed on a build
that offered one.

## The fix

1. Declare `android.permission.ACCESS_NETWORK_STATE`. It is a normal permission: granted at install, no prompt.
2. `onMeteredNetwork` must never throw. If the state cannot be read, treat the network as **metered**, so the
   "≈15 MB over mobile data?" confirmation is shown. Asking once is harmless; crashing is not.
3. Look for any other system call on the update path that needs an undeclared permission, and list what you
   checked at the gate.

## Tests

- A JVM test that reads `androidApp/src/main/AndroidManifest.xml` and asserts the permission is declared.
  Sabotage: remove it → red.
- The failure path of the metered check, through a seam that throws: it returns "metered" and nothing crashes.
  Sabotage: drop the catch → red.
- **The real update path, end to end, on the emulator. This is the pass A88 never had.**
  - Build a non-dev APK of your head with a versionCode below the current `latest` and install it.
  - Tap Update, with the real `latest` release as the newer version.
  - The download runs and the system installer prompt appears. Screenshot it.
  - Do it again with the network marked metered, if the emulator lets you (the confirmation appears first).
  - Report what you could not simulate.

## VLL's tablet afterwards

His installed build has the bug, so it cannot update itself, even after this lands. It needs **one manual
install**: open the `latest` release's APK in Chrome on the tablet and install it over the app. It has the same
signing key, so concerts, notes and sign-in are kept. Say so in the LANDED note. The Fable relay tells him in
French. After that one install, self-update works.
