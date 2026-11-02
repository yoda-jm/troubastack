// A88 — the Android half of the in-app update: fetch the GitHub `latest` release, download its APK with progress,
// VERIFY it (same signing certificate as the installed app, same package, the advertised size) and hand it to
// PackageInstaller — Android then asks the person to confirm; the install is never silent and never uninstalls.
// The decision of WHETHER to offer it is the pure :shared code (updateVerdict / shouldAutoCheck).
//
// A plain HttpURLConnection on purpose, not HttpTransport: GitHub must never receive the band server's session
// cookie, and this talks to nobody else.
package com.troubastack.app

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInfo
import android.content.pm.PackageInstaller
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.os.Build
import com.troubastack.shared.buildinfo.LatestRelease
import com.troubastack.shared.buildinfo.UPDATE_ASSET_APK
import com.troubastack.shared.buildinfo.parseRelease
import com.troubastack.shared.buildinfo.UPDATE_RELEASE_URL
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import kotlin.coroutines.coroutineContext

/** Why a downloaded update was refused (shown in plain words; never followed by an uninstall offer). */
class UpdateRefused(message: String) : Exception(message)

object AppUpdater {
    private const val TIMEOUT_MS = 10_000

    /** The `latest` release, or null when it can't be read (offline, rate limit, no APK asset). */
    suspend fun fetchLatest(): LatestRelease? = withContext(Dispatchers.IO) {
        runCatching {
            val c = (URL(UPDATE_RELEASE_URL).openConnection() as HttpURLConnection).apply {
                connectTimeout = TIMEOUT_MS; readTimeout = TIMEOUT_MS
                setRequestProperty("Accept", "application/vnd.github+json")
                setRequestProperty("User-Agent", "TroubaStage")
            }
            try {
                if (c.responseCode != 200) return@runCatching null
                parseRelease(c.inputStream.bufferedReader().use { it.readText() }, UPDATE_ASSET_APK)
            } finally { c.disconnect() }
        }.getOrNull()
    }

    /** True on a metered connection (mobile data): the ~15 MB download is confirmed first (§2). */
    fun onMeteredNetwork(context: Context): Boolean =
        onMeteredNetwork { (context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager).isActiveNetworkMetered }

    /**
     * A95 — never throws. Every build from A88 to A95 crashed on Update: reading the network needs
     * ACCESS_NETWORK_STATE, which the manifest did not declare (a SecurityException). When Android will not say
     * (that, or no connectivity service), the network counts as metered: asking once about mobile data is harmless,
     * a crash is not.
     */
    internal fun onMeteredNetwork(isMetered: () -> Boolean): Boolean =
        try { isMetered() } catch (e: Exception) { true }

    /** Download [release]'s APK to the app cache, reporting progress 0..1. Cancellable. */
    suspend fun download(context: Context, release: LatestRelease, onProgress: (Float) -> Unit): File = withContext(Dispatchers.IO) {
        val dir = File(context.cacheDir, "app-update").apply { deleteRecursively(); mkdirs() }
        val out = File(dir, UPDATE_ASSET_APK)
        val c = (URL(release.assetUrl).openConnection() as HttpURLConnection).apply {
            connectTimeout = TIMEOUT_MS; readTimeout = 30_000; instanceFollowRedirects = true
            setRequestProperty("User-Agent", "TroubaStage")
        }
        try {
            if (c.responseCode != 200) throw UpdateRefused("The download failed (HTTP ${c.responseCode}).")
            c.inputStream.use { input ->
                out.outputStream().use { output ->
                    val buf = ByteArray(64 * 1024)
                    var done = 0L
                    while (true) {
                        coroutineContext.ensureActive()
                        val n = input.read(buf)
                        if (n < 0) break
                        output.write(buf, 0, n); done += n
                        onProgress((done.toFloat() / release.assetSize).coerceIn(0f, 1f))
                    }
                }
            }
        } finally { c.disconnect() }
        out
    }

