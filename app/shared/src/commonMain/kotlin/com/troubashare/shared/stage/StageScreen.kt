// TroubaStage UI — Compose Multiplatform (commonMain). A pager over composited pages plus the
// minimal reading controls (I12). Image DECODING is injected as a plain function from the platform
// (androidApp) — NOT a new expect/actual seam (seams are capped at three, I15). Every decode returns
// a Result and is treated as "maybe a placeholder", so a bad image never crashes the performance.
package com.troubashare.shared.stage

import com.troubashare.shared.bundle.LayerImage
import com.troubashare.shared.bundle.SongCue
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.ui.draw.clip
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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import androidx.compose.material3.Switch
import androidx.compose.material3.DrawerValue
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.FloatingActionButtonDefaults
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
import androidx.compose.runtime.State
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

// P201/R10: the key includes the content [ver] (raster/overlay hash), NOT just the blob ref. Baked
// blob filenames are STABLE across revs (blobs/s0-p0-raster.png, …-L-<id>.png), so a rehearsal
// auto-update swaps a page's CONTENT under the same ref; keying on ref alone served the stale bitmap
// (task #23 — found by the attended 2-device test). With the hash in the key a content change is a
// natural cache miss; unchanged pages still hit (their hash is unchanged), and old entries age out.
internal fun cacheKey(ref: String, ver: String, w: Int, h: Int): String = "$ref#$ver@${w}x$h"

