# T03 — UI: one accent color + dark-mode fixes

**Priority:** 3 · **Size:** S · **Area:** `web/studio/src/styles.css` (CSS only)

## Context

The stylesheet has a token system (`:root` custom properties, a `prefers-color-scheme:
dark` override) but it has drifted, and the product visibly shows it:

1. **Two competing blues.** The chrome uses `--brand: #4f46e5` (indigo). The entire
   editor (tool buttons, layer highlights, selection boxes, swatch outlines, pills) uses
   `var(--accent, #2563eb)` — **but `--accent` is never defined anywhere**, so every one
   of those ~20 usages silently falls back to the hardcoded `#2563eb` (a different blue).
   Side-by-side, the app looks like two products.
2. **Hardcoded colors that break dark mode.** Confirmed visually: in dark mode the
   `.preset-btn` ("Outline/Box/Highlight") and `.layer-access-btn` (lock) render as white
   chips on dark surfaces because they hardcode `background: #fff`. Other offenders:
   `.editor-reject-notice` (`#fee2e2`/`#991b1b`/`#fecaca`), `.lock-pill` (`#e5e7eb`/`#374151`),
   `.focused-pill` (`#ddd6fe`/`#5b21b6`), `.edit-layer-btn` (`#fffbeb`/`#f59e0b`/`#b45309`),
   `.edit-layer-hint`, `.draw-locked-hint`, `.annotation-list-locked-hint` (`#b45309`),
   `.conn-pill.*` (raw hex fallbacks), `.shape-style` (amber rgba), `.selected-bbox.*`
   and `.selection-box` (raw rgba blues/grays/ambers), `.bbox-lock`, `.resize-handle`
   (`#fff`), the various `var(--muted, #6b7280)`-style fallbacks with wrong hexes.
3. **A third hue in the brand.** `.brand` renders the logo as an indigo→teal gradient
   (`linear-gradient(90deg, var(--brand), #0ea5a0)`), adding yet another color.

Goal: the calm, single-accent look of Google-suite tools. This task is **CSS-only** —
markup and class names do not change.

## Changes

1. Define the accent once: in `:root`, set `--accent: var(--brand);` (and in the dark
   block if needed). Then remove every literal fallback: `var(--accent, #2563eb)` →
   `var(--accent)`. One primary color everywhere.
2. Add semantic tokens for the recurring hardcoded roles and use them in both themes:
   - `--warn-bg` / `--warn-fg` (the amber "locked layer / edit-anyway" family)
   - `--selection-fill` (translucent accent used by `.selection-box`/`.selected-bbox` —
     derive with `color-mix(in srgb, var(--accent) 10%, transparent)`)
   - `--chip-bg` / `--chip-fg` (neutral pills like `.lock-pill`)
   Replace all the raw hexes/rgba listed above with these tokens; give each a dark-mode
   value that keeps contrast (spot-check with devtools emulation).
3. `.brand`: drop the gradient text; plain `color: var(--brand)`, keep the weight.
4. Optional but recommended: tone the page background down — replace the two radial
   gradient tints (`--bg-grad`) with a flat `var(--bg)` (or keep at most one *very*
   subtle tint). The current green+purple wash reads busy next to a white score page.
5. Sweep for leftovers: `grep -nE '#[0-9a-fA-F]{3,8}|rgba?\(' web/studio/src/styles.css`
   — after the change, remaining literals should only be in the `:root`/dark token
   blocks (plus genuinely one-off values like the white PDF page background, which must
   stay `#fff` because the rendered score is white in both themes).

## Acceptance criteria

- `--accent` is defined; `grep -c '2563eb' styles.css` returns 0.
- `make demo`, open the song editor (`marie`/`demo` → The Troubadours → Wonderwall) in
  **dark mode** (devtools → emulate `prefers-color-scheme: dark`): no white chips or
  light-mode islands in the toolbar, layers panel, or annotation list.
- Light mode: tool buttons, selection boxes, active-layer outline, and top-bar links all
  use the same single accent color.
- `make e2e` green (class names unchanged, so specs should be unaffected).

## Out of scope

- Layout/markup changes (T04, T05), component restructuring, new theme toggle UI.
