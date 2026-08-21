# T90 — The text tool traps you: every tap is another modal prompt

**Priority:** high — the editor becomes unusable until you hit one small button · **Size:** S ·
**Area:** `web/studio` (`pages/song-editor/WetCanvas.tsx`, `Viewer.tsx`, e2e). Lane: Web & Core.

VLL, 2026-08-21, using the studio in a mobile browser: *"when inserting a text I cannot click anywhere
(exit song, change tools, …) without creating another text"*.

## The bug, pinned

`WetCanvas.tsx:719–723`, on **pointerdown**:

```ts
if (tool === "text") {
  const text = window.prompt("Text annotation");
  if (text && text.trim()) onCommitDraw("text", page, [pt], text.trim());
  return;
}
```

`commitDraw` selects the new object (`setSelectedUuids([obj.uuid])`) but **never changes the tool**.
So the text tool stays armed forever, and *every* subsequent tap anywhere on the canvas opens another
blocking `window.prompt`. The canvas is everything that is not a small floating pill, so on a phone
that is effectively the whole screen.

The chrome is not actually blocked — `.edit-canvas` has no `z-index` and sits under
`.viewer-chrome` (`z-index: 8`), so the buttons do work. But `window.prompt` is **modal**: while it is
up nothing else is tappable, and dismissing it puts you one stray tap away from the next one. The
lived experience is exactly what VLL describes.

### The part that makes this worse than an annoyance

Chrome and Firefox offer **"Prevent this page from creating additional dialogs"** after several
dialogs in a row — which is precisely the state this bug drives the user into. Once ticked,
`window.prompt` returns `null` for the rest of the page load, and the guard above (`if (text && …)`)
**silently returns**. The text tool then does nothing at all, with no message, no notice, no console
error — and the user has no idea why. So the trap can escalate into a silently dead tool, and the
"fix" is a page reload that nobody would guess at.

## The fix

**Make text a one-shot placement tool: after the prompt resolves, return the active tool to
`select`.** Arm it, place one text, it disarms. That is how annotation tools generally behave for
text, and it removes the trap completely — the next tap selects or deselects instead of re-prompting.

**Revert on cancel too, not just on commit.** This is a judgement call and I want it recorded: if
cancel left the tool armed, two consecutive mis-taps would still trap the user, which is the whole
complaint. The cost is one extra tap on the tool button when someone genuinely wants to place two
texts in a row; the benefit is that the dead end becomes impossible. If VLL would rather keep the tool
armed after a *cancel*, say so and I will amend.

Plumb it however is cleanest — `onCommitDraw` already flows back to `Viewer`, so a `setTool("select")`
there (plus one on the cancel path) is likely enough. Do **not** special-case it inside `WetCanvas` if
that means duplicating tool state.

## Acceptance criteria

- After placing a text, the active tool is **`select`** — assert both the tool button's pressed state
  **and** the behaviour: a second tap on the canvas opens **no** prompt and creates **no** object
  (assert `object-count` is unchanged). The behavioural half is the one that matters; a button state
  alone would pass even if the canvas handler still fired.
- **Cancelling** the prompt also returns to `select`, and creates nothing.
- The text that was placed is still selected and still correct (the existing commit path is unchanged).
- e2e drives the prompt via Playwright's dialog handling (`page.on("dialog", …)`), covering both
  accept-with-text and dismiss.
- **Teeth-check:** remove the revert and confirm the "second tap creates nothing" test goes red — and
  that it is the only one.
- `tsc -b studio` clean; **full `make e2e`** on isolated ports.

## Out of scope — but read this

- **The icon/stamp tool stays repeat-armed.** T51 made it a palette you stamp from repeatedly, it opens
  no modal, and nobody has complained. Do not "consistency-fix" it into a one-shot.
- **Replacing `window.prompt` with an inline text editor is a separate task**, and it is the real cure
  for the dialog-suppression hazard above. It needs a positioned input over the canvas, mobile keyboard
  handling, and a commit/cancel affordance — too much to bolt onto this fix. **File it, do not build
  it here.** If you think the suppression hazard makes it urgent, say so at the gate and I will
  prioritise it rather than you widening this task.
