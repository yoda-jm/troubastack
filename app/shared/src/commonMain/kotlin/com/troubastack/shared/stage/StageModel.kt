// TroubaStage presenter — SHARED Kotlin (commonMain). It performs pre-baked bundles (A02 model)
// and does NOTHING else (I12): a pure image compositor + pager. No annotation model, no
// access-control, NO network, NO writes. Every input/environment problem is a STATE the UI renders
// (never an exception) — a crash or dead-end mid-performance is this product's worst failure.
package com.troubastack.shared.stage

import com.troubastack.shared.bundle.BakedSong
import com.troubastack.shared.bundle.BundleIssue
import com.troubastack.shared.bundle.BundleMember
import com.troubastack.shared.bundle.ConcertBundle
import com.troubastack.shared.bundle.LayerImage
import com.troubastack.shared.bundle.LoadResult
import com.troubastack.shared.bundle.SongCue

/**
 * The reading mode, cycled on the single Stage toggle (A14). FIT_PAGE shows the whole page (and is
 * the only mode that goes two-up in landscape, A12); FIT_WIDTH fills width and scrolls one page
 * vertically; SCROLL is a continuous vertical column of every page at fit-width.
 */
enum class FitMode {
    FIT_PAGE, FIT_WIDTH, SCROLL;

    companion object {
        /** Parse a persisted mode (A14 persistence, A10 pattern); null/unknown → FIT_PAGE (the default). */
        fun parse(raw: String?): FitMode = entries.firstOrNull { it.name == raw } ?: FIT_PAGE
    }
}

/** Cycle the reading mode on the single toggle (A14): page → width → scroll → page. */
fun nextFitMode(mode: FitMode): FitMode = when (mode) {
    FitMode.FIT_PAGE -> FitMode.FIT_WIDTH
    FitMode.FIT_WIDTH -> FitMode.SCROLL
    FitMode.SCROLL -> FitMode.FIT_PAGE
}

/** Scroll-mode page turn (A14): move the topmost page forward one, clamped to the concert. */
fun scrollNextPage(top: Int, pageCount: Int): Int = (top + 1).coerceIn(0, (pageCount - 1).coerceAtLeast(0))

/** Scroll-mode page turn (A14): move the topmost page back one, clamped at the first page. */
fun scrollPrevPage(top: Int): Int = (top - 1).coerceAtLeast(0)

/** T149 — the breathing margin left below the last glyph so the final line isn't flush against the next
 *  song's title (permille of full page height). "A few percent" (spec). */
const val SCROLL_TRIM_BREATHING_PERMILLE = 40

/**
 * T149 — the fraction (0..1) of a page's full height to draw in SCROLL mode. Only a SONG'S LAST page is
 * trimmed, and only when the baker measured content: [contentBottomPermille] in 1..999 ⇒ draw down to the
 * ink bottom plus a breathing margin (capped at full). Everything else — a non-last page, an old bundle
 * (0/absent), or a page whose ink already reaches the bottom (>=1000) — draws the FULL page. The caller
 * (ScrollReader) only ever runs in SCROLL mode, so FIT_PAGE/two-up are untouched by construction.
 *
 * The baker already wrote max(text ink, overlay ink), so obeying this value keeps a mark BELOW the text
 * visible (T149's protect-the-annotation case) — the app just obeys; it never re-measures.
 */
fun scrollTrimFraction(isLastPageOfSong: Boolean, contentBottomPermille: Int): Double {
    if (!isLastPageOfSong || contentBottomPermille <= 0 || contentBottomPermille >= 1000) return 1.0
    return ((contentBottomPermille + SCROLL_TRIM_BREATHING_PERMILLE) / 1000.0).coerceAtMost(1.0)
}

/**
 * N1 — did navigating from page [from] to page [to] cross into a different song? A continuous advance
 * is a performance requirement (pedal users can't stop at every song end), but crossing must READ as
 * crossing, so this drives the transient boundary cue. Out-of-range or same-song ⇒ false. Pure.
 */
internal fun crossedSongBoundary(pages: List<StagePage>, from: Int, to: Int): Boolean {
    val a = pages.getOrNull(from) ?: return false
    val b = pages.getOrNull(to) ?: return false
    return a.songId != b.songId
}

