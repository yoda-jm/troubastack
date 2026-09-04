// The presenter's view-model: holds StageState in a StateFlow and exposes ONLY reading-behavior
// actions (navigate, fit mode, layer visibility, local role). No writes, no network (I12). Every
// transition is total and clamped — out-of-range navigation is coerced, never thrown.
package com.troubastack.shared.stage

import com.troubastack.shared.bundle.LoadResult
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

class StageViewModel(
    loadResult: LoadResult,
    role: String = "",
    identity: String = "",
    initialFit: FitMode = FitMode.FIT_PAGE,
    // A46 (A33 drill 2): the persisted reading POSITION for this concert (logical songId + page-in-song),
    // so reopening lands where the performer left off, not page 0. "" ⇒ start at the top (old behaviour).
    initialSongId: String = "",
    initialPageInSong: Int = 0,
) {

    // P205 Stage 3a: the loaded bundle is retained so setIdentity can re-derive cues + the default
    // seed for a newly-picked identity (applyUpdate swaps it for a fresh rev).
    private var result: LoadResult = loadResult

    // A14: the reading mode is a persisted global preference; the entrypoint seeds it here (A10 pattern).
    // A46: also seed the persisted reading position (resolveStartPage survives a re-bake reorder).
    private val _state = MutableStateFlow(
        stageStateFrom(loadResult, role, identity).let { s ->
            s.copy(fitMode = initialFit, current = resolveStartPage(s, initialSongId, initialPageInSong))
        },
    )
    val state: StateFlow<StageState> = _state.asStateFlow()

    // NOT the app's page navigation — production turns route through StageScreen's turnNext/turnPrev
    // (mode-aware: pages, width-spreads, or scroll cross/step). These are page-±1 helpers used ONLY by
    // StageViewModelTest to exercise goToPage's clamping; do not wire UI to them (A60 P5 note).
    fun next() = goToPage(_state.value.current + 1)
    fun previous() = goToPage(_state.value.current - 1)

    /** Clamp to a valid page; a no-op on an empty bundle. */
    fun goToPage(index: Int) = _state.update { s ->
        if (s.pages.isEmpty()) s else s.copy(current = index.coerceIn(0, s.pages.lastIndex))
    }

    /** Jump to the first page of the given song; ignores an out-of-range song index. */
    fun goToSong(songIndex: Int) = _state.update { s ->
        val song = s.songs.getOrNull(songIndex) ?: return@update s
        s.copy(current = song.firstPage.coerceIn(0, s.pages.lastIndex.coerceAtLeast(0)))
    }

    /** Cycle the reading mode: page → width → scroll → page (A14). */
    fun toggleFit() = _state.update { s -> s.copy(fitMode = nextFitMode(s.fitMode)) }

    /** Set the reading mode directly (A2 segmented control: Page | Width | Scroll). */
    fun setFitMode(mode: FitMode) = _state.update { s -> s.copy(fitMode = mode) }

    /**
     * Show/hide a layer FOR THE CURRENT SONG ONLY (A1 per-song visibility); the choice is remembered
     * for that song this session. A mandatory layer cannot be hidden (I12) — the request is ignored.
     */
    fun setLayerVisible(layerId: String, visible: Boolean) = _state.update { s ->
        val layer = s.layers.find { it.layerId == layerId } ?: return@update s
        if (layer.mandatory) return@update s
        val songId = s.currentPage?.songId ?: return@update s
        val cur = s.visibleFor(songId)
        val updated = if (visible) cur + layerId else cur - layerId
        s.copy(visibleBySong = s.visibleBySong + (songId to updated))
    }

    /** Set the local reading role; RE-SEEDS every song's visibility from the default rule, CLEARING any
     *  per-song manual overrides (A1). Mandatory layers stay on. */
    fun setRole(role: String) = _state.update { s ->
        val defaults = defaultVisibleLayers(s.layers, role, s.identity)
        s.copy(role = role, visibleBySong = s.songs.associate { it.songId to defaults })
    }

    /** P205 Stage 3a: set the viewer's IDENTITY (a member id, "" = anonymous). Re-derives this member's
     *  cues (member_cues) and RE-SEEDS every song's visibility from the (role, identity) default rule,
     *  CLEARING per-song manual overrides — the A18 role-change semantics. A local view preference (I12):
     *  no account, no writes here; the host persists the choice per concert/device. Page/fit/auto-update
     *  are preserved. */
    fun setIdentity(identity: String) = _state.update { s ->
        val fresh = stageStateFrom(result, s.role, identity)
        if (fresh.pages.isEmpty()) return@update s.copy(identity = identity)
        // T137: page sequences are per-identity (member_pages), so the OLD identity's flat index — and its
        // page-in-song — do not map onto the new identity's sequence (a flat index would land on an
        // unrelated page mid-set). INVALIDATE the position: keep the SONG you were on but land at its first
        // page in the new sequence. songId is identity-independent, so re-resolving via resolveStartPage at
        // page-in-song 0 is exact when the song is present, and 0 otherwise.
        val songId = s.currentPage?.songId ?: ""
        fresh.copy(
            current = resolveStartPage(fresh, songId, 0),
            fitMode = s.fitMode,
            autoUpdate = s.autoUpdate,
        )
    }

    /** P201/I13: the transient rehearsal auto-update toggle. In-memory only — a new
     *  StageViewModel (a fresh Stage entry) always starts it false, so leaving Stage
     *  resets it; nothing is written through the Storage seam. */
    fun setAutoUpdate(on: Boolean) = _state.update { s -> s.copy(autoUpdate = on) }

    /** T143: the view calls this once it has shown the auto-update notice, so it self-dismisses and does
     *  not re-appear on the next recomposition. */
    fun clearUpdateNotice() = _state.update { s -> if (s.updateNotice == null) s else s.copy(updateNotice = null) }

    /**
     * P201/R10: swap in a freshly re-baked concert (the host fetched + imported a new rev
     * while auto-update was on) WITHOUT moving the page the performer is on. Rebuilds the
     * state from [newResult] then remaps position: the current page's content hash finds
     * its counterpart in the new bundle (unchanged page → exact same spot); failing that,
     * the same (songId, pageInSong); failing that, the nearest page index. Fit mode,
     * PER-SONG layer visibility (by songId+layerId), role, and the auto-update flag are preserved.
     * Facing pages (A12) and scroll mode (A14) follow automatically: they derive the
     * spread / scroll position from `current`, which this maps correctly.
     */
    fun applyUpdate(newResult: LoadResult) = _state.update { old ->
        result = newResult // P205: keep the retained bundle current for a later setIdentity
        // T143: the sheet just changed under the performer — say a word (self-dismissing, non-focus-
        // stealing; the view shows it and calls clearUpdateNotice). Names the rev that arrived.
        val notice = (newResult as? LoadResult.Loaded)?.let { "Updated to rev ${it.bundle.concertRev}" }
        val fresh = stageStateFrom(newResult, old.role, old.identity)
            .copy(fitMode = old.fitMode, autoUpdate = old.autoUpdate, updateNotice = notice)
        if (fresh.pages.isEmpty()) return@update fresh
        val target = remapCurrent(old, fresh)
        // A1: merge PER SONG. For a song that existed before, keep its overrides for layers that still
        // exist, plus the default for any genuinely-new layer; a brand-new song takes fresh's default
        // seed. An auto-update mid-rehearsal must never clobber a per-song layer choice.
        val newLayerIds = fresh.layers.map { it.layerId }.filter { id -> old.layers.none { it.layerId == id } }.toSet()
        val merged = fresh.songs.associate { song ->
            val oldSet = old.visibleBySong[song.songId]
            if (oldSet == null) {
                song.songId to fresh.visibleFor(song.songId) // new song → its fresh default seed
            } else {
                val kept = oldSet.filter { id -> fresh.layers.any { it.layerId == id } }.toSet()
                val newDefaults = fresh.visibleFor(song.songId).filter { it in newLayerIds }
                song.songId to (kept + newDefaults)
            }
        }
        fresh.copy(current = target, visibleBySong = merged)
    }
}

/** R10 position remap: find where the OLD current page lands in the NEW state. */
internal fun remapCurrent(old: StageState, fresh: StageState): Int {
    val cur = old.pages.getOrNull(old.current) ?: return 0
    // 1) exact content match — an UNCHANGED page keeps the reader exactly in place.
    if (cur.rasterHash.isNotEmpty()) {
        val byHash = fresh.pages.indexOfFirst { it.rasterHash == cur.rasterHash }
        if (byHash >= 0) return byHash
    }
    // 2) same logical page (song + page-in-song) — content changed but position is stable.
    val byId = fresh.pages.indexOfFirst { it.songId == cur.songId && it.pageInSong == cur.pageInSong }
    if (byId >= 0) return byId
    // 3) same song, nearest page (the song grew/shrank) — stay in the song near where we were.
    val songPages = fresh.pages.withIndex().filter { it.value.songId == cur.songId }
    if (songPages.isNotEmpty()) {
        return songPages.minByOrNull { kotlin.math.abs(it.value.pageInSong - cur.pageInSong) }!!.index
    }
    // 4) structural change (the song vanished) — clamp the old index into the new range.
    return old.current.coerceIn(0, fresh.pages.size - 1)
}
