# T66 — Move-first/default + select-as-dashed-rect icon + compact mobile editor chrome

**Lane:** web-core (studio editor) · **Size:** M · **Status:** SPEC'd 2026-07-26 (VLL device+desktop feedback after trying T65) · **Depends on:** T65 (landed)

VLL, after trying T65 on the demo: *"move tool should be first by default and with no tool I suppose; on mobile it is now multiline with a large margin on top … adjust all; the select tool should be a dotted/dashed rectangle if possible."* Three adjustments, all in the editor toolbar/chrome.

## Part A — Move is FIRST and the default tool ("no tool" = pan)

Today the default active tool is `select` (`Viewer.tsx:141` `useState<Tool>("select")`) and the palette lists `select` first, then `move` (`Toolbar.tsx:31` prepends select). VLL wants the resting/neutral state to be **Move (pan)**:

- **Default active tool = `move`** — open the editor in pan mode (`useState<Tool>("move")`). Opening a document, the first thing you do is navigate, not select — this is the "with no tool I suppose" state.
- **Move is FIRST in the palette** (before Select). Reorder the `TOOLS` build in `Toolbar.tsx` so `move` precedes `select`; keep the rest.
- **Consequence to honor (and confirm with VLL if it feels wrong on device):** selecting/moving objects now requires explicitly picking the Select tool — the neutral state no longer selects. That matches VLL's words. Verify the existing select/marquee behavior is unchanged once Select IS active (T43 marquee, T65 dashed marquee, editor-uxfix multi-select all stay green). The draw tools are unaffected.
- Check every `tool === "select"` default-assumption site (`Viewer.tsx:808,838`; Toolbar neutral state `:326,334`) still behaves correctly when the neutral tool is `move` — e.g. `ctxShown`, clearing selection on tool change, the "no draw type selected" ctx state.

## Part B — Select tool icon = dashed/dotted rectangle

The Select button shows an arrow/cursor glyph (`SELECT_ICON`, `Toolbar.tsx:13-17`). VLL wants a **dashed (or dotted) rectangle** — the conventional marquee-select affordance, and it now matches the **already-dashed marquee** (`.selection-box`, T65 Part B) and the dashed `.selected-bbox`. Swap `SELECT_ICON` for a small SVG rectangle with a `stroke-dasharray` (dashed) outline, no fill. (Marquee rendering itself is already dashed — this is the toolbar ICON only.)

## Part C — Compact the mobile editor chrome (kill the multiline + large top margin)

On a 390px phone the editor's `.viewer-chrome.topbar-pill` wraps into **3 rows** — (1) back + song title, (2) the 8 tools, (3) zoom `− Fit width +` · Layers · Notes · Details — a **~117px pill**, and the score's `scroll-padding-top` reserves **~182px** (measured on the live T65 demo). The large top margin is a SYMPTOM: the reserve correctly tracks `--chrome-h` (the pill height, published by the ResizeObserver at `Viewer.tsx:268`, consumed by `scroll-padding-top`/padding in `styles.css` — the T59 invariant that keeps the score from hiding under the chrome). So **reduce the pill's row count on phone and the margin follows automatically.**

