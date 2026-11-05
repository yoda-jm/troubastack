# T209 — Studio's PDF viewer works on older WebViews

Owner: **web-core**, after WIN08 stage 2. Spec: Fable, 2026-11-06, from mobile's A99 measurement. Size: S.

## What was seen

On an API 34 emulator image whose WebView is **Chromium 113**, Studio's song viewer fails with "Promise.withResolvers
is not a function". `pdfjs-dist` 4.10 (`web/studio/package.json`) uses it, and it arrived in Chromium 119. So a tablet
whose WebView isn't updated can't open songs in Studio, inside the app or in its browser.

VLL's tablet works today, so its WebView is newer. That's typical, because Play Store updates WebView. But a tablet
without Play, or with updates held back, is stuck at its image's version.

## The work

- Load pdf.js's **legacy build** (`pdfjs-dist/legacy/build/…`, with its worker), which targets older browsers,
  instead of the default build. Say what it costs in bundle size.
- Don't add hand-written polyfills unless the legacy build is measured to fall short.

## Acceptance

- **The real check, on mobile's rig:** the API 34 image with WebView 113. A demo song opens in Studio, inside the app
  and in the emulator's browser. Mobile runs it; ask them through me.
- **A CI vector:** an e2e test that removes `Promise.withResolvers` before Studio's scripts run, then opens a demo
  song: it renders.
  - **Teeth:** switch back to the default build → red.
  - pdf.js's worker is a separate global, so say whether the vector reaches it. If it doesn't, the emulator check is
    the authority for the worker.
- **Bundle size:** before and after.
- **CI:** the usual set, green.
