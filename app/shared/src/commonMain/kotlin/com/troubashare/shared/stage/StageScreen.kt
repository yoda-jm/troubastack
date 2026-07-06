// TroubaStage UI — Compose Multiplatform (commonMain). A pager over composited pages plus the
// minimal reading controls (I12). Image DECODING is injected as a plain function from the platform
// (androidApp) — NOT a new expect/actual seam (seams are capped at three, I15). Every decode returns
// a Result and is treated as "maybe a placeholder", so a bad image never crashes the performance.
package com.troubashare.shared.stage

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.focusable
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.painter.BitmapPainter
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

/**
 * Decodes a blob ref (resolved by the platform against the bundle dir) to an [ImageBitmap], DOWNSAMPLED
 * toward the target size to avoid OOM. Total: returns a failed [Result] rather than throwing.
 */
fun interface ImageDecoder {
    fun decode(ref: String, targetW: Int, targetH: Int): Result<ImageBitmap>
}

/**
 * A small most-recently-used cache of decoded bitmaps so we never decode the whole bundle. Accessed
 * only from the composition (main) thread; the heavy decode runs off-thread and results are stored
 * back here on the main thread.
 */
class PageImageCache(maxEntries: Int = 12) {
    // Thin typed wrapper over the generic access-order [LruCache] (the LRU logic lives there and is
    // unit-tested off-device; Stage behaviour is unchanged).
    private val lru = LruCache<String, ImageBitmap>(maxEntries)

    fun get(key: String): ImageBitmap? = lru.get(key)

    fun put(key: String, bmp: ImageBitmap) = lru.put(key, bmp)
}

private fun cacheKey(ref: String, w: Int, h: Int): String = "$ref@${w}x$h"

/** Root of the Stage UI: failure screen, empty state, or the performing pager. */
@Composable
fun StageScreen(
    vm: StageViewModel,
    decoder: ImageDecoder,
    onExit: () -> Unit,
    // A10: local night-mode preference, injected + persisted by the entrypoint (app DI, no seam).
    initialColorMode: StageColorMode = StageColorMode.NORMAL,
    onColorModeChange: (StageColorMode) -> Unit = {},
) {
    val state by vm.state.collectAsState()
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        when {
            state.failure != null -> CenteredMessage(
                title = "Can't open this concert",
                body = state.failure ?: "",
                onExit = onExit,
            )
            state.pages.isEmpty() -> CenteredMessage(
                title = "Nothing to perform",
                body = "This concert has no pages.",
                onExit = onExit,
            )
            else -> Performing(state, vm, decoder, onExit, initialColorMode, onColorModeChange)
        }
    }
}

