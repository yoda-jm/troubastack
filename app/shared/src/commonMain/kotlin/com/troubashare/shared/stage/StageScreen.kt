// TroubaStage UI — Compose Multiplatform (commonMain). A pager over composited pages plus the
// minimal reading controls (I12). Image DECODING is injected as a plain function from the platform
// (androidApp) — NOT a new expect/actual seam (seams are capped at three, I15). Every decode returns
// a Result and is treated as "maybe a placeholder", so a bad image never crashes the performance.
package com.troubashare.shared.stage

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
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
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalDrawerSheet
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.NavigationDrawerItem
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDrawerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.painter.BitmapPainter
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
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
class PageImageCache(maxEntries: Int = 64) {
    // Thin typed wrapper over the generic access-order [LruCache] (the LRU logic lives there and is
    // unit-tested off-device). B1: budget raised from 12 → 64 so a page's own raster+overlays (and a
    // few neighbours) stay resident across re-decodes; [pin] additionally makes the on-screen page's
    // entries eviction-proof — together these stop "same page, fewer annotations".
    private val lru = LruCache<String, ImageBitmap>(maxEntries)

    fun get(key: String): ImageBitmap? = lru.get(key)

    fun put(key: String, bmp: ImageBitmap) = lru.put(key, bmp)

    /** Protect [owner]'s displayed-page cache keys from eviction (B1); additive across owners. */
    fun pin(owner: Any, keys: Set<String>) = lru.pin(owner, keys)

    /** Release [owner]'s pins when its page leaves composition (B1). */
    fun unpin(owner: Any) = lru.unpin(owner)
}

private fun cacheKey(ref: String, w: Int, h: Int): String = "$ref@${w}x$h"

/** The cache keys a page occupies at a given size (raster + visible overlays) — used to pin it (B1). */
private fun pageCacheKeys(rasterRef: String, overlayRefs: List<String>, w: Int, h: Int): Set<String> =
    buildSet {
        add(cacheKey(rasterRef, w, h))
        overlayRefs.forEach { add(cacheKey(it, w, h)) }
    }

/** Root of the Stage UI: failure screen, empty state, or the performing pager. */
@Composable
fun StageScreen(
    vm: StageViewModel,
    decoder: ImageDecoder,
    onExit: () -> Unit,
    // A10: local night-mode preference, injected + persisted by the entrypoint (app DI, no seam).
    initialColorMode: StageColorMode = StageColorMode.NORMAL,
    onColorModeChange: (StageColorMode) -> Unit = {},
    // A14: persist the reading mode (page/width/scroll) globally; the entrypoint seeds the VM's
    // initialFit and writes this callback (A10 pattern). No-op default ⇒ mode simply isn't persisted.
    onFitModeChange: (FitMode) -> Unit = {},
    // P201/I13: show the rehearsal "Auto-update" toggle. Only true when the concert is
    // server-backed (the host can poll for a new rev). The toggle state is TRANSIENT —
    // it lives in the VM (StageState.autoUpdate) and resets when Stage is left; the host
    // watches it to run/stop the poll loop. Default false ⇒ no toggle (iOS host, tests).
    canAutoUpdate: Boolean = false,
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
            else -> Performing(state, vm, decoder, onExit, initialColorMode, onColorModeChange, onFitModeChange, canAutoUpdate)
        }
    }
}

