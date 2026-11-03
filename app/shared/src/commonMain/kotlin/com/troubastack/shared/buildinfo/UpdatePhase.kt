// A96 — where an app update is once Update was pressed, in the same words on Home's banner and in Parameters → About
// (VLL updated from About and saw nothing for the whole download: "it was just super long"), and the two pure rules
// around it: the one-time "Updated ·" line, and whether the reopening notification is for this update. PURE.
package com.troubastack.shared.buildinfo

/** An update in progress, from the moment Update is pressed. null (no phase) = no update running. */
sealed interface UpdatePhase {
    /** The app's package downloads; [fraction] 0..1. */
    data class Downloading(val fraction: Float) : UpdatePhase

    /** The download is verified (on Android, then written into the install session). */
    data object Checking : UpdatePhase

    /**
     * Android shows its own confirmation, then installs (Android only: the desktop closes into its helper instead).
     * [reopenYourself]: no notification will reopen the app afterwards (refused, or notifications switched off).
     */
    data class Confirming(val reopenYourself: Boolean) : UpdatePhase
}

/** The status line for [phase]. Phase 1's words are A88's, unchanged. */
fun updatePhaseLine(phase: UpdatePhase): String = when (phase) {
    is UpdatePhase.Downloading -> "Downloading the app update… ${(phase.fraction * 100).toInt()} %"
    UpdatePhase.Checking -> "Checking the download…"
    is UpdatePhase.Confirming -> "Android will ask you to confirm the update." + if (phase.reopenYourself) " Then open TroubaStage again." else ""
}

/** Shown once (in the "App update" dialog) when Android reports the install failed or was cancelled in its dialog. */
const val UPDATE_NOT_INSTALLED_LINE = "The update was not installed. Nothing was changed."

/**
 * The line shown once when the app first runs after an update — WIN04's words (`HandoffResult.Updated`), or null.
 * [lastRunBuild] is the build line stored by the last run (null: none stored); [runningBuild] this one's. With nothing
 * stored — a first install, or the first run of the build that introduced this line — [updatedSinceInstall] (Android:
 * `firstInstallTime != lastUpdateTime`) tells an update from a fresh install, once.
 */
fun updatedLine(lastRunBuild: String?, runningBuild: String, updatedSinceInstall: Boolean): String? = when {
    lastRunBuild == null -> if (updatedSinceInstall) "Updated · $runningBuild" else null
    lastRunBuild != runningBuild -> "Updated · $runningBuild"
    else -> null
}

/**
 * Whether to post "TroubaStage was updated": only for the update this app started. [expectedVersionCode] is the marker
 * Update stored — the verified package's versionCode — just before handing it to Android (null: no marker); a stale
 * marker (a cancelled update, then an install from Chrome or adb) names another build, so it posts nothing.
 */
fun shouldPostReopen(expectedVersionCode: Long?, runningVersionCode: Long): Boolean =
    expectedVersionCode != null && expectedVersionCode == runningVersionCode

/** Whether to show the app's own line before Android's notification prompt (API 33+), when Update is pressed. */
enum class NotifyAsk { NOT_NEEDED, ASK, SKIP }

/**
 * [granted]: notifications are allowed (always true below API 33, where there is no prompt). [askedForRelease]: the
 * app already asked during this release's update (at most once per release). [askedBefore] + ![showRationale]: the
 * person refused for good — Android will not show its prompt again, so the app's line would lead nowhere.
 */
fun notifyAsk(granted: Boolean, askedForRelease: Boolean, askedBefore: Boolean, showRationale: Boolean): NotifyAsk = when {
    granted -> NotifyAsk.NOT_NEEDED
    askedForRelease -> NotifyAsk.SKIP
    askedBefore && !showRationale -> NotifyAsk.SKIP
    else -> NotifyAsk.ASK
}
