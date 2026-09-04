# BRAND04 — TroubaCore wears the brand

**Status:** **CODE LANDED** — 3 commit(s), `bef559fe`…`be798b71` (last 2026-09-03). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".
**Asked by:** VLL, same ask: colour, a link to the page, icons, per product line.

## The finding that shapes this task: Core has almost no pixels

Surveyed rather than assumed. Core's human-visible surfaces are:

| surface | who owns the pixels |
|---|---|
| the SPA it serves (`spa.go` → `webassets/dist`) | **Studio** — BRAND03 owns it, including the favicon Core hands out |
| the CLI (`cmd/troubacore`) — invite links, `gc`, `purge-render-cache` | Core, and it is **text** |
| container image metadata | Core, and it is **labels** |
| HTTP error responses | Core, and they are **JSON** |

So the honest answer to "brand Core" is: **there is almost nothing to paint, and inventing
something to paint would be worse than leaving it alone.** A server that grows a splash
screen has gained nothing an operator wanted. This spec is deliberately small, and that is
the finding, not a shortfall.

What Core *can* carry is provenance: what this binary is, what version, and where to read
about it.

## Work

1. **OCI image labels** on the `Dockerfile` — the standard set, so `docker inspect` and any
   registry UI identify the image without guesswork:
   - `org.opencontainers.image.title=TroubaCore`
   - `org.opencontainers.image.description` — one line, the same claim the page opens with
   - `org.opencontainers.image.url=https://yoda-jm.github.io/troubastack/`
   - `org.opencontainers.image.source=https://github.com/yoda-jm/troubastack`
   - `org.opencontainers.image.licenses=Apache-2.0`
   - `org.opencontainers.image.revision` / `.version` from the build args already threaded
     through for the version string — **reuse them, do not add a second source of truth.**
2. **One startup line**, at info level, naming the product, the version and the page URL.
   One line, once, at boot. Not a banner, not ASCII art: this goes into `journalctl` for the
   rest of the deployment's life.
3. **`--help` header** — product name and the page URL above the flag list.
4. **Leave the JSON alone.** No `"_brand"` key, no link field in API responses. Machines
   read those.

## Colour: only if a Core-owned page ever exists

Core renders no HTML of its own today — `spa.go` sets `text/html` to hand back Studio's
shell, and the `testdata/*.html` files are lyric-import fixtures, not surfaces. **Do not add
a stylesheet to Core on the strength of this task.**

If a Core-owned page is ever added (a maintenance page, a standalone error page), the
measured layer-3 ink against the Studio grounds it would sit beside is:

| ink | on #fffdfa | on #f7f4ee | verdict |
|---|---|---|---|
| `#2A8FE9` — brand layer 3 | 3.33 | 3.08 | fails body text (4.5); clears the 3:1 large/UI bar, but by 0.08 on `--bg` |
| `#1472C5` — same hue, darkened | 4.87 | 4.51 | passes both |

Same lesson as BRAND03: the mark's own colour is for the mark and for large non-text UI, not
for body text on paper. Note how thin the margin is — layer 3 clears 3:1 on `--bg` by eight
hundredths, so any darkening of that ground breaks it. Re-measure against whatever ground
actually ships; do not carry these numbers forward on trust.

## Explicitly out of scope

- The favicon and any served-page styling — that is Studio's shell, BRAND03.
- The mobile app's launcher icons. The adaptive foreground/background were generated in
  BRAND01 and are **not yet wired into `androidApp`**; that is a third sibling task, not
  started and not specified here. Say so rather than letting it look covered.

## Done when

- `docker inspect` on a built image shows the label set, with `url` pointing at the live page.
- Booting the server prints exactly one identifying line.
- `--help` names the product and the page.
- No stylesheet, no HTML template and no image asset has been added to `core/`.
