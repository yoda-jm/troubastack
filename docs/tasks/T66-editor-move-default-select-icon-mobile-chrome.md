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

- Target: the phone editor chrome is **≤ 2 rows** (ideally: row 1 = back + title + a compact actions cluster; row 2 = the tools, already single-row-scrollable from T65). Fold zoom + Layers/Notes/Details into something compact on phone — e.g. a single right-aligned actions row shared with the title, or a small overflow/"⋯" menu for Layers/Notes/Details with the zoom stepper kept inline. Executor's layout call; the requirement is fewer rows.
- **The top margin must shrink materially** (the score sits just below the chrome, à la T59) — do NOT just clamp the reserve smaller than the chrome (that re-hides the score, the T59 bug); shrink the CHROME so the reserve (which tracks `--chrome-h`) shrinks with it.
- **Invariants to keep:** the T27/T42 zero-shift (toggling the ctx bar / a live banner never moves the score — the reserve stays mode-independent), the T59 edges reachable (page-1 top / last-page bottom scroll clear of the chrome), the T65 tool row stays one scrollable line with the overflow fade (no column-wrap — the T32 HOLD), and desktop (>640px) chrome is unchanged.
- Re-screenshot phone before/after (light+dark) — the reviewer will pixel-verify the row count + margin.

## Out of scope
- Desktop toolbar layout (only the move-first reorder + select icon apply there; the row structure is fine).
- The move-tool pan mechanics, the marquee behavior, the overflow-fade mechanism (all T65, keep).
- App/iOS native — inherited via the T46 WebView (mobile heads-up; no app work).

## Acceptance
1. e2e: the editor opens with `tool-move` active (Move is default); Move is the first palette button; picking Select then still marquee-selects (T43/T65/uxfix green). Red-first on "default tool is move".
2. Select button renders a dashed-rect icon (assert the SVG has a `stroke-dasharray`, or a snapshot); the marquee stays dashed.
3. Mobile chrome: at 390px the editor top chrome is ≤ 2 rows and `--chrome-h` / the reserved `scroll-padding-top` are materially smaller than today's ~117/~182px (assert a concrete ceiling, e.g. chrome ≤ ~2 rows tall), with the score's first page reachable (not hidden) and zero-shift intact. No column-wrap at any width.
4. Reviewer pixel-check (light+dark): phone editor before/after; desktop toolbar move-first + dashed select icon.
5. `tsc`/build clean; full `editor*` e2e suite green (esp. `editor-phone-breakpoint`, `editor-touch-marquee`, `editor-ctx-thin`, `editor-t65`); no dist churn.

## Notes for the executor
- Part A is the load-bearing behavior change (default tool) — grep all `"select"` / `isNonDraw` / neutral-state sites and make `move` the clean neutral. Part C is where the care is: the margin is downstream of `--chrome-h`, so fix the chrome height, not the reserve. Present at the gate; cite VLL 2026-07-26 (via Fable). Mobile lane: heads-up, WebView inherits it, no app work.
