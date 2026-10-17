// A84 — which build this is, in the app, without adb (VLL: "un hash ou version dans un écran de l'app"). The values
// come from the generated BuildInfo (git at build time, never hand-maintained); these pure formatters make the
// lines the Parameters → About section shows, and are unit-tested without a literal hash.
package com.troubastack.shared.buildinfo

/** This app's build as the About line shows it: `TroubaStage <hash> · built <date>` (or `TroubaStage unknown`). */
val appBuildLine: String get() = buildLine(BuildInfo.GIT.takeUnless { it == "unknown" }, BuildInfo.BUILT_AT)

/**
 * `TroubaStage 73dee974 · built 2026-10-09 15:07 UTC`. [hash] is `git describe --always --dirty` (a `-dirty` suffix
 * is kept: honest for a local build, a red flag for CI); [builtAtIso] is an ISO instant (`2026-10-09T15:07:00Z`).
 * No hash → `TroubaStage unknown`: never an invented value.
 */
fun buildLine(hash: String?, builtAtIso: String?): String {
    val h = hash?.trim()?.takeIf { it.isNotEmpty() } ?: return "TroubaStage unknown"
    val at = builtAtIso?.let(::utcMinute)
    return if (at == null) "TroubaStage $h" else "TroubaStage $h · built $at"
}

/** `2026-10-09T15:07:00Z` → `2026-10-09 15:07 UTC`; anything that is not a UTC ISO instant → null. */
fun utcMinute(iso: String): String? {
    val m = Regex("""^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})(:\d{2}(\.\d+)?)?Z$""").find(iso.trim()) ?: return null
    return "${m.groupValues[1]} ${m.groupValues[2]} UTC"
}

/** A joined server's version, as the About section shows it next to its address. */
sealed interface ServerVersion {
    /** The probe has not answered yet. */
    data object Checking : ServerVersion

    /** Offline, or the server did not answer `GET /api/version` — shown as *unreachable*, never an error dialog. */
    data object Unreachable : ServerVersion

    /** The server's own `version` (a 40-hex commit is shortened to 8 so it reads beside the app's hash). */
    data class Known(val version: String) : ServerVersion
}

/** One About line for a joined server: `http://host:8080 · 195ebe9a`, `… · unreachable`, `… · checking…`. */
fun serverVersionLine(address: String, version: ServerVersion): String = "$address · " + when (version) {
    ServerVersion.Checking -> "checking…"
    ServerVersion.Unreachable -> "unreachable"
    is ServerVersion.Known -> version.version.trim().let { if (Regex("^[0-9a-f]{40}$").matches(it)) it.take(8) else it }
        .ifEmpty { "unknown" }
}

/** A88 — this build's identity for the update check (the generated BuildInfo is internal to :shared). */
val appBuildGit: String get() = BuildInfo.GIT
val appBuildBuiltAt: String get() = BuildInfo.BUILT_AT
/** `"ci"` (a clean GitHub Actions build: checks for updates on its own) or `"dev"` (never on its own). */
val appBuildChannel: String get() = BuildInfo.CHANNEL
