package com.troubastack.app

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.border
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import com.troubastack.shared.studio.groupConcerts
import kotlinx.coroutines.launch
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.IconButton
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.TabRowDefaults
import androidx.compose.material3.TabRowDefaults.tabIndicatorOffset
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.troubastack.shared.ui.LocalBrandAccents

/** BRAND10 two-tone wordmark: "Trouba" in [ink], the product word in its [accent]. Shared by the
 *  native product launcher pages (Studio browse here, the Stage concert list in MainActivity). */
internal fun brandTitle(product: String, accent: Color, ink: Color) = buildAnnotatedString {
    withStyle(SpanStyle(color = ink)) { append("Trouba") }
    withStyle(SpanStyle(color = accent)) { append(product) }
}

/**
 * A65 — the native "Studio" browse: two LAUNCHER lists (Concerts, Bands). Concerts is every concert
 * across the user's bands, newest-dated first; Bands is the band list. A row taps straight into the
 * Studio WebView at that context ([onOpen] with the deep-link path + the band name for the frame title).
 *
 * LAUNCHERS, plus TWO row actions (A83). A65's ruling was "name/date/venue + tap-to-open, and NOTHING else";
 * VLL overrode it on 2026-10-09 for exactly two concert actions, in a ⋯ menu: **Bake** (which opens Studio's
 * OWN bake dialog through a deep link — no native bake UI, so P205's "never capture layer defaults silently"
 * keeps one dialog in one place) and **Arm / Disarm live mode**. Create, rename, delete, search and all other
 * authoring stay in Studio (I10). Concerts are grouped under band headers (A83 ⟨D1⟩, [groupConcerts]).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudioBrowseScreen(
    transport: HttpTransport,
    onOpen: (initialPath: String, bandName: String, bandId: String) -> Unit,
    onShowQr: (bandId: String, bandName: String) -> Unit,
    onBack: () -> Unit,
) {
    // A65 (VLL): the Studio browse wears the Studio brand — two-tone "TroubaStudio" title + pink tabs.
    val pink = LocalBrandAccents.current.studio
    BackHandler { onBack() }
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                title = { Text(brandTitle("Studio", accent = pink, ink = MaterialTheme.colorScheme.onSurface)) },
                navigationIcon = { TextButton(onClick = onBack) { Text("‹  Back") } },
            )
            // A91: the concert list moved to TroubaStage (ONE list, with Re-bake and live mode in each row's ⋯), so the
            // Studio browse keeps only its Bands list (band pages, invites, QR).
            Box(Modifier.weight(1f).fillMaxWidth()) { BandsTab(transport, onOpen, onShowQr) }
        }
    }
}

@Composable
private fun BandsTab(transport: HttpTransport, onOpen: (String, String, String) -> Unit, onShowQr: (String, String) -> Unit) {
    val rows by produceState<List<HttpTransport.StudioBand>?>(null) { value = transport.fetchStudioBands() }
    ListScaffold(rows, empty = "No bands yet") { list ->
        items(list, key = { it.id }) { b ->
            LauncherRow(
                title = b.name.ifBlank { "Unnamed band" },
                meta = "",
                onClick = { onOpen("/bands/${b.id}", b.name, b.id) },
                // A65 (VLL): admins get a per-row "Show band QR" — more natural than an overflow on the
                // WebView. Non-admins never see it (they'd land on an empty invites page).
                trailing = if (b.isAdmin) ({ QrButton { onShowQr(b.id, b.name) } }) else null,
            )
        }
    }
}

/** Loading spinner → empty message → the list. Keeps every tab's states consistent. */
@Composable
private fun <T> ListScaffold(rows: List<T>?, empty: String, content: androidx.compose.foundation.lazy.LazyListScope.(List<T>) -> Unit) {
    when {
        rows == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
        rows.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Text(empty, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
        }
        else -> LazyColumn(Modifier.fillMaxSize()) { content(rows) }
    }
}

/** One tappable launcher row: title (+ an optional [badge] after it) + optional grey meta line, with an
 *  optional [trailing] action. */
@Composable
private fun LauncherRow(title: String, meta: String, onClick: () -> Unit, trailing: (@Composable () -> Unit)? = null, badge: (@Composable () -> Unit)? = null) {
    Row(Modifier.fillMaxWidth().clickable(onClick = onClick).padding(start = 20.dp, end = 8.dp, top = 14.dp, bottom = 14.dp), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f).padding(end = 12.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(title, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                badge?.invoke()
            }
            if (meta.isNotEmpty()) {
                Text(meta, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        trailing?.invoke()
    }
    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
}

/** A65 (VLL) — the per-row "Show band QR" affordance: a small QR glyph in the Studio accent. */
@Composable
private fun QrButton(onClick: () -> Unit) {
    IconButton(onClick = onClick) { QrGlyph(LocalBrandAccents.current.studio, Modifier.size(24.dp)) }
}

/** A minimal QR-looking glyph (three finder squares + a couple modules) drawn on a Canvas — reads as
 *  "QR" without pulling in material-icons-extended for one icon. */
@Composable
private fun QrGlyph(tint: Color, modifier: Modifier) {
    Canvas(modifier) {
        val u = size.minDimension / 7f
        fun finder(cx: Float, cy: Float) {
            drawRect(tint, topLeft = Offset(cx, cy), size = Size(3 * u, 3 * u), style = Stroke(u * 0.6f))
            drawRect(tint, topLeft = Offset(cx + u, cy + u), size = Size(u, u))
        }
        finder(0f, 0f); finder(4 * u, 0f); finder(0f, 4 * u)
        drawRect(tint, topLeft = Offset(5 * u, 5 * u), size = Size(u, u))
        drawRect(tint, topLeft = Offset(4 * u, 4 * u), size = Size(u, u))
    }
}