/** The cache keys a page occupies at a given size (raster + visible overlays) — used to pin it (B1). */
private fun pageCacheKeys(rasterRef: String, rasterVer: String, overlays: List<LayerImage>, w: Int, h: Int): Set<String> =
    buildSet {
        add(cacheKey(rasterRef, rasterVer, w, h))
        overlays.forEach { add(cacheKey(it.imageRef, it.contentHash, w, h)) }
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
    // P205 Stage 3a: persist the viewer's identity pick per concert. Called when the "Who are you?"
    // picker or the "Switch" affordance sets an identity; the host writes it (I12 — a view preference).
    onIdentityChange: (String) -> Unit = {},
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
            else -> Performing(state, vm, decoder, onExit, initialColorMode, onColorModeChange, onFitModeChange, canAutoUpdate, onIdentityChange)
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
    onIdentityChange: (String) -> Unit = {},
) {
    var colorMode by remember { mutableStateOf(initialColorMode) }
    val cache = remember { PageImageCache() }
    val page = state.currentPage ?: return
    var showLayers by remember { mutableStateOf(false) }
    // P205 Stage 3a: identity picker state. On a band-wide bundle with no resolved identity we prompt
    // once (dismiss ⇒ anonymous this session); the settings sheet's "Switch" re-opens it any time.
    var switchIdentity by remember { mutableStateOf(false) }
    var pickDismissed by remember { mutableStateOf(false) }
    var showRole by remember { mutableStateOf(false) }
    // A2: the settings sheet (reading mode / layers / role / day-night) opened from the ⚙ FAB.
    var showSettings by remember { mutableStateOf(false) }
    // A2: immersive chrome. The score is edge-to-edge on black; controls fade/slide in on a middle-tap
    // and auto-hide after a timeout so performance is fullscreen. Starts revealed so the controls are
    // discoverable on entry, then hides itself.
    var chromeVisible by remember { mutableStateOf(true) }
    // A34: the visual beat (metronome/count-in). Scoped to the current SONG, not the page, so the beat
    // keeps ticking as you turn pages within a song (VLL: navigating must not stop it); a song change —
    // where the tempo differs — resets it. The FAB taps it, the edge frame + centre count render it.
    val stageBeat = rememberStageBeat(resetKey = state.currentSong)
    // A15: song-jump navigation drawer, opened from the ☰ menu FAB.
    val drawerState = rememberDrawerState(DrawerValue.Closed)
    val scope = rememberCoroutineScope()
    // Auto-hide the chrome while nothing modal is open; any re-reveal or opened surface restarts it.
    val overlayOpen = drawerState.isOpen || showSettings || showLayers || showRole
    LaunchedEffect(chromeVisible, overlayOpen) {
        if (chromeVisible && !overlayOpen) autoHideChrome(CHROME_AUTO_HIDE_MS) { chromeVisible = false }
    }
    // N1 + N7 share ONE transient center-overlay layer (latest-wins): the N1 song-boundary card and the
    // N7 blocked-turn glyph are mutually exclusive per action, so each trigger CLEARS the other and bumps
    // [cueEpoch]; a single timeout keyed on the epoch restarts on every trigger (rapid pedal-at-the-wall
    // keeps one cue, refreshed, then one fade). blockedForward is retained across the fade so the glyph
    // renders the right direction while it fades out.
    var boundaryCueVisible by remember { mutableStateOf(false) } // N1/A20: song-entry flash is up
    var blockedCueVisible by remember { mutableStateOf(false) }  // N7: end/start glyph
    var blockedForward by remember { mutableStateOf(true) }      // N7: last blocked direction (for the fade)
    var cueIsEntry by remember { mutableStateOf(false) }         // A20: this flash is the INITIAL concert entry
    var cueEpoch by remember { mutableStateOf(0) }
    // N1/A20: flash on EVERY song entry, including the first — so opening concert mode shows the first
    // song's cues too (A20; VLL). lastCueSong seeded to -1 so the first run (song 0) fires; cueIsEntry
    // marks that initial run so the TITLE card is suppressed on entry (chrome already shows the title,
    // and entry isn't a boundary CROSS) while the CUE squares still flash. A cross clears any blocked cue.
    var lastCueSong by remember { mutableStateOf(-1) }
    LaunchedEffect(state.currentSong) {
        if (state.currentSong != lastCueSong) {
            cueIsEntry = (lastCueSong == -1)
            lastCueSong = state.currentSong
            boundaryCueVisible = true; blockedCueVisible = false; cueEpoch++
        }
    }
    // Shared cue timeout (clock-injectable via autoHideChrome, like A17): restarts on each trigger.
    LaunchedEffect(cueEpoch) {
        if (!boundaryCueVisible && !blockedCueVisible) return@LaunchedEffect
        val ms = if (blockedCueVisible) BLOCKED_TURN_CUE_MS else BOUNDARY_CUE_MS
        autoHideChrome(ms) { boundaryCueVisible = false; blockedCueVisible = false }
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
        val heightPx = with(LocalDensity.current) { maxHeight.roundToPx() }
        // N2: in scroll mode the column holds ONLY the current song's pages, so the LazyList index is
        // LOCAL to the song. songRange is that song's global span; map the local top back to a global
        // page for the label. Empty off scroll mode.
        val songRange = if (scrollMode) songPageRange(state, state.current) else IntRange.EMPTY
        val localTop = scrollListState.firstVisibleItemIndex
        val topPage = if (scrollMode) (songRange.first + localTop).coerceIn(0, state.pages.lastIndex.coerceAtLeast(0)) else state.current
        // N6: two-up spreads are SONG-ALIGNED — the facing-pages math takes each song's first global
        // page so a spread never straddles a song boundary.
        val songStarts = state.songs.map { it.firstPage }
        // N9: prefetch the pages a NEXT/PREV turn would show, so the page-turn slide reveals REAL
        // content instead of a blank decode. Fires on settle (PREFETCH_SETTLE_MS after the page/layout
        // changes) so it never competes with the current page's own decode; decodes at the SAME size the
        // incoming PageView will use (full area, or half-width per page in two-up) so the cache keys hit.
        // decodeCached puts UNPINNED (pin #1 — only displayed pages pin, A19), so prefetched neighbours
        // stay evictable under memory pressure. Skipped in scroll mode (no horizontal page slide there).
        if (!scrollMode && widthPx > 0 && heightPx > 0) {
            LaunchedEffect(state.current, twoUp, widthPx, heightPx, state.pageCount) {
                delay(PREFETCH_SETTLE_MS)
                val targets = prefetchTargets(state.current, state.pageCount, twoUp, songStarts)
                val pw = if (twoUp) widthPx / 2 else widthPx
                withContext(Dispatchers.Default) {
                    targets.forEach { idx ->
                        state.pages.getOrNull(idx)?.let { decodeCached(cache, it.rasterRef, it.rasterHash, pw, heightPx, decoder) }
                    }
                }
            }
        }
        // One navigation entry point (keys, taps, swipes, arrows, volume) so a "turn" means the same
        // everywhere: spread-aware in two-up; in scroll a turn steps WITHIN the current song's column and
        // at a column EDGE crosses to the adjacent song (goToPage clamps and, by changing the song, fires
        // the N1 boundary cue). Vertical motion thus always reads within the song; crossing is explicit.
        // N7: in page/width a turn blocked at the very first/last page is a true no-op (A22); flash the
        // end/start glyph so a dead swipe reads as "you're at the edge". Scroll's native rubber-band
        // already communicates the edge, so N7 is page/width only.
        val flashBlocked: (Boolean) -> Unit = { forward ->
            blockedForward = forward; blockedCueVisible = true; boundaryCueVisible = false; cueEpoch++
        }
        val turnNext: () -> Unit = {
            if (scrollMode) {
                val step = scrollNextPage(localTop, songRange.count())
                if (step != localTop) scope.launch { scrollListState.animateScrollToItem(step) }
                else vm.goToPage(songRange.last + 1) // column end → next song's first page
            } else if (isBlockedTurn(state.current, state.pageCount, twoUp, PageTurn.NEXT, songStarts)) {
                flashBlocked(true)
            } else vm.goToPage(turnTarget(state.current, state.pageCount, twoUp, PageTurn.NEXT, songStarts))
        }
        val turnPrev: () -> Unit = {
            if (scrollMode) {
                val step = scrollPrevPage(localTop)
                if (step != localTop) scope.launch { scrollListState.animateScrollToItem(step) }
                else vm.goToPage(songRange.first - 1) // column top → previous song's last page
            } else if (isBlockedTurn(state.current, state.pageCount, twoUp, PageTurn.PREV, songStarts)) {
                flashBlocked(false)
            } else vm.goToPage(turnTarget(state.current, state.pageCount, twoUp, PageTurn.PREV, songStarts))
        }
        // N8: in scroll mode the vertical axis IS the column, so a HORIZONTAL swipe crosses SONGS (the
        // free axis) — right = previous song, left = next — reusing the same cross path as the
        // scroll-edge turn (goToPage to the adjacent song, which fires the N1 cue + repositions the
        // column). At the first/last song it's a blocked cross → the N7 glyph (scroll's rubber-band only
        // communicates VERTICAL ends). "Horizontal swipe advances the unit" now holds in every mode.
        val scrollSwipeNext: () -> Unit = {
            if (isBlockedSongCross(state.currentSong, state.songs.size, forward = true)) flashBlocked(true)
            else vm.goToPage(songRange.last + 1)
        }
        val scrollSwipePrev: () -> Unit = {
            if (isBlockedSongCross(state.currentSong, state.songs.size, forward = false)) flashBlocked(false)
            else vm.goToPage(songRange.first - 1)
        }
        val latestScrollNext = rememberUpdatedState(scrollSwipeNext)
        val latestScrollPrev = rememberUpdatedState(scrollSwipePrev)
        val spread = spreadPages(state.current, songStarts, state.pageCount)

        // A13: Android volume keys can't reach Compose; androidApp intercepts them in the Activity and
        // calls back through this registrar. Publish the SAME spread-aware turns every other input uses
        // so two-up turns by a whole spread (one press = one spread) instead of the old turn-by-1.
        // rememberUpdatedState keeps the forwarded handler current as the page/layout change without
        // re-registering; unregister on dispose so volume keys behave normally outside Stage (this is
        // also A09's dispose contract). iOS provides no registrar → the default no-op.
        val volumeTurnRegistrar = LocalVolumeTurnRegistrar.current
        // rememberUpdatedState keeps the CURRENT turn handlers readable from long-lived callbacks whose
        // host isn't re-created every recomposition — the volume registrar (DisposableEffect keyed on
        // the registrar) AND the swipe gesture (pointerInput keyed on the layout, not the page). Reading
        // .value at fire time avoids the stale-closure bug where a swipe kept turning from page 0.
        val latestNext = rememberUpdatedState(turnNext)
        val latestPrev = rememberUpdatedState(turnPrev)
        DisposableEffect(volumeTurnRegistrar) {
            volumeTurnRegistrar { pt -> if (pt == PageTurn.NEXT) latestNext.value() else latestPrev.value() }
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
        // N3/N8: the page floats edge-to-edge on a BLACK canvas. ANY tap toggles the chrome (stageTaps,
        // all modes). A HORIZONTAL swipe navigates in EVERY mode — page/width turns pages, scroll crosses
        // songs (N8) — via the axis-locked detectHorizontalDragGestures, which only claims horizontal-
        // dominant drags so the LazyColumn keeps its vertical scroll untouched.
        Box(
            Modifier
                .fillMaxSize()
                .background(Color.Black)
                .stageTaps(state.pageCount to (twoUp to scrollMode)) { chromeVisible = !chromeVisible }
                .then(
                    if (scrollMode) Modifier.pointerInputSwipe(Unit, latestScrollPrev, latestScrollNext)
                    else Modifier.pointerInputSwipe(twoUp, latestPrev, latestNext)
                ),
        ) {
            when {
                scrollMode -> ScrollReader(state, scrollListState, decoder, cache, colorMode.pageColorFilter(), widthPx)
                // N4: page/width turns animate as a direction-aware horizontal slide (presentation only —
                // the turn is still the single goToPage funnel, so swipe/FABs/pedals/keys/volume all
                // animate identically). Keyed on state.current: a turn mid-animation just retargets, the
                // slide catches up to the new page (no queue, no dropped turn — target state always wins).
                // The content renders from `cur` (the animated page), so the OUTGOING page keeps its own
                // pages during the slide. Scroll mode keeps its own vertical motion and is excluded.
                else -> AnimatedContent(
                    targetState = state.current,
                    // N9: Material "shared-axis X" — a SHORT directional slide (not full-width) + fade,
                    // decelerate easing. Combined with N9 prefetch (incoming page already decoded) this
                    // reads as an intentional page turn rather than a blank pane sweeping across. N4's
                    // interruptibility (keyed on state.current, target-state-wins) is unchanged.
                    transitionSpec = {
                        val forward = targetState > initialState
                        val d = PAGE_TURN_ANIM_MS
                        val shift = { w: Int -> w / SHARED_AXIS_SHIFT_DIVISOR }
                        val enter = slideInHorizontally(tween(d, easing = FastOutSlowInEasing)) { w -> if (forward) shift(w) else -shift(w) } +
                            fadeIn(tween(d))
                        val exit = slideOutHorizontally(tween(d, easing = FastOutSlowInEasing)) { w -> if (forward) -shift(w) else shift(w) } +
                            fadeOut(tween(d))
                        enter togetherWith exit
                    },
                    modifier = Modifier.fillMaxSize(),
                    label = "page-turn",
                ) { cur ->
                    val placeholder = colorMode.pagePlaceholder()
                    if (twoUp) {
                        // A lone last page (spread of 1) fills the row; ContentScale.Fit centres it.
                        Row(Modifier.fillMaxSize()) {
                            spreadPages(cur, songStarts, state.pageCount).forEach { idx ->
                                PageView(state.pages[idx], state.visibleFor(state.pages[idx].songId), state.fitMode, decoder, cache, colorMode.pageColorFilter(), placeholder, Modifier.weight(1f).fillMaxHeight())
                            }
                        }
                    } else {
                        state.pages.getOrNull(cur)?.let { p ->
                            PageView(p, state.visibleFor(p.songId), state.fitMode, decoder, cache, colorMode.pageColorFilter(), placeholder, Modifier.fillMaxSize())
                        }
                    }
                }
            }
        }

        // A34: the visual beat — a pulsing frame on the page border PLUS a big, faint, tinted beat
        // number (1 2 3 4 …) in the middle so the player keeps their place. Above the page, below the
        // chrome. Purely visual (no pointer input) so a tap still turns the page / toggles the chrome;
        // you stop the beat from its FAB. Dark unless running.
        StageBeatFrame(stageBeat)

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
                        // A34: the metronome + its ∞ loop toggle, joined into ONE segmented capsule so
                        // they read as strongly connected. Only when the song has a tempo. Tapping the
                        // metronome starts it and closes the chrome for a clean page; ∞ = keep-running
                        // vs count-in.
                        if (page.tempo > 0) StageBeatControl(page.tempo, stageBeat, onStart = { chromeVisible = false })
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
                        Box(Modifier.weight(1f)) { spread.getOrNull(0)?.let { MetaStrip(state.pages[it]) } }
                        Box(Modifier.weight(1f)) { spread.getOrNull(1)?.let { MetaStrip(state.pages[it]) } }
                    }
                    else -> MetaStrip(page)
                }
            }
        }

        // N1 + A20: the song-entry cue — ONE overlay, ONE timeout. On a CROSS the title/position card
        // flashes at top (N1) — but not while full chrome is up (it already shows the title) nor on the
        // initial concert entry (that's not a boundary cross). The entered song's personal cues (A20)
        // flash LARGE in the center on EVERY entry, including the first (so opening the concert shows
        // "mic + red guitar" for song 1) — regardless of chrome, since they duplicate nothing there.
        AnimatedVisibility(
            visible = boundaryCueVisible,
            enter = fadeIn(tween(200)),
            exit = fadeOut(tween(400)),
            modifier = Modifier.fillMaxSize(),
        ) {
            Box(Modifier.fillMaxSize()) {
                if (!cueIsEntry && !chromeVisible) {
                    Box(Modifier.align(Alignment.TopCenter).fillMaxWidth().statusBarsPadding().padding(horizontal = 12.dp, vertical = 8.dp), contentAlignment = Alignment.TopCenter) {
                        TitleCard(
                            title = state.songs.getOrNull(state.currentSong)?.name ?: "",
                            position = stagePositionLabel(state, topPage, twoUp),
                        )
                    }
                }
                state.songs.getOrNull(state.currentSong)?.cues?.takeIf { it.isNotEmpty() }?.let { cues ->
                    CueFlashCard(cues, Modifier.align(Alignment.Center))
                }
            }
        }

        // N7: the end-of-bounds cue — a big semitransparent glyph flashed in the CENTER when a turn is
        // blocked at the very first/last page, so a dead swipe reads as "you're at the edge" not a broken
        // turn. Shares the N1 layer (latest-wins). blockedForward is retained across the fade.
        AnimatedVisibility(
            visible = blockedCueVisible,
            enter = fadeIn(tween(120)),
            exit = fadeOut(tween(400)),
            modifier = Modifier.align(Alignment.Center),
        ) {
            BlockedTurnGlyph(forward = blockedForward)
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
        canAutoUpdate = canAutoUpdate,
        onToggleAutoUpdate = { vm.setAutoUpdate(!state.autoUpdate) },
        onFitMode = { vm.setFitMode(it); onFitModeChange(it) },
        onLayers = { showSettings = false; showLayers = true },
        onRole = { showSettings = false; showRole = true },
        onToggleColor = { colorMode = colorMode.next(); onColorModeChange(colorMode) },
        onSwitchIdentity = { showSettings = false; switchIdentity = true },
        onDismiss = { showSettings = false },
    )
    if (showLayers) LayersDialog(state, vm) { showLayers = false }
    if (showRole) RoleDialog(state, vm) { showRole = false }
    // P205 Stage 3a: the "Who are you?" picker. Shows once on a band-wide bundle with no resolved
    // identity (VLL: connected+match auto-selects and SKIPS this — resolveIdentity already did), or
    // whenever the reader taps "Switch". Picking re-seeds the VM (setIdentity) and persists (host).
    if (switchIdentity || (needsIdentityPick(state.roster, state.identity) && !pickDismissed)) {
        WhoAreYouDialog(
            roster = state.roster,
            onPick = { m -> vm.setIdentity(m); onIdentityChange(m); switchIdentity = false; pickDismissed = false },
            onDismiss = { switchIdentity = false; pickDismissed = true },
        )
    }
}

