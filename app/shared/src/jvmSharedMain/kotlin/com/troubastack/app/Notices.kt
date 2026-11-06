package com.troubastack.app

import com.troubastack.shared.ui.LicenceDocument
import com.troubastack.shared.ui.LicenceEntry

/** A101 — About's line where the app ships its open-source notices (Android, Linux, macOS; Windows keeps S2.8's). */
const val OPEN_SOURCE_LINE = "TroubaStage is built with open-source software."

/**
 * A101 — the Licences screen's entries from the notices index the build generated (OpenSourceNotices, app/buildSrc):
 * each entry opens with its "Components" (group:artifact version — licence) or, for native code, what carries it, then
 * its texts ([text] reads one by id; a missing one is left out). `delegated` lines (SWT's texts live in their own
 * entry) are not entries here.
 */
fun parseNotices(index: String, text: (id: String) -> String?): List<LicenceEntry> {
    val entries = ArrayList<LicenceEntry>()
    var name: String? = null
    val components = ArrayList<String>()
    val carriers = ArrayList<String>()
    val docs = ArrayList<LicenceDocument>()
    fun flush() {
        val n = name ?: return
        val head = buildList {
            if (components.isNotEmpty()) add(LicenceDocument("Components", components.joinToString("\n")))
            if (carriers.isNotEmpty()) add(LicenceDocument("Inside", carriers.joinToString("\n")))
        }
        entries += LicenceEntry(n, head + docs)
        components.clear(); carriers.clear(); docs.clear()
    }
    for (line in index.lineSequence()) {
        if (line.isBlank() || line.startsWith("#")) continue
        val f = line.split('\t')
        when (f[0]) {
            "entry" -> { flush(); name = f.getOrNull(1) }
            "component" -> if (f.size >= 4) components += "${coordinates(f[1])} — ${f[3]}"
            "carrier" -> if (f.size >= 2) carriers += coordinates(f[1])
            "doc" -> if (f.size >= 3) text(f[2])?.let { docs += LicenceDocument(f[1], it) }
        }
    }
    flush()
    return entries
}

/** "group:artifact:version" as "group:artifact version". */
private fun coordinates(gav: String): String = gav.substringBeforeLast(':') + " " + gav.substringAfterLast(':')
