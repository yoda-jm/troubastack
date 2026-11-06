package com.troubastack.app

import android.Manifest
import android.util.Log
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.rememberCoroutineScope
import kotlinx.coroutines.delay
import com.troubastack.shared.distribution.embeddedUrl
import com.troubastack.shared.distribution.reloadUrl
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.WebViewHost

private const val CORE_URL_KEY = "coreUrl"

/**
 * Hosts the canonical TroubaStudio web editor in a WebView (I10). No native editor logic — login,
 * editing and realtime sync are all Studio's own.
 *
 * A16: a real app bar (title / back / overflow), NOT a URL bar, so Edit reads as an app screen rather
 * than a browser; the server URL moves into an overflow dialog. The load URL carries `?embedded=1`
 * (the signal Studio uses to hide its own nav/logout — web-core T46; persisted in sessionStorage so
 * SPA navigation keeps embedded mode). [initialPath] deep-links Studio to a context (e.g.
 * `/bands/{id}/songs/{id}`) when launched from one; null opens the band list. Back navigates Studio
 * history before leaving.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun EditScreen(storage: Storage, onBack: () -> Unit, initialPath: String? = null, bandTitle: String? = null) {
    val context = LocalContext.current
    var serverUrl by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: AndroidAppPlatform.DEFAULT_SERVER_URL) }
    var showSettings by remember { mutableStateOf(false) }
    var menuOpen by remember { mutableStateOf(false) }
    var state by remember { mutableStateOf<WebViewHost.State>(WebViewHost.State.Loading) }
    val loadUrl = embeddedUrl(serverUrl, initialPath)

    // A99: Studio's downloads — the line at the bottom, shown once, and Android's storage prompt (API 26–28) behind it.
    val scope = rememberCoroutineScope()
    var line by remember { mutableStateOf<DownloadLine?>(null) }
    var storageAsk by remember { mutableStateOf<DownloadLine.AskStorage?>(null) }
    val storagePrompt = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        storageAsk?.then?.invoke(granted)
        storageAsk = null
    }
    val (host, downloads) = remember {
        val h = WebViewHost(context)
        val d = StudioDownloads(context, storage, h.view, serverUrl = { serverUrl }, scope = scope, show = { line = it })
        h.view.addJavascriptInterface(d.Bridge(), StudioDownloads.INTERFACE_NAME) // before the first load
        h.view.setDownloadListener { url, _, contentDisposition, mimetype, _ -> d.onDownloadStart(url, contentDisposition, mimetype) }
        h.onState = { state = it; if (it == WebViewHost.State.Loaded) d.inject() }
        h.onStudioMessage { json -> Log.i("ShellBridge", "studio → shell: $json") }
        h to d
    }
    fun open(url: String) {
        // Seed the app's Connect session (B03) into the WebView so Edit doesn't re-prompt: the session
        // is an app-side ktor cookie; the WebView has its own jar. sessionCookieFor() returns it ONLY
        // when it was issued by this origin, so another server's session never leaks to a typed URL.
        seedSessionCookie(serverUrl, sessionCookieFor(storage::getSecret, serverUrl))
        state = WebViewHost.State.Loading
        host.load(url)
    }
    // A93: Reload and Retry stay on the page you are on (or the one that failed), not the entry page.
    fun reload() = open(reloadUrl(host.currentUrl, loadUrl))
    LaunchedEffect(loadUrl) { open(loadUrl) } // first load, and a new server's entry page

    BackHandler { if (host.canGoBack()) host.goBack() else onBack() }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                // A65: name where you are — the band when launched from a row, else the product. "Edit"
                // told you nothing about which band/server you'd opened.
                title = { Text(bandTitle?.ifBlank { null } ?: "TroubaStudio") },
                navigationIcon = { TextButton(onClick = onBack) { Text("‹  Back") } },
                actions = {
                    Box {
                        TextButton(onClick = { menuOpen = true }) { Text("⋮") }
                        DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                            DropdownMenuItem(text = { Text("Reload") }, onClick = { menuOpen = false; reload() })
                            DropdownMenuItem(text = { Text("Server URL…") }, onClick = { menuOpen = false; showSettings = true })
                        }
                    }
                },
            )
            Box(Modifier.weight(1f).fillMaxWidth()) {
                AndroidView(factory = { host.view }, modifier = Modifier.fillMaxSize())
                when (val s = state) {
                    WebViewHost.State.Loading -> Text("Loading…", Modifier.align(Alignment.Center))
                    is WebViewHost.State.Error -> ErrorCover(s.message, onRetry = { reload() }, onBack = onBack)
                    WebViewHost.State.Loaded -> Unit
                }
                line?.let { l ->
                    DownloadLineBar(
                        l,
                        onOpen = { saved -> downloads.open(saved); line = null },
                        onAllowStorage = { ask -> storageAsk = ask; line = null; storagePrompt.launch(Manifest.permission.WRITE_EXTERNAL_STORAGE) },
                        // Clear first: a refused storage ask then shows "Couldn't save …" (set by then(false)).
                        onDismiss = { line = null; if (l is DownloadLine.AskStorage) l.then(false) },
                        modifier = Modifier.align(Alignment.BottomCenter),
                    )
                    // "Saved" and "Couldn't save" go by themselves; the storage question waits for an answer.
                    if (l !is DownloadLine.AskStorage) LaunchedEffect(l) { delay(8_000); if (line == l) line = null }
                }
            }
        }
    }

    if (showSettings) {
        ServerDialog(serverUrl, onDismiss = { showSettings = false }) { newUrl ->
            serverUrl = newUrl.trim()
            dropSessionIfOriginChanged(storage, serverUrl) { _, _ -> clearWebViewCookies() } // new server ⇒ drop old session (Android drops all)
            storage.putSecret(CORE_URL_KEY, serverUrl)
            showSettings = false
        }
    }
}

/** Drop every cookie in the WebView's shared jar (sign-out; the server's origin changed). */
internal fun clearWebViewCookies() {
    android.webkit.CookieManager.getInstance().removeAllCookies(null)
}

