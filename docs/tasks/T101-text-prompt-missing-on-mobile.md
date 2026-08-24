# T101 — BUG: the text-annotation prompt doesn't appear on mobile

**Priority:** high — a core annotation tool is dead on the surface VLL actually uses · **Size:** S–M
**Area:** `web/studio` (Web & Core lane). **Assigned by VLL**, 2026-08-24.

> VLL: *"when trying to add a text annotation I don't have a popup, at least on mobile with current
> live version, probably linked to the other bug reported this weekend about text always rearming,
> fix is probably wrong."*

## 1. Status: NOT confirmed by me. Reproduce on a real device FIRST.

I investigated and could **not** establish the mechanism. Do not build a fix on my hypothesis without
reproducing first — I am handing over a lead, not a diagnosis.

- A touch repro at a **desktop viewport** (`hasTouch`, `page.touchscreen.tap`) — **PASSES**. The dialog
  opens. So a synthetic tap alone does not break it.
- Under **Pixel 5 emulation** the dialog never appears — but the **mouse control test fails there too**,
  and `locator.tap()` on `pdf-page` times out entirely. Two independent signs that *my mobile harness*
  is broken, not proof of the product bug. I am not counting it as a reproduction.

## 2. The hypothesis worth checking first

The prompt is opened from **pointerdown**, while the finger is still down:

- `WetCanvas.tsx:725` — the `tool === "text"` branch runs in the pointer-DOWN handler and calls the
  async in-app `prompt(...)`, which mounts a full-screen modal backdrop under the finger.
- `Dialog.tsx:137-138` — the backdrop cancels on **`onMouseDown`** when `e.target === backdropRef.current`.

On a real phone, the browser fires **compatibility mouse events** (`mousedown`/`mouseup`/`click`) after
`touchend`, targeted at whatever is under the touch point *at that moment* — which is now the
just-mounted backdrop. That would cancel the dialog with the very same tap that opened it: a flash, or
nothing at all. A mouse press can't do this, because its real `mousedown` fires before the backdrop
exists.

T91's commit message asserts the opposite — *"its modal backdrop covers the canvas so no stray tap
lands while it is open"* — which is sound for later taps and is exactly the reasoning to re-examine for
**the tap that opened it**.

If that's the cause, the fix is about *when* the modal is opened or *which* event dismisses it (open on
pointer-UP; and/or ignore a backdrop dismissal that arrives within the same gesture as the open).
**Decide that after reproducing, not before.**

## 3. Ruled out — don't spend time here

- **The Fullscreen API is not used** anywhere in `web/studio/src`, so the classic "portal to `document.body`
  is invisible while an element is fullscreened" trap does **not** apply. The studio's fullscreen is a
  CSS layout mode.
- **T90's one-shot disarm ordering is correct.** VLL suspected the re-arm fix. T91 moved the
  `onTextResolved?.()` call *inside* the promise's `.then()`, so it still fires after the prompt
  resolves, not before. The one-shot logic itself is not obviously the culprit — though the T90+T91
  *interaction* (a modal opened from pointerdown, where a blocking `window.prompt` used to sit) is
  precisely the suspect above.

## 4. The real coverage gap — fix this regardless of the root cause

**Every T90/T91 test drives `page.mouse.click` at a desktop viewport.** `text-oneshot.spec.ts` and
`in-app-dialogs.spec.ts` never exercise touch, and never a phone viewport. The tool VLL reported as
broken is one he uses on a phone, and the suite is green.

This is the same shape as the `crypto.randomUUID` blind spot in T99: *the suite passes because every
test drives the one environment where the bug cannot occur.* A green suite is not evidence here.

So: whatever the root cause turns out to be, this task must leave behind **a touch-driven test of the
text-placement path that fails today**. Getting a mobile-viewport harness that is actually trustworthy
is part of the work — mine was not, and that gap is itself a finding.

## 5. Acceptance

- A **real-device or trustworthy-emulation reproduction** recorded at the gate before any fix.
- A test that **fails before the fix and passes after**, driving **touch**, on the text-placement path.
- The T90 one-shot behaviour still holds (`text-oneshot.spec.ts` green): after placing or cancelling,
  the tool reverts to `select` and a second tap opens nothing.
- The T91 guarantee still holds: no native `window.prompt`; suppression-proof.
- If the mobile-viewport harness needed fixing to get there, say so explicitly — that's reusable.