// N5 — Stage control styling. The A17 disc was translucent-DARK on a BLACK canvas, so on the black
// margins the disc vanished and only the glyph floated (VLL: "black navigation on black"). Fix per the
// N5 ruling: keep the dark body (white glyph stays high-contrast, silhouette unchanged) and add a light
// HAIRLINE OUTLINE that delineates the disc on pure black; on the white page the dark body carries it.
// Reads on both extremes in day/night without the light-frost-on-white inversion Fable warned about.
private val STAGE_FAB_CONTAINER = Color(0xCC1F1F1F)
private val STAGE_FAB_OUTLINE = Color(0xB3FFFFFF)
private val STAGE_FAB_OUTLINE_WIDTH = 1.5.dp

/** A2/N5 — a translucent Stage control (reference-app look): a white glyph on a dark disc with a light
 *  hairline outline so it reads on both the black canvas and the white page. */
@Composable
private fun StageFab(glyph: String, size: Dp = 56.dp, container: Color = STAGE_FAB_CONTAINER, onClick: () -> Unit) {
    val shape = FloatingActionButtonDefaults.shape
    FloatingActionButton(
        onClick = onClick,
        containerColor = container,
        contentColor = Color.White,
        shape = shape,
        modifier = Modifier.size(size).border(STAGE_FAB_OUTLINE_WIDTH, STAGE_FAB_OUTLINE, shape),
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

/** N7 — the end-of-bounds glyph: a big semitransparent center disc with a direction-aware "arrow into a
 *  wall" (forward = blocked at the last page, backward = blocked at the first), ~2× a nav FAB so it
 *  reads as a distinct edge signal over the score. Flashed briefly then faded (shared N1 layer).
 *  N5: same dark-disc + light hairline outline as the FABs so it reads on BOTH the white page (day) and
 *  the black night page — a bare dark disc vanished on the night-black canvas. */
@Composable
private fun BlockedTurnGlyph(forward: Boolean) {
    Surface(
        color = STAGE_FAB_CONTAINER,
        shape = CircleShape,
        modifier = Modifier.size(120.dp).border(STAGE_FAB_OUTLINE_WIDTH, STAGE_FAB_OUTLINE, CircleShape),
    ) {
        Box(contentAlignment = Alignment.Center) {
            Text(
                if (forward) "›|" else "|‹",
                color = Color.White.copy(alpha = 0.9f),
                style = MaterialTheme.typography.displaySmall,
            )
        }
    }
}

/** A20 — the song-entry cue flash: the member's cues LARGE + tinted on a translucent dark card, so
 *  "what to prepare" reads over any score. Untinted cues render white (readable in day + night, like
 *  the chrome). Rides the N1 overlay (one visibility, one timeout). */
@Composable
private fun CueFlashCard(cues: List<SongCue>, modifier: Modifier = Modifier) {
    // Each cue is its OWN square tile (not one shared rectangle), so it reads at a glance as N
    // distinct things to prepare — "mic AND red guitar" = two tiles, not one blob. A "+" between
    // tiles reinforces that BOTH are needed (not a choice).
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        cues.forEachIndexed { i, cue ->
            if (i > 0) {
                // "+" on its own small dark chip so it reads on BOTH the white page and the night-black
                // canvas (a bare white "+" in the gap vanished on the white score — the N5 lesson).
                Surface(color = Color(0xCC000000), shape = CircleShape) {
                    Box(Modifier.size(36.dp), contentAlignment = Alignment.Center) {
                        Text("+", color = Color.White, style = MaterialTheme.typography.titleLarge)
                    }
                }
            }
            Surface(color = Color(0xCC000000), shape = MaterialTheme.shapes.large) {
                Box(Modifier.size(96.dp), contentAlignment = Alignment.Center) {
                    CueGlyphIcon(cue.icon, parseCueColor(cue.color, Color.White), size = 56.dp)
                }
            }
        }
    }
}

/** "Song 2/4  ·  3–4/12" — the title card's position line (A2). Song part omitted when there are none. */
private fun stagePositionLabel(state: StageState, topPage: Int, twoUp: Boolean): String {
    val pages = pagerLabel(topPage, state.pageCount, twoUp, state.songs.map { it.firstPage })
    val i = state.currentSong
    return if (i >= 0 && state.songs.isNotEmpty()) "Song ${i + 1}/${state.songs.size}  ·  $pages" else pages
}

/**
 * Scheme-A settings-sheet sweep: the app equivalent of the studio's Band/Mine AudienceTag. On the
 * Stage everything is view-local, but two controls read like they might change what BANDMATES see —
 * auto-update (pulls new bakes) and the per-song layer toggles (looks like hiding a layer for all).
 * This chip makes it explicit they are personal: they only change YOUR view. Pure, themeable.
 */
@Composable
private fun PersonalTag(modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        shape = MaterialTheme.shapes.small,
        color = MaterialTheme.colorScheme.secondaryContainer,
        contentColor = MaterialTheme.colorScheme.onSecondaryContainer,
    ) {
        Text(
            "👤 Just for you",
            style = MaterialTheme.typography.labelSmall,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp),
        )
    }
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
    canAutoUpdate: Boolean,
    onToggleAutoUpdate: () -> Unit,
    onFitMode: (FitMode) -> Unit,
    onLayers: () -> Unit,
    onRole: () -> Unit,
    onToggleColor: () -> Unit,
    onSwitchIdentity: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState()) {
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 24.dp).padding(bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // P205 Stage 3a: "Performing as <you> · Switch" — only on a band-wide bundle (roster present).
            // The switch is a free, unverified re-pick (VLL: no auth, a strong default when connected).
            if (state.roster.isNotEmpty()) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                    val who = state.roster.firstOrNull { it.memberId == state.identity }?.displayName ?: "anonymous"
                    Text("Performing as $who", style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f))
                    TextButton(onClick = onSwitchIdentity) { Text("Switch") }
                }
            }
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
            // Scheme-A sweep: auto-update lives in the sheet (not just the ● chrome FAB) and is
            // tagged personal — pulling new bakes only changes YOUR view, never a bandmate's.
            if (canAutoUpdate) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Text("Auto-update", style = MaterialTheme.typography.titleSmall)
                            PersonalTag()
                        }
                        Text("Apply new bakes as they arrive", style = MaterialTheme.typography.bodySmall)
                    }
                    Switch(checked = state.autoUpdate, onCheckedChange = { onToggleAutoUpdate() })
                }
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
    val neutral = MaterialTheme.colorScheme.onSurfaceVariant
    NavigationDrawerItem(
        selected = i == state.currentSong,
        label = {
            Column {
                Text(s.name, style = MaterialTheme.typography.titleMedium)
                if (meta != null) Text(meta, style = MaterialTheme.typography.labelMedium)
            }
        },
        // A20: the member's tinted cue icons for this song, right-aligned — the glanceable "what's
        // coming" list. Absent when the member has no cues on the song.
        badge = if (s.cues.isEmpty()) null else {
            {
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                    s.cues.forEach { cue -> CueGlyphIcon(cue.icon, parseCueColor(cue.color, neutral), size = 22.dp) }
                }
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
                MetaStrip(page) // renders only on the song's first page
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
    // R10/#23: key on the overlay MODELS (LayerImage carries contentHash), not the ref strings, so a
    // rehearsal auto-update that swaps content under the same blob filename re-decodes instead of
    // re-showing the cached stale bitmap.
    val overlays = page.overlays.filter { it.layerId in visibleLayers }
    // B1: each displayed page pins its own keys under a stable owner so several ScrollPages on screen
    // at once don't clobber each other's pins; release on leave so off-screen pages can be evicted.
    val owner = remember { Any() }
    DisposableEffect(owner) { onDispose { cache.unpin(owner) } }
    // B1: retry a failed overlay decode on the next few frames (a failure isn't cached, so a re-decode
    // re-attempts); if it keeps failing we badge it — never silently show fewer annotations.
    var retryTick by remember(page.rasterRef, page.rasterHash, overlays, widthPx) { mutableStateOf(0) }
    // Decode at column width with a generous height cap; the decoder preserves aspect ⇒ width-bound.
    val decoded by produceState<PageBitmaps?>(null, page.rasterRef, page.rasterHash, overlays, widthPx, retryTick) {
        value = null
        if (widthPx <= 0) return@produceState
        value = withContext(Dispatchers.Default) {
            val raster = decodeCached(cache, page.rasterRef, page.rasterHash, widthPx, widthPx * 3, decoder)
            val ov = decodeOverlays(overlays) { decodeCached(cache, it.imageRef, it.contentHash, widthPx, widthPx * 3, decoder) }
            cache.pin(owner, pageCacheKeys(page.rasterRef, page.rasterHash, overlays, widthPx, widthPx * 3)) // eviction-proof this page
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
    placeholder: Color,
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
        // R10/#23: key on the overlay MODELS (contentHash), not ref strings — see ScrollPage.
        val overlays = page.overlays.filter { it.layerId in visibleLayers }

        // B1: each displayed page pins under a stable owner (two-up composes two PageViews at once, so
        // the right page must not unpin the left); release on leave so off-screen pages can be evicted.
        val owner = remember { Any() }
        DisposableEffect(owner) { onDispose { cache.unpin(owner) } }
        // B1: retry a failed overlay decode a few frames, then badge — never silently fewer annotations.
        var retryTick by remember(page.rasterRef, page.rasterHash, overlays, wPx, hPx) { mutableStateOf(0) }
        val decoded by produceState<PageBitmaps?>(null, page.rasterRef, page.rasterHash, overlays, wPx, hPx, retryTick) {
            value = null
            if (wPx <= 0 || hPx <= 0) return@produceState
            value = withContext(Dispatchers.Default) {
                val raster = decodeCached(cache, page.rasterRef, page.rasterHash, wPx, hPx, decoder)
                val ov = decodeOverlays(overlays) { decodeCached(cache, it.imageRef, it.contentHash, wPx, hPx, decoder) }
                cache.pin(owner, pageCacheKeys(page.rasterRef, page.rasterHash, overlays, wPx, hPx)) // eviction-proof this page
                PageBitmaps(raster, ov.overlays, ov.missing)
            }
        }
        LaunchedEffect(decoded?.missingOverlays, retryTick) {
            if ((decoded?.missingOverlays ?: 0) > 0 && retryTick < OVERLAY_DECODE_RETRIES) { delay(250); retryTick++ }
        }

        val bitmaps = decoded
        when {
            // N9: page-tinted placeholder while decoding (colorMode-aware) so a turn never slides in a
            // BLACK void; with N9 prefetch the incoming page is usually already decoded, so this only
            // shows for the rare uncached page.
            bitmaps == null -> Box(Modifier.fillMaxSize().background(placeholder))
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
private fun MetaStrip(page: StagePage) {
    if (page.pageInSong != 0) return
    // notes · key only; the tempo/metronome control moved to the top-bar FAB (A34), no chip here.
    val prefix = metaStripText(page.displayNotes, page.key, 0) ?: return
    Surface(Modifier.fillMaxWidth(), color = MaterialTheme.colorScheme.surface.copy(alpha = 0.75f)) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                prefix,
                Modifier.weight(1f),
                style = MaterialTheme.typography.labelMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** The metronome tint: amber/aqua on the lit beat, the accent while running-but-dark, else the muted
 *  outline. Shared by the chip and the top-bar FAB so they read identically. */
@Composable
private fun beatTint(beat: StageBeat): Color {
    val f = beat.frame
    return when {
        f != null && f.downbeat -> Color(0xFFFFB02E)
        f != null -> Color(0xFF3EE0D4)
        beat.running -> MaterialTheme.colorScheme.primary
        else -> MaterialTheme.colorScheme.outline
    }
}

/** The metronome + its ∞ loop toggle, joined into ONE segmented capsule (a shared dark disc with a
 *  hairline outline and a divider) in the TOP BAR, so they read as one strongly-connected control:
 *   • left segment = the metronome — tap to start/stop; [onStart] fires on start (caller closes the
 *     chrome). Lifts to the accent container while running so "on" shows even between pulses.
 *   • right segment = ∞ — keep-running vs an 8-beat count-in; accent container while on.
 *  Styled to match [StageFab]. */
@Composable
private fun StageBeatControl(tempo: Int, beat: StageBeat, onStart: () -> Unit, height: Dp = 56.dp) {
    val enabled = tempoIntervalMs(tempo) != null
    val shape = FloatingActionButtonDefaults.shape
    val accent = Color(0xE6198060)
    Row(
        Modifier
            .height(height)
            .clip(shape)
            .background(STAGE_FAB_CONTAINER)
            .border(STAGE_FAB_OUTLINE_WIDTH, STAGE_FAB_OUTLINE, shape),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .fillMaxHeight()
                .width(height)
                .background(if (beat.running) accent else Color.Transparent)
                .clickable {
                    if (enabled) {
                        beat.toggle(tempo)
                        if (beat.running) onStart()
                    }
                },
            contentAlignment = Alignment.Center,
        ) { MetronomeIcon(beatTint(beat), Modifier.size(height * 0.42f)) }
        Box(Modifier.width(STAGE_FAB_OUTLINE_WIDTH).fillMaxHeight().background(STAGE_FAB_OUTLINE.copy(alpha = 0.55f)))
        Box(
            Modifier
                .fillMaxHeight()
                .width(height)
                .background(if (beat.continuous) accent else Color.Transparent)
                .clickable { beat.continuous = !beat.continuous },
            contentAlignment = Alignment.Center,
        ) {
            Text(
                "∞",
                color = if (beat.continuous) Color.White else MaterialTheme.colorScheme.outline,
                style = MaterialTheme.typography.headlineSmall,
            )
        }
    }
}

private data class PageBitmaps(val raster: ImageBitmap?, val overlays: List<ImageBitmap>, val missingOverlays: Int = 0)

/** Cache-or-decode a single blob; null on any failure (the caller degrades gracefully). */
private fun decodeCached(cache: PageImageCache, ref: String, ver: String, w: Int, h: Int, decoder: ImageDecoder): ImageBitmap? {
    val key = cacheKey(ref, ver, w, h)
    cache.get(key)?.let { return it }
    val bmp = decoder.decode(ref, w, h).getOrNull() ?: return null // decode the REAL ref; version only keys the cache
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
    // N10: list ONLY the current song's layers (not the concert-wide aggregate) — the dialog is
    // per-song (title + toggle, A1), so the list must be too. Friendlier interim labels until real
    // names ride the bundle (T53).
    val layers = songLayerLabels(state, songId)
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = { TextButton(onClick = onDismiss) { Text("Done") } },
        title = { Text(if (songName.isEmpty()) "Layers — this song" else "Layers — $songName") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Overrides just this song; changing your role resets it.", style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
                    PersonalTag() // Scheme-A: toggling a layer only changes YOUR view, not a bandmate's
                }
                layers.forEach { (layer, label) ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(
                            checked = layer.layerId in visible,
                            enabled = !layer.mandatory, // mandatory layers are locked on (I12)
                            onCheckedChange = { vm.setLayerVisible(layer.layerId, it) },
                        )
                        Text(label + if (layer.mandatory) " (required)" else "")
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

// N3 page-turn swipe. [onPrev]/[onNext] are passed as State and read at drag-END so the turn always
// targets the CURRENT page: [key] intentionally excludes state.current (restarting the detector on
// every turn is wasteful and could drop an in-flight drag), so a captured lambda would go stale and
// keep turning from page 0 — reading .value at fire time is the fix.
private fun Modifier.pointerInputSwipe(key: Any, onPrev: State<() -> Unit>, onNext: State<() -> Unit>): Modifier =
    pointerInput(key) {
        var dx = 0f
        detectHorizontalDragGestures(
            onDragStart = { dx = 0f },
            onDragEnd = {
                val threshold = size.width * 0.15f
                if (dx > threshold) onPrev.value() else if (dx < -threshold) onNext.value()
            },
        ) { _, amount -> dx += amount }
    }
