package com.troubastack.shared.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlin.concurrent.Volatile
import kotlin.time.TimeMark
import kotlin.time.TimeSource

/** A101 — the last time an entry of the Licences screen took to show, and the slowest so far; each prints one line
 *  (`A101-TIME entry-open name=… ms=…`, on Android in logcat). The desktop About smoke reads [slowest]. */
object LicenceOpenTiming {
    @Volatile var slowest: Pair<String, Long>? = null
        private set
    @Volatile var count: Int = 0
        private set

    fun record(name: String, ms: Long) {
        println("A101-TIME entry-open name=$name ms=$ms")
        if (ms > (slowest?.second ?: -1)) slowest = name to ms
        count++
    }
}

/** WIN08 S2.8 — one open-source component the app ships, and the texts that come with it (its licence, its notices). */
data class LicenceEntry(val name: String, val documents: List<LicenceDocument>)

/** One text of a [LicenceEntry], read from the file the app ships. */
data class LicenceDocument(val title: String, val text: String)

/**
 * WIN08 S2.8 — Parameters → About → Licences: the open-source components the app ships, as a list of entries (SWT in
 * the Windows app today; more entries can join it), and the chosen entry's texts. The caller holds [opened] so its
 * platform Back can step from an entry back to the list.
 */
@Composable
fun LicencesScreen(entries: List<LicenceEntry>, opened: LicenceEntry?, onOpen: (LicenceEntry?) -> Unit, onBack: () -> Unit) {
    // A101 — how long an entry takes to show: from its press to the first frame after it composed (LicenceOpenTiming).
    val pressedAt = remember { mutableStateOf<TimeMark?>(null) }
    LaunchedEffect(opened) {
        val mark = pressedAt.value ?: return@LaunchedEffect
        if (opened == null) return@LaunchedEffect
        withFrameNanos { }
        LicenceOpenTiming.record(opened.name, mark.elapsedNow().inWholeMilliseconds)
        pressedAt.value = null
    }
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).readableWidth(LocalMaxContentWidth.current)
                .windowInsetsPadding(WindowInsets.systemBars).padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                TextButton(onClick = { if (opened != null) onOpen(null) else onBack() }) { Text("‹  Back") }
                Text(opened?.name ?: "Open-source licences", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary)
            }
            // Each view is its own composable: the two loops then live in separate groups (inlined into one branch
            // pair, a press that switched views read the list's remembered click lambda as the texts' content lambda).
            if (opened == null) EntryList(entries) { pressedAt.value = TimeSource.Monotonic.markNow(); onOpen(it) } else EntryTexts(opened)
        }
    }
}

@Composable
private fun EntryList(entries: List<LicenceEntry>, onOpen: (LicenceEntry) -> Unit) {
    for (entry in entries) {
        TextButton(onClick = { onOpen(entry) }, modifier = Modifier.fillMaxWidth()) {
            Text(entry.name, Modifier.fillMaxWidth(), style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Composable
private fun EntryTexts(entry: LicenceEntry) {
    for (doc in entry.documents) {
        Text(doc.title, style = MaterialTheme.typography.titleMedium)
        SelectionContainer { Text(doc.text, style = MaterialTheme.typography.bodySmall) }
    }
}
