// A88 — the app-update flow's state, for Home's banner and Parameters → About. Decisions are the pure :shared
// functions (shouldAutoCheck / updateVerdict); this keeps the two remembered values (last check, last "Not now") in
// the device's preferences and drives AppUpdater (download → verify → PackageInstaller). A96: the phase the update is
// in, shown the same on both surfaces, and the notification that reopens the app once Android has replaced it.
package com.troubastack.app

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.troubastack.shared.buildinfo.NotifyAsk
import com.troubastack.shared.buildinfo.UPDATE_NOT_INSTALLED_LINE
import com.troubastack.shared.buildinfo.UpdatePhase
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.buildinfo.appBuildBuiltAt
import com.troubastack.shared.buildinfo.appBuildChannel
import com.troubastack.shared.buildinfo.appBuildGit
import com.troubastack.shared.buildinfo.appBuildLine
import com.troubastack.shared.buildinfo.notifyAsk
import com.troubastack.shared.buildinfo.shouldAutoCheck
import com.troubastack.shared.buildinfo.updateVerdict
import com.troubastack.shared.buildinfo.utcMinute
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private const val K_LAST_CHECK = "appUpdate.lastCheckMs"
private const val K_DISMISSED = "appUpdate.dismissedCommit"
private const val K_AUTO = "appUpdate.autoChecks"
private const val K_NOTIFY_ASKED_BEFORE = "appUpdate.notifyAskedBefore"
private const val K_NOTIFY_ASKED_RELEASE = "appUpdate.notifyAskedRelease"

/**
 * [showNotifyRationale]: Android's shouldShowRequestPermissionRationale for POST_NOTIFICATIONS (it needs the Activity).
 * After a refusal for good it is false again, and Android will not show its prompt.
 */
