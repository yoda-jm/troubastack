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
