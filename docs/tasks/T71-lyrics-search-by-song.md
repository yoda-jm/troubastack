# T71 — Studio: "search by artist/title" in the lyrics dialog (the UI half of lyrics.ovh)

**Priority:** normal (completes the server work landed in `531c11c`) · **Size:** S ·
**Area:** `web/studio` (`pages/song-editor/SongDetails.tsx`, `api.ts`, `styles.css`) + a
Playwright spec. **No server change** — the endpoint already accepts `{artist,title}`.

## Context

`POST /api/bands/{b}/lyrics-import` now takes **either** `{url}` (fetch + scrape an
arbitrary page) **or** `{artist,title}` (query lyrics.ovh). The search path exists because
the URL path is unreliable in practice — azlyrics/genius are Cloudflare-walled, so an honest
GET usually bounces and the user ends up pasting. Studio still exposes only URL + paste, so
the better path is unreachable from the app.

Today's dialog (`LyricsDialog`): chart name, a URL field + **Fetch**, a textarea to paste,
a "label sections" toggle, then **Create** (which runs `normalizeLyrics` / `detectSections`).
Keep all of that — including its `data-testid`s (repo ground rule 5).

## Design (decided)

1. **Search is the primary row, URL demoted to secondary.** Order in the dialog: chart name →
   **Search by song** (Artist, Title, `Search`) → *"or fetch from a URL"* (existing field +
   Fetch) → textarea → toggle → Create. The URL row keeps its markup and testids untouched;
   it is only reordered and re-labelled.
2. **Prefill artist and title from the song's metadata.** The dialog opens inside the song
   editor, where title/artist are already known (`meta-title` / `meta-artist` hold them), so
   the common case is *open dialog → click Search*. This is the point of the feature; do not
   ship an empty pair of boxes the user has to retype.
3. **`api.ts` gets a sibling, not a wider signature:** keep `lyricsImport(bandId, url)` and
   add `lyricsSearch(bandId, artist, title)` posting `{artist,title}` to the same endpoint
   with the same return type. Two call sites, two intents, no boolean/union parameter.
4. **Every non-`ok` outcome degrades to paste — that is the existing contract, keep it.**
   `Search` is disabled while either field is empty or a request is in flight. On non-`ok`,
   show the server's `reason` when present (they are curated, user-facing strings: *"no
   lyrics found for that artist/title"*, *"lyrics search is disabled on this server"*,
   *"invalid artist or title"*) and fall back to the current behaviour — message + focus the
   textarea. Never throw, never a dead end.
5. **Disabled server needs no new contract.** When an operator sets
   `TROUBA_LYRICS_OVH_BASE=off`, the request returns `status:"error"` with the disabled
   reason; showing that message and leaving paste available is the whole handling. Do **not**
   add a capabilities endpoint or probe on mount for this.
6. **Testids:** `lyrics-artist`, `lyrics-title`, `lyrics-search` (mirroring the existing
   `lyrics-name` / URL row naming).

## Acceptance criteria

- Search row present, prefilled from the song's title/artist; `Search` disabled when either
  field is blank or a request is in flight; success fills the textarea and shows the same
  "review then create" affordance as the URL path.
- The existing URL + paste + label-sections + Create flow is unchanged, with its testids
  still attached to the equivalent elements; the existing lyrics e2e still passes untouched.
- **Playwright spec that never touches the network:** intercept
  `**/api/bands/*/lyrics-import` with `page.route` and assert (a) a `{artist,title}` body is
  sent when Search is clicked, (b) an `ok` response populates the textarea, (c) an `error`
  response with the disabled reason shows that message and leaves paste usable and Create
  working. CI must not depend on lyrics.ovh being reachable — this is a hard requirement,
  not a preference.
- `cd web && studio/node_modules/.bin/tsc -b studio` clean; `make e2e` green.
- Screenshot of the dialog (before/after) in the handoff — it is a visible surface.

## Out of scope

- Multi-result pickers / "did you mean" (lyrics.ovh returns a single best match).
- Additional providers, or client-side fetching (the SSRF guard lives server-side and must
  stay the only fetcher).
- Auto-creating the chart from a search hit without the review step — the human read of the
  fetched text before Create is deliberate.
- Any change to `lyricsimport.go`.
