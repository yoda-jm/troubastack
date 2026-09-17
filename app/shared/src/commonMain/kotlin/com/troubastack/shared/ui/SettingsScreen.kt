package com.troubastack.shared.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.troubastack.shared.stage.FitMode
import com.troubastack.shared.stage.MidiSignal
import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.PedalBindings
import com.troubastack.shared.stage.PedalLinkState
import com.troubastack.shared.stage.StageColorMode
import com.troubastack.shared.stage.isLearnableKey
import com.troubastack.shared.stage.keyToken

/**
 * A36 — the native Parameters hub (VLL: the app was "missing a parameters native content, for
 * example to set dark/light theme"). Gathers the app-wide reading preferences in one place:
 *
 *  - **Theme** — the A36 light/dark/system choice, the one that drives [TroubaTheme].
 *  - **Stage** — reading mode + colour mode. These are the SAME persisted keys the Stage ⚙ sheet
 *    writes (VLL: keep them in concert mode too, it's easier to change there — so they live in
 *    BOTH; both edit the one stored default, so they stay in sync).
 *
 * Update policy is deliberately absent: it is per-concert today (PROMPT/FROZEN/AUTO, set in Manage),
 * not a single global switch — a global default would be a new pref, flagged for a later pass.
 *
 * All values are hoisted; the host owns persistence (Storage) so this stays platform-agnostic.
 */
@Composable
fun SettingsScreen(
    themePref: ThemePref,
    onThemePref: (ThemePref) -> Unit,
    fitMode: FitMode,
    onFitMode: (FitMode) -> Unit,
    colorMode: StageColorMode,
    onColorMode: (StageColorMode) -> Unit,
    // A72/A76 — device-local pedal bindings + learn/forget. onLearnPedal takes a KIND-TAGGED token (KEY:/MIDI:)
    // and returns the action it used to drive (if any), for the "that button was …" notice; the host learns +
    // persists. A76 adds the BLE-MIDI link: its state, an explicit Connect, and the stream of MIDI presses so
    // Learn takes either kind.
    pedalBindings: PedalBindings = emptyMap(),
    onLearnPedal: (PageTurn, String) -> PageTurn? = { _, _ -> null },
    onForgetPedals: () -> Unit = {},
    midiState: PedalLinkState = PedalLinkState.NOT_CONNECTED,
    onConnectPedal: () -> Unit = {},
    midiSignal: MidiSignal? = null,
    onBack: () -> Unit,
) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            // A72/VLL: pad for the status + nav bars (edge-to-edge), so in portrait the header clears the
            // status bar and the last section clears the nav bar — otherwise the bottom hides and there's
            // nothing to scroll to. Inside verticalScroll, so the clearance is part of the scroll range.
            Modifier.fillMaxSize().verticalScroll(rememberScrollState())
                .windowInsetsPadding(WindowInsets.systemBars).padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = onBack) { Text("‹  Back") }
                Text("Parameters", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
            }

            Section("Appearance") {
                ChoiceRow(
                    label = "Theme",
                    options = listOf("System" to ThemePref.SYSTEM, "Light" to ThemePref.LIGHT, "Dark" to ThemePref.DARK),
                    selected = themePref,
                    onSelect = onThemePref,
                )
            }

            Section("Stage", subtitle = "Also on the ⚙ in concert mode — same setting, whichever is handier.") {
                ChoiceRow(
                    label = "Reading mode",
                    options = listOf("Page" to FitMode.FIT_PAGE, "Width" to FitMode.FIT_WIDTH, "Scroll" to FitMode.SCROLL),
                    selected = fitMode,
                    onSelect = onFitMode,
                )
                ChoiceRow(
                    // A37: all four reading schemes are directly selectable here; the on-stage tap
                    // ping-pongs (StageColorMode). A direct pick is a fresh walk — the Stage resets the
                    // cycle direction to UP on its next entry (Ruling 1b).
                    label = "Colour mode",
                    options = StageColorMode.entries.map { it.label() to it },
                    selected = colorMode,
                    onSelect = onColorMode,
                )
            }

            Section("Foot pedal", subtitle = "Turn pages with a Bluetooth pedal — a keyboard/HID pedal, or a BLE-MIDI controller. For a MIDI pedal, Connect first; then Learn a button. The panel shows every press, so an empty panel is the answer, not a failure.") {
                PedalLearnSection(pedalBindings, onLearnPedal, onForgetPedals, midiState, onConnectPedal, midiSignal)
            }
        }
    }
}

/** A titled group on a warm surface card, matching the Home cards' look. */
@Composable
private fun Section(title: String, subtitle: String? = null, content: @Composable () -> Unit) {
    Surface(
        Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = MaterialTheme.typography.titleMedium)
                if (subtitle != null) {
                    Text(subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            content()
        }
    }
}

/** A labelled row of selectable pills (one choice). Selected = brand tint + indigo outline. */
@Composable
private fun <T> ChoiceRow(label: String, options: List<Pair<String, T>>, selected: T, onSelect: (T) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(label, style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            options.forEach { (name, value) ->
                val on = value == selected
                Surface(
                    onClick = { onSelect(value) },
                    shape = MaterialTheme.shapes.small,
                    color = if (on) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant,
                    contentColor = if (on) MaterialTheme.colorScheme.onPrimaryContainer else MaterialTheme.colorScheme.onSurfaceVariant,
                    border = BorderStroke(if (on) 1.5.dp else 1.dp, if (on) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant),
                ) {
                    Text(
                        name,
                        Modifier.widthIn(min = 76.dp).padding(horizontal = 16.dp, vertical = 10.dp),
                        style = MaterialTheme.typography.labelLarge,
                        textAlign = TextAlign.Center,
                    )
                }
            }
        }
    }
}