- **RULED 2026-07-26 (VLL, after trying the 2-row T66 build): make it ONE horizontally-scrolling row, not two.** VLL: *"I was imagining the toolbar scrolling horizontally."* His direct word supersedes the earlier ≤2-row target. Build the **pin-Back hybrid** (the lane's proposal, approved): **Back (+ the song title) stays pinned** on the left as a fixed navigation anchor; **everything else — the tools, the zoom stepper, Layers/Notes/Details — lives in ONE horizontal-scroll region** with the T65 overflow fade. One compact row (~48px + inset), even smaller than the 2-row version.
  - Accepted trade-off (VLL's ruling): zoom + Layers/Notes/Details can scroll off behind the fade — that's fine on touch because pinch-zoom and the new double-tap-zoom (Part D) cover zoom, and the fade signals more. Back never scrolls away (pinned). If, on device, the zoom stepper scrolling off feels bad, pin it too as a fast-follow — but ship pin-Back-only first.
- **The top margin still shrinks from `--chrome-h`** (the score sits just below the chrome, à la T59) — do NOT clamp the reserve below the chrome height (that re-hides the score, the T59 bug); the one-row chrome makes `--chrome-h` small and the reserve follows.
- **Invariants to keep:** the T27/T42 zero-shift (toggling the ctx bar / a live banner never moves the score — the reserve stays mode-independent), the T59 edges reachable (page-1 top / last-page bottom scroll clear of the chrome), the T65 tool row stays one scrollable line with the overflow fade (no column-wrap — the T32 HOLD), and desktop (>640px) chrome is unchanged.
- Re-screenshot phone before/after (light+dark) — the reviewer will pixel-verify the row count + margin.

## Part D — double-tap / double-click to zoom (RULED 2026-07-26, VLL: "double click can zoom? review the idiomatic double click")

Approved as idiomatic and conflict-free: double-tap-to-zoom is standard in PDF/image/map viewers, and the usual editor conflict (double-click-to-edit-an-object) doesn't apply because the editor now DEFAULTS to Move mode (Part A) where there's no object interaction.

- **Scope to Move mode ONLY.** In `tool === "move"`, a double-tap (touch) / double-click (mouse) **zooms to the tapped point, toggling Fit-width ↔ ~2×** (a second double-tap zooms back to fit). Reserve double-click in Select/draw modes for future object-editing — do NOT bind it there now.
- **Reuse the existing zoom pipeline** — the pinch path already zooms-to-point (`updateGesture(scale,dx,dy)` + the commit reconcile); drive the same commit so it re-rasters once and keeps the T27 zero-shift/re-raster-once invariant. Don't add a second zoom mechanism.
- Guard the double-tap so it doesn't fight a single-tap or a pan-drag (tap-count + small movement threshold); a pan that moved is never a double-tap.

## Out of scope
- Desktop toolbar layout (only the move-first reorder + select icon apply there; the row structure is fine).
- Double-click in Select/draw modes (reserved for future object editing — Part D is Move-mode only).
- The move-tool pan mechanics, the marquee behavior, the overflow-fade mechanism (all T65, keep).
- App/iOS native — inherited via the T46 WebView (mobile heads-up; no app work).

## Acceptance
1. e2e: the editor opens with `tool-move` active (Move is default); Move is the first palette button; picking Select then still marquee-selects (T43/T65/uxfix green). Red-first on "default tool is move".
2. Select button renders a dashed-rect icon (assert the SVG has a `stroke-dasharray`, or a snapshot); the marquee stays dashed.
3. Mobile chrome: at 390px the editor top chrome is ONE row (Back+title pinned, the rest in a single horizontal-scroll region with the T65 fade); `--chrome-h`/`scroll-padding-top` materially smaller than today's ~117/~182px (assert a concrete ceiling ~1 row); score's first page reachable (not hidden), zero-shift intact, no column-wrap at any width. Back stays visible regardless of scroll.
4. Double-tap/double-click in Move mode zooms to the point (Fit-width↔~2× toggle), re-rasters once; in Select/draw modes double-click does NOT zoom. e2e or a behavior assert on the zoom-toggle.
5. Reviewer pixel-check (light+dark): phone editor before/after (one-row scrolling bar); desktop toolbar move-first + dashed select icon.
6. `tsc`/build clean; full `editor*` e2e suite green (esp. `editor-phone-breakpoint`, `editor-touch-marquee`, `editor-ctx-thin`, `editor-t65`, `editor-wheelzoom`); no dist churn.

## Part E — REGRESSION GUARD: draw tools must stay clickable + touch-drawable (VLL: "writing tools are not clickable anymore")

VLL reported the writing (draw) tools "not clickable anymore" — on the mobile T66 chrome. I could NOT reproduce it on the deployed :8080 build (draw-tool buttons activate; rect draws — objects incremented — on both desktop and synthetic touch; `.edit-canvas` is correctly `touch-action:none`, styles.css:1050). Prime suspect: the **single-scroll toolbar (Part C)** — a horizontally-scrollable flex row can swallow a real-finger tap as a scroll-drag, so a draw-tool button "taps" but never activates on a real device (synthetic pointer events bypass this). Second suspect: the move-default (Part A) interacting with the draw-enable state.

- **MUST:** in the T66 build, verify on a real touch path that (a) tapping any draw tool in the single-scroll bar ACTIVATES it (aria-pressed flips) even when the bar is mid-scroll / the button is partially in the fade, and (b) a finger draw then creates an object. Add an e2e that taps a draw tool inside the scrolled bar and touch-draws.
- If the scroll region eats taps: give the tool buttons priority (e.g. a small drag threshold before the row scrolls, or `touch-action: pan-x` on the scroller so a tap still fires as a click) — without breaking the horizontal scroll.
- Reviewer will re-test draw-on-touch in the T66 build before GO.

## Notes for the executor
- Part A is the load-bearing behavior change (default tool) — grep all `"select"` / `isNonDraw` / neutral-state sites and make `move` the clean neutral. Part C is where the care is: the margin is downstream of `--chrome-h`, so fix the chrome height, not the reserve. Present at the gate; cite VLL 2026-07-26 (via Fable). Mobile lane: heads-up, WebView inherits it, no app work.