@Composable
@OptIn(ExperimentalMaterial3Api::class)
private fun Performing(
    state: StageState,
    vm: StageViewModel,
    decoder: ImageDecoder,
    onExit: () -> Unit,
    initialColorMode: StageColorMode,
    onColorModeChange: (StageColorMode) -> Unit,
    onFitModeChange: (FitMode) -> Unit,
    canAutoUpdate: Boolean,
) {
    var colorMode by remember { mutableStateOf(initialColorMode) }
    val cache = remember { PageImageCache() }
    val page = state.currentPage ?: return
    var showLayers by remember { mutableStateOf(false) }
    var showRole by remember { mutableStateOf(false) }
    // A2: the settings sheet (reading mode / layers / role / day-night) opened from the ⚙ FAB.
    var showSettings by remember { mutableStateOf(false) }
    // A2: immersive chrome. The score is edge-to-edge on black; controls fade/slide in on a middle-tap
    // and auto-hide after a timeout so performance is fullscreen. Starts revealed so the controls are
    // discoverable on entry, then hides itself.
    var chromeVisible by remember { mutableStateOf(true) }
    // A15: song-jump navigation drawer, opened from the ☰ menu FAB.
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    // Auto-hide the chrome while nothing modal is open; any re-reveal or opened surface restarts it.
    val overlayOpen = drawerState.isOpen || showSettings || showLayers || showRole
    LaunchedEffect(chromeVisible, overlayOpen) {
        if (chromeVisible && !overlayOpen) autoHideChrome(CHROME_AUTO_HIDE_MS) { chromeVisible = false }
    }
    // N1: continuous advance is a performance requirement (pedal users can't stop at every song end),
    // but crossing a song boundary must READ as crossing — so on a cross-song move we flash the
    // title/position card ALONE (~2s), not the full chrome. We watch the current song index: it only
    // changes when navigation crosses a boundary (in ANY mode). The first composition seeds lastCueSong
    // so entering Stage never fires a spurious cue. Reuses the A17 auto-hide timer (clock-injectable).
    var boundaryCueVisible by remember { mutableStateOf(false) }
    var lastCueSong by remember { mutableStateOf(state.currentSong) }
    LaunchedEffect(state.currentSong) {
        if (state.currentSong != lastCueSong) {
            lastCueSong = state.currentSong
            boundaryCueVisible = true
            autoHideChrome(BOUNDARY_CUE_MS) { boundaryCueVisible = false }
        }
    }
    // A14: the continuous-scroll column's list state — the source of truth for the topmost page while
    // in SCROLL mode (pager label + page turns read/drive it). Unused in page/width modes.
    val scrollListState = rememberLazyListState()

    // Hardware page turns (A09): BT pedals/keyboards send PageUp/Down, arrows, Space. Capture at the
    // root before children so a keyboard turns the page while on-screen taps still work. (Android
    // volume keys can't reach Compose; androidApp forwards them via onKeyDown.)
    val keyFocus = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { keyFocus.requestFocus() } }

    // A15: the song drawer wraps the whole presenter so its scrim covers the pages when open. Swipe
    // gestures are enabled only WHILE open (swipe-to-close) — a left-edge swipe must never open it
    // mid-performance, which would fight the page-turn swipe (A12/A04). Open is via the Songs button.
    ModalNavigationDrawer(
        drawerState = drawerState,
        gesturesEnabled = drawerState.isOpen,
        drawerContent = {
            SongDrawerSheet(state) { i ->
                // N2: a jump changes the current song in EVERY mode. In scroll mode that switches the
                // per-song column (the positioning effect lands it at the song's top); in page/width it
                // moves the discrete current page. Either way the drawer closes.
                vm.goToSong(i)
                scope.launch { drawerState.close() }
            }
        },
    ) {
    // A12: facing pages. When the viewport is landscape (w > h) AND the fit is FIT_PAGE we show two
    // adjacent pages (2k/2k+1) side by side and turn by the spread; portrait or FIT_WIDTH stays
    // single-page exactly as before. The measurement decides the layout, so it wraps everything.
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val scrollMode = state.fitMode == FitMode.SCROLL
        // A14: scroll wins over two-up (a single column); two-up only in landscape FIT_PAGE.
        val twoUp = maxWidth > maxHeight && state.fitMode == FitMode.FIT_PAGE
        val widthPx = with(LocalDensity.current) { maxWidth.roundToPx() }
        // N2: in scroll mode the column holds ONLY the current song's pages, so the LazyList index is
        // LOCAL to the song. songRange is that song's global span; map the local top back to a global
        // page for the label. Empty off scroll mode.
        val songRange = if (scrollMode) songPageRange(state, state.current) else IntRange.EMPTY
        val localTop = scrollListState.firstVisibleItemIndex
        val topPage = if (scrollMode) (songRange.first + localTop).coerceIn(0, state.pages.lastIndex.coerceAtLeast(0)) else state.current
        // One navigation entry point (keys, taps, swipes, arrows, volume) so a "turn" means the same
        // everywhere: spread-aware in two-up; in scroll a turn steps WITHIN the current song's column and
        // at a column EDGE crosses to the adjacent song (goToPage clamps and, by changing the song, fires
        // the N1 boundary cue). Vertical motion thus always reads within the song; crossing is explicit.
        val turnNext: () -> Unit = {
            if (scrollMode) {
                val step = scrollNextPage(localTop, songRange.count())
                if (step != localTop) scope.launch { scrollListState.animateScrollToItem(step) }
                else vm.goToPage(songRange.last + 1) // column end → next song's first page
            } else vm.goToPage(turnTarget(state.current, state.pageCount, twoUp, PageTurn.NEXT))
        }
        val turnPrev: () -> Unit = {
            if (scrollMode) {
                val step = scrollPrevPage(localTop)
                if (step != localTop) scope.launch { scrollListState.animateScrollToItem(step) }
                else vm.goToPage(songRange.first - 1) // column top → previous song's last page
            } else vm.goToPage(turnTarget(state.current, state.pageCount, twoUp, PageTurn.PREV))
        }
        val spread = spreadPages(state.current, state.pageCount)

        // A13: Android volume keys can't reach Compose; androidApp intercepts them in the Activity and
        // calls back through this registrar. Publish the SAME spread-aware turns every other input uses
        // so two-up turns by a whole spread (one press = one spread) instead of the old turn-by-1.
        // rememberUpdatedState keeps the forwarded handler current as the page/layout change without
        // re-registering; unregister on dispose so volume keys behave normally outside Stage (this is
        // also A09's dispose contract). iOS provides no registrar → the default no-op.
        val volumeTurnRegistrar = LocalVolumeTurnRegistrar.current
        val latestNext by rememberUpdatedState(turnNext)
        val latestPrev by rememberUpdatedState(turnPrev)
        DisposableEffect(volumeTurnRegistrar) {
            volumeTurnRegistrar { pt -> if (pt == PageTurn.NEXT) latestNext() else latestPrev() }
            onDispose { volumeTurnRegistrar(null) }
        }

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
        // N3: the page floats edge-to-edge on a BLACK canvas. ANY tap toggles the chrome (stageTaps,
        // all modes); page turns come from horizontal swipe (page/width modes) + ‹ › FABs + pedals/keys.
        // In scroll mode the LazyColumn owns the vertical drag (swipe disabled) and any tap still toggles.
        Box(
            Modifier
                .fillMaxSize()
                .background(Color.Black)
                .stageTaps(state.pageCount to (twoUp to scrollMode)) { chromeVisible = !chromeVisible }
                .then(if (scrollMode) Modifier else Modifier.pointerInputSwipe(state.pageCount to twoUp, turnPrev, turnNext)),
        ) {
            when {
                scrollMode -> ScrollReader(state, scrollListState, decoder, cache, colorMode.pageColorFilter(), widthPx)
                twoUp -> {
                    // A lone last page (spread of 1) fills the row; ContentScale.Fit centres it.
                    Row(Modifier.fillMaxSize()) {
                        spread.forEach { idx ->
                            PageView(state.pages[idx], state.visibleFor(state.pages[idx].songId), state.fitMode, decoder, cache, colorMode.pageColorFilter(), Modifier.weight(1f).fillMaxHeight())
                        }
                    }
                }
                else -> PageView(page, state.visibleFor(page.songId), state.fitMode, decoder, cache, colorMode.pageColorFilter(), Modifier.fillMaxSize())
            }
        }

        // A2: TOP chrome — ☰ song drawer · centered title+position card · [● Live] · ✕ exit. Fades and
        // slides in on reveal; the A08 meta strip rides inside it (score stays clean when hidden). In
        // scroll mode the strip is inline in the column (ScrollReader), so it's omitted here.
        AnimatedVisibility(
            visible = chromeVisible,
            enter = fadeIn(tween(250)) + slideInVertically(tween(250)) { -it },
            exit = fadeOut(tween(250)) + slideOutVertically(tween(250)) { -it },
            modifier = Modifier.align(Alignment.TopCenter),
        ) {
            Column(Modifier.fillMaxWidth().statusBarsPadding().padding(horizontal = 12.dp, vertical = 8.dp)) {
                Row(Modifier.fillMaxWidth(), Arrangement.SpaceBetween, Alignment.CenterVertically) {
                    StageFab("☰") { scope.launch { drawerState.open() } }
                    TitleCard(
                        title = state.songs.getOrNull(state.currentSong)?.name ?: "",
                        position = stagePositionLabel(state, topPage, twoUp),
                        modifier = Modifier.weight(1f).padding(horizontal = 12.dp),
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                        if (canAutoUpdate) StageFab(if (state.autoUpdate) "●" else "○") { vm.setAutoUpdate(!state.autoUpdate) }
                        // Settings lives in the TOP bar, not the bottom: MIUI's bottom gesture zone
                        // intercepts taps flush to the screen bottom, making a bottom ⚙ hard to hit.
                        StageFab("⚙") { showSettings = true }
                        StageFab("✕", container = Color(0xCCB3261E)) { onExit() }
                    }
                }
                when {
                    scrollMode -> {}
                    twoUp -> Row(Modifier.fillMaxWidth()) {
                        Box(Modifier.weight(1f)) { spread.getOrNull(0)?.let { MetaStrip(state.pages[it], resetKey = it) } }
                        Box(Modifier.weight(1f)) { spread.getOrNull(1)?.let { MetaStrip(state.pages[it], resetKey = it) } }
                    }
                    else -> MetaStrip(page, resetKey = state.current)
                }
            }
        }

        // N1: the song-boundary cue — the title/position card ALONE (no FABs, no meta strip), flashed
        // for ~2s on a cross-song advance so continuous advance still announces the new song. Suppressed
        // while full chrome is up (which already shows the card), so the two never stack.
        AnimatedVisibility(
            visible = boundaryCueVisible && !chromeVisible,
            enter = fadeIn(tween(200)),
            exit = fadeOut(tween(400)),
            modifier = Modifier.align(Alignment.TopCenter),
        ) {
            Box(Modifier.fillMaxWidth().statusBarsPadding().padding(horizontal = 12.dp, vertical = 8.dp), contentAlignment = Alignment.TopCenter) {
                TitleCard(
                    title = state.songs.getOrNull(state.currentSong)?.name ?: "",
                    position = stagePositionLabel(state, topPage, twoUp),
                )
            }
        }

        // A2: BOTTOM chrome — ‹ previous · ⚙ settings · next › (big round FABs).
        AnimatedVisibility(
            visible = chromeVisible,
            enter = fadeIn(tween(250)) + slideInVertically(tween(250)) { it },
            exit = fadeOut(tween(250)) + slideOutVertically(tween(250)) { it },
            modifier = Modifier.align(Alignment.BottomCenter),
        ) {
            Row(
                // ‹ › page-turn FABs at the thumb corners (reference-app). Extra bottom clearance keeps
                // them above MIUI's bottom gesture zone as much as possible; page turns also work via
                // edge-tap/swipe/pedals, so these are a convenience, not the only path.
                Modifier.fillMaxWidth().navigationBarsPadding().padding(start = 24.dp, end = 24.dp, top = 16.dp, bottom = 48.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                StageFab("‹", size = 64.dp) { turnPrev() }
                StageFab("›", size = 64.dp) { turnNext() }
            }
        }
        }
    }
    } // ModalNavigationDrawer

    if (showSettings) SettingsSheet(
        state = state,
        colorMode = colorMode,
        onFitMode = { vm.setFitMode(it); onFitModeChange(it) },
        onLayers = { showSettings = false; showLayers = true },
        onRole = { showSettings = false; showRole = true },
        onToggleColor = { colorMode = colorMode.next(); onColorModeChange(colorMode) },
        onDismiss = { showSettings = false },
    )
    if (showLayers) LayersDialog(state, vm) { showLayers = false }
    if (showRole) RoleDialog(state, vm) { showRole = false }
}