/**
 * A72 — the Learn panel, and per ⟨D1⟩ it is also the diagnostic. Arm one action, press a button: the panel
 * shows the RAW code of EVERY press (recognised, unrecognised, or refused). If pressing the pedals shows
 * "nothing received yet", that is the finding — the pedal is not an HID keyboard. Back/Home are refused with
 * the reason (⟨D4⟩) but still shown. The learn + persist is the host's (onLearn); this owns only the capture.
 */
@Composable
private fun PedalLearnSection(
    bindings: PedalBindings,
    onLearn: (PageTurn, String) -> PageTurn?,
    onForget: () -> Unit,
    midiState: PedalLinkState,
    onConnect: () -> Unit,
    midiSignal: MidiSignal?,
) {
    var armed by remember { mutableStateOf<PageTurn?>(null) }
    var lastReceived by remember { mutableStateOf<String?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }
    var lastMidiSeq by remember { mutableStateOf(-1L) }
    val capture = remember { FocusRequester() }
    LaunchedEffect(armed) { if (armed != null) runCatching { capture.requestFocus() } }

    // learn a KEY: or MIDI: token — the one place both paths converge (⟨D2⟩ one store, one learn).
    fun learn(token: String, label: String) {
        lastReceived = label
        val a = armed ?: return
        notice = when (onLearn(a, token)) {
            PageTurn.NEXT -> "Learned. (That was Next — moved to Previous.)"
            PageTurn.PREV -> "Learned. (That was Previous — moved to Next.)"
            null -> "Learned."
        }
        armed = null
    }

    // A76 ⟨D1⟩: a MIDI press always updates the "received" line (the diagnostic); while armed it also learns.
    LaunchedEffect(midiSignal?.seq) {
        val sig = midiSignal ?: return@LaunchedEffect
        if (sig.seq == lastMidiSeq) return@LaunchedEffect
        lastMidiSeq = sig.seq
        lastReceived = sig.label
        if (armed != null) learn(sig.token, sig.label)
    }

    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        // ⟨D1⟩ — the link state, so "nothing received yet" means mute, not disconnected. Explicit Connect.
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text(
                when (midiState) {
                    PedalLinkState.CONNECTED -> "MIDI pedal: connected"
                    PedalLinkState.SCANNING -> "MIDI pedal: searching…"
                    PedalLinkState.NOT_CONNECTED -> "MIDI pedal: not connected"
                },
                style = MaterialTheme.typography.bodySmall,
                color = if (midiState == PedalLinkState.CONNECTED) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f),
            )
            if (midiState != PedalLinkState.CONNECTED) {
                TextButton(onClick = onConnect, enabled = midiState != PedalLinkState.SCANNING) {
                    Text(if (midiState == PedalLinkState.SCANNING) "Searching…" else "Connect")
                }
            }
        }

        for (action in PageTurn.entries) {
            val toks = bindings[action].orEmpty()
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Column(Modifier.weight(1f)) {
                    Text(if (action == PageTurn.NEXT) "Next page" else "Previous page", style = MaterialTheme.typography.labelLarge)
                    Text(
                        if (toks.isEmpty()) "no learned button" else "learned: " + toks.sorted().joinToString(", "),
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                TextButton(onClick = { notice = null; lastReceived = null; armed = action }) {
                    Text(if (armed == action) "Press…" else "Learn")
                }
            }
        }

        val armedNow = armed
        if (armedNow != null) {
            Surface(
                Modifier.fillMaxWidth()
                    .focusRequester(capture)
                    .focusable()
                    .onPreviewKeyEvent { e ->
                        if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                        lastReceived = "key ${e.key.keyCode}" // ⟨D1⟩: show EVERY press
                        if (!isLearnableKey(e.key)) {
                            notice = "That key can't be learned — Back and Home would trap you in the Stage."
                        } else {
                            learn(keyToken(e.key.keyCode), "key ${e.key.keyCode}")
                        }
                        true // never let a press escape while learning
                    },
                shape = MaterialTheme.shapes.medium,
                color = MaterialTheme.colorScheme.surfaceVariant,
                border = BorderStroke(1.dp, MaterialTheme.colorScheme.primary),
            ) {
                Column(Modifier.padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("Learning ${if (armedNow == PageTurn.NEXT) "Next page" else "Previous page"} — press a pedal button.", style = MaterialTheme.typography.bodyMedium)
                    Text(
                        lastReceived?.let { "received: $it" } ?: "nothing received yet",
                        style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary,
                    )
                    if (midiState != PedalLinkState.CONNECTED) {
                        Text("For a MIDI pedal, tap Connect above first.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    TextButton(onClick = { armed = null }) { Text("Cancel") }
                }
            }
        }

        notice?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        if (bindings.isNotEmpty()) {
            TextButton(onClick = { onForget(); notice = "Learned buttons cleared." }) { Text("Forget learned buttons") }
        }
    }
}
