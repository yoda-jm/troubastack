// The presenter's view-model: holds StageState in a StateFlow and exposes ONLY reading-behavior
// actions (navigate, fit mode, layer visibility, local role). No writes, no network (I12). Every
// transition is total and clamped — out-of-range navigation is coerced, never thrown.
package com.troubashare.shared.stage

import com.troubashare.shared.bundle.LoadResult
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

class StageViewModel(loadResult: LoadResult, role: String = "", initialFit: FitMode = FitMode.FIT_PAGE) {

    // A14: the reading mode is a persisted global preference; the entrypoint seeds it here (A10 pattern).
    private val _state = MutableStateFlow(stageStateFrom(loadResult, role).copy(fitMode = initialFit))
    val state: StateFlow<StageState> = _state.asStateFlow()

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

    /** Show/hide a layer. A mandatory layer cannot be hidden (I12) — the request is ignored. */
    fun setLayerVisible(layerId: String, visible: Boolean) = _state.update { s ->
        val layer = s.layers.find { it.layerId == layerId } ?: return@update s
        if (layer.mandatory) return@update s
        s.copy(visibleLayers = if (visible) s.visibleLayers + layerId else s.visibleLayers - layerId)
    }

    /** Set the local reading role; re-seeds layer visibility from the default rule (mandatory stays on). */
    fun setRole(role: String) = _state.update { s ->
        s.copy(role = role, visibleLayers = s.layers.filter { defaultVisible(it, role) }.map { it.layerId }.toSet())
    }

    /** P201/I13: the transient rehearsal auto-update toggle. In-memory only — a new
     *  StageViewModel (a fresh Stage entry) always starts it false, so leaving Stage
     *  resets it; nothing is written through the Storage seam. */
    fun setAutoUpdate(on: Boolean) = _state.update { s -> s.copy(autoUpdate = on) }

    /**
     * P201/R10: swap in a freshly re-baked concert (the host fetched + imported a new rev
     * while auto-update was on) WITHOUT moving the page the performer is on. Rebuilds the
     * state from [newResult] then remaps position: the current page's content hash finds
     * its counterpart in the new bundle (unchanged page → exact same spot); failing that,
     * the same (songId, pageInSong); failing that, the nearest page index. Fit mode,
     * per-layer visibility (by layerId), role, and the auto-update flag are preserved.
     * Facing pages (A12) and scroll mode (A14) follow automatically: they derive the
     * spread / scroll position from `current`, which this maps correctly.
     */
    fun applyUpdate(newResult: LoadResult) = _state.update { old ->
        val fresh = stageStateFrom(newResult, old.role).copy(fitMode = old.fitMode, autoUpdate = old.autoUpdate)
        if (fresh.pages.isEmpty()) return@update fresh
        val target = remapCurrent(old, fresh)
        // Keep the viewer's layer choices for layers that still exist; a brand-new layer
        // takes its default visibility (defaultVisible, already in fresh.visibleLayers).
        val keptOld = fresh.layers.map { it.layerId }.filter { it in old.visibleLayers }.toSet()
        val newLayerDefaults = fresh.visibleLayers.filter { id -> old.layers.none { it.layerId == id } }
        fresh.copy(current = target, visibleLayers = keptOld + newLayerDefaults)
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
