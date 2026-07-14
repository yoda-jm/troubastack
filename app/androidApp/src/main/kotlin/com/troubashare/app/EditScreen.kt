package com.troubashare.app

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
import com.troubashare.shared.distribution.embeddedUrl
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.seams.WebViewHost

private const val CORE_URL_KEY = "coreUrl"
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080" // host machine, from the Android emulator

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
fun EditScreen(storage: Storage, onBack: () -> Unit, initialPath: String? = null) {
    val context = LocalContext.current
    var serverUrl by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL) }
    var showSettings by remember { mutableStateOf(false) }
    var menuOpen by remember { mutableStateOf(false) }
    var state by remember { mutableStateOf<WebViewHost.State>(WebViewHost.State.Loading) }
    val loadUrl = embeddedUrl(serverUrl, initialPath)

    val host = remember {
        WebViewHost(context).apply {
            onState = { state = it }
            onStudioMessage { json -> Log.i("ShellBridge", "studio → shell: $json") }
        }
    }
    fun reload() {
        // Seed the app's Connect session (B03) into the WebView so Edit doesn't re-prompt: the session
        // is an app-side ktor cookie; the WebView has its own jar. sessionCookieFor() returns it ONLY
        // when it was issued by this origin, so another server's session never leaks to a typed URL.
        seedSessionCookie(serverUrl, sessionCookieFor(storage, serverUrl))
        state = WebViewHost.State.Loading
        host.load(loadUrl)
    }
    LaunchedEffect(loadUrl) { reload() }

    BackHandler { if (host.canGoBack()) host.goBack() else onBack() }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                title = { Text("Edit") },
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
            }
        }
    }

    if (showSettings) {
        ServerDialog(serverUrl, onDismiss = { showSettings = false }) { newUrl ->
            serverUrl = newUrl.trim()
            dropSessionIfOriginChanged(storage, serverUrl) // pointing at a new server ⇒ drop the old session
            storage.putSecret(CORE_URL_KEY, serverUrl)
            showSettings = false
        }
    }
}

/**
 * Seed the WebView's shared cookie jar with the app's persisted session (B03) for [url]'s origin, so
 * Studio sees the same login the app made via Connect. [cookie] is the stored "name=value" (the app
 * keeps only that part); a blank/absent session is a no-op (offline / never connected). HttpOnly is
 * fine — the WebView still sends it; JS just can't read it.
 */
private fun seedSessionCookie(url: String, cookie: String?) {
    if (cookie.isNullOrEmpty()) return
    val cm = android.webkit.CookieManager.getInstance()
    cm.setAcceptCookie(true)
    cm.setCookie(url, cookie)
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
