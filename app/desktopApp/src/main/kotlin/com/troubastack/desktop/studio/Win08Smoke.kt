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
}

val win08SmokeRunner: Win08SmokeRunner? by lazy {
    runCatching {
        ServiceLoader.load(Win08SmokeRunner::class.java, Win08SmokeRunner::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
