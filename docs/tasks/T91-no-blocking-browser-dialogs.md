# T91 — Replace the studio's blocking browser dialogs (they can silently disable the app)

**Priority:** high for **Part A**, normal for Part B · **Size:** M · **Area:** `web/studio`
(9 call sites, a shared dialog, e2e + walkthrough). Lane: Web & Core. **After T90**, which touches
the same line in `WetCanvas.tsx`.

Filed by me rather than left to the lane: I found the hazard while speccing T90 and it is wider than
that task, so it deserves its own rationale.

## Why this is not cosmetic

The studio calls a **blocking browser dialog in nine places**:

| kind | where |
|---|---|
| `window.prompt` | `WetCanvas.tsx:721` (text annotation), `SongDetails.tsx:272` (rename file) |
| `window.confirm` | `SongDetails.tsx:284` (delete file), `:1016` (delete song), `SetlistDetail.tsx:985` (delete setlist), `BandSettings.tsx:173` (remove member), `:187` (leave band), `:388` (delete band) |

Chrome and Firefox offer **"Prevent this page from creating additional dialogs"** after a few dialogs
in a row. That checkbox is not exotic — the T90 text-tool trap drives a user straight into it, and any
session doing a batch of deletes gets offered it too.

**Once it is ticked, for the rest of the page load:**

- `window.confirm()` returns **`false`** — and every one of those seven sites reads
  `if (!window.confirm(…)) return;`, so **every destructive action silently does nothing**;
- `window.prompt()` returns **`null`** — so rename and text-annotation silently do nothing.

No message, no console error, no visible difference from a broken app. The user clicks Delete, nothing
happens, clicks again, nothing happens. The only cure is a page reload that nobody would guess at.
This is a **T30 violation** (`no silent ink` — a user action must never vanish without saying why),
introduced accidentally through a browser affordance rather than through our code.

Secondary reasons, which alone would not justify the work but come free with it: native dialogs ignore
the theme entirely (they will look wrong beside A36's palette), they cannot carry context the way
`DeleteLayerDialog` does (T83 names the object count before you delete a layer), and on a phone they
are especially jarring — which is where VLL is hitting the product now.

## The shape

T83 already established the pattern with **`DeleteLayerDialog`** (`SidePanels.tsx:262`) — an in-app,
themed, context-carrying dialog. It is purpose-built, not generic. **Generalise it into one reusable
confirm/prompt** and adopt it at the nine sites.

- **Do not regress T83.** The layer dialog's hard-confirm behaviour (naming the object count, and the
  stronger confirm for a mandatory layer) is specified behaviour — it either keeps its bespoke dialog
  or the generic one must express those cases.
- Keep the copy honest and unchanged in meaning: the delete-song confirm says *"This cannot be
  undone"* and that must stay true (it is a different claim from a layer delete, which I5 keeps
  recoverable via `SnapshotAt` — do not copy-paste the wrong sentence between them).
- Escape and outside-click cancel; the destructive action is never the default focus.

### Part A (high) — the two `prompt`s

`WetCanvas.tsx:721` and `SongDetails.tsx:272`. These are the acute ones: text annotation is where VLL
hit the trap, and both silently no-op under suppression. **Land after T90**, which makes the text tool
one-shot — the two changes touch the same handler and would collide.

### Part B (normal) — the seven `confirm`s

Higher blast radius, lower urgency. Landing Part A first is fine and encouraged.

## The sweep this needs — and it is the T87 lesson again

Replacing a native dialog with in-app UI **breaks every test that drives the native one**, and no
`data-testid` is removed, so the dangling-testid sweep will not catch it. Before presenting, grep and
update:

- `web/studio/e2e/` — **3 call sites across 2 files** register `page.on("dialog", …)`;
- `web/studio/walkthrough/` — **2 more**.

Also note Playwright **auto-dismisses** dialogs it has no handler for. That means any destructive path
without an explicit handler is currently exercising the *cancel* branch — so some of these paths may
never have been tested at all. Check before assuming a green suite means the new dialog works.

## Acceptance criteria

- Every site in the table above uses the in-app dialog; **`grep -rn "window\.\(prompt\|confirm\|alert\)" web/studio/src` returns nothing** (make that the assertion — it is checkable and it is the real goal).
- **The suppression scenario is covered by a test.** Simulate it — stub `window.confirm`/`window.prompt`
  to return `false`/`null` and assert the app no longer depends on them (i.e. the flows still work,
  because nothing calls them any more). This is the regression that motivated the task; it must not
  rest on the grep alone.
- Confirm and cancel both asserted for at least one destructive flow per file touched.
- T83's layer-delete behaviour unchanged — object count still named, mandatory-layer confirm still
  harder. Re-run its tests specifically.
- Escape and outside-click cancel; destructive action not default-focused.
- e2e + walkthrough dialog handlers updated (5 known call sites); **full `make e2e`** on isolated
  ports; `tsc -b studio` clean; dangling-testid sweep.
- **Teeth-check:** revert one adopted site to `window.confirm` and confirm the suppression test goes
  red — and only that one.

## Out of scope

- `BakeDialog.tsx`'s local `confirm()` function — that is a normal in-app handler that happens to
  share the name, not a browser dialog. Leave it.
- Redesigning any of the flows themselves; this is a mechanism swap, not a UX rework.
- Toast/undo for destructive actions. Worth having, separate task, do not fold it in.
