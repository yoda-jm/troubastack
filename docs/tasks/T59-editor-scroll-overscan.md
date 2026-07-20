# T59 — Editor scroll overscan: free the first page's top + last page's bottom from the chrome

**Priority:** high (VLL 2026-07-20, hits all three surfaces: desktop, mobile
browser, app WebView — one web fix covers all) · **Size:** S · **Area:**
`web/studio` viewer scroll container CSS (+ possibly the fit math). Web-core.

## The bug (VLL, verbatim intent)

The floating chrome (top pill/toolbar; bottom ctx bar) overlays the score, and
the scroll range ends AT the page edges — so the top of the FIRST page and the
bottom of the LAST page can never be scrolled out from under the chrome. "We
should be able to scroll a little bit more up and down so that they are on top
of/before the first page and after the last."

## Fix (ruled shape)

**Constant scroll overscan** on the viewer scroll container:
- `padding-top` ≥ the full top-chrome height (the floating pill, incl. its
  live-banner-shifted variant),
- `padding-bottom` ≥ the bottom chrome height (T42 already reserves the ctx-bar
  band — extend/verify it covers the WHOLE bottom overlay, always, not just some
  modes).
Constant padding (mode-independent) so zero-shift holds — this is the T42
approach completed for both ends. If fit-height math consumes the padding,
adjust the fit computation accordingly (fit still fills the viewport; the
overscan is scroll RANGE, not page size).

## Acceptance

- **The un-trap probe (red-first):** scrolled fully up at fit-width, the first
  page's top-left corner is reachable — `elementFromPoint` at that corner
  returns the page canvas, NOT chrome (this fails today); symmetric probe for
  the last page's bottom edge vs the bottom chrome. These two probes are the
  guard spec.
- Zero-shift + noflicker + wheelzoom suites green (padding is constant; prove no
  regression).
- Pixels: desktop light + 412px (both ends scrolled clear); note in the memo
  that the app WebView inherits (T46 embedded shows the same editor).
- Annotations near page edges are now reachable/editable at the extremes —
  include one e2e placing + selecting an object at the very top of page 1.
