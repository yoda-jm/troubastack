package com.troubastack.desktop

import com.troubastack.shared.buildinfo.LatestRelease
import java.io.File
import java.net.URI
import java.security.MessageDigest

/** Why a downloaded update was refused (shown in plain words; nothing is installed). */
class UpdateRefused(message: String) : Exception(message)

/**
 * WIN04 (H1) — the update helper, written by the app with this FIXED body. Its inputs arrive as environment variables
 * (TS_PID, TS_MSI, TS_LOG, TS_EXE, TS_RESULT), never on a command line, and are expanded only inside double quotes with
 * delayed expansion off and no parenthesised blocks — so nothing a path contains (a space, &, (, ), %) is read as a
 * command. It is started as `cmd.exe /d /c update.cmd` from its own folder (the working directory is not parsed).
 *
 * It waits for the app (TS_PID) to exit — 60 s at most; if the app is still running it installs NOTHING and records
 * `notclosed` (H2). Then it runs the per-user MSI (`/passive`: a progress bar, no questions) and records msiexec's code.
 * Code 0 reopens the new build with `--after-update`; 3010 (finished at the next reboot) reopens nothing (H3); any
 * other code reopens the UNCHANGED build (the MSI is transactional), so a failed update never leaves the performer
 * without the app. The system tools are called by full path (cmd would look in the working directory first).
 */
val UPDATE_SCRIPT: String = listOf(
    "@echo off",
    "rem TroubaStage update helper (WIN04), written by the app. Fixed body: inputs come from environment variables",
    "rem and are expanded only inside double quotes.",
    "setlocal EnableExtensions DisableDelayedExpansion",
    "set /a TS_WAIT=0",
    ":wait",
    "\"%SystemRoot%\\System32\\tasklist.exe\" /FI \"PID eq %TS_PID%\" /NH | \"%SystemRoot%\\System32\\find.exe\" \"%TS_PID%\" >nul",
    "if errorlevel 1 goto closed",
    "set /a TS_WAIT+=1",
    "if %TS_WAIT% GEQ 60 goto notclosed",
    "\"%SystemRoot%\\System32\\ping.exe\" -n 2 127.0.0.1 >nul",
    "goto wait",
    ":notclosed",
    ">\"%TS_RESULT%\" echo notclosed",
    "exit /b 1",
    ":closed",
    "start \"\" /wait \"%SystemRoot%\\System32\\msiexec.exe\" /i \"%TS_MSI%\" /passive /norestart /l*v \"%TS_LOG%\"",
    "set TS_CODE=%ERRORLEVEL%",
    ">\"%TS_RESULT%\" echo %TS_CODE%",
    "if \"%TS_CODE%\"==\"0\" start \"\" \"%TS_EXE%\" --after-update",
    "if \"%TS_CODE%\"==\"0\" exit /b 0",
    "if \"%TS_CODE%\"==\"3010\" exit /b 0",
    "start \"\" \"%TS_EXE%\"",
    "exit /b 0",
).joinToString("\r\n", postfix = "\r\n")

/** The environment variables the script reads, the only way its inputs reach it. */
const val SCRIPT_ENV_PID = "TS_PID"
const val SCRIPT_ENV_MSI = "TS_MSI"
const val SCRIPT_ENV_LOG = "TS_LOG"
const val SCRIPT_ENV_EXE = "TS_EXE"
const val SCRIPT_ENV_RESULT = "TS_RESULT"

/** What the helper recorded for the last handoff, read by the app at its next start. */
sealed interface HandoffResult {
    /** msiexec 0: the new build is installed (and the helper reopened it). */
    data object Updated : HandoffResult

    /** msiexec 3010: installed, but files were in use; Windows finishes at the next restart (H3). */
    data object RestartWindows : HandoffResult

    /** The app did not close within the cap: nothing was installed (H2). */
    data object NotClosed : HandoffResult

    /** Any other msiexec code: the installed build is unchanged (the MSI is transactional). */
    data class Failed(val code: String) : HandoffResult
}

