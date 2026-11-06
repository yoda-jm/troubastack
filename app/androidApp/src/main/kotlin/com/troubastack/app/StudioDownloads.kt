// A99 — Studio's downloads in the tablet app. The WebView had no download handling, so all five of Studio's downloads
// saved nothing (measured). Two kinds:
//  - a server URL (".tstage"): DownloadManager, with the session cookie only for its own origin (R1);
//  - a blob: URL (PDFs, the band export): Studio revokes most of them right after a.click(), so a capture script,
//    injected on every page load, takes the Blob in a capture-phase click listener (which runs before the revoke) and
//    sends it here in base64 chunks; the app writes it to Downloads.
// Studio's own code and its bridge are untouched.
package com.troubastack.app

import android.Manifest
import android.app.DownloadManager
import android.content.ActivityNotFoundException
import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.media.MediaScannerConnection
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.os.Handler
import android.os.Looper
import android.provider.MediaStore
import android.webkit.JavascriptInterface
import android.webkit.WebView
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.FutureTask
import java.util.concurrent.TimeUnit

/** What the Studio screen shows at its bottom, once. */
sealed interface DownloadLine {
    data class Saved(val name: String, val uri: Uri, val mime: String) : DownloadLine
    data class Failed(val name: String) : DownloadLine
    /** API 26–28: the line before Android's storage prompt; [then] saves (granted) or fails (refused). */
    data class AskStorage(val name: String, val then: (granted: Boolean) -> Unit) : DownloadLine
}

