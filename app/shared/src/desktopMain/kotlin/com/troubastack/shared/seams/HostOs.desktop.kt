package com.troubastack.shared.seams

/** DESK01 — the desktop OS the app runs on: the one place it is read (the data root, the window's keys, the update). */
enum class HostOs { WINDOWS, MACOS, LINUX }

/** This JVM's OS. Anything that is neither Windows nor macOS is treated as Linux (the XDG desktop). */
val hostOs: HostOs by lazy { hostOsOf(System.getProperty("os.name").orEmpty()) }

/** Pure, for tests: `os.name` → [HostOs]. */
fun hostOsOf(osName: String): HostOs = when {
    osName.startsWith("Windows", ignoreCase = true) -> HostOs.WINDOWS
    osName.startsWith("Mac", ignoreCase = true) -> HostOs.MACOS
    else -> HostOs.LINUX
}
