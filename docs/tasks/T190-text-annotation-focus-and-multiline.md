# T190 — The text annotation box: typing goes in straight away, and a note can have several lines

**Lane:** web-core · **Status:** specced 2026-10-10, not started. Small. · **Origin:** VLL, 2026-10-10:
*"when choosing text in studio, a input text popup shows, could be nice if it can have focus directly so we
can directly type"*, then *"also maybe a textarea is nice for multiline"*.

## 1. Focus: tap or click with the Text tool, then type

**Today:** with the Text tool, a click on the page opens the "Text annotation" prompt (`EditCanvas.tsx`,
`tool === "text"` → `prompt(…)`), but the keyboard does not go into its field. VLL has to click the field
before typing.

**Likely cause, a hypothesis to confirm, not a prescription.** `Dialog.tsx` does focus the input in its mount
effect. But the prompt is opened from the canvas's **pointerdown**, and the browser's default action for the
same gesture's **mousedown** (it moves focus to the pressed point, here a non-focusable canvas, so the body)
runs *after* that focus and takes it away. Confirm it before you fix it: log `document.activeElement` right
after the prompt opens and again after the click completes.

**Property:** after the click (mouse) or tap (touch) that opens the prompt, `document.activeElement` **is**
the prompt's text field, and keystrokes land in it. The mechanism is yours, for example deferring the focus
until after the gesture, or opening on `pointerup`. **Constraints:**
- **Keep T101's guard** (the compatibility mousedown after a touch must not cancel the prompt). Its long
  comment in `Dialog.tsx` explains the timing; do not regress it.
- **Keep T90:** the text tool disarms after the prompt resolves.
- **A shared fix in `Dialog.tsx` is welcome** if it is the right place: every `prompt()` (tag rename, song
  details) gets the same benefit. Check that those callers still behave.
- On a phone, focusing the field should raise the on-screen keyboard. Check it on Android Chrome or the app's
  Studio WebView. If a browser refuses to raise it outside a user gesture, say which one; that is acceptable.

## 2. Several lines

**⟨D1⟩ The text annotation prompt uses a multi-line field** (`textarea`, about 3 rows, growing to about 8).
- **Enter makes a new line.** **Ctrl+Enter / Cmd+Enter adds** the annotation, like the **Add** button.
  **Escape cancels.** Show the shortcut as a hint under the field (*Ctrl+Enter to add*, using *⌘* on a Mac).
- **Other `prompt()` callers stay single-line.** A tag name or a song title has no business holding a newline.
  Make multi-line an option of `prompt()` (for example `multiline: true`), not a change to every caller.
- Trim the text as today, but **keep inner newlines**. Collapse three or more consecutive newlines to two, so a
  slip on Enter doesn't make a tall empty box.
- **Editing an existing text annotation**, if Studio offers it anywhere, uses the same multi-line field.

**⟨D2⟩ Multi-line text renders as lines, everywhere.** `web/ink`'s `drawText` makes a single `fillText` call
today, so a newline would not render as a line break. `textBBox` (`editor.ts`) **already** splits on `\n`, at
a 1.2 line height. **Make `drawText` agree with it:** split on `\n` and step each line by `fontPx × 1.2`,
which is exactly the box the selection outline already draws. One renderer serves Studio and the bake
(`web/bake` draws through `web/ink`), so the baked PDF and the stage page get the same lines. **Stage needs no
change**: it shows the baked raster.
- **The I8 parity test** (bake vs browser, the bundled font) gains a **multi-line text vector**, so the bake
  and the browser are proven to break the lines identically.

## 3. Not in this task

- Rich text, alignment, or automatic wrapping to a width. A line breaks only where the writer pressed Enter.
- Changing the text tool's one-shot behaviour (T90).

## 4. Acceptance

**e2e (studio):**
- **Focus:** select the Text tool, click the page, then `keyboard.type("hello")` **without clicking the
  field**, then Ctrl+Enter. A text object `hello` exists (assert the stored object, not just the DOM). Run it
  for both a mouse click and a touch tap (`hasTouch` context, `tap()`).
- **Teeth for focus:** remove the fix. The typed-without-clicking test goes **red**: the text lands nowhere
  and no object is created. Print what you removed.
- **Multi-line:** type `line one`, Enter, `line two`, then Ctrl+Enter. The stored text is
  `"line one\nline two"`. The selection box is two lines tall. A pixel probe finds ink on both lines' rows and
  none in a third row below.
- **Enter alone** does not submit the multi-line prompt. **Enter** in the tag-rename prompt still submits it.
  This proves the other callers stayed single-line.
- **T101 still holds:** the existing T101 test passes unchanged.

**Unit / parity:**
- `drawText` with `"a\nb"` draws two `fillText` calls at `y` and `y + 1.2·fontPx`. Use a recording ctx; the
  test is pure.
- The I8 parity harness passes with the new multi-line vector.

**Regression:** studio typecheck, unit suite, and the existing e2e for text, tags and song details are green.
