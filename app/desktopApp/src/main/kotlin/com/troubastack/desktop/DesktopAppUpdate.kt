package com.troubastack.desktop

import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.troubastack.app.AppUpdate
import com.troubastack.shared.buildinfo.LatestRelease
import com.troubastack.shared.buildinfo.UPDATE_ASSET_APPIMAGE
import com.troubastack.shared.buildinfo.UPDATE_ASSET_DMG
import com.troubastack.shared.buildinfo.UPDATE_ASSET_MSI
import com.troubastack.shared.buildinfo.UPDATE_RELEASE_URL
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.buildinfo.appBuildBuiltAt
import com.troubastack.shared.buildinfo.appBuildChannel
import com.troubastack.shared.buildinfo.appBuildGit
import com.troubastack.shared.buildinfo.appBuildLine
import com.troubastack.shared.buildinfo.parseRelease
import com.troubastack.shared.buildinfo.shouldAutoCheck
import com.troubastack.shared.buildinfo.updateVerdict
import com.troubastack.shared.buildinfo.utcMinute
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant
import kotlin.coroutines.coroutineContext

private const val K_LAST_CHECK = "appUpdate.lastCheckMs"
private const val K_DISMISSED = "appUpdate.dismissedCommit"
private const val K_AUTO = "appUpdate.autoChecks"
private const val K_HANDOFF_FROM = "appUpdate.handoffFrom"
private const val TIMEOUT_MS = 10_000

/** H4 — the line a copy that cannot install shows instead of Update. */
const val CANNOT_INSTALL_LINE = "This copy of TroubaStage can't update itself: install it from the MSI on the latest release."

/** DESK01 (link mode, Linux and macOS) — the line under Update, which opens the download instead of installing. */
const val OPENS_DOWNLOAD_LINE = "Opens the download in your browser"

/** DESK01 D4 — the `latest` asset that is this OS's app. */
fun updateAssetFor(os: HostOs): String = when (os) {
    HostOs.WINDOWS -> UPDATE_ASSET_MSI
    HostOs.LINUX -> UPDATE_ASSET_APPIMAGE
    HostOs.MACOS -> UPDATE_ASSET_DMG
}

/**
 * WIN04 — the Windows app's update (A88's twin): the shared decisions (updateVerdict, shouldAutoCheck), the same
 * surfaces and words (Home's banner, Parameters' rows), the MSI from the `latest` release.
 *
 * Fetching uses a plain HttpURLConnection with no cookie handler, so GitHub never sees the band server's session. The
 * download is verified (size, and SHA-256 when the release gives it) before anything runs. Install hands the MSI to the
 * fixed helper script ([UPDATE_SCRIPT]) and closes the app ([exitForInstall]); the helper installs and reopens it.
 *
 * [installedExe] is the MSI-installed launcher, or null when this copy is not one (H4): then Update is not offered.
 * [releaseUrl] and [hookMode] are the CI hook's (`--update-from`, C1): its release AND the installer it names must be
 * on loopback, a digest is required, and no redirect is followed. Normal use is GitHub's release over https.
 *
 * DESK01 — [openInBrowser] set is the LINK mode (Linux, macOS): the same check, banner and rows for [assetName], and
 * Update opens the asset's https download in the browser instead of installing it ([OPENS_DOWNLOAD_LINE] says so).
 */
