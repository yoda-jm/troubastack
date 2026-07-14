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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.seams.WebViewHost

private const val CORE_URL_KEY = "coreUrl"
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080" // host machine, from the Android emulator

/**
 * Hosts the canonical TroubaStudio web editor in a WebView (I10). No native editor logic — login,
 * editing and realtime sync are all Studio's own. Back navigates Studio history before leaving.
 */
@Composable
fun EditScreen(storage: Storage, onBack: () -> Unit) {
    val context = LocalContext.current
    var url by remember { mutableStateOf(storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL) }
    var showSettings by remember { mutableStateOf(false) }
    var state by remember { mutableStateOf<WebViewHost.State>(WebViewHost.State.Loading) }

    val host = remember {
        WebViewHost(context).apply {
            onState = { state = it }
            onStudioMessage { json -> Log.i("ShellBridge", "studio → shell: $json") }
        }
    }
    LaunchedEffect(url) {
        // Carry the app's Connect session (B03) into the WebView so Edit doesn't make you log in
        // again: the session lives as an app-side ktor cookie; the WebView has its own jar, so seed
        // CookieManager before loading Studio. sessionCookieFor() only returns the cookie when it was
        // issued by THIS origin, so we never hand another server's session to a user-typed URL.
        seedSessionCookie(url, sessionCookieFor(storage, url))
        state = WebViewHost.State.Loading
        host.load(url)
    }

    BackHandler { if (host.canGoBack()) host.goBack() else onBack() }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().statusBarsPadding()) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextButton(onClick = onBack) { Text("Back") }
                Text(
                    url,
                    Modifier.weight(1f).padding(horizontal = 8.dp),
                    style = MaterialTheme.typography.labelLarge,
                    maxLines = 1,
                )
                TextButton(onClick = { showSettings = true }) { Text("Server") }
            }
            Box(Modifier.fillMaxSize()) {
                AndroidView(factory = { host.view }, modifier = Modifier.fillMaxSize())
                when (val s = state) {
                    WebViewHost.State.Loading -> Text("Loading…", Modifier.align(Alignment.Center))
                    is WebViewHost.State.Error -> ErrorCover(s.message, onRetry = { state = WebViewHost.State.Loading; host.load(url) }, onBack = onBack)
                    WebViewHost.State.Loaded -> Unit
                }
            }
        }
    }

    if (showSettings) {
        ServerDialog(url, onDismiss = { showSettings = false }) { newUrl ->
            url = newUrl.trim()
            dropSessionIfOriginChanged(storage, url) // pointing at a new server ⇒ drop the old session
            storage.putSecret(CORE_URL_KEY, url)
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
