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
}

val win08SmokeRunner: Win08SmokeRunner? by lazy {
    runCatching {
        ServiceLoader.load(Win08SmokeRunner::class.java, Win08SmokeRunner::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
