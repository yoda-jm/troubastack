package com.troubashare.shared.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.troubashare.shared.stage.FitMode
import com.troubashare.shared.stage.StageColorMode

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
    onBack: () -> Unit,
) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(20.dp),
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
                    label = "Colour mode",
                    options = listOf("Normal" to StageColorMode.NORMAL, "Night" to StageColorMode.NIGHT),
                    selected = colorMode,
                    onSelect = onColorMode,
                )
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