/**
 * N8 — is a horizontal song-cross swipe (scroll mode) blocked because there's no adjacent song?
 * [forward] at the LAST song, or backward at the FIRST, ⇒ blocked → the N7 end-of-bounds cue (scroll's
 * native rubber-band only signals VERTICAL edges). No songs ⇒ blocked. Pure.
 */
internal fun isBlockedSongCross(currentSong: Int, songCount: Int, forward: Boolean): Boolean {
    if (songCount <= 0) return true
    return if (forward) currentSong >= songCount - 1 else currentSong <= 0
}

/**
 * N2 — the global page-index range (inclusive) of the song containing [page]. Per-song scroll shows
 * ONLY these pages so vertical motion always reads WITHIN the current song and crossing to another
 * song is always an explicit act. Empty when there are no pages. Songs' firstPage is monotonic in
 * bundle order, so the containing song is the last one whose firstPage is at or before [page]. Pure.
 */
internal fun songPageRange(state: StageState, page: Int): IntRange {
    if (state.pages.isEmpty()) return IntRange.EMPTY
    val p = page.coerceIn(0, state.pages.lastIndex)
    val songIdx = state.songs.indexOfLast { it.firstPage <= p }.coerceAtLeast(0)
    val first = state.songs.getOrNull(songIdx)?.firstPage ?: 0
    val last = (state.songs.getOrNull(songIdx + 1)?.firstPage ?: state.pageCount) - 1
    return first..last.coerceAtLeast(first)
}

/** A page either performs or shows a neutral placeholder — never a crash. */
enum class PageStatus { READY, UNAVAILABLE }

/** One performable page, flattened across songs. Blob refs are resolved by the decoder at draw time. */
data class StagePage(
    val songId: String,
    val songName: String,
    val pageInSong: Int,            // 0-based index within its song
    val rasterRef: String,
    val rasterHash: String = "",    // content hash (R10): identifies an UNCHANGED page across a re-bake
    val overlays: List<LayerImage>, // already sorted by z-order (A02 loader)
    val status: PageStatus,         // UNAVAILABLE when the raster blob was flagged missing/empty
    // Setlist metadata (A08) — carried from BakedSong; shown only on the song's first page (pageInSong 0).
    val displayNotes: String = "",
    val key: String = "",
    val tempo: Int = 0,
    val meter: String = "",         // A35: the song's metre (proto 12); "" ⇒ 4/4 (pre-T86 bundles)
    // T149: the page's ink bottom as a fraction of full height, in permille (baker = max(raster, overlays));
    // 0/absent ⇒ full page. Stage trims a SONG'S LAST page to this (+ a breathing margin) in SCROLL mode.
    val contentBottomPermille: Int = 0,
)

/**
 * A song entry for the picker: jumps to the song's first global page. [onCall] marks a bench/encore
 * song (T23) — baked and jumpable but outside the running order; the drawer groups these separately.
 */
data class SongInfo(
    val songId: String,
    val name: String,
    val firstPage: Int,
    // P207: the artist, a bake-time snapshot; empty is normal (many songs have none) and MUST behave as
    // before (no artist line in the drawer). Shown after the title in grey, clipped before the title.
    val artist: String = "",
    val onCall: Boolean = false,
    // T50/A20: the baked-for member's personal cues for this song (icon + tint), in bake order. Shown
    // in the A15 drawer row and flashed center on song entry. Empty when the member has none.
    val cues: List<SongCue> = emptyList(),
)

/** A distinct layer aggregated across the bundle, for the Layers panel.
 *  P205: [owner] ("" = band/shared, else a member id) and [defaultOn] (bake-time default; null =
 *  compute as today) drive the Stage-3a default-visibility precedence. */
data class LayerInfo(
    val layerId: String,
    val mandatory: Boolean,
    val roleTag: String,
    val name: String = "",
    val owner: String = "",
    val defaultOn: Boolean? = null,
)

/**
 * The whole presenter state. Pure data, no Android deps — all transitions are total and clamped, so
 * this is where the resilience contract is unit-tested. When [failure] is non-null the loader could
 * not read the bundle at all and the UI shows a friendly failure screen with a way back.
 */
