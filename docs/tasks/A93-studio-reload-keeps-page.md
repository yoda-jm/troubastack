# A93 — Studio in the app: Reload reloads the page you are on, not the band

Owner: mobile. Spec: Fable, 2026-10-31. Small; present the patch.

## Why (VLL, 2026-10-31, on the tablet)

> "reload on tablet reload the band and not the song I navigate to after"

## Cause (read in code)

`EditScreen.kt`: `loadUrl = embeddedUrl(serverUrl, initialPath)` is the ENTRY url (the band, or the context
the screen was launched with). `reload()` seeds the session cookie and calls `host.load(loadUrl)`. The
overflow menu's **Reload** and the error cover's **Retry** both call `reload()`, so after any navigation inside
Studio (band → song), Reload throws the user back to the entry page.

## Bindings

1. **Reload** reloads the page currently shown in the WebView (its current URL), with the session cookie seeded
   as now. If the WebView has no current URL yet (the first load failed before committing), fall back to
   `loadUrl`.
2. **Retry** on the error cover retries the URL that failed. That is the current or last-attempted URL, not the
   entry URL.
3. The reloaded URL must keep Studio in embedded mode. Studio reads `?embedded=1` once and keeps it in
   `sessionStorage`; a reload of the same WebView keeps that storage. Do not drop the flag if you rebuild a URL.
4. The `LaunchedEffect(loadUrl)` path is unchanged: a server-URL change still loads the new server's entry page.
5. Rotation is unaffected: the activity handles configuration changes in place (manifest `configChanges`).
   Confirm that on the tablet in portrait and landscape while you are there.

## Tests

- A unit test on the seam that picks the URL to load: current URL wins, entry URL is the fallback, and a
  server change resets to the entry. Sabotage it back to "always the entry URL" and show it goes red.
- **Device pass on VLL's tablet** (11", portrait): open a band in Studio, open a song, ⋮ → Reload, and check that
  the song is still shown and the Studio nav bar did not come back (embedded mode kept).

## Delivery

One commit, presented at the gate. VLL installs it through the A88 self-update.