class AppUpdateController(
    private val context: Context,
    private val storage: Storage,
    private val scope: CoroutineScope,
    private val showNotifyRationale: () -> Boolean = { false },
) {
    /** A question the person must answer before the download: mobile data, Android's "Install unknown apps", or
     *  (API 33+) the notification that reopens the app after the update. */
    enum class Ask { METERED, UNKNOWN_SOURCES, NOTIFY }

    var verdict by mutableStateOf<UpdateVerdict?>(null); private set
    var failed by mutableStateOf(false); private set
    var checking by mutableStateOf(false); private set
    /** A96 — the update in progress (downloading, checking, Android confirming), or null. */
    var phase by mutableStateOf<UpdatePhase?>(null); private set
    /** A plain sentence to show once (a refused or failed update, or "Updated · …"). */
    var message by mutableStateOf<String?>(null)
    var ask by mutableStateOf<Ask?>(null); private set
    private var job: Job? = null
    /** Set (on the install thread) the moment the session is committed: Android owns the update from then on, so a
     *  Cancel no longer applies and the status stays "Android will ask you…" even if the job was cancelled meanwhile. */
    @Volatile private var committed = false
    private var meteredAnswered = false
    private val marker = ReopenMarker(storage)

    init {
        // A96 — the first run after an update says so once, in WIN04's words. Nothing stored yet (the update INTO this
        // build): Android's install and update times tell an update from a fresh install.
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        updatedLineOnce(storage, appBuildLine, updatedSinceInstall = info.firstInstallTime != info.lastUpdateTime)?.let { message = it }
        ReopenNotice.cancel(context)
        UpdateSessionEvents.onNotInstalled = {
            phase = null
            message = UPDATE_NOT_INSTALLED_LINE
        }
    }

    /** The Automatic checks switch: the stored choice, else on for a "ci" build and off for a "dev" one (⟨D2⟩). */
    var autoChecks by mutableStateOf(storage.getSecret(K_AUTO)?.toBooleanStrictOrNull() ?: (appBuildChannel == "ci")); private set
    fun chooseAutoChecks(on: Boolean) { autoChecks = on; storage.putSecret(K_AUTO, on.toString()) }

    /** Called when Home shows (never from a concert): checks if the once-a-day rule says so. */
    fun maybeAutoCheck(inConcert: Boolean = false, noteBusy: Boolean = false) {
        val stored = storage.getSecret(K_AUTO)?.toBooleanStrictOrNull()
        val last = storage.getSecret(K_LAST_CHECK)?.toLongOrNull()
        if (shouldAutoCheck(System.currentTimeMillis(), last, appBuildChannel, stored, inConcert, noteBusy)) check(manual = false)
    }

    fun check(manual: Boolean) {
        if (checking || phase != null) return
        checking = true
        scope.launch {
            storage.putSecret(K_LAST_CHECK, System.currentTimeMillis().toString()) // a failure counts: try again tomorrow
            val release = AppUpdater.fetchLatest()
            failed = release == null
            if (release != null) verdict = updateVerdict(appBuildGit, appBuildBuiltAt, release, storage.getSecret(K_DISMISSED), manual)
            checking = false
        }
    }

    /** "Not now": hide exactly this release until a newer one appears. */
    fun notNow() {
        val r = (verdict as? UpdateVerdict.Available)?.release ?: return
        storage.putSecret(K_DISMISSED, r.commit)
        verdict = UpdateVerdict.Dismissed(r)
    }

    /** The banner's text: `App update available · built 2026-10-17 16:52 UTC`, or null when there is nothing to show. */
    val bannerText: String?
        get() = (verdict as? UpdateVerdict.Available)?.let { "App update available · built " + (utcMinute(it.release.publishedAt) ?: it.release.publishedAt) }

    /**
     * Update: ask about "Install unknown apps", mobile data and (API 33+) the reopening notification first — the
     * download starts only once they are answered — then download → verify → hand to Android.
     */
    fun update(meteredOk: Boolean = meteredAnswered, notifyAnswered: Boolean = false) {
        val release = when (val v = verdict) { is UpdateVerdict.Available -> v.release; is UpdateVerdict.Dismissed -> v.release; else -> return }
        if (job != null || phase != null) return
        if (!AppUpdater.canInstall(context)) { ask = Ask.UNKNOWN_SOURCES; return }
        if (!meteredOk && AppUpdater.onMeteredNetwork(context)) { ask = Ask.METERED; return }
        meteredAnswered = meteredOk
        if (!notifyAnswered && notifyAskNow(release.commit) == NotifyAsk.ASK) { ask = Ask.NOTIFY; return }
        ask = null
        meteredAnswered = false
        committed = false
        phase = UpdatePhase.Downloading(0f)
        job = scope.launch {
            try {
                val apk = AppUpdater.download(context, release) { phase = UpdatePhase.Downloading(it) }
                phase = UpdatePhase.Checking
                val versionCode = withContext(Dispatchers.IO) { AppUpdater.verify(context, apk, release) }
                marker.arm(versionCode)
                AppUpdater.install(context, apk) { committed = true }
            } catch (e: CancellationException) {
                throw e
            } catch (e: UpdateRefused) {
                message = e.message
            } catch (e: Exception) {
                message = "The update could not be downloaded. Check the connection and try again."
            } finally {
                if (committed) {
                    // Android shows its own confirmation, then ends this process to replace the app; the new version
                    // posts the notification that reopens it — or, when notifications can't show, the line says so.
                    phase = UpdatePhase.Confirming(reopenYourself = !ReopenNotice.canShow(context))
                } else {
                    marker.clear()
                    AppUpdater.dropDownload(context)
                    phase = null
                }
                job = null
            }
        }
    }

    /** Cancel while downloading or checking: the status goes back to Update at once. A check already running finishes
     *  in the background, its result is dropped, the package is never handed to Android, and the file is deleted. */
    fun cancel() {
        if (committed || phase is UpdatePhase.Confirming) return
        job?.cancel()
        phase = null
    }

    /** Whether to show the app's own line before Android's notification prompt (API 33+ only). */
    private fun notifyAskNow(releaseCommit: String): NotifyAsk {
        if (Build.VERSION.SDK_INT < 33) return NotifyAsk.NOT_NEEDED
        return notifyAsk(
            granted = context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED,
            askedForRelease = storage.getSecret(K_NOTIFY_ASKED_RELEASE) == releaseCommit,
            askedBefore = storage.getSecret(K_NOTIFY_ASKED_BEFORE) == "true",
            showRationale = showNotifyRationale(),
        )
    }

    /**
     * The notification question was answered — Android's prompt ran ([promptShown]), or our line was dismissed — so
     * this release asks no more, and the update goes ahead whatever the answer. Only a prompt that ran counts towards
     * "refused for good": a dismissed line never showed Android's prompt (whose rationale is false until it has).
     */
    fun notifyAnswered(promptShown: Boolean) {
        ask = null
        if (promptShown) storage.putSecret(K_NOTIFY_ASKED_BEFORE, "true")
        when (val v = verdict) {
            is UpdateVerdict.Available -> storage.putSecret(K_NOTIFY_ASKED_RELEASE, v.release.commit)
            is UpdateVerdict.Dismissed -> storage.putSecret(K_NOTIFY_ASKED_RELEASE, v.release.commit)
            else -> {}
        }
        update(notifyAnswered = true)
    }

    /** The answer to [ask]: go on (mobile data), or open Android's "Install unknown apps" page for TroubaStage. */
    fun answer(yes: Boolean) {
        val a = ask ?: return
        ask = null
        if (!yes) { meteredAnswered = false; return }
        when (a) {
            Ask.METERED -> update(meteredOk = true)
            Ask.UNKNOWN_SOURCES -> context.startActivity(
                Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${context.packageName}")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
            )
            Ask.NOTIFY -> notifyAnswered(promptShown = false)
        }
    }
}
