# T65 — Editor: move/pan tool + dashed marquee + scrollable tool row

**Lane:** web-core (studio editor + ink; inherited by the app WebView — no app work) · **Size:** M · **Status:** SPEC'd 2026-07-26 (VLL: a move tool that pans "like the double touch"; a dashed rectangle for select; on mobile let too-many tools slide with an overflow indication) · **Depends on:** nothing open

Three cohesive editor-UX changes (all in the same toolbar/canvas files). Part A adds an
8th tool button, which is exactly what motivates Part C.

## Part A — a Move (pan) tool

VLL: *"a move tool that moves across the pdf like the double touch."* Add an explicit
tool that pans the document with a single pointer drag (mouse or one finger) — doing
what the two-finger gesture already does, without needing two fingers.

- **Reuse the existing pan pipeline — NO new pan math.** One-finger pan already exists
  for draw tools under palm-rejection: `WetCanvas.tsx:610-623` begins it and
  `:733-735` applies `updateGesture(1, dx, dy)` (scale 1, pan only), committed at
  `:804-806`; the apply fn is `usePdfDocument.ts:573-581` (sets the content transform,
  reconciled to real scroll offset on settle at `:463-465`). The Move tool drives that
  SAME path, unconditionally, whenever `tool === "move"` and a single pointer drags —
  drop the `doPan = tool!=="select" && penSeenRef` gate (`WetCanvas.tsx:610`) for the
  move case (no pen-seen requirement; mouse drag pans too).
- **Move is a THIRD tool category — it draws nothing.** The type model assumes
  non-`select` ⇒ a draw tool (`Tool` union `editor.ts:25`; `DrawTool = Exclude<Tool,
  "select">` `editor.ts:28`; `toolObjectType` `editor.ts:31`). Add `"move"` and make it,
  like `select`, NOT a `DrawTool` and NOT in `toolObjectType` — the draw dispatch and
  `pickAt` (`editor.ts:462-500`) must treat `move` as non-drawing (no object created, no
  select/marquee). Cleanest: `type NonDrawTool = "select" | "move"` and gate the draw
  path on `!isNonDraw(tool)`.
- **Toolbar:** add the button in `Toolbar.tsx` (TOOLS array `:19-28`, render `:233-266`);
  place it right after `select` (the two non-drawing tools together), testid
  `tool-move`, a hand/grab icon (new SVG, same style as the others). Cursor: `grab`
  idle / `grabbing` while dragging (a `cursorClass` per `ToolDef` `types.ts:41`).
- **Two-finger pan/pinch is untouched** in every tool (`WetCanvas.tsx:567-593`,
  `:715-731`) — Move just makes single-pointer pan always-on in its own mode.
- **Desktop: show the tool there too — VLL RULED 2026-07-26** (was the one open product
  choice; now decided). The move button appears in the desktop toolbar (right after
  Select) exactly as on mobile — not desktop-suppressed. Rationale: consistency across the
  app WebView + discoverability (desktop drag-to-pan is native-scroll only today and
  non-obvious). So a desktop mouse drag with Move active pans the document, same as one
  finger on touch. (Reviewer pixel-checks the desktop toolbar + move cursor at the gate.)

## Part B — dashed marquee rectangle

VLL: *"for the select I would prefer a dashed rectangle."* The marquee (drag-select
box) is a DOM div `.selection-box` (`WetCanvas.tsx:962-973`) styled **solid** at
`styles.css:929-933` (`border: 1.5px solid var(--accent)`). Change it to **dashed** —
and note the already-selected-object box `.selected-bbox` (`styles.css:923-928`) is
ALREADY `1.5px dashed`, so this is a one-line change that also makes the two consistent.
Purely visual: the marquee geometry/selection behavior (`WetCanvas.tsx:678-684,786-787,
857-861`; `editor.ts:521-544`) is untouched.

## Part C — scrollable tool row with an overflow indication (mobile)

VLL: *"on mobile if there is too much tools they can slide with an overflow indication."*
Today the fullscreen tool row `.topbar-pill .tool-palette` is deliberately
`flex-wrap:nowrap` (`styles.css:355-361` — wrapping degenerates into a floating column,
the T32 HOLD), so on a narrow phone it just **overflows/clips** past the right edge —
tools become unreachable. With Part A adding an 8th button this bites now.

- Make the tool row **horizontally scrollable** at narrow widths: keep `nowrap`, add
  `overflow-x: auto` + hidden scrollbar — the SAME pattern `.style-controls` already uses
  (`styles.css:648-658`: `flex-wrap:nowrap; overflow-x:auto; scrollbar-width:none`). So
  the row scrolls instead of clipping.
- **Overflow indication (the ask):** an edge affordance — a fade/gradient (e.g.
  `mask-image` linear-gradient, or a small chevron) at whichever edge has more content —
  shown ONLY when the row actually overflows/can scroll (not a permanent gradient on a
  row that fits). A tiny scroll-position check toggling a class is fine; keep it
  dependency-free. Apply to the tool row; **also add it to `.style-controls`**, which
  scrolls today with no indication (same affordance, one implementation).
- Respect the existing phone block (`styles.css:771-814`, `@media max-width:600px`) and
  the `flex-wrap:wrap` on the enclosing `.viewer-chrome.topbar-pill` (`:789`) — the tool
  row stays one scrollable line within the wrapping bar; do not reintroduce column-wrap.

## Out of scope
- Any change to two-finger pan/pinch, wheel-zoom, or the draw tools themselves.
- App/iOS native work — the app gets this free through the T46 WebView editor (mobile
  heads-up only; nothing to build app-side).
- Reordering/removing existing tools; a tool-overflow "…more" popover (scroll is the ask).

## Acceptance
1. e2e (studio): **Move tool** — activate `tool-move`, single-pointer drag on the canvas
   pans the document (scroll offset changes) and creates NO object; two-finger pan still
   works in other tools (keep `editor-touch.spec`/`editor-touch-stucknav.spec` green);
   select-mode one-finger marquee (T43 `editor-touch-marquee.spec`) still selects (Move
   didn't break it). Red-first: the pan-on-single-drag assertion fails before Part A.
2. **Dashed marquee** — assert `.selection-box` computed `border-style: dashed` (was
   solid); the marquee still selects intersecting objects (`editor-uxfix.spec` multi-select
   stays green).
3. **Overflow** — at a narrow viewport with the full tool set, the tool row is
   scrollable (scrollWidth > clientWidth, no clip) and the overflow indicator is present;
   at a wide viewport it's absent. No column-wrap at any width (the T32 guard —
   `editor-phone-breakpoint.spec` stays green).
4. Reviewer pixel-checks: the move-tool cursor, the dashed marquee, and the phone tool
   row with the overflow fade (light + dark).
5. `go`/tsc/build clean; full `editor*` e2e suite green; no dist churn.

## Notes for the executor
- The whole point of Part A is to reuse `updateGesture`/`beginGesture`/`endGesture` +
  the commit reconcile — do NOT write a second pan path (it would drift from the gesture).
- `move` must be excluded everywhere `select` is excluded from drawing — grep the
  `tool === "select"` / `DrawTool` sites and handle `move` alongside.
- Present at the gate; cite the work order (VLL 2026-07-26, relayed via Fable). Mobile
  lane: heads-up only — the WebView editor gains a move tool + scrollable tools; no app work.
