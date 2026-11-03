package com.troubastack.desktop.studio

import com.troubastack.shared.seams.desktopDataRoot
import java.io.File

/**
 * WIN08 diagnostics — a durable, per-line-flushed log under the data root. A packaged Windows app has no captured
 * stdout, and a hung smoke is killed before it can summarise, so each stage is appended here immediately; the CI
 * step reads this file back even on a kill. (Trimmed down before the final PR.)
 */
internal val win08LogFile: File by lazy { File(desktopDataRoot(), "win08.log") }

internal fun win08log(msg: String) {
    runCatching { win08LogFile.appendText("${System.currentTimeMillis()} $msg\n") }
    println(msg)
}