/** A2 — a round translucent Stage control (reference-app look): a glyph on a dark disc, white text. */
@Composable
private fun StageFab(glyph: String, size: Dp = 56.dp, container: Color = Color(0xC0000000), onClick: () -> Unit) {
    FloatingActionButton(
        onClick = onClick,
        containerColor = container,
        contentColor = Color.White,
        modifier = Modifier.size(size),
    ) { Text(glyph, style = MaterialTheme.typography.headlineSmall) }
}

/** A2 — the centered translucent song-title + position card (reference-app look). */
@Composable
private fun TitleCard(title: String, position: String, modifier: Modifier = Modifier) {
    if (title.isEmpty() && position.isEmpty()) { Spacer(modifier); return }
    Surface(modifier, color = Color(0xC0000000), shape = MaterialTheme.shapes.medium) {
        Column(
            Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (title.isNotEmpty()) Text(title, color = Color.White, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (position.isNotEmpty()) Text(position, color = Color.White.copy(alpha = 0.85f), style = MaterialTheme.typography.bodySmall, maxLines = 1)
        }
    }
}

/** "Song 2/4  ·  3–4/12" — the title card's position line (A2). Song part omitted when there are none. */
private fun stagePositionLabel(state: StageState, topPage: Int, twoUp: Boolean): String {
    val pages = pagerLabel(topPage, state.pageCount, twoUp)
    val i = state.currentSong
    return if (i >= 0 && state.songs.isNotEmpty()) "Song ${i + 1}/${state.songs.size}  ·  $pages" else pages
}

/**
 * A2 (Q2) — the Stage settings sheet: the reading-mode segmented control (Page | Width | Scroll) plus
 * the setup-time controls (layers, role, day/night) that used to clutter the top bar. Opened from the
 * ⚙ FAB; auto-hide pauses while it's up. Layers/Role open their existing dialogs (A1 will refine them).
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SettingsSheet(
    state: StageState,
    colorMode: StageColorMode,
    onFitMode: (FitMode) -> Unit,
    onLayers: () -> Unit,
    onRole: () -> Unit,
    onToggleColor: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState()) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text("Reading mode", style = MaterialTheme.typography.titleSmall)
            val modes = listOf(FitMode.FIT_PAGE to "Page", FitMode.FIT_WIDTH to "Width", FitMode.SCROLL to "Scroll")
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                modes.forEachIndexed { i, (mode, label) ->
                    SegmentedButton(
                        selected = state.fitMode == mode,
                        onClick = { onFitMode(mode) },
                        shape = SegmentedButtonDefaults.itemShape(i, modes.size),
                    ) { Text(label) }
                }
            }
            // A1/Q3 — ROLE-FIRST: picking a role seeds the right layers for the whole concert; most
            // users never open Layers. Layers is the demoted "Advanced" exception and scopes to the
            // CURRENT song only (per-song, A1). Order reflects that: Role, then day/night, then Layers.
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedButton(onClick = onRole, modifier = Modifier.weight(1f)) { Text(if (state.role.isEmpty()) "Role" else "Role: ${state.role}") }
                OutlinedButton(onClick = onToggleColor, modifier = Modifier.weight(1f)) { Text(if (colorMode == StageColorMode.NIGHT) "Night" else "Day") }
                if (state.layers.isNotEmpty()) OutlinedButton(onClick = onLayers, modifier = Modifier.weight(1f)) { Text("Layers…") }
            }
        }
    }
}

/**
 * A15 — the song-jump navigation drawer. Lists every song in setlist order with the A08 meta line
 * (notes · key · ♩=tempo) from its first page; the current song is highlighted; items are large
 * touch targets for a stage. Read-only (I12): tapping jumps via [StageViewModel.goToSong] (which is
 * spread-aligned in two-up, A12) and closes. The scrim/back closes it without navigating.
 */
@Composable
private fun SongDrawerSheet(state: StageState, onJump: (Int) -> Unit) {
    ModalDrawerSheet {
        Text(
            "Songs",
            Modifier.padding(horizontal = 28.dp, vertical = 16.dp),
            style = MaterialTheme.typography.titleLarge,
        )
        // T23: bench/encore songs group below the running order under an "On call" header. Each item
        // keeps its ORIGINAL song index (via withIndex) so a jump still lands on the right pages,
        // regardless of how the bundle ordered them.
        val (bench, main) = state.songs.withIndex().partition { it.value.onCall }
        main.forEach { (i, s) -> SongDrawerItem(state, i, s, onJump) }
        if (bench.isNotEmpty()) {
            HorizontalDivider(Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
            Text(
                "On call",
                Modifier.padding(horizontal = 28.dp, vertical = 4.dp),
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            bench.forEach { (i, s) -> SongDrawerItem(state, i, s, onJump) }
        }
    }
}

/** One row of the song drawer: title + the A08 meta line; highlighted when it is the current song. */
@Composable
private fun SongDrawerItem(state: StageState, i: Int, s: SongInfo, onJump: (Int) -> Unit) {
    val meta = songMetaLine(state, i)
    NavigationDrawerItem(
        selected = i == state.currentSong,
        label = {
            Column {
                Text(s.name, style = MaterialTheme.typography.titleMedium)
                if (meta != null) Text(meta, style = MaterialTheme.typography.labelMedium)
            }
        },
        onClick = { onJump(i) },
        modifier = Modifier.padding(horizontal = 12.dp, vertical = 2.dp),
    )
}

/** Approx page aspect (w/h) used to reserve a scroll item's height before its bitmap has decoded. */
private const val SCROLL_PLACEHOLDER_ASPECT = 0.773f // US Letter portrait (8.5 / 11)

/**
 * N2 — PER-SONG continuous scroll: a lazy vertical column of ONLY the current song's pages at
 * fit-width, with A08's metadata strip inline above the song's first page. Vertical motion thus always
 * reads within the current song; crossing to another song is an explicit act (the parent's turns cross
 * at a column edge, the drawer jumps) so "where am I" is never ambiguous. Whenever the song changes the
 * column is (re)positioned: a cross lands at the song's top/bottom, and re-entering scroll mid-song
 * lands on the current page. Laziness + the shared LRU cache keep decode pressure bounded; night mode
 * (A10) applies via [colorFilter] exactly as elsewhere.
 */
@Composable
private fun ScrollReader(
    state: StageState,
    listState: LazyListState,
    decoder: ImageDecoder,
    cache: PageImageCache,
    colorFilter: androidx.compose.ui.graphics.ColorFilter?,
    widthPx: Int,
) {
    val range = songPageRange(state, state.current)
    val songPages = if (range.isEmpty()) emptyList() else state.pages.subList(range.first, range.last + 1)
    LaunchedEffect(state.currentSong) {
        // Land on the current page WITHIN this song's column (local index); a cross set current to the
        // song's first/last page, so this positions the column at its top/bottom accordingly.
        listState.scrollToItem((state.current - range.first).coerceIn(0, (songPages.size - 1).coerceAtLeast(0)))
    }
    LazyColumn(state = listState, modifier = Modifier.fillMaxSize()) {
        itemsIndexed(songPages) { index, page ->
            Column(Modifier.fillMaxWidth()) {
                MetaStrip(page, resetKey = range.first + index) // renders only on the song's first page
                ScrollPage(page, state.visibleFor(page.songId), decoder, cache, colorFilter, widthPx)
            }
        }
    }
}

/** One page in the scroll column: fills width, height from the decoded aspect; degrades like PageView. */
@Composable
private fun ScrollPage(
    page: StagePage,
    visibleLayers: Set<String>,
    decoder: ImageDecoder,
    cache: PageImageCache,
    colorFilter: androidx.compose.ui.graphics.ColorFilter?,
    widthPx: Int,
) {
    if (page.status == PageStatus.UNAVAILABLE) {
        PlaceholderCard(Modifier.fillMaxWidth().aspectRatio(SCROLL_PLACEHOLDER_ASPECT))
        return
    }
    val overlayRefs = page.overlays.filter { it.layerId in visibleLayers }.map { it.imageRef }
    // B1: each displayed page pins its own keys under a stable owner so several ScrollPages on screen
    // at once don't clobber each other's pins; release on leave so off-screen pages can be evicted.
    val owner = remember { Any() }
    DisposableEffect(owner) { onDispose { cache.unpin(owner) } }
    // B1: retry a failed overlay decode on the next few frames (a failure isn't cached, so a re-decode
    // re-attempts); if it keeps failing we badge it — never silently show fewer annotations.
    var retryTick by remember(page.rasterRef, overlayRefs, widthPx) { mutableStateOf(0) }
    // Decode at column width with a generous height cap; the decoder preserves aspect ⇒ width-bound.
    val decoded by produceState<PageBitmaps?>(null, page.rasterRef, overlayRefs, widthPx, retryTick) {
        value = null
        if (widthPx <= 0) return@produceState
        value = withContext(Dispatchers.Default) {
            val raster = decodeCached(cache, page.rasterRef, widthPx, widthPx * 3, decoder)
            val ov = decodeOverlays(overlayRefs) { decodeCached(cache, it, widthPx, widthPx * 3, decoder) }
            cache.pin(owner, pageCacheKeys(page.rasterRef, overlayRefs, widthPx, widthPx * 3)) // eviction-proof this page
            PageBitmaps(raster, ov.overlays, ov.missing)
        }
    }
    LaunchedEffect(decoded?.missingOverlays, retryTick) {
        if ((decoded?.missingOverlays ?: 0) > 0 && retryTick < OVERLAY_DECODE_RETRIES) { delay(250); retryTick++ }
    }
    val bitmaps = decoded
    when {
        bitmaps?.raster == null -> Box(Modifier.fillMaxWidth().aspectRatio(SCROLL_PLACEHOLDER_ASPECT)) {
            if (bitmaps != null) PlaceholderCard(Modifier.fillMaxSize()) // decoded but raster failed
        }
        else -> {
            val aspect = bitmaps.raster.width.toFloat() / bitmaps.raster.height.toFloat()
            Box(Modifier.fillMaxWidth().aspectRatio(aspect), contentAlignment = Alignment.Center) {
                Image(BitmapPainter(bitmaps.raster), contentDescription = null, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.FillWidth, colorFilter = colorFilter)
                bitmaps.overlays.forEach {
                    Image(BitmapPainter(it), contentDescription = null, modifier = Modifier.fillMaxSize(), contentScale = ContentScale.FillWidth, colorFilter = colorFilter)
                }
                MissingLayersBadge(bitmaps.missingOverlays, Modifier.align(Alignment.TopEnd))
            }
        }
    }
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

        // B1: each displayed page pins under a stable owner (two-up composes two PageViews at once, so
        // the right page must not unpin the left); release on leave so off-screen pages can be evicted.
        val owner = remember { Any() }
        DisposableEffect(owner) { onDispose { cache.unpin(owner) } }
        // B1: retry a failed overlay decode a few frames, then badge — never silently fewer annotations.
        var retryTick by remember(page.rasterRef, overlayRefs, wPx, hPx) { mutableStateOf(0) }
        val decoded by produceState<PageBitmaps?>(null, page.rasterRef, overlayRefs, wPx, hPx, retryTick) {
            value = null
            if (wPx <= 0 || hPx <= 0) return@produceState
            value = withContext(Dispatchers.Default) {
                val raster = decodeCached(cache, page.rasterRef, wPx, hPx, decoder)
                val ov = decodeOverlays(overlayRefs) { decodeCached(cache, it, wPx, hPx, decoder) }
                cache.pin(owner, pageCacheKeys(page.rasterRef, overlayRefs, wPx, hPx)) // eviction-proof this page
                PageBitmaps(raster, ov.overlays, ov.missing)
            }
        }
        LaunchedEffect(decoded?.missingOverlays, retryTick) {
            if ((decoded?.missingOverlays ?: 0) > 0 && retryTick < OVERLAY_DECODE_RETRIES) { delay(250); retryTick++ }
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
                MissingLayersBadge(bitmaps.missingOverlays, Modifier.align(Alignment.TopEnd))
            }
        }
    }
}

/** B1 — a small visible badge when one or more annotation layers failed to decode, so a missing layer
 *  is never mistaken for "no layer". Renders nothing when all overlays are present. */
@Composable
private fun MissingLayersBadge(missing: Int, modifier: Modifier = Modifier) {
    if (missing <= 0) return
    Surface(
        modifier.padding(8.dp),
        color = Color(0xCCB3261E),
        shape = MaterialTheme.shapes.small,
    ) {
        Text(
            "⚠ $missing layer${if (missing == 1) "" else "s"} unavailable",
            Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
            color = Color.White,
            style = MaterialTheme.typography.labelSmall,
        )
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

private data class PageBitmaps(val raster: ImageBitmap?, val overlays: List<ImageBitmap>, val missingOverlays: Int = 0)

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
    // A1: layer visibility is PER-SONG — show/toggle the CURRENT song's set. Role-first (Q3): this is
    // the "Advanced: layers" exception path; most users just pick a role and let the defaults ride.
    val songId = state.currentPage?.songId ?: ""
    val visible = state.visibleFor(songId)
    val songName = state.songs.getOrNull(state.currentSong)?.name ?: ""
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = onDismiss) { Text("Done") } },
        title = { Text(if (songName.isEmpty()) "Layers — this song" else "Layers — $songName") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text("Overrides just this song; changing your role resets it.", style = MaterialTheme.typography.bodySmall)
                state.layers.forEach { layer ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(
                            checked = layer.layerId in visible,
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

/**
 * N3 — page-area taps: EVERY tap, in EVERY mode, toggles the chrome (the pure [tapAction] pins that
 * contract). Page turns are swipe ([pointerInputSwipe], page/width) + ‹ › FABs + pedals/keys only —
 * edge-tap-turn was dropped because an accidental turn reads as a rendering glitch mid-performance.
 */
private fun Modifier.stageTaps(
    key: Any,
    onToggleChrome: () -> Unit,
): Modifier = pointerInput(key) {
    detectTapGestures {
        when (tapAction()) {
            TapAction.TOGGLE_CHROME -> onToggleChrome()
            TapAction.PREV, TapAction.NEXT -> {} // taps never navigate after N3
        }
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
