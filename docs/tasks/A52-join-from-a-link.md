# A52 — Joining a band from a link: deep link in, manual entry as the floor

**Lane:** Mobile · **Kind:** new feature · **Verified against `b6d23b7`**
**Depends on:** A51 (`parseTroubaLink`, `joinDecision`) — do not start before it lands.
**Recommended:** T123 (server-identity probe) lands first, so the "switch server?" step can be honest.
**Files:** `app/androidApp/src/main/AndroidManifest.xml`, `MainActivity.kt`, `ConnectScreen.kt`,
`HttpTransport.kt`, plus a new join sheet. **No camera in this task.**

## Goal

A person receives an invite — as a link in a message, or as the QR the studio already renders
(`web/studio/src/components/InviteLinks.tsx:128`) scanned with the phone's own camera app — and lands in
the band **inside TroubaStage**, without hand-typing a server URL.

Today that link opens Chrome, because the manifest declares no `VIEW`/`BROWSABLE` filter at all
(`app/androidApp/src/main/AndroidManifest.xml`, 32 lines, one `MAIN`/`LAUNCHER` filter).

**Manual entry is not the consolation prize — it is the floor.** It is the only path that works when the
camera is denied or absent, it is the only path on iOS, and it is the only part of this feature that can be
exercised without hardware. Build it first; the deep link is then a second door onto the same room.

## Deliverable

### 1. Manual entry on the Connect screen

`ConnectScreen.kt:68-150` currently offers Server URL / Username / Password. Add a way to paste an invite
link and act on it, routed through `joinDecision`. When the pasted link names a server, that server URL is
what gets used — the person does not type `http://<host>:8080` by hand. That elimination is most of the
value of the whole feature; the camera in A53 only removes the paste.

### 2. Deep link

```xml
<intent-filter>
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="http"  android:host="*" android:pathPrefix="/join/" />
    <data android:scheme="https" android:host="*" android:pathPrefix="/join/" />
</intent-filter>
```

Three things to be honest about, all of which the spec accepts:

- **No App Links verification is possible here.** `android:autoVerify` needs `https` plus a hosted
  `/.well-known/assetlinks.json`. The servers this product runs on are plain-http LAN boxes. So Android
  shows a **chooser** ("Chrome / TroubaStage") rather than opening the app silently. That is the correct
  behaviour anyway for a URL that arrived from outside.
- **`host="*"` means any URL whose path starts with `/join/` offers TroubaStage** — including unrelated
  services that happen to use that path. This is deliberate: we cannot enumerate our own hosts. It is safe
  only because A51's `ConfirmServer` gate is load-bearing, and A51 has a named test for exactly this case.
- **`MainActivity` must handle a second intent.** With no `launchMode` set, a deep link into a running app
  can produce a second instance. Handle both the cold-start intent and `onNewIntent`, and choose the
  launch mode deliberately — do not leave a performer with two Stages.

### 3. The join flow

Drive it from `joinDecision`:

- **`Redeem`** — `GET /api/invite-links/{token}` to preview, show band name and role, then
  `POST /api/invite-links/{token}/accept` on confirm, then refresh bands.
- **`SignIn`** — same server, no session: sign in, *then* redeem. The token has to survive the login
  round-trip.
- **`ConfirmServer`** — show the **target host** and require an explicit confirmation before switching.
  With T123 landed, probe the target first and refuse to show a password field for a host that does not
  identify itself as a TroubaStack server. Without T123, say plainly in the dialog that the server is
  unverified.
- **`Blocked`** — explain, offer nothing. A `PasswordReset` link says to open it in a browser.

Server errors already carry meaning: accept returns **410 Gone** for expired / revoked / exhausted
(`webapi.go:518-531`), and the preview returns a machine-readable `reason`. Use those words, do not invent
a generic failure.

### 4. Token hygiene — non-negotiable

The pending token is a **bearer credential**:

- **Never persist it.** In memory only; gone on process death. Do not put it in the encrypted prefs beside
  the session cookie — a token that outlives the flow is a token that can be replayed off a stolen device.
- **Clear it** on success, on failure, on cancel, and on navigating away.
- **Never log it.** Log the host if you must log anything. This includes crash breadcrumbs.

## Verification

Report honestly which of these were run:

- unit coverage for whatever pending-token state you introduce, if it can be expressed purely (report the
  new suite total against **272**);
- **device**: paste path and deep-link path, each ending in an actual membership — check the band appears
  after the flow, not merely that the sheet said "joined". *Assert the artefact, not the gesture.*
- **device**: the `ConfirmServer` path, by deep-linking a `/join/` URL on a host that is not your server,
  and confirming the app names that host and does not offer a password field (with T123) or warns
  explicitly (without it).
- a link that is expired or already exhausted, showing the server's own reason.

If a leg was not run, say so; a step that can only skip cannot report that it stopped working.

## Out of scope

The camera (A53) · iOS · changing invite-link defaults (T122) · any core change (T123 is separate).
