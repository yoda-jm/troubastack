# OPS03 — What actually helps the project page get found (the README link does not)

**Lane:** web-core · **Status:** specced, not started · **Priority:** low.
**Origin:** VLL, 2026-09-28: *"maybe a link from the base README to the github.io page of the project could be
nice to help referencement."*

## 1. The link is worth having, but not for that reason

Added to the README in this commit. **It will not help ranking:** GitHub has applied `rel="nofollow"` to
external links in user-generated content — READMEs, About fields, issues, wikis — since around 2020, so no
link equity passes. Google treats nofollow as a hint and may still follow it to *discover* a URL, and the
link sends real people. Both are reasons to keep it; neither is *référencement*.

The repo's **About → Website** field is the same story, and worth setting anyway for the same human reason.

## 2. What the site already gets right — do not redo it

`web/site/index.html` carries a real `<title>`, a `meta description`, `og:title/description/url/image` with
**absolute** URLs, and `og:type`. There is even a comment recording *why* `og:` values must be absolute (a
relative `og:image` renders a link card with no picture). The metadata is not the gap.

## 3. The actual gaps, cheapest first

- **No `sitemap.xml`.** The single highest-value addition for a new site: it is how a crawler learns the page
  exists without waiting to find a link to it, and how it learns the page changed. For a one-page site this is
  a dozen lines, generated at build alongside the existing `{{SITE_URL}}` substitution.
- **No `robots.txt`.** Should exist and should point at the sitemap. Absent, crawlers guess.
- **No `<link rel="canonical">`.** The page is reachable with and without a trailing slash today, and would
  gain a second address the day a custom domain appears. A canonical says which one is the page.

## 4. Constraint

`{{SITE_URL}}` is already substituted at build for the `og:` tags — **reuse that substitution** for the
sitemap, the robots line and the canonical rather than hard-coding the host in three new places. One source
for the site's own address.

## 5. Acceptance

- `/sitemap.xml` and `/robots.txt` are served by the deployed Pages site, not merely present in the repo.
- `robots.txt` names the sitemap with an absolute URL.
- The canonical URL matches `og:url` exactly — **asserted**, since two sources for one address is the defect
  this task is about.


## 6. CORRECTED 2026-09-28 (VLL) — a project page cannot be discovered automatically

VLL: *"this is not the root of the hostname, how will Google find it?"* He is right, and it invalidates half
of what §3 assumed.

- **`robots.txt` is honoured only at the HOST root.** Crawlers fetch `https://yoda-jm.github.io/robots.txt`,
  which **404s** — there is no user-page repo. The file written under `/troubastack/` is never fetched, so its
  `Sitemap:` line discovers nothing. Verified by request, not assumed.
- **The sitemap ping endpoint is gone.** Google retired it at the end of 2023 (most unauthenticated
  submissions were spam); it now returns 404. So there is no programmatic submission either.

**Both automated discovery paths are therefore closed for this site as it is addressed today.** The sitemap
itself is correct and useful — but only once something points at it.

### What actually works, and who can do it

1. **Google Search Console, URL-prefix property** on `https://yoda-jm.github.io/troubastack/`, then submit the
   sitemap by hand. This is the immediate answer and **only VLL can do it** — it needs his Google account.
   Verification is an HTML file at that path, which `build.sh` can drop if he pastes the token.
2. **A `yoda-jm.github.io` user-page repo.** Then the host root exists, and a root `robots.txt` can name this
   project's sitemap — discovery becomes automatic and stays automatic. It also gives him a landing page that
   links here, which is an inbound link the README cannot be (nofollow).
3. **A custom domain on the project.** Then the site *is* the host root, the `robots.txt` already written
   starts working with no code change, and `SITE_URL` is the single variable that moves.

**2 and 3 are structural; 1 is manual and immediate.** Nothing else in this repo can fix it, which is why
this section ends in a question for him rather than a task for a lane.
