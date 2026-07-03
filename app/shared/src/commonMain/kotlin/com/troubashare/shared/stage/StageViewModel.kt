// The presenter's view-model: holds StageState in a StateFlow and exposes ONLY reading-behavior
// actions (navigate, fit mode, layer visibility, local role). No writes, no network (I12). Every
// transition is total and clamped — out-of-range navigation is coerced, never thrown.
package com.troubashare.shared.stage

import com.troubashare.shared.bundle.LoadResult
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

class StageViewModel(loadResult: LoadResult, role: String = "") {

    private val _state = MutableStateFlow(stageStateFrom(loadResult, role))
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

    fun toggleFit() = _state.update { s ->
        s.copy(fitMode = if (s.fitMode == FitMode.FIT_PAGE) FitMode.FIT_WIDTH else FitMode.FIT_PAGE)
    }

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
}
