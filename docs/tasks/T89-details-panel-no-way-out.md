# T89 — The Details panel has no way out on a phone

**Priority:** high — a dead end in the editor, reported from real use · **Size:** S ·
**Area:** `web/studio` (`pages/song-editor/Viewer.tsx`, `styles.css`, e2e). Lane: Web & Core.

VLL, 2026-08-21, using the studio in a mobile browser: *"when the details is open I don't know how to
close it or something else"*.

## The bug, pinned

`setEditorOpen(false)` is reachable from **exactly one control in the entire app**:

| `Viewer.tsx` | what it does |
|---|---|
| `:1091` | `onClick={() => setEditorOpen((o) => !o)}` — the top-bar Details pill, **the only closer** |
| `:1159` | `setEditorOpen(true)` — open only |
| `:1382` | `setEditorOpen(true)` — open only |

There is **no `✕` inside the panel, no Escape handler** (`grep Escape Viewer.tsx` → zero matches) and
**no outside-click / scrim close**. So the single exit is that one pill — and on a phone that pill is
the worst possible target:

- `.topbar-pill .pill-label { display: none }` at ≤600px (`styles.css:883`), so it is an **unlabelled
  icon** — three dots and lines that read as a generic menu glyph, not "close this";
- it lives inside `.tb-scroll`, a **horizontally scrolling strip**, so it can simply be **scrolled out
  of view**;
- meanwhile `.details-panel` is `position: absolute; z-index: 9` at `top: calc(chrome-h + .9rem)` and
  `width: min(680px, 100% - 1.75rem)` — on a phone it covers essentially the whole screen below the
  bar, so the user is staring at the panel and not at the strip that dismisses it.

Nothing here is broken code; it is a missing affordance, and it produces a genuine dead end.

## The fix

1. **A `✕` close button inside the panel.** Put it in the `.details-tabs` row, which is already
   `position: sticky; top: -0.4rem` — so it stays visible while the panel body scrolls. Right-aligned;
   note `.details-tab-admin` currently takes `margin-left: auto`, so the close button needs to sit
   after it (or the tabs row needs a spacer) — check both the admin and non-admin cases.
   `aria-label="Close details"`, `data-testid="details-close"`.
2. **Escape closes it.** A `document` keydown listener while open (same shape T87 landed for
   `RowMenu`).
3. **A click outside the panel closes it.** The panel is effectively modal on a phone, and every other
   overlay in the editor already dismisses this way.

Keep the pill as a toggle — it still reads correctly on desktop where the label is visible.

**Do not add a full-screen scrim.** The panel deliberately floats over the score rather than blocking
it, and a scrim would also swallow the first tap meant for the page.

## Acceptance criteria

- **At a phone viewport** (≈390×844): open Details, then close it three ways — the `✕`, Escape, and a
  click outside. Each asserted separately.
- **The `✕` survives scrolling the panel body.** Scroll the panel to the bottom (the Band tab is long)
  and assert the close button is still in the viewport. This is the actual failure mode — a close
  button that scrolls away is the same bug in a new place.
- Works on **both** the admin and non-admin tab layouts (the `margin-left: auto` interaction).
- Desktop behaviour unchanged: the pill still toggles, the panel still floats over the score.
- **Teeth-check:** remove the `✕` handler and confirm the close-by-✕ test goes red, and only that one.
- `tsc -b studio` clean; **full `make e2e`** on isolated ports; dangling-testid sweep.

## Out of scope

- Redesigning the Details panel or its tabs.
- The separate `.details-section` collapsible on the non-fullscreen song page (`SongDetails.tsx`'s
  `Details`) — that one has a visible labelled `▾ Title` toggle at its top and is not the surface VLL
  hit. Leave it alone.
