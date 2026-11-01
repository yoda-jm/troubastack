package com.troubastack.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import com.troubastack.shared.ui.ThemePref
import com.troubastack.shared.ui.TroubaTheme
import androidx.compose.runtime.setValue
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.stage.PageTurn

/**
 * The thin Android entrypoint (I15). WIN01: the app itself is the shared [App] (jvmSharedMain); this is its glue —
 * the theme holder, the deep-link intents, resume, the volume keys — and [AndroidAppPlatform] hands App() the
 * Android capabilities.
 */
class MainActivity : ComponentActivity() {
    // Set by App() only while a Stage is open (A09) so the hardware VOLUME keys turn pages; null
    // otherwise → volume behaves normally. Volume keys don't reach Compose, so they're intercepted
    // here (keyboard/pedal keys are handled in the shared StageScreen via onPreviewKeyEvent).
    var stageVolumeTurn: ((PageTurn) -> Unit)? = null

    // A31: bumped on every ON_RESUME so Home can re-run its live connection probe when the app comes
    // back to the foreground (not just on nav re-entry). A Compose State, so reads are tracked.
    val resumeTick = mutableStateOf(0)

    // A52: the invite link that arrived via a VIEW intent (the /join/ deep link), exposed to Compose and
    // consumed (then cleared) by the join sheet. A bearer token rides in this string, so it lives ONLY in
    // this in-memory State — never persisted. Set on cold start (onCreate) and while running (onNewIntent).
    val pendingLink = mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        pendingLink.value = inviteLinkFrom(intent)
        setContent {
            // A36: the theme choice lives ABOVE TroubaTheme (it drives it), so hold it here and thread
            // the setter down to the Parameters screen. Persisted; defaults to SYSTEM.
            val themeStore = remember { Storage(applicationContext) }
            var themePref by remember { mutableStateOf(ThemePref.parse(themeStore.getSecret(ThemePref.KEY))) }
            val platform = remember { AndroidAppPlatform(this@MainActivity) }
            TroubaTheme(dark = themePref.resolveDark()) {
                App(platform, themePref = themePref, onThemePref = { themePref = it; themeStore.putSecret(ThemePref.KEY, it.name) })
            }
        }
    }

    // A52: with launchMode=singleTask, a /join/ deep link into the RUNNING app is delivered here (not a
    // fresh Activity). setIntent so getIntent() reflects it; publish it for the join sheet.
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        inviteLinkFrom(intent)?.let { pendingLink.value = it }
    }

    override fun onResume() {
        super.onResume()
        resumeTick.value++
    }

    override fun onKeyDown(keyCode: Int, event: android.view.KeyEvent?): Boolean {
        stageVolumeTurn?.let { turn ->
            when (keyCode) {
                android.view.KeyEvent.KEYCODE_VOLUME_DOWN -> { turn(PageTurn.NEXT); return true }
                android.view.KeyEvent.KEYCODE_VOLUME_UP -> { turn(PageTurn.PREV); return true }
            }
        }
        return super.onKeyDown(keyCode, event)
    }
}

// A52: the raw invite URL of a VIEW (deep-link) intent, or null for a plain launch. Kept as a string so
// the shared parser (A51's parseTroubaLink) — not Android's Uri — is the single grammar the app trusts.
private fun inviteLinkFrom(intent: Intent?): String? =
    intent?.takeIf { it.action == Intent.ACTION_VIEW }?.data?.toString()

/** A88 — the update flow's two questions (mobile data; "Install unknown apps") and its one-line outcome messages. */
@Composable
internal fun AppUpdateDialogs(c: AppUpdateController) {
    when (c.ask) {
        AppUpdateController.Ask.METERED -> AlertDialog(
            onDismissRequest = { c.answer(false) },
            title = { Text("Download on mobile data?") },
            text = { Text("The app update is about 15 MB.") },
            confirmButton = { TextButton(onClick = { c.answer(true) }) { Text("Download") } },
            dismissButton = { TextButton(onClick = { c.answer(false) }) { Text("Not now") } },
        )
        AppUpdateController.Ask.UNKNOWN_SOURCES -> AlertDialog(
            onDismissRequest = { c.answer(false) },
            title = { Text("Allow TroubaStage to install its updates") },
            text = { Text("Android asks once: turn on \u201cAllow from this source\u201d for TroubaStage, then come back and tap Update again.") },
            confirmButton = { TextButton(onClick = { c.answer(true) }) { Text("Open settings") } },
            dismissButton = { TextButton(onClick = { c.answer(false) }) { Text("Not now") } },
        )
        null -> {}
    }
    c.message?.let { m ->
        AlertDialog(
            onDismissRequest = { c.message = null },
            title = { Text("App update") },
            text = { Text(m) },
            confirmButton = { TextButton(onClick = { c.message = null }) { Text("OK") } },
        )
    }
}
