# A07 — Native wet-ink overlay (seam 2)

**Priority:** A-track LAST · **Size:** L · **Area:** `app/shared` seams, `web/studio` bridge

## ⛔ BLOCKED — do not start

This task exists to **prevent** premature work as much as to describe it. Per
`docs/design/03-rendering-and-ink.md`, the native overlay is justified **only if** the
*optimized* in-browser wet-ink path still feels laggy on the real target tablet:

1. T06 (web wet-ink optimization) must be merged, and
2. the stylus-feel spike must have been run **on the actual Android tablet** inside the
   A06 WebView, and judged insufficient by a human.

If the optimized web path feels fine, this task is **closed unbuilt** — that is the
architecture working as designed (I10: the native overlay is an optional accelerator).
Record the spike verdict in the PR/issue either way.

## Scope when unblocked (summary, not yet an executable spec)

- Implement `seams/InkOverlay.kt` Android actual: Jetpack Ink `InProgressStrokesView`
  over `GLFrontBufferedRenderer`, rendering **only** the in-progress freehand stroke
  (I9) — every other tool stays in the web layer.
- Formalize the shell↔Studio bridge protocol on top of A06's transport, following
  design/03 §"wet→dry handoff" **exactly**: viewport transform captured at stroke-start
  (never per-frame), pen-up posts the completed stroke (normalized `[0,1]` points, I3)
  with a transient correlation id, Studio renders it into the dry layer via
  `@troubastack/ink` and acks, native clears on ack. Native knows nothing about UUIDs,
  the server, or sync — Studio owns the domain object (I9/I15).
- Golden-image parity test vs `web/ink` output (the claim InkOverlay.kt's comment
  already makes; it must become true here) — visual closeness is the bar, per I8 the
  authoritative pixels are always the web dry layer that replaces the wet stroke at
  pen-up.
- Feature detection + kill switch: the in-browser wet path remains the fallback
  everywhere (I10).

Whoever unblocks this should first rewrite it as a full task file (context, exact
changes, acceptance criteria) against the codebase as it exists then.

## Code-side stylus spike (mobile lane, 2026-07-20)

Per VLL's 2026-07-20 dispatch: do the CODE-side spike + write up what needs the
physical stylus. **Verdict: A07 stays BLOCKED — correctly.** Gate #1 is met; gate #2
is inherently un-automatable and needs a ~15-min VLL stylus session.

**Gate #1 — T06 (optimized web wet-ink) is MERGED** (`9fac1f4`): the production web
path is already the low-latency one — `getCoalescedEvents()` drained once per rAF,
painted off-React straight to canvas (WetCanvas.tsx `coalescedPoints`/`onPointerMove`).
So there is no "throwaway spike to build" (design/03 §"First build step") — the real app
IS the optimized path; the spike is now just *judging it on the tablet*.

**Already implemented + testable WITHOUT a stylus (and is tested):**
- **Pressure** → variable freehand width: `web/ink` feeds `[x,y,pressure]` to
  perfect-freehand, `simulatePressure` when absent (`web/ink/src/index.ts`). Logic is
  deterministic; exercised by the ink unit/golden tests.
- **Palm rejection**: the pen-seen idiom — once any `pointerType==="pen"` is seen, a
  finger becomes pan/nav, not ink (`WetCanvas.tsx` #4, `penSeenRef`). Covered by e2e
  via CDP `Input.dispatchTouchEvent` with synthetic `pointerType`
  (`editor-touch-stucknav.spec.ts`, `editor-touch-marquee.spec.ts`,
  `editor-wet-alpha.spec.ts`) — no hardware needed for the branching logic.
- Coalesced-point batching, wet→dry commit-on-up (I8), viewport transform at
  stroke-start — all code-level, all covered.

**NOT testable without hardware (this is the whole point of A07):**
- **Input→photon latency** — pen tip → wet line under it, mid-stroke (design/03: the
  *only* reason the native overlay exists). This is a human *perceptual* judgment on the
  real digitizer + display; CDP/synthetic events say nothing about it.
- **Real palm rejection** with a hand resting on the glass (a palm is a large,
  multi-touch, moving contact the synthetic single `touch` can't reproduce).
- **Pressure-curve feel** — does variable width feel natural with a real pressure pen.

**What needs VLL's physical stylus (the go/no-go session, ~15 min):**
1. Open a song in the app's embedded Studio (A06 WebView) on the tablet, pick the pen,
   draw with the stylus across a page.
2. Judge the wet-ink lag: does the line keep up under the tip acceptably?
   - **Feels fine → close A07 UNBUILT** (I10: the native overlay is an optional
     accelerator; the web path sufficing is the architecture working as designed).
   - **Still laggy → UNBLOCK A07**, rewrite it as a full spec, build the Jetpack-Ink
     `InProgressStrokesView` native overlay (in-progress freehand only, I9).
3. Sanity-check palm rejection with a resting hand while drawing.

Record the verdict here either way (design/03 requirement). No code change lands from
this spike — it's the analysis + the framing of VLL's one attended judgment.