class DesktopAppUpdate(
    private val storage: Storage,
    private val scope: CoroutineScope,
    private val dataRoot: File,
    private val installedExe: File?,
    private val releaseUrl: String = UPDATE_RELEASE_URL,
    private val hookMode: Boolean = false,
    private val assetName: String = UPDATE_ASSET_MSI,
    private val openInBrowser: ((String) -> Unit)? = null,
    private val exitForInstall: () -> Unit,
) : AppUpdate {
    private val linkMode: Boolean get() = openInBrowser != null
    enum class Ask { CONFIRM_INSTALL }

    override var verdict by mutableStateOf<UpdateVerdict?>(null); private set
    override var failed by mutableStateOf(false); private set
    override var checking by mutableStateOf(false); private set
    override var progress by mutableStateOf<Float?>(null); private set
    var message by mutableStateOf<String?>(null)
    var ask by mutableStateOf<Ask?>(null); private set
    private var job: Job? = null
    private var verified: File? = null

    override var autoChecks by mutableStateOf(storage.getSecret(K_AUTO)?.toBooleanStrictOrNull() ?: (appBuildChannel == "ci")); private set
    override fun chooseAutoChecks(on: Boolean) { autoChecks = on; storage.putSecret(K_AUTO, on.toString()) }

    private val updateDir: File get() = File(dataRoot, "tmp/app-update")

    override val installNote: String? get() = if (!linkMode && installedExe == null) CANNOT_INSTALL_LINE else null

    override val actionNote: String? get() = if (linkMode) OPENS_DOWNLOAD_LINE else null

    override val bannerText: String?
        get() = if (!linkMode && installedExe == null) null
        else (verdict as? UpdateVerdict.Available)?.let { "App update available · built " + (utcMinute(it.release.publishedAt) ?: it.release.publishedAt) }

    override fun maybeAutoCheck() {
        val stored = storage.getSecret(K_AUTO)?.toBooleanStrictOrNull()
        val last = storage.getSecret(K_LAST_CHECK)?.toLongOrNull()
        if (shouldAutoCheck(System.currentTimeMillis(), last, appBuildChannel, stored, inConcert = false, noteBusy = false)) check(manual = false)
    }

    override fun check(manual: Boolean) {
        if (checking || progress != null) return
        checking = true
        scope.launch { checkNow(manual); checking = false }
    }

    /** The check itself (the button's and the CI hook's): the release, then the shared verdict. */
    suspend fun checkNow(manual: Boolean): UpdateVerdict? {
        storage.putSecret(K_LAST_CHECK, System.currentTimeMillis().toString()) // a failure counts: try again tomorrow
        val fetched = fetchLatest()
        failed = fetched == null
        // A release without this OS's asset is no update here, not a failure (e.g. an older release predating it).
        verdict = fetched?.release?.let { updateVerdict(appBuildGit, appBuildBuiltAt, it, storage.getSecret(K_DISMISSED), manual) }
        return verdict
    }

    override fun notNow() {
        val r = (verdict as? UpdateVerdict.Available)?.release ?: return
        storage.putSecret(K_DISMISSED, r.commit)
        verdict = UpdateVerdict.Dismissed(r)
    }

    /** Update: download and verify the MSI, then ask once before closing to install it. */
    override fun update() {
        val release = when (val v = verdict) { is UpdateVerdict.Available -> v.release; is UpdateVerdict.Dismissed -> v.release; else -> return }
        openInBrowser?.let { open ->
            message = runCatching { checkAssetUrl(release.assetUrl, hook = false); open(release.assetUrl) }
                .exceptionOrNull()?.let { (it as? UpdateRefused)?.message ?: "The download page could not be opened." }
            return
        }
        if (installedExe == null) { message = CANNOT_INSTALL_LINE; return }
        if (job != null) return
        progress = 0f
        job = scope.launch {
            try {
                verified = downloadAndVerify(release)
                ask = Ask.CONFIRM_INSTALL
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

    override fun cancel() { job?.cancel() }

    /** The answer to [ask]: Install hands off and closes the app; Not now keeps the verified MSI for the next Update. */
    fun answer(install: Boolean) {
        ask = null
        if (install) verified?.let { install(it) }
    }

    /** The download (with progress) and its verification — the button's and the CI hook's path. */
    suspend fun downloadAndVerify(release: LatestRelease): File = withContext(Dispatchers.IO) {
        checkAssetUrl(release.assetUrl, hook = hookMode) // before anything is fetched (5a403976)
        val dir = updateDir.apply { deleteRecursively(); mkdirs() }
        val out = File(dir, UPDATE_ASSET_MSI)
        val c = (URL(release.assetUrl).openConnection() as HttpURLConnection).apply {
            connectTimeout = TIMEOUT_MS; readTimeout = 30_000
            instanceFollowRedirects = !hookMode // GitHub redirects to its file host (https → https); the hook may not
            setRequestProperty("User-Agent", "TroubaStage")
        }
        try {
            if (c.responseCode != 200) throw UpdateRefused("The download failed (HTTP ${c.responseCode}). Nothing was installed.")
            c.inputStream.use { input ->
                out.outputStream().use { output ->
                    val buf = ByteArray(64 * 1024)
                    var done = 0L
                    while (true) {
                        coroutineContext.ensureActive()
                        val n = input.read(buf)
                        if (n < 0) break
                        output.write(buf, 0, n); done += n
                        progress = (done.toFloat() / release.assetSize).coerceIn(0f, 1f)
                    }
                }
            }
        } finally { c.disconnect() }
        verifyDownload(out, release, requireDigest = hookMode)
        out
    }

    /** H1 — write the fixed helper and start it with its inputs in its environment, then close the app. */
    fun install(msi: File) {
        val exe = installedExe ?: run { message = CANNOT_INSTALL_LINE; return }
        val dir = msi.absoluteFile.parentFile
        val script = File(dir, "update.cmd").apply { writeText(UPDATE_SCRIPT) }
        val result = File(dir, "result.txt").apply { delete() }
        storage.putSecret(K_HANDOFF_FROM, appBuildLine)
        ProcessBuilder("cmd.exe", "/d", "/c", script.name)
            .directory(dir)
            .redirectInput(ProcessBuilder.Redirect.from(File("NUL")))
            .redirectOutput(File(dir, "script.log"))
            .redirectErrorStream(true)
            .apply {
                environment()[SCRIPT_ENV_PID] = ProcessHandle.current().pid().toString()
                environment()[SCRIPT_ENV_MSI] = msi.absolutePath
                environment()[SCRIPT_ENV_LOG] = File(dir, "msiexec.log").absolutePath
                environment()[SCRIPT_ENV_EXE] = exe.absolutePath
                environment()[SCRIPT_ENV_RESULT] = result.absolutePath
            }
            .start()
        exitForInstall()
    }

    /**
     * At start: what the helper recorded for the last handoff, if anything — shown once, and written to
     * `<data root>\last-update.txt` (time, from, to, result) for diagnosis and for CI. The helper's file is consumed.
     */
    fun reportLastHandoff() {
        val resultFile = File(updateDir, "result.txt")
        val result = handoffResult(resultFile.takeIf { it.isFile }?.readText()) ?: return
        resultFile.delete()
        val from = storage.getSecret(K_HANDOFF_FROM) ?: "unknown"
        val (line, summary) = when (result) {
            HandoffResult.Updated -> "Updated · $appBuildLine" to "success"
            HandoffResult.RestartWindows -> "Updated — restart Windows to finish." to "restart-windows (msiexec 3010)"
            HandoffResult.NotClosed -> "The update did not start: TroubaStage did not close in time. Nothing was changed." to "the app did not close"
            is HandoffResult.Failed -> "The update could not be installed (Windows Installer code ${result.code}). TroubaStage is unchanged." to "failed (msiexec ${result.code})"
        }
        message = line
        runCatching {
            File(dataRoot, "last-update.txt").writeText(
                "time=${Instant.now()}\nfrom=$from\nto=$appBuildLine\nversion=${System.getProperty("jpackage.app-version") ?: "unknown"}\nresult=$summary\n",
            )
        }
    }

    /** The release as fetched: [release] is null when it has no asset for this OS. */
    private class Fetched(val release: LatestRelease?)

    /** null: the check failed (network, HTTP, unreadable JSON). */
    private suspend fun fetchLatest(): Fetched? = withContext(Dispatchers.IO) {
        runCatching {
            val c = (URL(releaseUrl).openConnection() as HttpURLConnection).apply {
                connectTimeout = TIMEOUT_MS; readTimeout = TIMEOUT_MS
                setRequestProperty("Accept", "application/vnd.github+json")
                setRequestProperty("User-Agent", "TroubaStage")
            }
            try {
                if (c.responseCode != 200) return@runCatching null
                Fetched(parseRelease(c.inputStream.bufferedReader().use { it.readText() }, assetName))
            } finally { c.disconnect() }
        }.getOrNull()
    }

    @Composable
    override fun Dialogs() {
        if (ask == Ask.CONFIRM_INSTALL) {
            AlertDialog(
                onDismissRequest = { answer(false) },
                title = { Text("Install the update now?") },
                text = { Text("TroubaStage closes, installs it and opens again.") },
                confirmButton = { TextButton(onClick = { answer(true) }) { Text("Install") } },
                dismissButton = { TextButton(onClick = { answer(false) }) { Text("Not now") } },
            )
        }
        message?.let { m ->
            AlertDialog(
                onDismissRequest = { message = null },
                title = { Text("App update") },
                text = { Text(m) },
                confirmButton = { TextButton(onClick = { message = null }) { Text("OK") } },
            )
        }
    }
}