data class StageState(
    val failure: String? = null,
    val pages: List<StagePage> = emptyList(),
    val songs: List<SongInfo> = emptyList(),
    val layers: List<LayerInfo> = emptyList(),
    val current: Int = 0,
    val fitMode: FitMode = FitMode.FIT_PAGE,
    // A1: layer visibility is PER-SONG (keyed by songId), not concert-wide. The role seeds each song's
    // default-visible layers; a manual toggle changes only the current song's set and is remembered for
    // it within the Stage session (I12 — session-scoped, nothing persisted).
    val visibleBySong: Map<String, Set<String>> = emptyMap(),
    val role: String = "",
    // P205 Stage 3a: the band roster (from the bundle) + the viewer's resolved identity (a member id,
    // "" = none/anonymous-unpicked). Identity is a LOCAL view preference (I12 — no account); it picks
    // this member's cues (member_cues) and seeds their personal layers on. Host resolves/persists it.
    val roster: List<BundleMember> = emptyList(),
    val identity: String = "",
    // P201/I13 rehearsal auto-update: TRANSIENT — lives only in this in-memory state, is
    // never written through the Storage seam, and resets to false whenever Stage is left
    // (a fresh StageViewModel starts it false). While true, the host polls for a new
    // concert rev and applies it via applyUpdate (viewport-preserving, R10).
    val autoUpdate: Boolean = false,
    // T143: a self-dismissing notice that an auto-update was just applied under the performer (VLL was in
    // the bake when it silently swapped). Set by applyUpdate, cleared by the view after it's shown; it
    // NEVER moves the page (the R10 remap already preserves position) — it only says a word.
    val updateNotice: String? = null,
    // T147: the rehearsal chronometer — a pure state machine (start instant + accumulated, not a tick
    // counter) so it survives screen-off/process death. It times the SESSION, so it must be PRESERVED
    // across song navigation, setIdentity and applyUpdate — never rebuilt to a fresh Chrono().
    val chrono: Chrono = Chrono(),
    // T147: whether the bottom-right time-of-day clock overlay is shown. An overlay preference; toggling
    // it must NOT move the page or change page geometry (it is never part of the layout flow).
    val clockVisible: Boolean = false,
    // T147: analog (default, VLL) or digital clock face. A view preference like clockVisible.
    val clockStyle: ClockStyle = ClockStyle.ANALOG,
) {
    val pageCount: Int get() = pages.size
    val currentPage: StagePage? get() = pages.getOrNull(current)

    /** The song index the current page belongs to (for highlighting the picker), or -1 if none. */
    val currentSong: Int
        get() = songs.indexOfLast { it.firstPage <= current }

    /**
     * The visible layer ids for [songId] (A1 per-song visibility). Mandatory layers are unioned in HERE,
     * at read — the single enforcement point (I12: mandatory is ALWAYS visible, the viewer cannot hide
     * it). This closes a P201 hole: a layer that a performer legally hid while optional and that a
     * later re-bake marks mandatory would otherwise stay hidden through the merge; enforcing at read
     * covers the merge, any future persistence, and every other path. Empty stored set ⇒ just mandatory.
     */
    fun visibleFor(songId: String): Set<String> =
        (visibleBySong[songId] ?: emptySet()) + layers.filter { it.mandatory }.map { it.layerId }
}

/** The default-visible layer ids for (role, identity) — the A1 seed for every song. */
internal fun defaultVisibleLayers(layers: List<LayerInfo>, role: String, identity: String = ""): Set<String> =
    layers.filter { defaultVisible(it, role, identity) }.map { it.layerId }.toSet()

/**
 * Default layer visibility. Precedence (P205 Stage 3a ruling), highest first:
 *  1. mandatory (I12) — always visible, viewer cannot hide.
 *  2. identity — a layer I OWN is on for me; another member's personal layer is NOT for me (off; Stage
 *     3b drops it from the list entirely).
 *  3. bake-time default_on (∧ the role_tag rule) when present; absent ⇒ computed as today.
 *  4. legacy role_tag rule: empty roleTag ⇒ visible; else visible only when it equals the reading role.
 * (Manual per-song toggles sit ABOVE this in the effective view — they override the seed, A1.)
 */
