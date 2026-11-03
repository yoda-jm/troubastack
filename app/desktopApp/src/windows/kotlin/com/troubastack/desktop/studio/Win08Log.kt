package com.troubastack.desktop.studio

import com.troubastack.shared.seams.desktopDataRoot
import java.io.File

/**
 * WIN08 diagnostics — a durable, per-line-flushed log under the data root. A packaged Windows app has no captured
 * stdout, and a hung smoke is killed before it can summarise, so each stage is appended here immediately; the CI
 * step reads this file back even on a kill. (Trimmed down before the final PR.)
 */
internal val win08LogFile: File by lazy { File(desktopDataRoot(), "win08.log") }

/** The durable file is written only when diagnostics are on — the `--win08-smoke` runner sets this. In normal use
 *  the embedded host stays quiet (no debug file under the data root); the println is lost on a GUI app anyway. */
internal fun enableWin08FileLog() { System.setProperty("win08.log", "1") }
private val fileLogOn: Boolean get() = System.getProperty("win08.log") == "1"

internal fun win08log(msg: String) {
    if (fileLogOn) runCatching { win08LogFile.appendText("${System.currentTimeMillis()} $msg\n") }
    println(msg)
}