/**
 * Seed the WebView's shared cookie jar with the app's persisted session (B03) for [url]'s origin, so
 * Studio sees the same login the app made via Connect. [cookie] is the stored "name=value" (the app
 * keeps only that part); a blank/absent session is a no-op (offline / never connected). A98: it is seeded
 * HttpOnly ([sessionCookieSeed]), as the server sets it — the WebView sends it, Studio's scripts can't read
 * it — and it replaces a cookie an older build seeded without the attribute (same name, domain and path).
 */
private fun seedSessionCookie(url: String, cookie: String?) {
    if (cookie.isNullOrEmpty()) return
    val cm = android.webkit.CookieManager.getInstance()
    cm.setAcceptCookie(true)
    cm.setCookie(url, sessionCookieSeed(cookie, url))
    cm.flush()
}

/** Opaque full-size cover shown instead of a blank WebView when the server can't be reached. */
@Composable
private fun ErrorCover(message: String, onRetry: () -> Unit, onBack: () -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center,
        ) {
            Text("Can't reach the server", style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
            Text(message, Modifier.padding(top = 8.dp), style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
            Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = onBack) { Text("Back") }
                OutlinedButton(onClick = onRetry) { Text("Retry") }
            }
        }
    }
}

@Composable
private fun ServerDialog(current: String, onDismiss: () -> Unit, onSave: (String) -> Unit) {
    var text by remember { mutableStateOf(current) }
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = { onSave(text) }) { Text("Save") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        title = { Text("Core server URL") },
        text = {
            Column {
                Text("The TroubaCore server to edit against (default is the emulator's host).")
                OutlinedTextField(value = text, onValueChange = { text = it }, singleLine = true, modifier = Modifier.padding(top = 8.dp))
            }
        },
    )
}

/** A99 — the one line at the bottom of the Studio screen after a download. */
@Composable
private fun DownloadLineBar(
    line: DownloadLine,
    onOpen: (DownloadLine.Saved) -> Unit,
    onAllowStorage: (DownloadLine.AskStorage) -> Unit,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(modifier.fillMaxWidth().padding(12.dp), shape = MaterialTheme.shapes.medium, tonalElevation = 6.dp, shadowElevation = 4.dp) {
        Row(Modifier.padding(horizontal = 16.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
            val text = when (line) {
                is DownloadLine.Saved -> "Saved to Downloads: ${line.name}"
                is DownloadLine.Failed -> "Couldn't save ${line.name}."
                is DownloadLine.AskStorage -> "Allow storage access so TroubaStage can save files to Downloads."
            }
            Text(text, Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
            when (line) {
                is DownloadLine.Saved -> TextButton(onClick = { onOpen(line) }) { Text("Open") }
                is DownloadLine.AskStorage -> TextButton(onClick = { onAllowStorage(line) }) { Text("Continue") }
                is DownloadLine.Failed -> Unit
            }
            TextButton(onClick = onDismiss) { Text(if (line is DownloadLine.AskStorage) "Not now" else "OK") }
        }
    }
}