internal fun defaultVisible(layer: LayerInfo, role: String, identity: String = ""): Boolean {
    if (layer.mandatory) return true
    val mine = identity.isNotEmpty() && layer.owner == identity
    if (mine) return true
    if (layer.owner.isNotEmpty()) return false // another member's personal layer — not for me
    val roleOk = layer.roleTag.isEmpty() || layer.roleTag == role
    return layer.defaultOn?.let { it && roleOk } ?: roleOk
}

/** The cues to show for [song] given the viewer's [identity]: their member_cues entry (band-wide bundle),
 *  falling back to the song's own `cues` field (a per-member/-mine bake, or an old bundle). */
internal fun cuesForIdentity(song: BakedSong, identity: String): List<SongCue> =
    song.memberCues.firstOrNull { it.memberId == identity }?.cues?.takeIf { it.isNotEmpty() } ?: song.cues

/**
 * P205 Stage 3b — is this overlay shown to the viewer? A SHARED layer (owner "") or one the viewer
 * OWNS is kept; another member's personal layer is DROPPED at load — never composited, never listed
 * anywhere (the Layers dialog can't even show it). Anonymous (identity "") sees only shared layers.
 */
internal fun visibleToIdentity(overlay: LayerImage, identity: String): Boolean =
    overlay.owner.isEmpty() || overlay.owner == identity

/**
 * T137 — the pool page indices this [identity] READS for [song], in order. `song.pages` is a shared pool
 * (the union of every member's files' pages); this resolves the viewer's own SEQUENCE into it, mirroring
 * the LayerImage.owner "" convention (see proto MemberPages):
 *  1. the member_pages entry whose member_id == [identity], else
 *  2. the "" default entry (the no-selection / anonymous reader), else
 *  3. every pool page in order (an undivergent or pre-T137 bundle carrying no member_pages).
 * Indices outside the pool are dropped defensively — a malformed sequence can never crash the reader
 * mid-set; it just reads fewer pages.
 */
internal fun resolvePageSequence(song: BakedSong, identity: String): List<Int> {
    val seq = song.memberPages.firstOrNull { it.memberId == identity }?.page
        ?: song.memberPages.firstOrNull { it.memberId == "" }?.page
    return (seq ?: song.pages.indices.toList()).filter { it in song.pages.indices }
}

/** Build the initial [StageState] from a loader result. Total: a Failed load becomes a failure state. */
internal fun stageStateFrom(result: LoadResult, role: String, identity: String = ""): StageState = when (result) {
    is LoadResult.Failed -> StageState(failure = result.reason, role = role, identity = identity)
    is LoadResult.Loaded -> buildLoaded(result.bundle, result.issues, role, identity)
}

/**
 * A46 (found by A33 drill 2): resolve a saved reading position — the LOGICAL (songId, pageInSong) — back
 * to a global page index in [state], so reopening a concert (cold start, process death, or the A27
 * "Resume" button) lands back where the performer left off instead of page 0. Keyed on the logical
 * position (not the raw index) so it survives a re-bake that reorders songs/pages:
 *  - exact (song + page-in-song) if it still exists;
 *  - else, if the song is still present but that page is gone (a shorter re-bake), **clamp to the song's
 *    LAST page** — never off the end, and closer to where the performer was than its first (A46 §3);
 *  - else (song removed, or nothing saved) ⇒ 0.
 * Never returns an out-of-range index. Empty [songId]/empty [state] ⇒ 0. Pure/testable.
 */
internal fun resolveStartPage(state: StageState, songId: String, pageInSong: Int): Int {
    if (state.pages.isEmpty() || songId.isEmpty()) return 0
    val exact = state.pages.indexOfFirst { it.songId == songId && it.pageInSong == pageInSong }
    if (exact >= 0) return exact
    val lastOfSong = state.pages.indexOfLast { it.songId == songId }
    return if (lastOfSong >= 0) lastOfSong else 0
}

/**
 * A50 — should the Stage hold hardware key/pedal focus right now? True only when NO focus-stealing
 * surface is open. The Stage's focus effect keys on this (not `Unit`) so focus is RE-requested each time
 * the Stage becomes unobscured — a dialog/drawer/sheet closing hands focus back to nothing otherwise, and
 * the pedal goes dead for the rest of the set after the first surface opens ("the exact failure this
 * product cannot have").
 *
 * DELIBERATELY SEPARATE from `overlayOpen` (chrome auto-hide) and never widened onto it: the identity
 * pick is a focus-stealer here but is (correctly) absent from `overlayOpen`, and the two lists are
 * allowed to diverge — an omission in chrome is a cosmetic beat, the same omission here is a dead pedal.
 * [switchIdentity]/[needsIdentityPick]/[pickDismissed] mirror the WhoAreYouDialog's own open condition so
 * the identity pick counts in BOTH of its forms (Settings→Switch, and an unresolved roster).
 */
