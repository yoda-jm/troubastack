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
    // Parts left by a save that never got its storage answer (the screen closed while asking) go now.
    private val tempDir = File(context.cacheDir, "studio-downloads").apply { listFiles()?.forEach { it.delete() } }
    private val stamp = System.nanoTime()
    private fun partFile(part: Int) = File(tempDir, "transfer-$stamp-$part.part")
    private val transfers = BlobTransfers(allowed = ::topLevelOnServer, open = { part -> tempDir.mkdirs(); partFile(part).outputStream() })

    /** Saves waiting for the storage answer (API 26–28, main thread only): one answer covers them all. */
    private class Waiting(val name: String, val save: () -> Unit, val refused: () -> Unit)
    private val waiting = mutableListOf<Waiting>()

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
            val part = partFile(ended.part)
            main.post { if (ended.ok) saveBlob(ended.name, ended.mime, part) else { part.delete(); show(DownloadLine.Failed(ended.name)) } }
        }

        /** A server-URL `<a download>` click: hand it to DownloadManager under the link's name. */
        @JavascriptInterface fun download(url: String, name: String) {
            main.post { if (topLevelOnServer()) serverDownload(url, null, null, name) }
        }
    }

    /** The WebView's own download (a navigation the server answered with an attachment). */
    fun onDownloadStart(url: String, contentDisposition: String?, mimetype: String?) {
        if (!topLevelOnServer()) return
        serverDownload(url, contentDisposition, mimetype, null)
    }

    private fun serverDownload(url: String, contentDisposition: String?, mimetype: String?, linkName: String?) {
        val name = downloadFileName(contentDisposition, linkName, url)
        if (!downloadManagerCanFetch(url)) return show(DownloadLine.Failed(name))
        withStorage(name) {
            val dm = context.getSystemService(DownloadManager::class.java)
            // Building the request throws too (a Downloads folder it can't create): that's "Couldn't save", not a crash.
            val id = runCatching {
                dm.enqueue(DownloadManager.Request(Uri.parse(url)).apply {
                    downloadRequestHeaders(storage::getSecret, url, webView.settings.userAgentString).forEach { (k, v) -> addRequestHeader(k, v) }
                    mimetype?.takeIf { it.isNotBlank() }?.let(::setMimeType)
                    setTitle(name)
                    setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                    setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
                })
            }.getOrNull() ?: return@withStorage show(DownloadLine.Failed(name))
            // Only the ids this screen started (the GO): watched while the screen is open.
            scope.launch {
                while (true) {
                    delay(1_000)
                    val row = withContext(Dispatchers.IO) {
                        dm.query(DownloadManager.Query().setFilterById(id))?.use { c ->
                            if (!c.moveToFirst()) null
                            else c.getInt(c.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS)) to c.getString(c.getColumnIndexOrThrow(DownloadManager.COLUMN_LOCAL_URI))
                        }
                    } ?: (DownloadManager.STATUS_FAILED to null)
                    when (row.first) {
                        DownloadManager.STATUS_SUCCESSFUL -> {
                            val uri = dm.getUriForDownloadedFile(id)
                            // The name DownloadManager wrote ("…-1.tstage" when Downloads already had one).
                            val written = row.second?.let { Uri.parse(it).lastPathSegment } ?: name
                            show(if (uri != null) DownloadLine.Saved(written, uri, dm.getMimeTypeForDownloadedFile(id) ?: "*/*") else DownloadLine.Failed(name))
                            return@launch
                        }
                        DownloadManager.STATUS_FAILED -> { show(DownloadLine.Failed(name)); return@launch }
                    }
                }
            }
        }
    }

    private fun saveBlob(name: String, mime: String, part: File) {
        withStorage(name, refused = { part.delete() }) {
            scope.launch {
                val saved = withContext(Dispatchers.IO) { runCatching { writeToDownloads(part, name, mime) }.getOrNull() }
                part.delete()
                show(saved ?: DownloadLine.Failed(name))
            }
        }
    }

    /** API 26–28 need WRITE_EXTERNAL_STORAGE for the public Downloads folder (DownloadManager's too): our line first,
     *  then Android's prompt; every save that came in while asking waits for the same answer, and a refusal says
     *  "Couldn't save". From API 29 nothing is asked. */
    private fun withStorage(name: String, refused: () -> Unit = {}, save: () -> Unit) {
        if (Build.VERSION.SDK_INT >= 29 || context.checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED) return save()
        waiting += Waiting(name, save, refused)
        // Asked again for each one, so a line shown in between can't leave a save unasked.
        show(DownloadLine.AskStorage(name, ::storageAnswered))
    }

    private fun storageAnswered(granted: Boolean) {
        val answered = waiting.toList().also { waiting.clear() }
        for (w in answered) if (granted) w.save() else { w.refused(); show(DownloadLine.Failed(w.name)) }
    }

    /** The saved file, under the name it was written as (MediaStore and the 26–28 path add " (1)" to a taken name). */
    private suspend fun writeToDownloads(part: File, name: String, mime: String): DownloadLine.Saved? {
        val type = mime.ifBlank { "*/*" }
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
            val written = resolver.query(uri, arrayOf(MediaStore.Downloads.DISPLAY_NAME), null, null, null)?.use { c -> if (c.moveToFirst()) c.getString(0) else null }
            return DownloadLine.Saved(written ?: name, uri, type)
        }
        @Suppress("DEPRECATION")
        val dir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS).apply { mkdirs() }
        val target = uniqueFile(dir, name)
        part.inputStream().use { input -> target.outputStream().use { input.copyTo(it) } }
        // The media scan hands back the content URI that Open needs (a file:// URI can't be shared on API 24+).
        val uri = kotlinx.coroutines.suspendCancellableCoroutine<Uri?> { cont ->
            MediaScannerConnection.scanFile(context, arrayOf(target.path), arrayOf(mime.ifBlank { null })) { _, uri -> cont.resumeWith(Result.success(uri)) }
        }
        return uri?.let { DownloadLine.Saved(target.name, it, type) }
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
