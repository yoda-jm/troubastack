# BRAND06 — fix the ACCENT table, and outline the wordmarks

**Lane:** web-core (the brand system is `docs/brand/`, stdlib Python).
**Status:** **CODE LANDED** — 3 commit(s), `a1b6e38d`…`72735804` (last 2026-09-03). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".

## 1. ACCENT becomes a pair per mark, one per ground

`build.py:151` holds **one** accent per mark:

```python
ACCENT = {"troubastack": "#5A6674", "troubastudio": "#D62A8A",
          "troubastage": "#C8912A", "troubacore": "#1769D1"}
```

…but `WORDMARK_TEMPLATE` takes a `{ground}`, so the same accent is painted on more than one
background. Measured, it does not survive that:

| mark | accent | on the dark tile | on paper |
|---|---|---|---|
| Stack | `#5A6674` | **2.43** | 5.85 |
| Stage | `#C8912A` | 5.11 | **2.65** |

Against a 3:1 large-text bar, each fails on one ground. The project page hit this and had to
correct it by hand — `#AEBAC6` for Stack on dark, `#936B1F` for layer 1 on paper. **Those hand
corrections are the evidence that the table is wrong, not that the page was special.**

**A comment in that file used to claim the accents held on both grounds. That claim was mine and
it was false.** This task removes the cause, not just the sentence.

### Work

- `ACCENT` becomes `{mark: {"dark": …, "paper": …}}`; the wordmark generator selects by the
  `ground` it is rendering.
- **Derive each value the same way the page did**: same hue and saturation as the mark's own
  colour, moved in lightness only until it clears the bar on that ground. Keep the measured ratio
  in a comment beside each value — measure once, record it, do not re-measure at build time
  (`build.py` stays stdlib-only and deterministic).
- **Propagate to everything generated**, per VLL: the wordmark SVGs, the family sheet
  (`sheet.py`), and any raster ladder that shows a wordmark. A corrected table that the sheet does
  not reflect is a table nobody will trust.
- **Guard with teeth**: `sheet.py` already refuses to build when a swatch names a colour nothing
  draws. Add the companion check — **every ACCENT value must clear its ground's bar** — and
  teeth-check it by putting `#5A6674` back as Stack's dark accent and confirming the build fails.
  A guard proven only by passing is not proven.
- Update `docs/brand/README.md`'s palette section to describe pairs.

## 2. Wordmarks: outline once, commit the paths

**Decided: outline once and commit, NOT at build time.** The repo already does exactly this —
`src/monogram.svg` is *"TS as paths — no font dependency"*.

Outlining inside `build.py` would need a font engine, which breaks two properties that were
settled deliberately in BRAND01: `build.py` is **stdlib-only**, and its output is
**deterministic**. Worse, the glyph outlines would depend on whichever Inter is installed, so
CI's regeneration guard would produce a different diff on a different machine — the exact failure
mode that ruling exists to prevent.

### Work

- Outline the wordmark text (product name + tagline) once, at whatever size the template uses,
  and commit the path data as a brick.
- **Keep the source string and the recipe beside the paths** — which font, which weight, which
  size, which tool — so the next person can redo it rather than reverse-engineer it.
- `WORDMARK_TEMPLATE` stops emitting `<text>` and `FONT` stops being consulted for the lockup.
- **State the cost in the README**: the wordmark text is now frozen. Changing a product name or a
  tagline means re-outlining, and that is a manual step by design.

## Done when

- No `<text>` element remains in any generated wordmark.
- A wordmark renders identically on a machine without Inter installed — check by rendering with
  the font uninstalled or with a deliberately broken font path, not by trusting the change.
- Every ACCENT value clears its ground's bar, with the measured ratio recorded beside it.
- Reverting one accent to its old value makes the build **fail**.
- `build.py` still imports nothing outside the standard library, and two consecutive runs produce
  byte-identical output.