fun stageHoldsKeyFocus(
    drawerOpen: Boolean,
    showSettings: Boolean,
    showLayers: Boolean,
    showRole: Boolean,
    switchIdentity: Boolean,
    needsIdentityPick: Boolean,
    pickDismissed: Boolean,
): Boolean {
    val identityPickOpen = switchIdentity || (needsIdentityPick && !pickDismissed)
    return !(drawerOpen || showSettings || showLayers || showRole || identityPickOpen)
}

private fun buildLoaded(bundle: ConcertBundle, issues: List<BundleIssue>, role: String, identity: String): StageState {
    // Refs flagged missing/empty for a specific (song,page) — used to mark a page's RASTER unavailable.
    val badRefs: Set<String> = issues
        .filter { it.kind == BundleIssue.Kind.MISSING_BLOB || it.kind == BundleIssue.Kind.EMPTY_BLOB }
        .map { blobKey(it.songId, it.page, it.ref) }
        .toSet()

    val pages = ArrayList<StagePage>()
    val songs = ArrayList<SongInfo>()
    bundle.songs.forEachIndexed { songIdx, song ->
        // T26: the baked title names the song; empty/absent falls back to the "Song N" client default.
        val songName = song.title.ifBlank { "Song ${songIdx + 1}" }
        // P205 Stage 3a: show the viewer identity's cues (member_cues), falling back to the song's own
        // `cues` (a -mine bake or an old bundle). Anonymous (identity "") ⇒ the fallback.
        songs.add(SongInfo(song.songId, songName, pages.size, artist = song.artist, onCall = song.onCall, cues = cuesForIdentity(song, identity)))
        // T137: read the viewer identity's own SEQUENCE into the pool, not the raw pool order. pageInSong
        // counts within that resolved sequence (so it stays 0..N and songStarts/facing-pages derive from
        // it); the POOL index (`poolIdx`) still keys the loader's per-page blob-availability check.
        resolvePageSequence(song, identity).forEachIndexed { pageInSong, poolIdx ->
            val page = song.pages[poolIdx]
            val rasterBad = blobKey(song.songId, poolIdx, page.pageRasterRef) in badRefs
            pages.add(
                StagePage(
                    songId = song.songId,
                    songName = songName,
                    pageInSong = pageInSong,
                    rasterRef = page.pageRasterRef,
                    rasterHash = page.rasterHash,
                    // Stage 3b: drop other members' personal overlays (owner != me) — never composited.
                    overlays = page.overlays.filter { visibleToIdentity(it, identity) },
                    status = if (rasterBad) PageStatus.UNAVAILABLE else PageStatus.READY,
                    displayNotes = song.displayNotes,
                    key = song.key,
                    tempo = song.tempo,
                    meter = song.meter,
                    contentBottomPermille = page.contentBottomPermille,
                ),
            )
        }
    }

    val layers = aggregateLayers(bundle, identity)
    // A1 + Stage 3a: seed every song with the (role, identity) default-visible layers; per-song
    // overrides diverge later.
    val defaults = defaultVisibleLayers(layers, role, identity)
    val visibleBySong = songs.associate { it.songId to defaults }
    return StageState(
        pages = pages, songs = songs, layers = layers, visibleBySong = visibleBySong,
        role = role, roster = bundle.roster, identity = identity,
    )
}

/** Distinct layers across the bundle, insertion-ordered; mandatory if any occurrence is mandatory.
 *  Stage 3b: other members' personal layers (owner != identity, owner non-empty) are excluded — they
 *  are not the viewer's and must not appear anywhere (Layers dialog included). */
