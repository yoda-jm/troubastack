# T39 — Rich pseudo-md source editor for the chart dialect (syntax highlighting)

**Priority:** normal (VLL, 2026-07-12) · **Size:** S/M · **Area:** `web/studio`
(the T19/T25 ChartEditor source pane) + e2e.

## Decision (RE-SPECCED 2026-07-12 — VLL's settled intent, superseding the earlier live-preview reading)

VLL's "edit the pseudo-md in preview mode" reconciled to: make the SOURCE side a
**rich editor** — syntax highlighting for the chart dialect, source stays
primary. **This is NOT live preview**: the earlier T39 (debounced auto-render)
is superseded, and **T25's on-demand preview decision STANDS** (no reversal — the
Preview button keeps rendering on click). WYSIWYG-on-the-PDF stays declined.

## Dependency ruling (the lane's open question): NO new editor library — custom highlighter

The dialect is **tiny and decoration-only** (`# title`, `## Section`, chord-only
lines, `**bold**`, plain text — ~4 token types, no autocomplete/folding/LSP).
CodeMirror (a real dependency + bundle weight) is disproportionate and cuts
against studio's deliberate minimal-dep posture (no-workspaces; ink from source).

**Build a lightweight highlighter over the existing textarea** using the standard
**overlay technique**: a highlighted `<pre>` layer positioned exactly behind a
transparent-text `<textarea>` (caret + editing from the textarea; color from the
`<pre>`). **Make the source pane MONOSPACE** — the chart is chords-over-words, so
monospace is correct anyway, AND fixed-width glyphs make overlay alignment
reliable (the overlay technique's one real failure mode — misaligned highlight —
is a non-issue at a fixed advance width).

Overlay requirements (the gotchas, as hard requirements — get these right or the
highlight drifts):
- textarea and `<pre>` share identical `font`, `line-height`, `padding`,
  `white-space` (`pre-wrap`), `word-break`, and box size; the `<pre>` scrolls in
  lockstep with the textarea (`onScroll` → mirror `scrollTop`/`scrollLeft`).
- The textarea text is transparent (`color: transparent`) with a visible caret
  (`caret-color`), sitting ABOVE the `<pre>` (which is `aria-hidden`).
- Highlighting is a pure `tokenizeChartLine(line) → spans` per line (re-uses the
  dialect rules already in `chartpdf`/the renderer's mental model: a line whose
  tokens are all chords → chord color; `##`/`#` → heading color; `**bold**`
  inline; else plain). Keep it a pure function, unit-shaped (studio e2e covers
  it, but structure it so the token rules are testable).
- Tab/newline/resize behave exactly as the plain textarea did (don't regress
  editing); the `chart-source` testid stays ON the textarea (specs type into it).

**Fallback (honest):** if monospace + overlay still can't hold alignment for the
dialect in practice, STOP and come back to the gate before adding CodeMirror —
don't silently pull in the dependency.

## Tests

- e2e: type dialect source into `chart-source` → the highlight overlay shows the
  expected token classes (assert on the `<pre>`'s spans: a `## Chorus` line gets
  the heading class, a chord-only line gets the chord class, a lyric line plain).
  Editing still works (type/delete/select round-trips through the textarea).
  Preview still renders ON DEMAND via the button (unchanged — assert it still
  works, and that NO auto-render happens on typing — the anti-regression of the
  reverted live-preview idea).
- The token function's rules covered (heading / chord-line / bold / plain).

## Acceptance criteria

- Source pane is a monospace highlighted editor; token classes correct; editing
  unregressed; **preview stays on-demand** (no auto-render on type); no new
  runtime dependency added to `web/studio/package.json`; full suite green;
  `tsc -b studio` clean; pixels at the gate (highlighted source, both themes).

## Out of scope

- Live/debounced auto-preview (superseded — T25 on-demand stands); WYSIWYG on the
  PDF; CodeMirror/Monaco/any editor lib (ruled out above — flag before adding);
  autocomplete, folding, linting; changing the chart dialect or the renderer.
