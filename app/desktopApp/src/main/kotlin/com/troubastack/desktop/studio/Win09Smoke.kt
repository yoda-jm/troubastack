package com.troubastack.desktop.studio

import java.io.File
import java.util.ServiceLoader

/**
 * WIN09 — the band-QR CI smoke (`--win09-qr <outDir>`). Opens the app's own TroubaStudio screen (the embedded Studio,
 * with its ⋮ menu) against a loopback fake core, signed in, and checks: the ⋮ "Show band QR…" item is absent for a
 * non-admin and present for an admin; the QR in a window capture decodes to the link the server served; opening the
 * screen sends no create request; keep-awake is on while it shows and off after Back (measured with
 * `powercfg /requests`, or reported UNMEASURED); the overlay takes the focus and Back returns to Studio where it was.
 * Each check prints a `WIN09-CHECK …: PASS|FAIL|UNMEASURED` line to win08.log. The implementation imports SWT, so it
 * lives in `src/windows` and is resolved at runtime; elsewhere there is no runner.
 */
interface Win09SmokeRunner {
    /** Run the smoke; return the process exit code (0 = no check failed). */
    fun run(outDir: File): Int
}

val win09SmokeRunner: Win09SmokeRunner? by lazy {
    runCatching {
        ServiceLoader.load(Win09SmokeRunner::class.java, Win09SmokeRunner::class.java.classLoader).firstOrNull()
    }.getOrNull()
}
