# T37 — New text chart from lyrics: best-effort URL fetch + paste fallback

**Priority:** HIGH (VLL made it a must-have, 2026-07-12: "azlyrics is a must") ·
**Size:** S/M · **Area:** `core` (fetch endpoint) + `web/studio` (dialog) + a
pure normalizer/parser + tests. **Depends on T36** (the Files section must be
reachable).

## The decision (REVISED 2026-07-12 — VLL override)

The original ruling declined the scraper (paste-only). **VLL overrode it:**
azlyrics-link import is a required feature; a generic-URL fetch is a wanted
follow-on; paste is the fallback. VLL owns the ToS/copyright call — a private,
self-hosted band tool, his content decision, explicitly on the record. The arch
gate accepts the override and specs the mechanics below.

**Boundary that does NOT move (both roles agree, recorded):** we build an
**honest** fetch only — a plain HTTP GET with a truthful User-Agent, no anti-bot
evasion (no rotating fingerprints, no headless-browser Cloudflare-challenge
solving, no CAPTCHA defeat). That is detection-evasion tooling and is out of
scope on principle, not preference. **Consequence stated plainly in the UI and
the spec:** azlyrics is Cloudflare-gated, so an honest server GET will OFTEN get
a 403/JS-challenge and the fetch will fail; **the paste fallback must ship
alongside so the user is never stuck.** This is best-effort by construction.

### The three gate questions, answered

1. **Server-side fetch endpoint — YES.** Client-side is CORS-blocked, so it must
   be `POST /api/bands/{bandId}/lyrics-import` (authed; band-scoped — only a
   member can spend the server's egress). Body `{url}`; returns
   `{status:"ok", text}` or `{status:"blocked"|"error", reason}`. **SSRF guard
   is mandatory** (see §3) — a URL-fetch endpoint is an SSRF vector.
2. **azlyrics parser now + generic readability follow-on — ONE endpoint, host
   dispatch.** The endpoint picks a parser by host: azlyrics → the known-marker
   extractor; everything else → a generic readability-ish text extract, shipped
   in the SAME task (VLL wants it "eventually" and it's cheap once the endpoint
   + SSRF guard exist — no reason to split). Both funnel through the T37
   normalizer.
3. **Fallback UX — the dialog is paste-native; fetch is an accelerator.** The
   dialog (below) always has the paste textarea. A "Fetch from URL" field sits
   above it; on success it FILLS the textarea (user still reviews before
   creating). On block/error it shows a plain, honest message ("Couldn't fetch —
   the site blocked the request. Paste the lyrics below instead.") and leaves
   focus in the textarea. The user is never dead-ended.

## Changes

1. **Dialog: "＋ New chart from lyrics"** beside "＋ New text chart" in the
   Details panel's Files section (T36's home). Fields: name (prefilled from the
   song title); a "Fetch from URL" input + Fetch button; a big paste textarea.
   Create → a text chart pre-filled with `normalizeLyrics(text)`, opened in the
   T19 chart editor for cleanup. No new file type — the existing create-chart
   call with initial content.
2. **`normalizeLyrics` (pure, unit-tested), deliberately minimal:** normalize
   CRLF→LF; trim outer whitespace; collapse 3+ consecutive blank lines to one
   blank line (section break); strip trailing all-site-cruft lines ONLY when
   they match a tiny conservative blacklist (e.g. lines exactly "Submit
   Corrections" / "Writer(s):…" / "Thanks to … for these lyrics") — when in
   doubt, KEEP the line. Do NOT touch section labels, chords, brackets, or case.
3. **Core fetch endpoint** `POST /api/bands/{bandId}/lyrics-import` (authed):
   - **SSRF guard FIRST (mandatory, non-negotiable):** accept only
     `http`/`https` schemes; resolve the host and REJECT if it resolves to a
     private/loopback/link-local/ULA range (block 10/8, 172.16/12, 192.168/16,
     127/8, 169.254/16, ::1, fc00::/7, and 0.0.0.0) — re-check after any
     redirect (or disable redirects). This endpoint makes the server fetch an
     arbitrary URL; without this it is an SSRF hole into the deploy's LAN.
     Also: a hard timeout (~5 s), a response-size cap (~1 MB — lyrics are tiny),
     and follow at most a couple of redirects.
   - **Honest GET:** a truthful `User-Agent` identifying troubacore; NO evasion.
   - **Host dispatch parser:** azlyrics host → extract the lyrics `<div>` after
     the known `<!-- Usage ... -->` comment marker (the site's stable landmark),
     strip tags to text; any other host → a generic readability-ish extract
     (largest text-dense block, tags stripped). Both → `normalizeLyrics`.
   - **Return** `{status:"ok",text}` | `{status:"blocked",reason}` (403/challenge
     detected) | `{status:"error",reason}` (timeout/parse/size). NEVER 500 on a
     blocked upstream — a block is a normal, expected outcome.
4. **Tests:**
   - Go: the **SSRF guard is the priority test** — a table rejecting
     loopback/private/link-local hosts and non-http schemes (this is the
     security-critical unit; it must be exhaustive). Parser unit tests run
     against COMMITTED fixture HTML (a saved azlyrics-shaped page + a generic
     article) — never a live network call in CI. Blocked-response mapping
     (a 403 fixture → `status:"blocked"`).
   - normalizer unit table (CRLF, blank-line collapse, cruft blacklist,
     keep-ambiguous).
   - e2e: dialog opens (red-first — the button doesn't exist pre-fix); paste
     path creates a chart; a fetch that returns `blocked` (stub the endpoint or
     point at a fixture) shows the honest fallback message and leaves the paste
     box focused. The live-azlyrics reachability is NOT a CI assertion (network
     + Cloudflare non-determinism) — verified once by hand at the gate and noted.

## Acceptance criteria

- The SSRF guard rejects every private/loopback/link-local/non-http case (Go
  table green) — this gates the endpoint's existence.
- Paste path: red-first e2e, chart created, normalized; the fallback message
  shows on a blocked fetch and the user can still paste.
- Parser fixtures (azlyrics-shaped + generic) extract clean text off-network;
  full suite green; `tsc -b studio` + `go vet`/`gofmt` clean; dialog pixels at
  the gate. A hand-run azlyrics fetch result (ok OR honestly-blocked) reported
  in the PR — either outcome is acceptable; "blocked" is not a failure.

## Out of scope

- Any anti-bot / Cloudflare-challenge evasion (out on principle — detection
  evasion); persisting fetched HTML; chord/ChordPro parsing; auto-formatting
  beyond the normalizer; app-side changes (charts already flow through the bake).