@Composable
private fun Performing(
    state: StageState,
    vm: StageViewModel,
    decoder: ImageDecoder,
    onExit: () -> Unit,
    initialColorMode: StageColorMode,
    onColorModeChange: (StageColorMode) -> Unit,
) {
    var colorMode by remember { mutableStateOf(initialColorMode) }
    val cache = remember { PageImageCache() }
    val page = state.currentPage ?: return
    var showLayers by remember { mutableStateOf(false) }
    var showRole by remember { mutableStateOf(false) }
    var showSongs by remember { mutableStateOf(false) }

    // Hardware page turns (A09): BT pedals/keyboards send PageUp/Down, arrows, Space. Capture at the
    // root before children so a keyboard turns the page while on-screen taps still work. (Android
    // volume keys can't reach Compose; androidApp forwards them via onKeyDown.)
    val keyFocus = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { keyFocus.requestFocus() } }

    // A12: facing pages. When the viewport is landscape (w > h) AND the fit is FIT_PAGE we show two
    // adjacent pages (2k/2k+1) side by side and turn by the spread; portrait or FIT_WIDTH stays
    // single-page exactly as before. The measurement decides the layout, so it wraps everything.
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val twoUp = maxWidth > maxHeight && state.fitMode == FitMode.FIT_PAGE
        // One navigation entry point (keys, taps, swipes, arrows) so turn-by-spread is applied once.
        val turnNext: () -> Unit =
            { vm.goToPage(if (twoUp) nextSpreadPage(state.current, state.pageCount) else state.current + 1) }
        val turnPrev: () -> Unit =
            { vm.goToPage(if (twoUp) prevSpreadPage(state.current) else state.current - 1) }
        val spread = spreadPages(state.current, state.pageCount)

        Box(
            Modifier
                .fillMaxSize()
                .focusRequester(keyFocus)
                .focusable()
                .onPreviewKeyEvent { e ->
                    if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
                    when (stageKeyAction(e.key)) {
                        PageTurn.NEXT -> { turnNext(); true }
                        PageTurn.PREV -> { turnPrev(); true }
                        null -> false
                    }
                },
        ) {
        // Page area with tap-thirds + horizontal swipe navigation (spread-aware in two-up).
        Box(
            Modifier
                .fillMaxSize()
                .pointerNavigation(navKey = state.pageCount to twoUp, onPrev = turnPrev, onNext = turnNext),
        ) {
            if (twoUp) {
                // A lone last page (spread of 1) fills the row; ContentScale.Fit centres it.
                Row(Modifier.fillMaxSize()) {
                    spread.forEach { idx ->
                        PageView(state.pages[idx], state.visibleLayers, state.fitMode, decoder, cache, colorMode.pageColorFilter(), Modifier.weight(1f).fillMaxHeight())
                    }
                }
            } else {
                PageView(page, state.visibleLayers, state.fitMode, decoder, cache, colorMode.pageColorFilter(), Modifier.fillMaxSize())
            }
        }

        // Top overlays: the chrome bar, plus (A08) a footprint-stable setlist-metadata strip stacked
        // beneath it on a song's first page. Both are overlays over the page area — when there's no
        // metadata the strip doesn't render and the layout is pixel-identical to before (I12).
        Column(Modifier.align(Alignment.TopCenter).fillMaxWidth()) {
            Surface(
                Modifier.fillMaxWidth().statusBarsPadding(),
                color = MaterialTheme.colorScheme.surface.copy(alpha = 0.92f),
            ) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                    horizontalArrangement = Arrangement.spacedBy(4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    TextButton(onClick = onExit) { Text("Back") }
                    Spacer(Modifier.weight(1f))
                    TextButton(onClick = vm::toggleFit) {
                        Text(if (state.fitMode == FitMode.FIT_PAGE) "Fit: page" else "Fit: width")
                    }
                    if (state.layers.isNotEmpty()) TextButton(onClick = { showLayers = true }) { Text("Layers") }
                    TextButton(onClick = { showRole = true }) { Text("Role") }
                    TextButton(onClick = { colorMode = colorMode.next(); onColorModeChange(colorMode) }) {
                        Text(if (colorMode == StageColorMode.NIGHT) "Night" else "Day")
                    }
                }
            }
            // A12: in two-up the strip is per-side (each half over its page's top); one-up is full-width.
            if (twoUp) {
                Row(Modifier.fillMaxWidth()) {
                    Box(Modifier.weight(1f)) { spread.getOrNull(0)?.let { MetaStrip(state.pages[it], resetKey = it) } }
                    Box(Modifier.weight(1f)) { spread.getOrNull(1)?.let { MetaStrip(state.pages[it], resetKey = it) } }
                }
            } else {
                MetaStrip(page, resetKey = state.current)
            }
        }

        // Bottom nav bar: pager + song picker (the spec's "thin bottom bar").
        Surface(
            Modifier.align(Alignment.BottomCenter).fillMaxWidth().navigationBarsPadding(),
            color = MaterialTheme.colorScheme.surface.copy(alpha = 0.92f),
        ) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                TextButton(onClick = turnPrev) { Text("‹") }
                Text(
                    pagerLabel(state.current, state.pageCount, twoUp),
                    style = MaterialTheme.typography.labelLarge,
                )
                TextButton(onClick = turnNext) { Text("›") }
                Spacer(Modifier.weight(1f))
                Box {
                    TextButton(onClick = { showSongs = true }) { Text("Songs") }
                    DropdownMenu(expanded = showSongs, onDismissRequest = { showSongs = false }) {
                        state.songs.forEachIndexed { i, s ->
                            DropdownMenuItem(
                                text = { Text(if (i == state.currentSong) "• ${s.name}" else s.name) },
                                onClick = { showSongs = false; vm.goToSong(i) },
                            )
                        }
                    }
                }
            }
        }
        }
    }

    if (showLayers) LayersDialog(state, vm) { showLayers = false }
    if (showRole) RoleDialog(state, vm) { showRole = false }
}

