# BRAND08 — the Studio shows its mark, not just its name

**Lane:** web-core. **Size:** S. **Status:** spec, not started.
**Asked by:** VLL — *"you probably also have logos and product names that would deserve a brand
styling?"* — and surveyed by the lane rather than invented, which is why this spec only has to settle
the decisions.

## The finding

BRAND03 gave the Studio its colours and its **favicon**, and stopped there. On screen the product
identity is still plain text: `Shell.tsx:97` renders the bare string in the masthead, the About link
is text, and **no wordmark or mark SVG is used anywhere in `web/studio`**. The assets have existed all
along in `docs/brand/dist` — only the marketing page draws them.

## The three decisions, settled

**1. Two surfaces, and they want different things.**

- **The topbar masthead** — persistent, small, present on every screen.
- **The login/register screen** — the first thing a user sees, and the only screen with no context.
  This one matters more than it looks: TroubaStack is **self-hosted**, so a musician may have more
  than one server, and the login screen is where "which thing am I signing into?" is answered.

Nothing else. Not dialogs, not the About link (it is a link, and text is the right affordance).

**2. Different assets, for a reason that is not taste.**

- **Topbar: the compact mark *beside* the existing text, not instead of it.** Around 20–24 px.
  Replacing the string with an image costs selectable, translatable, screen-readable text and buys
  nothing at that size, where a wordmark is illegible anyway. The mark adds identity; the text keeps
  doing its job.
- **Login: the full wordmark**, given room. There is space, it is the identity moment, and the
  lockup is what the brand sheet is built around.

**3. Sourcing: the favicon rule, unchanged.** One source of truth in `docs/brand/dist`, emitted at
build by the existing Vite plugin (`vite.config.ts:24`) — generalise it rather than adding a second
plugin, and **do not commit a copy into `public/`**; that is the drift BRAND03 deliberately avoided.

## ⚠ Two traps this task walks straight into

**The Docker build context.** The brand assets live outside `web/`, and the image build only sees the
paths the `Dockerfile` copies. That is exactly what broke `main` today when BRAND03's favicon plugin
reached for `docs/brand/dist` — fixed in `5edd038f` by copying that directory into the web stage. So
**this task inherits a working setup, but only for `docs/brand/dist`**. If you reach for anything
outside it, the image build breaks again and `vite build` will not tell you. **Build the image before
submitting.**

**Dark mode is not optional here, and the assets say so.** Each product ships a `-wordmark` *and* a
`-wordmark-dark`; the compact and minimal marks ship **one** file each. Read that as the brand's own
answer: the **mark is ground-independent** (carries its own tile), the **wordmark is not**. So the
topbar mark needs one asset, and the login wordmark needs the scheme swap — via `prefers-color-scheme`
and the `[data-theme]` stamps the Studio already uses, not by tinting an SVG in CSS.

## Done when

- The masthead shows the mark beside the product name; the name is still real text.
- The login screen shows the wordmark, and it **changes with the colour scheme** — check both
  schemes, and check the un-stamped "system" state, not only the explicit toggle.
- No brand SVG is committed under `web/studio`; the build emits them from `docs/brand/dist`.
- **The Docker image builds**, and the assets are actually served from it — fetch them from a running
  container, do not infer from `dist/`.
- BRAND03's drift guard still passes, and `tsc -b` is clean.
