// A88 — the app-update flow's state, for Home's banner and Parameters → About. Decisions are the pure :shared
// functions (shouldAutoCheck / updateVerdict); this keeps the two remembered values (last check, last "Not now") in
// the device's preferences and drives AppUpdater (download → verify → PackageInstaller).
package com.troubastack.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.buildinfo.appBuildBuiltAt
import com.troubastack.shared.buildinfo.appBuildChannel
import com.troubastack.shared.buildinfo.appBuildGit
import com.troubastack.shared.buildinfo.shouldAutoCheck
import com.troubastack.shared.buildinfo.updateVerdict
import com.troubastack.shared.buildinfo.utcMinute
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

private const val K_LAST_CHECK = "appUpdate.lastCheckMs"
private const val K_DISMISSED = "appUpdate.dismissedCommit"
private const val K_AUTO = "appUpdate.autoChecks"

class AppUpdateController(private val context: Context, private val storage: Storage, private val scope: CoroutineScope) {
    /** A question the person must answer before the download: mobile data, or Android's "Install unknown apps". */
    enum class Ask { METERED, UNKNOWN_SOURCES }

    var verdict by mutableStateOf<UpdateVerdict?>(null); private set
    var failed by mutableStateOf(false); private set
    var checking by mutableStateOf(false); private set
    /** 0..1 while the APK downloads; null otherwise. */
    var progress by mutableStateOf<Float?>(null); private set
    /** A plain sentence to show once (a refused or failed update). */
    var message by mutableStateOf<String?>(null)
    var ask by mutableStateOf<Ask?>(null); private set
    private var job: Job? = null

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
        if (checking || progress != null) return
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

    /** Update: ask about mobile data and "Install unknown apps" first, then download → verify → install. */
    fun update(meteredOk: Boolean = false) {
        val release = when (val v = verdict) { is UpdateVerdict.Available -> v.release; is UpdateVerdict.Dismissed -> v.release; else -> return }
        if (job != null) return
        if (!AppUpdater.canInstall(context)) { ask = Ask.UNKNOWN_SOURCES; return }
        if (!meteredOk && AppUpdater.onMeteredNetwork(context)) { ask = Ask.METERED; return }
        ask = null
        progress = 0f
        job = scope.launch {
            try {
                val apk = AppUpdater.download(context, release) { progress = it }
                AppUpdater.verify(context, apk, release)
                AppUpdater.install(context, apk) // Android shows its own confirmation; the app restarts on Home
            } catch (e: CancellationException) {
                throw e
            } catch (e: UpdateRefused) {
                message = e.message
            } catch (e: Exception) {
                message = "The update could not be downloaded. Check the connection and try again."
            } finally {
                progress = null; job = null
            }
        }
    }

    fun cancel() { job?.cancel() }

    /** The answer to [ask]: go on (mobile data), or open Android's "Install unknown apps" page for TroubaStage. */
    fun answer(yes: Boolean) {
        val a = ask ?: return
        ask = null
        if (!yes) return
        when (a) {
            Ask.METERED -> update(meteredOk = true)
            Ask.UNKNOWN_SOURCES -> context.startActivity(
                Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${context.packageName}")).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
            )
        }
    }
}
