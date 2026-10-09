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
    var tab by rememberSaveable { mutableStateOf(0) }
    // A65 (VLL): the Studio browse wears the Studio brand — two-tone "TroubaStudio" title + pink tabs.
    val pink = LocalBrandAccents.current.studio
    BackHandler { onBack() }
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize()) {
            TopAppBar(
                title = { Text(brandTitle("Studio", accent = pink, ink = MaterialTheme.colorScheme.onSurface)) },
                navigationIcon = { TextButton(onClick = onBack) { Text("‹  Back") } },
            )
            TabRow(
                selectedTabIndex = tab,
                contentColor = pink,
                // The selected-tab underline: TabRow's default indicator is colorScheme.primary (indigo),
                // ignoring contentColor — so paint it pink explicitly (VLL: the bottom border was blue).
                indicator = { positions -> TabRowDefaults.SecondaryIndicator(Modifier.tabIndicatorOffset(positions[tab]), color = pink) },
            ) {
                Tab(selected = tab == 0, onClick = { tab = 0 }, text = { Text("Concerts") }, selectedContentColor = pink, unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant)
                Tab(selected = tab == 1, onClick = { tab = 1 }, text = { Text("Bands") }, selectedContentColor = pink, unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Box(Modifier.weight(1f).fillMaxWidth()) {
                when (tab) {
                    0 -> ConcertsTab(transport, onOpen)
                    else -> BandsTab(transport, onOpen, onShowQr)
                }
            }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ConcertsTab(transport: HttpTransport, onOpen: (String, String, String) -> Unit) {
    var refresh by remember { mutableIntStateOf(0) }
    val rows by produceState<List<HttpTransport.StudioConcert>?>(null, refresh) { value = transport.fetchStudioConcerts() }
    val sections = remember(rows) {
        rows?.let { groupConcerts(it, { c -> c.bandId }, { c -> c.bandName }, { c -> c.name }, { c -> c.eventDate }) }
    }
    val snackbar = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf<String?>(null) } // the concert whose live toggle is in flight
    Box(Modifier.fillMaxSize()) {
        ListScaffold(rows, empty = "No concerts yet") { _ ->
            sections.orEmpty().forEach { section ->
                if (section.showHeader) {
                    stickyHeader(key = "band-" + section.bandId) { BandHeader(section.bandName) }
                }
                items(section.concerts, key = { it.bandId + "/" + it.setlistId }) { c ->
                    val live = isLive(c.liveUntil)
                    LauncherRow(
                        title = c.name.ifBlank { "Untitled concert" },
                        meta = concertMeta(c),
                        onClick = { onOpen("/bands/${c.bandId}/setlists/${c.setlistId}", c.bandName, c.bandId) },
                        badge = if (live) ({ LiveChip() }) else null,
                        trailing = {
                            ConcertMenu(
                                concert = c,
                                live = live,
                                busy = busy == c.setlistId,
                                // Bake opens Studio's own dialog (T189 makes ?bake=1 open it; before T189 the link
                                // lands on the concert page, where the Bake button is). Nothing native.
                                onBake = { onOpen("/bands/${c.bandId}/setlists/${c.setlistId}?bake=1", c.bandName, c.bandId) },
                                onToggleLive = {
                                    busy = c.setlistId
                                    scope.launch {
                                        val ok = transport.setConcertLive(c.bandId, c.setlistId, !live)
                                        busy = null
                                        if (ok) refresh++ else snackbar.showSnackbar("Couldn't change live mode")
                                    }
                                },
                            )
                        },
                    )
                }
            }
        }
        SnackbarHost(snackbar, Modifier.align(Alignment.BottomCenter))
    }
}

/** A83 ⟨D1⟩ — the band a run of concerts belongs to; pinned while its concerts scroll (stickyHeader). */
@Composable
private fun BandHeader(name: String) {
    Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxWidth()) {
        Text(
            name.ifBlank { "Unnamed band" },
            style = MaterialTheme.typography.titleSmall,
            color = LocalBrandAccents.current.studio,
            modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 16.dp, bottom = 6.dp),
        )
    }
}

/** A83 — "Live", like Studio's live chip: rehearsal live mode is on for this concert. */
@Composable
private fun LiveChip() {
    val pink = LocalBrandAccents.current.studio
    Text(
        "Live",
        style = MaterialTheme.typography.labelSmall,
        color = pink,
        modifier = Modifier.border(1.dp, pink, RoundedCornerShape(50)).padding(horizontal = 8.dp, vertical = 1.dp),
    )
}

/** A83 ⟨D2⟩⟨D3⟩ — the row's ⋯ menu. Everyone sees it; for a non-admin both items are DISABLED with the reason
 *  ("Admins only", from the band role — never from a failed call). Built so more items can join later. */
@Composable
private fun ConcertMenu(concert: HttpTransport.StudioConcert, live: Boolean, busy: Boolean, onBake: () -> Unit, onToggleLive: () -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }) {
            Text("⋯", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            val admin = concert.isAdmin
            MenuEntry(
                label = if (concert.lastBakedAt != null) "Re-bake" else "Bake",
                reason = when { !admin -> "Admins only"; concert.songCount == 0 -> "No songs yet"; else -> null },
                enabled = admin && concert.songCount > 0,
                onClick = { open = false; onBake() },
            )
            MenuEntry(
                label = if (live) "Disarm live mode" else "Arm live mode · auto-bakes for 3 h",
                reason = if (!admin) "Admins only" else null,
                enabled = admin && !busy,
                onClick = { open = false; onToggleLive() },
            )
        }
    }
}

@Composable
private fun MenuEntry(label: String, reason: String?, enabled: Boolean, onClick: () -> Unit) {
    DropdownMenuItem(
        text = {
            Column {
                Text(label)
                if (reason != null) Text(reason, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        },
        enabled = enabled,
        onClick = onClick,
    )
}

/** Live while `liveUntil` is in the future on the device clock; Go's zero time (year 1) and absence are off. */
private fun isLive(liveUntil: String?): Boolean =
    liveUntil != null && runCatching { java.time.Instant.parse(liveUntil).isAfter(java.time.Instant.now()) }.getOrDefault(false)

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

/** "2026-09-05 · Some Venue", either half omitted when absent (Fable's omitempty caution). */
private fun concertMeta(c: HttpTransport.StudioConcert): String =
    listOf(c.eventDate, c.venue).filter { it.isNotBlank() }.joinToString("  ·  ")
