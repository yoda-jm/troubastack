# T33 — Thin the contextual style pill to the main bar's height (one slim row)

**Priority:** normal (UX polish, VLL request 2026-07-10) · **Size:** S/M ·
**Area:** `web/studio` (Toolbar style part + ctx-bar CSS + spec mechanics).

## Context

Measured on main at 1280×800: the top pill is **46.7px** tall; the ctx style pill
is **96.5px** — over 2× — because every control renders as a three-layer stack
(small-caps label above · control · value below: COLOR/chip/#E11D48,
OPACITY/slider/100%, WIDTH/slider/4.0, FILL+BORDER checkboxes, BLEND/select).
VLL: "rework the ctx menu so that it is as thin as the main one — needs some
thinking on how to present all the options, but it feels possible."

## Design (decided — don't improvise)

One 46px row: **[target chip] [6 swatches + color chip] [opacity slider·value]
[width slider·value] [preset trio] [⋯]**

1. **Kill the label layer:** the small-caps labels (COLOR/OPACITY/WIDTH/BLEND)
   become `title` tooltips + `aria-label`s. Sighted users get affordance from the
   controls themselves (sliders, swatches); the labels were the main height cost.
2. **Fold values inline:** `100%` / `4.0` render as small text RIGHT of their
   slider (same line), not below. The hex value moves into the color chip's
   tooltip (and stays visible in the ⋯ popover). Keep `style-*-value` testids on
   the moved elements.
3. **Sliders stay inline** (do NOT bury them in a popover): one-tap style tweaks
   mid-annotation are pen/tablet-critical. 72px range inputs, value text beside.
4. **Presets become an icon trio** (▢ outline / ■ box / ▨ highlight) with
   `title`s — saves ~120px of button text; keep `preset-*` testids.
5. **⋯ overflow popover** (new, `data-testid="style-more"`): FILL and BORDER
   checkboxes, the BLEND select, and the hex readout. These are rare manual
   combos — presets cover the common cases. Popover = a small anchored panel
   (same pattern as the version chip popover), closes on outside-click/Esc.
   Existing `style-fill`/`style-stroke`/`style-blend` testids move INTO it
   unchanged.
6. **Text target:** font-size slider inline (same treatment as width); shape-only
   controls collapse (`style-slot-off` already handles this).

## Constraints (load-bearing)

- **Assertion freeze:** specs that FILL `style-opacity`/`style-width`/
  `style-blend`... may need a new *mechanic* (open ⋯ first for the popover trio) —
  allowed; `expect(...)` lines stay untouched.
- The ctx pill keeps: zero-shift (absolute float), the glass pass-through
  (`pointer-events` dance), `overflow-x: auto`, and the <600px sheet behavior
  from `f90a7ca`.
- Both themes; the reskin's token/idiom language (pill, hairline borders).

## Acceptance criteria

- **The number:** `ctx-bar` rendered height ≤ `topbar-pill` height + 2px, for
  BOTH a shape target and a text target — as a committed e2e assertion (extend
  `editor-phone-breakpoint.spec.ts` or a small new spec; also assert the ⋯
  popover opens and `style-blend` works inside it).
- Full editor e2e suite green (mechanic edits only — list them in the PR).
- `tsc -b studio` clean; pixels reviewed light + dark at the gate.

## Out of scope

- Reworking the TOP bar or drawer; changing any style semantics/defaults;
  mobile-specific gesture work.