private fun aggregateLayers(bundle: ConcertBundle, identity: String): List<LayerInfo> {
    val byId = LinkedHashMap<String, LayerInfo>()
    for (song in bundle.songs) {
        for (page in song.pages) {
            for (o: LayerImage in page.overlays) {
                if (!visibleToIdentity(o, identity)) continue
                val prev = byId[o.layerId]
                byId[o.layerId] = LayerInfo(
                    layerId = o.layerId,
                    mandatory = (prev?.mandatory ?: false) || o.mandatory,
                    roleTag = prev?.roleTag?.ifEmpty { o.roleTag } ?: o.roleTag,
                    name = prev?.name?.ifEmpty { o.name } ?: o.name, // T53: first non-empty baked name
                    owner = prev?.owner?.ifEmpty { o.owner } ?: o.owner, // P205: layer owner (member id / "")
                    defaultOn = prev?.defaultOn ?: o.defaultOn,         // P205: first-seen bake-time default
                )
            }
        }
    }
    return byId.values.toList()
}

private fun blobKey(songId: String, page: Int, ref: String): String = "$songId#$page#$ref"

/**
 * The one-line setlist-metadata strip for a song's first page (A08): notes · key · ♩=tempo, omitting
 * empty fields; tempo 0 is omitted. Returns null when nothing to show (⇒ no strip renders, layout
 * unchanged). Visual one-line truncation is the caller's job (Text maxLines=1 + ellipsis).
 */
internal fun metaStripText(displayNotes: String, key: String, tempo: Int, meter: String = ""): String? {
    val parts = buildList {
        if (displayNotes.isNotBlank()) add(displayNotes.trim())
        if (key.isNotBlank()) add(key.trim())
        // A35: the beat-note glyph follows the metre — ♩ simple · ♩. compound · ♪ irregular-additive.
        if (tempo > 0) {
            val note = when (tempoUnit(meterGroups(meter))) {
                TempoUnit.DOTTED_QUARTER -> "♩."
                TempoUnit.EIGHTH -> "♪"
                TempoUnit.QUARTER -> "♩"
            }
            add("$note=$tempo") // a display, not a click track
        }
    }
    return if (parts.isEmpty()) null else parts.joinToString("  ·  ")
}

/**
 * The A15 song-drawer meta line for the song at [songIndex]: the same notes · key · ♩=tempo as the
 * A08 strip, read from the song's FIRST page (where the setlist metadata is carried). Null when the
 * song or its page is missing, or when the song has no metadata to show. Pure so it's unit-tested.
 */
internal fun songMetaLine(state: StageState, songIndex: Int): String? {
    val song = state.songs.getOrNull(songIndex) ?: return null
    val page = state.pages.getOrNull(song.firstPage) ?: return null
    return metaStripText(page.displayNotes, page.key, page.tempo, page.meter)
}

/**
 * N10 — the layers that appear in [songId]'s pages, in concert order. The Layers dialog scopes to the
 * CURRENT song, not the concert-wide [StageState.layers] aggregate: everything the reader sees is "this
 * song" (matching per-song scroll N2, song-aligned spreads N6, per-song visibility A1), and listing all
 * 14 concert layers on a 5-layer song read as "too many / funky" (VLL). Empty for an unknown song. Pure.
 */
internal fun songLayers(state: StageState, songId: String): List<LayerInfo> {
    val ids = state.pages.asSequence()
        .filter { it.songId == songId }
        .flatMap { it.overlays.asSequence() }
        .map { it.layerId }
        .toSet()
    return state.layers.filter { it.layerId in ids }
}

/**
 * N10 — the human labels for a song's layers in the dialog. Label chain (T53): the real baked
 * layer name if present ("Guitar", "Lead vocal"); else a prettified role tag ("conductor" →
 * "Conductor"); else — an untagged, un-named layer has nothing human at all, so rather than
 * exposing its raw id (a hash like "L-86453186435184" reads as an internal id / "strange" — VLL)
 * we number them "Layer N" in list order. Numbering is per un-labelled layer so a named/role layer
 * never consumes a number. Pre-T53 bundles have no name and fall through to role→number. Pure.
 */
internal fun songLayerLabels(state: StageState, songId: String): List<Pair<LayerInfo, String>> {
    var n = 0
    return songLayers(state, songId).map { layer ->
        val label = when {
            layer.name.isNotEmpty() -> layer.name
            layer.roleTag.isNotEmpty() -> layer.roleTag.replaceFirstChar { it.uppercase() }
            else -> "Layer ${++n}"
        }
        layer to label
    }
}
