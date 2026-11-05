# A99 — Studio's downloads work in the tablet app

Owner: **mobile**. Spec: Fable, 2026-11-05. Size: M.

## Why

Studio, the web app, offers five downloads. In a browser, and in the Windows app (WebView2 saves to Downloads, as
WIN08 measured), each one saves a file. **The tablet's `EditScreen` WebView has no download handling at all**: no
`DownloadListener` anywhere in `app/`. So on the tablet these very likely do nothing. **Measure that first.**

The five downloads, grepped from `web/studio/src/pages/`:

| What the person presses | Where | How Studio downloads it |
|---|---|---|
| The song file's **"Download PDF…"** (T208) | `PdfDialog.tsx` | a `blob:` URL, through an `<a download>` click |
| The concert's **running order PDF** (T158) | `SetlistDetail.tsx` | a `blob:` URL, **revoked right after the click** |
| **Export the band** | `BandSettings.tsx` | a `blob:` URL, **revoked right after the click** |
| The setlist menu's **"Download PDF"** | `Setlists.tsx` | the server's URL, with `Content-Disposition` |
| **"Download .tstage"** (the bake) | `SetlistDetail.tsx` | the server's URL, with `<a download>` |

## The work

1. **Measure each of the five** on an API 34 emulator, against a scratch core with demo data only (never :8080).
   Say what happens today, with a screenshot each.
2. **Server-URL downloads:** a `DownloadListener` on the WebView hands them to Android's `DownloadManager`.
   - It sends the session cookie for that URL (`CookieManager.getCookie`) and the WebView's User-Agent.
   - It saves to the public **Downloads** folder, under the name from `Content-Disposition`, or from the
     `download` attribute.
   - Android's own download notification shows the result.
3. **`blob:` downloads:** host-injected, not a Studio change.
   - On each page load, inject a script that wraps `URL.createObjectURL` to remember each `Blob` by its URL. Two
     of the three revoke the URL right after the click, so reading it later fails.
   - The script listens for clicks on `<a download>` with a `blob:` href. It calls `preventDefault`, and sends the
     remembered blob to the app **in chunks** (for example 512 KB of base64 each) through a `@JavascriptInterface`.
     A band export can be large.
   - The app writes the file to **Downloads**: through `MediaStore.Downloads` from API 29; on API 26–28 through
     the public folder, with `WRITE_EXTERNAL_STORAGE` declared `maxSdkVersion="28"` and asked at the first save.
4. **The words, at the bottom of the Studio screen, once:**
   - success: **"Saved to Downloads: <file name>"**, with **Open**, which opens it with the system's viewer;
   - failure: **"Couldn't save <file name>."**
   - on API 26–28, before Android's prompt: **"Allow storage access so TroubaStage can save files to Downloads."**
     If the person refuses, the failure line shows.
5. **Nothing else changes.** Studio's own code is untouched. The Stage app, the bridge's `ready`/`hello`, and the
   session cookie (A98) stay as they are.

## Acceptance

- **Unit (pure):**
  - the file-name choice: `Content-Disposition` first, then the `download` attribute, then the URL's last
    segment;
  - the chunk reassembly: chunks out of order are rejected, and the total size is checked.

  **Teeth:** drop the size check, and a truncated file passes → red.
- **Emulator, API 34, each of the five:** the file is in Downloads, with the right name and its size equal to the
  server's or the blob's. Plus a screenshot of the "Saved to Downloads" line.
  - **The revoked-URL case:** the running order PDF and the band export.
  - **A large file:** a band export of a few MB, or a bake, to exercise the chunking.
- **API 29 or API 28 (one of them, say which):** one `blob:` and one server-URL download. On 28 that includes the
  storage prompt.
- **Pixels:** Studio's screen with no download in progress is identical to main.
- **CI:** the usual set, green.
- **VLL's tablet** (his pass, after landing): download a song's PDF from Studio and open it from the line.

## Not in this task

- Uploads (Studio's file picker). That's a separate question, if it doesn't work either. Report it if you see it.
- Windows: WebView2 already saves to Downloads (WIN08 S2.5 only checks it).