    /** §2 step 2 — refuse anything that is not this app, signed by this app's key, at the advertised size. */
    fun verify(context: Context, apk: File, release: LatestRelease) {
        if (apk.length() != release.assetSize) throw UpdateRefused("The download is incomplete (${apk.length()} of ${release.assetSize} bytes). Try again.")
        val pm = context.packageManager
        val archive = archiveInfo(pm, apk) ?: throw UpdateRefused("The downloaded file is not an app TroubaStage can install.")
        if (archive.packageName != context.packageName) throw UpdateRefused("The downloaded app is not TroubaStage (${archive.packageName}).")
        val installed = installedInfo(pm, context.packageName)
        val mine = certDigests(installed)
        val theirs = certDigests(archive)
        if (mine.isEmpty() || theirs != mine) {
            throw UpdateRefused("This update is signed with a different key than the TroubaStage on this tablet, so Android would not install it over this one. Your notes are safe; nothing was changed.")
        }
    }

    /** §2 step 3 — hand the verified APK to PackageInstaller. Android then shows its own confirmation. */
    fun install(context: Context, apk: File) {
        val installer = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL).apply {
            setAppPackageName(context.packageName)
            setSize(apk.length())
        }
        val id = installer.createSession(params)
        installer.openSession(id).use { session ->
            session.openWrite("troubastage.apk", 0, apk.length()).use { out -> apk.inputStream().use { it.copyTo(out) }; session.fsync(out) }
            val intent = Intent(context, UpdateInstallReceiver::class.java)
            val flags = PendingIntent.FLAG_UPDATE_CURRENT or (if (Build.VERSION.SDK_INT >= 31) PendingIntent.FLAG_MUTABLE else 0)
            session.commit(PendingIntent.getBroadcast(context, id, intent, flags).intentSender)
        }
    }

    /** Whether Android lets TroubaStage install apps; if not, the person is sent to "Install unknown apps" once. */
    fun canInstall(context: Context): Boolean = context.packageManager.canRequestPackageInstalls()

    @Suppress("DEPRECATION")
    private fun archiveInfo(pm: PackageManager, apk: File): PackageInfo? =
        if (Build.VERSION.SDK_INT >= 28) pm.getPackageArchiveInfo(apk.path, PackageManager.GET_SIGNING_CERTIFICATES)
        else pm.getPackageArchiveInfo(apk.path, PackageManager.GET_SIGNATURES)

    @Suppress("DEPRECATION")
    private fun installedInfo(pm: PackageManager, pkg: String): PackageInfo =
        if (Build.VERSION.SDK_INT >= 28) pm.getPackageInfo(pkg, PackageManager.GET_SIGNING_CERTIFICATES)
        else pm.getPackageInfo(pkg, PackageManager.GET_SIGNATURES)

    /** SHA-256 of each signing certificate (current signers; a rotated key's history is not accepted as a match). */
    @Suppress("DEPRECATION")
    internal fun certDigests(info: PackageInfo): Set<String> {
        val sigs = if (Build.VERSION.SDK_INT >= 28) info.signingInfo?.let { si -> if (si.hasMultipleSigners()) si.apkContentsSigners else si.signingCertificateHistory?.takeLast(1)?.toTypedArray() } else info.signatures
        return sigs.orEmpty().map { s -> MessageDigest.getInstance("SHA-256").digest(s.toByteArray()).joinToString("") { "%02x".format(it) } }.toSet()
    }
}

/** PackageInstaller's status callback. When Android needs the person's confirmation, show its prompt; other statuses
 *  (success restarts the app; failure/abort) need nothing here — the app reopens on Home after an update. */
class UpdateInstallReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE) == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            @Suppress("DEPRECATION")
            val confirm = (if (Build.VERSION.SDK_INT >= 33) intent.getParcelableExtra(Intent.EXTRA_INTENT, Intent::class.java) else intent.getParcelableExtra(Intent.EXTRA_INTENT)) ?: return
            context.startActivity(confirm.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        }
    }
}