class StudioDownloads(
    private val context: Context,
    private val storage: Storage,
    private val webView: WebView,
    private val serverUrl: () -> String,
    private val scope: CoroutineScope,
    private val show: (DownloadLine) -> Unit,
) {
    private val main = Handler(Looper.getMainLooper())
    private val tempDir = File(context.cacheDir, "studio-downloads")
    private val transfers = BlobTransfers(allowed = ::topLevelOnServer, open = { File(tempDir.apply { mkdirs() }, "transfer.part").outputStream() })
    private val linkNames = ConcurrentHashMap<String, String>()

    /** R1 — read on the UI thread at every call (the interface methods run on WebView's bridge thread). */
    private fun topLevelOnServer(): Boolean {
        if (Looper.myLooper() == Looper.getMainLooper()) return downloadAllowed(webView.url, serverUrl())
        val task = FutureTask { webView.url }
        main.post(task)
        val top = runCatching { task.get(2, TimeUnit.SECONDS) }.getOrNull()
        return downloadAllowed(top, serverUrl())
    }

    /** Called on every page load: the capture script, once per document. */
    fun inject() = webView.evaluateJavascript(CAPTURE_SCRIPT, null)

    /** The page's side of the transfers (the capture script calls these). */
    inner class Bridge {
        @JavascriptInterface fun begin(id: String, name: String, mime: String, size: Double): Boolean = transfers.begin(id, name, mime, size.toLong())

        @JavascriptInterface fun chunk(id: String, index: Int, base64: String): Boolean = transfers.chunk(id, index, base64)

        @JavascriptInterface fun end(id: String) {
            val ended = transfers.end(id) ?: return
            main.post { if (ended.ok) saveBlob(ended.name, ended.mime) else show(DownloadLine.Failed(ended.name)) }
        }

        /** A server-URL `<a download>` click: hand it to DownloadManager under the link's name. */
        @JavascriptInterface fun download(url: String, name: String) {
            linkNames[url] = name
            main.post { if (topLevelOnServer()) serverDownload(url, null, null) }
        }
    }

    /** The WebView's own download (a navigation the server answered with an attachment). */
    fun onDownloadStart(url: String, contentDisposition: String?, mimetype: String?) {
        if (!topLevelOnServer()) return
        serverDownload(url, contentDisposition, mimetype)
    }

    private fun serverDownload(url: String, contentDisposition: String?, mimetype: String?) {
        val name = downloadFileName(contentDisposition, linkNames.remove(url), url)
        withStorage(name) {
            val dm = context.getSystemService(DownloadManager::class.java)
            val request = DownloadManager.Request(Uri.parse(url)).apply {
                downloadRequestHeaders(storage::getSecret, url, webView.settings.userAgentString).forEach { (k, v) -> addRequestHeader(k, v) }
                mimetype?.takeIf { it.isNotBlank() }?.let(::setMimeType)
                setTitle(name)
                setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
            }
            val id = runCatching { dm.enqueue(request) }.getOrNull() ?: return@withStorage show(DownloadLine.Failed(name))
            // Only the ids this screen started (the GO): watched while the screen is open.
            scope.launch {
                while (true) {
                    delay(1_000)
                    val status = withContext(Dispatchers.IO) {
                        dm.query(DownloadManager.Query().setFilterById(id))?.use { c -> if (c.moveToFirst()) c.getInt(c.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS)) else DownloadManager.STATUS_FAILED }
                    } ?: DownloadManager.STATUS_FAILED
                    when (status) {
                        DownloadManager.STATUS_SUCCESSFUL -> {
                            val uri = dm.getUriForDownloadedFile(id)
                            show(if (uri != null) DownloadLine.Saved(name, uri, dm.getMimeTypeForDownloadedFile(id) ?: "*/*") else DownloadLine.Failed(name))
                            return@launch
                        }
                        DownloadManager.STATUS_FAILED -> { show(DownloadLine.Failed(name)); return@launch }
                    }
                }
            }
        }
    }

    private fun saveBlob(name: String, mime: String) {
        val part = File(tempDir, "transfer.part")
        withStorage(name) {
            scope.launch {
                val uri = withContext(Dispatchers.IO) { runCatching { writeToDownloads(part, name, mime) }.getOrNull() }
                part.delete()
                show(if (uri != null) DownloadLine.Saved(name, uri, mime.ifBlank { "*/*" }) else DownloadLine.Failed(name))
            }
        }
    }

    /** API 26–28 need WRITE_EXTERNAL_STORAGE for the public Downloads folder (DownloadManager's too): asked once, with
     *  our line first; a refusal says "Couldn't save". From API 29 nothing is asked. */
    private fun withStorage(name: String, save: () -> Unit) {
        if (Build.VERSION.SDK_INT >= 29 || context.checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED) return save()
        show(DownloadLine.AskStorage(name) { granted -> if (granted) save() else show(DownloadLine.Failed(name)) })
    }

    private suspend fun writeToDownloads(part: File, name: String, mime: String): Uri? {
        if (Build.VERSION.SDK_INT >= 29) {
            val resolver = context.contentResolver
            val values = ContentValues().apply {
                put(MediaStore.Downloads.DISPLAY_NAME, name)
                if (mime.isNotBlank()) put(MediaStore.Downloads.MIME_TYPE, mime)
                put(MediaStore.Downloads.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS)
                put(MediaStore.Downloads.IS_PENDING, 1)
            }
            val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values) ?: return null
            resolver.openOutputStream(uri)?.use { out -> part.inputStream().use { it.copyTo(out) } } ?: run { resolver.delete(uri, null, null); return null }
            resolver.update(uri, ContentValues().apply { put(MediaStore.Downloads.IS_PENDING, 0) }, null, null)
            return uri
        }
        @Suppress("DEPRECATION")
        val dir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS).apply { mkdirs() }
        val target = uniqueFile(dir, name)
        part.inputStream().use { input -> target.outputStream().use { input.copyTo(it) } }
        // The media scan hands back the content URI that Open needs (a file:// URI can't be shared on API 24+).
        return kotlinx.coroutines.suspendCancellableCoroutine { cont ->
            MediaScannerConnection.scanFile(context, arrayOf(target.path), arrayOf(mime.ifBlank { null })) { _, uri -> cont.resumeWith(Result.success(uri)) }
        }
    }

    private fun uniqueFile(dir: File, name: String): File {
        var f = File(dir, name)
        val stem = name.substringBeforeLast('.', name)
        val ext = name.substringAfterLast('.', "").let { if (it.isEmpty() || it == name) "" else ".$it" }
        var n = 1
        while (f.exists()) f = File(dir, "$stem ($n)$ext").also { n++ }
        return f
    }

    /** Open: the system's viewer for it, else Android's Downloads app (a .tstage or a .zip may have no viewer). */
    fun open(line: DownloadLine.Saved) {
        val view = Intent(Intent.ACTION_VIEW).setDataAndType(line.uri, line.mime)
            .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        try { context.startActivity(view) } catch (e: ActivityNotFoundException) {
            runCatching { context.startActivity(Intent(DownloadManager.ACTION_VIEW_DOWNLOADS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
        }
    }

    companion object {
        const val INTERFACE_NAME = "TroubaStageDownloads"

        /**
         * Injected on every page load (onPageFinished; Studio only creates blobs on a person's action, long after load).
         * It remembers each Blob by its URL, and on a click on an `<a download>` (Studio appends each to the page before
         * a.click(), so this capture-phase listener runs before the URL is revoked): a blob: link is sent here in 384 KiB
         * chunks (512 KiB of base64); any other link is handed to DownloadManager under its download name.
         */
        val CAPTURE_SCRIPT = """
            (function () {
              if (window.__troubastageDownloads) return;
              var H = window.$INTERFACE_NAME;
              if (!H) return;
              window.__troubastageDownloads = true;
              var blobs = new Map();
              var create = URL.createObjectURL, revoke = URL.revokeObjectURL;
              URL.createObjectURL = function (o) { var u = create.call(URL, o); if (o instanceof Blob) blobs.set(u, o); return u; };
              URL.revokeObjectURL = function (u) { blobs.delete(u); return revoke.call(URL, u); };
              document.addEventListener("click", function (e) {
                var a = e.target && e.target.closest ? e.target.closest("a[download]") : null;
                if (!a || !a.href) return;
                var name = a.getAttribute("download") || "";
                if (a.href.indexOf("blob:") === 0) {
                  var blob = blobs.get(a.href);
                  if (!blob) return;
                  e.preventDefault();
                  send(blob, name);
                } else {
                  e.preventDefault();
                  H.download(a.href, name);
                }
              }, true);
              function send(blob, name) {
                var id = Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
                if (!H.begin(id, name, blob.type || "", blob.size)) return;
                var size = 393216, i = 0;
                (function next() {
                  if (i * size >= blob.size) { H.end(id); return; }
                  var r = new FileReader();
                  r.onload = function () { var s = r.result; if (H.chunk(id, i, s.substring(s.indexOf(",") + 1))) { i++; next(); } else { H.end(id); } };
                  r.onerror = function () { H.end(id); };
                  r.readAsDataURL(blob.slice(i * size, (i + 1) * size));
                })();
              }
            })();
        """.trimIndent()
    }
}