/** The helper's result file → [HandoffResult]; null when there is no result to report. */
fun handoffResult(text: String?): HandoffResult? {
    val t = text?.trim()?.takeIf { it.isNotEmpty() } ?: return null
    return when (t) {
        "0" -> HandoffResult.Updated
        "3010" -> HandoffResult.RestartWindows
        "notclosed" -> HandoffResult.NotClosed
        else -> HandoffResult.Failed(t)
    }
}

/**
 * WIN04 (5a403976) — where the installer may be downloaded from, as the release JSON names it: over https in normal use
 * (GitHub's release asset, so a release without a digest is never checked by size alone over plain http), and on the
 * loopback interface only when the CI hook is in use (the hook's release must not send the app to another host).
 * Throws [UpdateRefused] otherwise; nothing is downloaded.
 */
fun checkAssetUrl(assetUrl: String, hook: Boolean) {
    val uri = runCatching { URI(assetUrl) }.getOrNull()
    val ok = when {
        uri == null -> false
        hook -> runCatching { loopbackReleaseUrl(assetUrl) }.isSuccess
        else -> uri.scheme == "https" && !uri.host.isNullOrEmpty()
    }
    if (!ok) throw UpdateRefused("The release points the installer at an address TroubaStage does not download from. Nothing was installed.")
}

/**
 * WIN04 (C1) — the CI hook's release URL: http(s) on the loopback interface only (127.0.0.1, localhost, ::1). Anything
 * else is refused here, so the hook can never fetch an update from another machine.
 */
fun loopbackReleaseUrl(raw: String): String {
    val uri = runCatching { URI(raw) }.getOrNull() ?: throw IllegalArgumentException("--update-from: not a URL: $raw")
    require(uri.scheme == "http" || uri.scheme == "https") { "--update-from: http or https only: $raw" }
    val host = uri.host?.lowercase()?.removePrefix("[")?.removeSuffix("]")
    require(host == "127.0.0.1" || host == "localhost" || host == "::1") { "--update-from: loopback only (127.0.0.1, localhost, ::1): $raw" }
    return raw
}

fun sha256Hex(file: File): String {
    val md = MessageDigest.getInstance("SHA-256")
    file.inputStream().use { input ->
        val buf = ByteArray(64 * 1024)
        while (true) {
            val n = input.read(buf)
            if (n < 0) break
            md.update(buf, 0, n)
        }
    }
    return md.digest().joinToString("") { "%02x".format(it) }
}

/**
 * The downloaded MSI against the release: its size, and its SHA-256 when the release gives one. [requireDigest] (the
 * `--update-from` hook, C1) refuses a release without a digest instead of falling back to the size alone.
 */
fun verifyDownload(file: File, release: LatestRelease, requireDigest: Boolean) {
    if (file.length() != release.assetSize) {
        throw UpdateRefused("The download is incomplete (${file.length()} of ${release.assetSize} bytes). Nothing was installed.")
    }
    val expected = release.assetSha256
    if (expected == null) {
        if (requireDigest) throw UpdateRefused("The release gives no SHA-256 for the installer. Nothing was installed.")
        return
    }
    if (sha256Hex(file) != expected.lowercase()) {
        throw UpdateRefused("The download does not match the release (SHA-256). Nothing was installed.")
    }
}

/**
 * WIN04 (H4) — whether [exe] is the TroubaStage the MSI installed: its folder is the InstallLocation of a "TroubaStage"
 * entry among the per-user uninstall entries ([installs]: DisplayName to InstallLocation). A `:desktopApp:run` or a
 * copied folder is not, and does not offer Install.
 */
fun isMsiInstalled(exe: File?, installs: List<Pair<String, String>>): Boolean {
    val dir = exe?.absoluteFile?.parentFile ?: return false
    return installs.any { (name, location) ->
        name == "TroubaStage" && location.isNotBlank() && runCatching { File(location).canonicalFile == dir.canonicalFile }.getOrDefault(false)
    }
}
