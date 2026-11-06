// A96 — the two things an app update remembers across the process Android ends when it installs: the build that ran
// last (so "Updated ·" shows once), and the versionCode the app itself handed to Android (so only that update posts
// "TroubaStage was updated"). The decisions are the pure updatedLine / shouldPostReopen; these read and write them.
package com.troubastack.app

import com.troubastack.shared.buildinfo.shouldPostReopen
import com.troubastack.shared.buildinfo.updatedLine
import com.troubastack.shared.seams.Storage

private const val K_LAST_RUN_BUILD = "appUpdate.lastRunBuild"
private const val K_REOPEN_VERSION_CODE = "appUpdate.reopenVersionCode"

/** "Updated · <build line>" when this run is the first after an update (else null), and this build is stored as the
 *  last run, so the line shows once. Stored only when it changes (A100: the same value rewrote the whole encrypted
 *  store at every launch, measured on API 28, 29 and 34). */
fun updatedLineOnce(storage: Storage, runningBuild: String, updatedSinceInstall: Boolean): String? {
    val last = storage.getSecret(K_LAST_RUN_BUILD)?.takeIf { it.isNotEmpty() }
    val line = updatedLine(last, runningBuild, updatedSinceInstall)
    if (last != runningBuild) storage.putSecret(K_LAST_RUN_BUILD, runningBuild)
    return line
}

/** The marker naming the update the app started: set just before Android takes the package, cleared on every outcome. */
class ReopenMarker(private val storage: Storage) {
    /** Update handed [versionCode] (the verified package's) to Android. */
    fun arm(versionCode: Long) = storage.putSecret(K_REOPEN_VERSION_CODE, versionCode.toString())

    /** Not installed (failed, cancelled, refused): nothing will expect it. */
    fun clear() = storage.putSecret(K_REOPEN_VERSION_CODE, "")

    /** On MY_PACKAGE_REPLACED: whether this is the update the app started — and the marker is cleared either way. */
    fun consume(runningVersionCode: Long): Boolean {
        val expected = storage.getSecret(K_REOPEN_VERSION_CODE)?.toLongOrNull()
        clear()
        return shouldPostReopen(expected, runningVersionCode)
    }
}
