# A98 — Studio's session cookie is seeded HttpOnly, as the server sets it

Owner: **mobile**. Spec: Fable, 2026-11-03, from mobile's measurement. Size: S.

## What was measured

The rig: an API 34 emulator with today's `latest` app (versionCode 3011), signed in to a scratch core with demo data,
and Studio opened from the app. Read over DevTools:
- `document.cookie` shows the session cookie, name and full value. **Studio's own scripts can read the session.**
- The WebView stores it as `httpOnly: false`, `sameSite: null`, a session cookie.
- **The server's own `Set-Cookie`**, the positive control: `Path=/; Expires=…; HttpOnly; SameSite=Lax`, plus
  `Secure` on https (`core/internal/httpapi/webapi.go`).

**The cause:** `seedSessionCookie` (`EditScreen.kt`) passes only the stored `name=value` to `CookieManager.setCookie`,
so every attribute the server set is dropped. Its KDoc ("HttpOnly is fine — the WebView still sends it; JS just can't
read it") describes the intent, not what the code does.

In a browser, Studio's scripts can't read the session. In the app they can. Nothing is known to exploit it today,
but any script injection in Studio would then reach the session.

WIN08's Windows host seeds the same `name=value` through SWT's `Browser.setCookie` (WIN08 stage-1 review, item 5).

## The work

1. **One shared function**, in `jvmSharedMain` next to `sessionCookieFor`: `sessionCookieSeed(cookie, url)`.
   - It returns the stored `name=value` plus `; Path=/; HttpOnly; SameSite=Lax`.
   - It adds `; Secure` when `url` is https.
   - **No `Expires`.** The cookie is seeded again on every Studio open, so a session cookie in the web view is
     enough. Say so in the KDoc.
2. **Android's `seedSessionCookie` uses it.** Fix its KDoc so it says what the code does.
3. **WIN08 uses the same function** for `Browser.setCookie`. SWT's `HttpCookie.parse` reads `HttpOnly` and `Secure`;
   SWT sets no SameSite. Web-core makes that change in its stage-1 rework, once this lands. If WIN08 gets there
   first, web-core adds the function as specified here, and this task only rewires Android.

## Acceptance

- **Unit (pure, jvmShared):**
  - http → `HttpOnly`, `Path=/`, `SameSite=Lax`, and no `Secure`;
  - https → the same, plus `Secure`;
  - the `name=value` is unchanged.

  **Teeth:** drop `HttpOnly` → red.
- **Emulator (API 34):** a scratch core on an isolated port, demo data only, never :8080.
  - **After the fix:** `document.cookie` no longer contains the session's name, and DevTools shows `httpOnly: true`.
  - **The positive control:** Studio is still signed in, and a page that needs the session loads.
  - **An existing cookie is replaced:** sign in and open Studio with main's APK (`httpOnly: false`). Install the
    head over it and open Studio again: `httpOnly: true`.
- **CI:** the usual set, green, `desktop (windows)` included, since this is a jvmShared change.

## Not in this task

- The core's own cookie.
- Studio itself.
