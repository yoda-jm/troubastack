package com.troubastack.desktop.studio

import java.io.File
import java.util.ServiceLoader

/**
 * WIN08 S1.8 — the CI smoke (`--win08-smoke <outDir>`). Launches the packaged app into the embedded Studio screen
 * against a loopback test page (no core) inside the real TroubaStage window, runs the checks (engine is Edge, the
 * browser's top window is the frame, the seeded cookie arrives, the bridge round-trips, off-origin navigation is
 * vetoed, the data dir is under the data root and `~/.swt` is absent), each with a `WIN08-CHECK …: PASS|FAIL`
 * marker, then closes the window and exits. The implementation imports SWT, so it lives in `src/windows` and is
 * resolved at runtime; on a non-Windows build there is no runner (the `--win08-smoke` path reports that and exits 2).
 */
interface Win08SmokeRunner {
    /** Run the smoke; return the process exit code (0 = all checks passed). Writes screenshots/markers under [outDir]. */
    fun run(outDir: File): Int

    /** S1.1 re-entry (item 3): leave and re-enter the embedded Studio screen twice (the bridge round-trips each
     *  time), and an immediate Back on entering leaves no window. Returns the exit code (0 = passed). */
    fun runReentry(outDir: File): Int

    /** S2.1 (R1) live Retry proof: start with the loopback server off (probe Unreachable → the cover), then turn it on
     *  and fire Retry; Studio returns on the SAME controller. Writes cover.png/studio.png + retry-verdict.txt. */
    fun runRetry(outDir: File): Int

    /** S2.2 recon: build same-origin history, synthesise the real mouse back button (XBUTTON1) and Alt+←, and record
     *  whether SWT/the injected page listener see them and how many navigations one press causes. Writes back-verdict.txt. */
    fun runBack(outDir: File): Int

    /** S2.3 sign-out reopen: controller 1 lets the server set a PERSISTENT win08sess, then closes; a clear is requested
     *  with no live Browser (→ a pending-clear file); controller 2 reopens SIGNED OUT (no seed) and applies the pending
     *  clear before loading, so its request carries no win08sess. Returns 0 if the reopen's Cookie header is clean. */
    fun runCookieClear(outDir: File): Int

    /** S2.3b ⋮ "Server URL…": open the in-canvas dialog (a Compose TextField focusable with the SwingPanel at 0.dp),
     *  then re-point between two loopback origins N times. Each re-point clears the old origin through the next-open
     *  path and a fresh controller seeds a new session, which must survive (the old controller never clears live). */
    fun runServerUrl(outDir: File): Int

    /** S2.4 links by origin: a captured link to the server's origin opens in place; another http(s) origin opens in the
     *  default browser (a recording stub) without the session; non-http(s) schemes are dropped; no in-app window opens. */
    fun runLinks(outDir: File): Int

    /** S2.5 downloads: run Studio's real saveBlob (fetch+credentials → Blob → <a download> → revoke) against a loopback
     *  /pdf (401 without the cookie, non-ASCII Content-Disposition), and assert the file lands in Downloads with that
     *  exact name and a SHA-256 matching the served bytes. Keeps WebView2's default; a panel PNG is left under outDir. */
    fun runDownload(outDir: File): Int

    /** S2.6 right-click (R3): a host-injected, window-level bubbling contextmenu listener suppresses the page menu on
     *  non-editable targets (the chart) but keeps WebView2's own edit menu on input/textarea/contenteditable, and
     *  leaves a target Studio already handled (defaultPrevented) untouched. Writes ctx-chart.png / ctx-field.png. */
    fun runContextMenu(outDir: File): Int

    /** S2.10 (report) — session expiry seam: the app's STORED session (what HttpTransport replays) and the WebView's
     *  seeded session are separate. Seed both to the same cookie, then expire/re-auth the WebView's session in place
     *  (fresh Set-Cookie) and show the app's own `probePresence()` still carrying the OLD stored cookie → 401 while
     *  the local store is non-empty (the stale-store finding). The positive control: with the store updated to the
     *  fresh cookie, the same request returns Online — so the 401 is cookie-specific, not a blanket failure. Report
     *  only; exit 0 iff the control discriminated. Writes s210-expiry-report.txt. */
    fun runS210Expiry(outDir: File): Int

    /** S2.10 (report) — cross-origin iframe injection recon. A loopback page on origin A embeds an <iframe> from a
     *  second loopback origin B; the child frame enumerates `typeof` each injected surface and (a) tries to call the
     *  host bridge DIRECTLY and (b) postMessages its report to the parent, which forwards it over its own bridge.
     *  Pass 1 (veto on): records whether the origin lock's LocationListener fired for the subframe and vetoed it (the
     *  positive control is that the lock has teeth on a top off-origin nav). Pass 2 (WIN08_PROBE_S210_IFRAME_RELAX=1,
     *  dispatch-only, never the default path): relaxes the veto for the subframe so B loads, then records which of the
     *  injected fns reached the cross-origin frame and whether its direct bridge call passed the top-URL guard. Report
     *  only; exit 0 iff the control held. Writes s210-iframe-report.txt. */
    fun runS210Iframe(outDir: File): Int
}

val win08SmokeRunner: Win08SmokeRunner? by lazy {
    runCatching {
        ServiceLoader.load(Win08SmokeRunner::class.java, Win08SmokeRunner::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
