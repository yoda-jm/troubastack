# OPS03 — publish the project page on GitHub Pages

**Status:** **LIVE** — verified 2026-09-04: the `pages` workflow is `active` with 3 successful runs, GitHub Pages is enabled on the repo (`/repos/.../pages` → 200), and <https://yoda-jm.github.io/troubastack/> serves 43.5 KB with the expected title and 14 images (sampled assets all 200). Checked the CONTENT, not just the status code — a 200 can be an empty page. The go-live this task existed for has happened.

**Asked by:** VLL — "je veux une 'page' github avec la page statique qu'on a faite […] je
veux une page standard (j'ai pas de nom de domaine pour le moment)."

## What is already done — do not rebuild it

Landed with the site commit:

- `web/site/index.html` — the page.
- `web/site/build.sh` — assembles `web/site/dist/` by regenerating the brand marks,
  copying an explicit screenshot **allow-list**, emitting the marker swipe from the same
  `band()` the icons use, and generating the QR.
- `.github/workflows/pages.yml` — installs `qrencode` + `librsvg2-bin`, runs `build.sh`,
  **verifies the expected files exist and are non-empty**, then uploads *only*
  `web/site/dist` and deploys. Triggers on push to `main` touching `web/site/**`,
  `docs/brand/**`, `docs/screenshots/**` or the workflow itself, plus `workflow_dispatch`.

## Two corrections to assumptions worth writing down

1. **Nothing extra needs committing.** The palette source (`docs/brand/src/_defs.svg`), the
   41 generated mark SVGs in `docs/brand/dist/`, and the 71 screenshots are already tracked.
   Only PNG rasters (`dist/*.png`) and `web/site/dist/` are ignored, deliberately: CI
   regenerates them, which is what stops a shipped asset from drifting from its source.
2. **The page already survives a subpath.** A project page is served under
   `/<repo>/`, not at the domain root, which breaks any `href="/…"`. Checked: **every
   reference in `index.html` is relative**, so it needs no rewriting.

## Recommendation: a project page, not a user page

| | Project page (**recommended**) | User page |
|---|---|---|
| URL | `https://yoda-jm.github.io/troubastack/` | `https://yoda-jm.github.io/` |
| Repo | this one | a second repo named `yoda-jm.github.io` |
| Assets | built from `docs/brand` + `docs/screenshots` **in place** | would have to be copied or submoduled into the other repo |

The shorter URL is the only thing the user page buys, and it costs the property the whole
build is designed around: the site is assembled from the sources beside it, so no asset can
go stale. Copying the marks into a second repo reintroduces exactly the drift `build.sh`
exists to prevent. Take the subpath.

## The link

```
https://yoda-jm.github.io/troubastack/
```

Verified today: the repository is **public** (unauthenticated `GET` → 200), so Pages is
available on any plan; the URL currently returns **404**, i.e. Pages is not enabled yet.

## The one step only VLL can do

**Settings → Pages → Build and deployment → Source: `GitHub Actions`.**

> ⚠ **Do not choose "Deploy from a branch" with `/docs`.** That would publish the entire
> internal documentation tree — handoffs, `reviews.md`, task specs, the project audit — as
> a public website. The workflow uploads `web/site/dist` and nothing else; the setting is
> the only thing that can override that intent.

Nothing else in the repo can set this; it is a repository setting.

## The one code change this needs

`og:image` is currently relative and `og:url` is missing. Relative `og:` values are not
reliably resolved by link unfurlers (Slack, Discord, Mastodon, WhatsApp), so the card would
render without its image the first time anyone pastes the link — which is the moment the
page is most likely to be shared.

Proposed shape: keep `index.html` free of any hard-coded origin and have `build.sh`
substitute a single `SITE_URL` token when it assembles `dist/`:

```
SITE_URL="${SITE_URL:-https://yoda-jm.github.io/troubastack}"
```

- `<meta property="og:url" content="{{SITE_URL}}/">`
- `<meta property="og:image" content="{{SITE_URL}}/assets/troubastack-512.png">`

Cost, stated plainly: opening `web/site/index.html` directly over `file://` then shows the
raw token in two `<meta>` tags. Nothing visible renders from them, so the preview workflow
is unaffected — and in exchange, moving to a custom domain later is one environment
variable rather than a hunt through the markup.

## First-deploy verification (do these, do not assume)

1. Actions → the `pages` run is green, and its "verify the assembled output" step printed a
   file count and a size.
2. `GET https://yoda-jm.github.io/troubastack/` → 200.
3. **Look at the page in a browser**, not just at the status code: the five screenshots
   render (not empty outlined boxes — that failure mode has bitten this page before), the
   marks appear, the highlighter swipe paints, and the lightbox opens on a click.
4. Scan the QR with an actual phone and confirm it lands on the CI runs page.
5. Paste the link into a chat client and confirm the unfurl shows the card image.
6. Confirm nothing under `docs/` is reachable: `GET …/troubastack/docs/` → 404.

## Later: the custom domain

When a domain exists: Settings → Pages → Custom domain. GitHub then serves the site there
and **301-redirects the `github.io` URL**, so links already shared keep working. Set
`SITE_URL` to the new origin in the workflow so the `og:` tags follow.

## Notes

- **No `.nojekyll` needed.** With the Actions source the artifact is served as uploaded;
  Jekyll never runs. Do not add one out of habit.
- The page is **public the moment this lands**, which makes the screenshot allow-list in
  `build.sh` load-bearing rather than advisory: adding a file to `docs/screenshots/` cannot
  put it on the web, because the list is explicit and never a glob. Keep it that way.
- Deploys are queued, not cancelled (`concurrency: cancel-in-progress: false`), so a rapid
  second push cannot leave a half-finished deploy live.
