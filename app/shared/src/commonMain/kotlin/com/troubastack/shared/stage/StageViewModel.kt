// The presenter's view-model: holds StageState in a StateFlow and exposes ONLY reading-behavior
// actions (navigate, fit mode, layer visibility, local role). No writes, no network (I12). Every
// transition is total and clamped — out-of-range navigation is coerced, never thrown.
package com.troubastack.shared.stage

import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteIndex
import com.troubastack.shared.stage.notes.NoteTool
import com.troubastack.shared.stage.notes.NoteTools
import com.troubastack.shared.stage.notes.NotesWarning
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
    // T147: a MONOTONIC millisecond source (Android: SystemClock.elapsedRealtime — advances through deep
    // sleep) for the chronometer. Injectable so the state machine is tested without sleeping; the default
    // stands still (fine for the many tests that never touch the chrono).
    private val monotonicNow: () -> Long = { 0L },
    // T147: seed a persisted chrono + clock-visibility so the chrono survives PROCESS DEATH (the host reads
    // them from Storage on open). Defaults reproduce the old behaviour (a fresh, hidden, stopped chrono).
    initialChrono: Chrono = Chrono(),
    initialClockVisible: Boolean = false,
    initialClockStyle: ClockStyle = ClockStyle.ANALOG,
    // A70: the rehearsal-notes index the host loaded for this concert (persisted off-device). Seeds the
    // live/orphan split; defaults empty (no notes, the common case). Tool/width/colour also seeded from
    // the host's per-device prefs (§3.6).
    initialNotes: List<NoteEntry> = emptyList(),
    initialNoteTool: NoteTool = NoteTool.PENCIL,
    initialNoteWidth: Int = NoteTools.FINE,
    initialNoteColour: Long = NoteTools.BLACK,
) {

    // P205 Stage 3a: the loaded bundle is retained so setIdentity can re-derive cues + the default
    // seed for a newly-picked identity (applyUpdate swaps it for a fresh rev).
    private var result: LoadResult = loadResult

    // A14: the reading mode is a persisted global preference; the entrypoint seeds it here (A10 pattern).
    // A46: also seed the persisted reading position (resolveStartPage survives a re-bake reorder).
    private val _state = MutableStateFlow(
        stageStateFrom(loadResult, role, identity).let { s ->
            s.copy(
                fitMode = initialFit,
                current = resolveStartPage(s, initialSongId, initialPageInSong),
                chrono = initialChrono,
                clockVisible = initialClockVisible,
                clockStyle = initialClockStyle,
                notes = initialNotes,
                noteTool = initialNoteTool,
                noteWidth = initialNoteWidth,
                noteColour = initialNoteColour,
            )
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
            // T147: identity is a view preference — the session chrono + clock keep running across it.
            chrono = s.chrono,
            clockVisible = s.clockVisible,
            clockStyle = s.clockStyle,
        )
    }

    /** P201/I13: the transient rehearsal auto-update toggle. In-memory only — a new
     *  StageViewModel (a fresh Stage entry) always starts it false, so leaving Stage
     *  resets it; nothing is written through the Storage seam. */
    fun setAutoUpdate(on: Boolean) = _state.update { s -> s.copy(autoUpdate = on) }

    /** T143: the view calls this once it has shown the auto-update notice, so it self-dismisses and does
     *  not re-appear on the next recomposition. */
    fun clearUpdateNotice() = _state.update { s -> if (s.updateNotice == null) s else s.copy(updateNotice = null) }

    // --- T147 chronometer + clock ---

    /** start / resume the chronometer from the current monotonic instant. A no-op while already running. */
    fun startChrono() = _state.update { s -> s.copy(chrono = s.chrono.started(monotonicNow())) }

    /** pause the chronometer, banking the live segment. A no-op while already paused. */
    fun pauseChrono() = _state.update { s -> s.copy(chrono = s.chrono.paused(monotonicNow())) }

    /** one-button convenience: pause if running, else start/resume. */
    fun toggleChrono() = _state.update { s ->
        s.copy(chrono = if (s.chrono.running) s.chrono.paused(monotonicNow()) else s.chrono.started(monotonicNow()))
    }

    /** reset the chronometer to 00:00, paused. */
    fun resetChrono() = _state.update { s -> s.copy(chrono = s.chrono.reset()) }

    /** the chrono's elapsed ms AT THE CURRENT INSTANT — the view reads this each display tick. Derived
     *  from the monotonic clock, so a missed tick never loses time. */
    fun chronoElapsedMs(): Long = _state.value.chrono.elapsedMs(monotonicNow())

    /** show/hide the bottom-right time-of-day clock overlay. Never touches page/geometry. */
    fun setClockVisible(on: Boolean) = _state.update { s -> if (s.clockVisible == on) s else s.copy(clockVisible = on) }

    /** choose the clock face (analog default / digital). A view preference; never touches page/geometry. */
    fun setClockStyle(style: ClockStyle) = _state.update { s -> if (s.clockStyle == style) s else s.copy(clockStyle = style) }

    /** N10: lock/unlock the crossing swipe (‹ ›, pedal and keys still navigate). A view preference. */
    fun toggleSwipeLock() = _state.update { s -> s.copy(swipeLocked = !s.swipeLocked) }

    /** P206 §4.1: toggle direct-jump (skip the go-to popup). A view preference; never touches the page. */
    fun toggleJumpDirect() = _state.update { s -> s.copy(jumpDirect = !s.jumpDirect) }

    // --- A70 rehearsal notes (§3.7) — note MODE is session state; the bitmaps live in the host port ---

    /**
     * §3.7 #14 — REQUEST note mode: show the confirmation dialog. Refused in scroll mode (out of scope,
     * §3.7) with a self-dismissing reason and no state change. Otherwise sets ONLY `noteModePending` — it
     * does NOT enter note mode and NEVER moves the page; `confirmNoteMode` does the entering. Returns
     * whether the dialog was raised.
     */
    fun requestNoteMode(): Boolean {
        if (_state.value.fitMode == FitMode.SCROLL) {
            _state.update { it.copy(noteModeRefusal = "Notes: switch to page mode") }
            return false
        }
        _state.update { it.copy(noteModePending = true) }
        return true
    }

    /** §3.7 — confirm the dialog: ENTER note mode, forcing the note layer visible for the current song
     *  (§3.8 — un-hide it). Never changes the page. */
    fun confirmNoteMode() = _state.update { s ->
        val songId = s.currentPage?.songId
        s.copy(
            noteMode = true, noteModePending = false,
            notesOffBySong = if (songId == null) s.notesOffBySong else s.notesOffBySong - songId,
        )
    }

    /** §3.7 — dismiss the confirmation without entering. Leaves everything as it was. */
    fun cancelNoteMode() = _state.update { s -> if (!s.noteModePending) s else s.copy(noteModePending = false) }

    /** §3.7 — leave note mode (Exit button / leaving Stage / applyUpdate). Never moves the page. */
    fun exitNoteMode() = _state.update { s -> if (!s.noteMode && !s.noteModePending) s else s.copy(noteMode = false, noteModePending = false) }

    /** The view calls this after showing the scroll-mode refusal, so it self-dismisses (like updateNotice). */
    fun clearNoteModeRefusal() = _state.update { s -> if (s.noteModeRefusal == null) s else s.copy(noteModeRefusal = null) }

    /** §3.6 — the current tool / width / colour, remembered per device by the host. */
    fun setNoteTool(tool: NoteTool) = _state.update { s -> if (s.noteTool == tool) s else s.copy(noteTool = tool) }
    fun setNoteWidth(width: Int) = _state.update { s -> if (s.noteWidth == width) s else s.copy(noteWidth = width) }
    fun setNoteColour(colour: Long) = _state.update { s -> if (s.noteColour == colour) s else s.copy(noteColour = colour) }

    /** §3.8 — show/hide the note layer FOR THE CURRENT SONG (default on when a live note exists). */
    fun setNoteLayerVisible(visible: Boolean) = _state.update { s ->
        val songId = s.currentPage?.songId ?: return@update s
        s.copy(notesOffBySong = if (visible) s.notesOffBySong - songId else s.notesOffBySong + songId)
    }

    /** The host replaces the note index after it saved/deleted a bitmap (§4.1). Never touches the page. */
    fun setNotes(notes: List<NoteEntry>) = _state.update { s -> s.copy(notes = notes) }

    /** Bumped after a commit so the view re-reads the stored bitmap (the wet stroke merged into it). */
    fun bumpNoteRevision() = _state.update { s -> s.copy(noteRevision = s.noteRevision + 1) }

    /** §3.4 — the `Notes ⚠` tab-label state across all stored notes. */
    fun notesTabWarning(): NotesWarning = _state.value.notesWarning

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
        // A70 §3.4: leave note mode (the flush already happened at the host before applyUpdate), age every
        // unsent note by one bake, and carry the notes across the swap (they are keyed by hash, not rev, so
        // an unchanged page keeps its note and a changed page orphans it — reflected by the derived counts).
        val bumped = NoteIndex.bumpBakes(old.notes)
        val fresh0 = stageStateFrom(newResult, old.role, old.identity)
            // T147: a bundle swap must NOT reset a running chrono or hide the clock — the sheet changed,
            // the session did not (T143 keeps the page; this keeps the timer).
            .copy(
                fitMode = old.fitMode,
                autoUpdate = old.autoUpdate,
                chrono = old.chrono,
                clockVisible = old.clockVisible,
                clockStyle = old.clockStyle,
                notes = bumped,
                noteMode = false,
                noteModePending = false,
                notesOffBySong = old.notesOffBySong,
                noteTool = old.noteTool,
                noteWidth = old.noteWidth,
                noteColour = old.noteColour,
            )
        // T143: the sheet just changed under the performer — say a word (self-dismissing, non-focus-
        // stealing; the view shows it and calls clearUpdateNotice). Names the rev, and §3.4 point 5: append
        // how many note pages were left behind by this bake (orphaned) when any were.
        val base = (newResult as? LoadResult.Loaded)?.let { "Updated to rev ${it.bundle.concertRev}" }
        val orphans = fresh0.orphanedNoteCount
        val notice = base?.let { if (orphans > 0) "$it · $orphans pages of notes are from the previous bake" else it }
        val fresh = fresh0.copy(updateNotice = notice)
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