/** Composites raster + visible overlays for one page; a decode failure degrades to a placeholder. */
@Composable
private fun PageView(
    page: StagePage,
    visibleLayers: Set<String>,
    fitMode: FitMode,
    decoder: ImageDecoder,
    cache: PageImageCache,
    colorFilter: androidx.compose.ui.graphics.ColorFilter?,
    modifier: Modifier,
) {
    if (page.status == PageStatus.UNAVAILABLE) {
        PlaceholderCard(modifier)
        return
    }
    androidx.compose.foundation.layout.BoxWithConstraints(modifier) {
        val density = LocalDensity.current
        val wPx = with(density) { maxWidth.toPx().toInt() }
        val hPx = with(density) { maxHeight.toPx().toInt() }
        val overlayRefs = page.overlays.filter { it.layerId in visibleLayers }.map { it.imageRef }

        val decoded by produceState<PageBitmaps?>(null, page.rasterRef, overlayRefs, wPx, hPx) {
            value = null
            if (wPx <= 0 || hPx <= 0) return@produceState
            value = withContext(Dispatchers.Default) {
                val raster = decodeCached(cache, page.rasterRef, wPx, hPx, decoder)
                val overlays = overlayRefs.mapNotNull { decodeCached(cache, it, wPx, hPx, decoder) }
                PageBitmaps(raster, overlays)
            }
        }

        val bitmaps = decoded
        when {
            bitmaps == null -> {} // brief blank while decoding; no spinner needed for local files
            bitmaps.raster == null -> PlaceholderCard(Modifier.fillMaxSize()) // decode failed at render time
            else -> {
                val aspect = bitmaps.raster.width.toFloat() / bitmaps.raster.height.toFloat()
                val scroll = rememberScrollState()
                val container =
                    if (fitMode == FitMode.FIT_WIDTH) Modifier.fillMaxSize().verticalScroll(scroll)
                    else Modifier.fillMaxSize()
                val imageMod =
                    if (fitMode == FitMode.FIT_WIDTH) Modifier.fillMaxWidth().aspectRatio(aspect)
                    else Modifier.fillMaxSize()
                val scale = if (fitMode == FitMode.FIT_WIDTH) ContentScale.FillWidth else ContentScale.Fit
                Box(container, contentAlignment = Alignment.Center) {
                    Image(BitmapPainter(bitmaps.raster), contentDescription = null, modifier = imageMod, contentScale = scale, colorFilter = colorFilter)
                    bitmaps.overlays.forEach {
                        Image(BitmapPainter(it), contentDescription = null, modifier = imageMod, contentScale = scale, colorFilter = colorFilter)
                    }
                }
            }
        }
    }
}

/**
 * The A08 setlist-metadata strip for one page: notes · key on the left (ellipsised), A11's tempo chip
 * on the right. Renders only on a song's FIRST page and only when there's something to show — otherwise
 * nothing (so the layout is unchanged, I12). [resetKey] cancels an in-progress count-in on a page turn;
 * in two-up it's the per-side page index so each half resets independently.
 */
@Composable
private fun MetaStrip(page: StagePage, resetKey: Any) {
    if (page.pageInSong != 0) return
    val prefix = metaStripText(page.displayNotes, page.key, 0) // notes · key; tempo is the chip (A11)
    val hasTempo = page.tempo > 0
    if (prefix == null && !hasTempo) return
    Surface(Modifier.fillMaxWidth(), color = MaterialTheme.colorScheme.surface.copy(alpha = 0.75f)) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            if (prefix != null) {
                Text(
                    prefix,
                    Modifier.weight(1f),
                    style = MaterialTheme.typography.labelMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            } else {
                Spacer(Modifier.weight(1f))
            }
            if (hasTempo) TempoChip(page.tempo, resetKey = resetKey)
        }
    }
}

/**
 * The A08 tempo, as a tappable chip that runs a silent visual count-in (A11): [COUNT_IN_BEATS] beats
 * at the song's tempo, a corner-of-the-eye pulse dot (downbeats emphasized), self-stopping. [resetKey]
 * (the current page) cancels an in-progress count on a page turn. Read-only, no audio, no full-screen
 * flash (stage lighting). Out-of-range tempo → the tap is a no-op.
 */
@Composable
private fun TempoChip(tempo: Int, resetKey: Any) {
    var running by remember(resetKey) { mutableStateOf(false) }
    var beat by remember(resetKey) { mutableStateOf(-1) }
    LaunchedEffect(running, resetKey) {
        if (!running) { beat = -1; return@LaunchedEffect }
        val ms = countInIntervalMs(tempo) ?: run { running = false; return@LaunchedEffect }
        for (b in 0 until COUNT_IN_BEATS) { beat = b; delay(ms) }
        beat = -1
        running = false
    }
    Row(
        Modifier.clickable { if (countInIntervalMs(tempo) != null) running = true }.padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Text("♩=$tempo", style = MaterialTheme.typography.labelMedium)
        val active = beat >= 0
        val dot = when { active && isDownbeat(beat) -> 12.dp; active -> 8.dp; else -> 7.dp }
        Box(
            Modifier.size(dot).background(
                if (active) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outline,
                CircleShape,
            ),
        )
    }
}

private data class PageBitmaps(val raster: ImageBitmap?, val overlays: List<ImageBitmap>)

/** Cache-or-decode a single blob; null on any failure (the caller degrades gracefully). */
private fun decodeCached(cache: PageImageCache, ref: String, w: Int, h: Int, decoder: ImageDecoder): ImageBitmap? {
    val key = cacheKey(ref, w, h)
    cache.get(key)?.let { return it }
    val bmp = decoder.decode(ref, w, h).getOrNull() ?: return null
    cache.put(key, bmp)
    return bmp
}

@Composable
private fun PlaceholderCard(modifier: Modifier) {
    Box(modifier, contentAlignment = Alignment.Center) {
        Surface(color = MaterialTheme.colorScheme.surfaceVariant, shape = MaterialTheme.shapes.medium) {
            Text("Page unavailable", Modifier.padding(24.dp), style = MaterialTheme.typography.bodyLarge)
        }
    }
}

@Composable
private fun CenteredMessage(title: String, body: String, onExit: () -> Unit) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(
            Modifier.padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(title, style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
            if (body.isNotEmpty()) Text(body, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center)
            OutlinedButton(onClick = onExit) { Text("Back") }
        }
    }
}

@Composable
private fun LayersDialog(state: StageState, vm: StageViewModel, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = onDismiss) { Text("Done") } },
        title = { Text("Layers") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                state.layers.forEach { layer ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(
                            checked = layer.layerId in state.visibleLayers,
                            enabled = !layer.mandatory, // mandatory layers are locked on (I12)
                            onCheckedChange = { vm.setLayerVisible(layer.layerId, it) },
                        )
                        val suffix = when {
                            layer.mandatory -> " (required)"
                            layer.roleTag.isNotEmpty() -> " (${layer.roleTag})"
                            else -> ""
                        }
                        Text(layer.layerId + suffix)
                    }
                }
            }
        },
    )
}

@Composable
private fun RoleDialog(state: StageState, vm: StageViewModel, onDismiss: () -> Unit) {
    var text by remember { mutableStateOf(state.role) }
    val suggestions = remember(state.layers) { state.layers.map { it.roleTag }.filter { it.isNotEmpty() }.distinct() }
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = { vm.setRole(text.trim()); onDismiss() }) { Text("Set") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        title = { Text("My role") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text("Sets which optional layers show by default. This is a reading preference, not a login.")
                OutlinedTextField(value = text, onValueChange = { text = it }, singleLine = true)
                if (suggestions.isNotEmpty()) {
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        suggestions.forEach { s -> OutlinedButton(onClick = { text = s }) { Text(s) } }
                    }
                }
            }
        },
    )
}

/** Tap the left/right third = previous/next; horizontal swipe does the same. */
private fun Modifier.pointerNavigation(navKey: Any, onPrev: () -> Unit, onNext: () -> Unit): Modifier = this
    .pointerInputTap(navKey, onPrev, onNext)
    .pointerInputSwipe(navKey, onPrev, onNext)

private fun Modifier.pointerInputTap(key: Any, onPrev: () -> Unit, onNext: () -> Unit): Modifier =
    pointerInput(key) {
        detectTapGestures { offset ->
            val third = size.width / 3f
            if (offset.x < third) onPrev() else if (offset.x > third * 2f) onNext()
        }
    }

private fun Modifier.pointerInputSwipe(key: Any, onPrev: () -> Unit, onNext: () -> Unit): Modifier =
    pointerInput(key) {
        var dx = 0f
        detectHorizontalDragGestures(
            onDragStart = { dx = 0f },
            onDragEnd = {
                val threshold = size.width * 0.15f
                if (dx > threshold) onPrev() else if (dx < -threshold) onNext()
            },
        ) { _, amount -> dx += amount }
    }
