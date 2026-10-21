// A91 — ONE concert list: one row per concert, one plain-words state, ONE primary action (VLL: "ça marche jamais comme
// on pense, les rebake les updates, les nouveaux"). This is the whole state machine, PURE and table-tested; Compose only
// renders what it returns. A row is the union, by concert id, of what the server lists and what is on this tablet —
// nothing is announced (no chips, no banners): the state is written on the row, and a concert deleted here stays
// listed as "On the server" with Download (A90, absorbed).
package com.troubastack.shared.distribution

import kotlinx.serialization.Serializable

/** A concert bundle on this tablet, as the list needs it. */
data class LocalBundle(
    val id: String,
    val rev: ULong,
    val bakedAt: Long, // epoch seconds
    val name: String,
    val bandId: String,
    val bandName: String,
    val damaged: Boolean = false,
    val songs: List<LocalSong> = emptyList(),
)

/** A song in a local bundle: its id, its SOURCE revision at bake (the same number the server's manifest lists per song),
 *  and its title (for "what changed"). */
data class LocalSong(val id: String, val rev: ULong, val title: String)

/** A concert as the server lists it: the manifest's revs and flags plus the Studio list's name, band, date and role.
 *  Serializable so the last answer is kept for offline rows ("last checked …", "v7 seen on the server"). */
@Serializable
data class ServerConcert(
    val id: String,
    val rev: ULong,
    val name: String = "",
    val bandId: String = "",
    val bandName: String = "",
    val eventDate: String = "", // ISO yyyy-mm-dd, or ""
    val venue: String = "",
    val finalLocked: Boolean = false,
    val archived: Boolean = false,
    val songs: List<ServerSong> = emptyList(),
    val isAdmin: Boolean = false,
    val live: Boolean = false,
)

/** A song as the manifest lists it: its id and current rev. */
@Serializable
data class ServerSong(val id: String, val rev: ULong)

/** Whether the server answered the last refresh. [lastCheckedAt]: epoch seconds of the last successful answer. */
sealed interface Reach {
    data class Online(val checkedAt: Long) : Reach
    data class Offline(val lastCheckedAt: Long?) : Reach
}

/** An operation running on a row. [progress] 0..1, or null when the total is unknown. */
sealed interface RowOp {
    data class Downloading(val progress: Float?) : RowOp
    data class Updating(val progress: Float?) : RowOp
    data class Baking(val label: String) : RowOp
}

enum class RowSection { MAIN, ARCHIVED }
enum class RowStateKind { BUSY, ARCHIVED, DAMAGED, NOT_ON_TABLET, LOCAL_ONLY, OFFLINE, UP_TO_DATE, FROZEN, FINAL, UPDATE_AVAILABLE }
enum class RowAction { OPEN, UPDATE, DOWNLOAD, CANCEL, NONE }
enum class RowMenuItem { OPEN_THIS_VERSION, FREEZE, UNFREEZE, REBAKE, LIVE_ON, LIVE_OFF, DELETE }

/** "What changed" between the copy here and the server's: titles where known, a count for songs only the server has. */
data class SongChanges(val changed: List<String>, val added: Int, val removed: List<String>) {
    val isEmpty: Boolean get() = changed.isEmpty() && added == 0 && removed.isEmpty()
    /** "2 songs changed, 1 added" — only the non-zero parts; "" when nothing differs song by song. */
    fun summary(): String = listOfNotNull(
        changed.size.takeIf { it > 0 }?.let { "$it song${if (it == 1) "" else "s"} changed" },
        added.takeIf { it > 0 }?.let { "$it added" },
        removed.size.takeIf { it > 0 }?.let { "$it removed" },
    ).joinToString(", ")
}

/** One row of the list: who, where it sits, its state in words, its one primary action, and its ⋯ menu. */
data class ConcertRow(
    val id: String,
    val name: String,
    val bandId: String,
    val bandName: String,
    val eventDate: String,
    val venue: String,
    val section: RowSection,
    val state: RowStateKind,
    val line: String,
    val primary: RowAction,
    val primaryEnabled: Boolean,
    val menu: List<RowMenuItem>,
    val changes: SongChanges? = null,
)

/** The songs that differ between [local] and [server] by id and rev. */
fun songChanges(local: List<LocalSong>, server: List<ServerSong>): SongChanges {
    val serverRevs = server.associate { it.id to it.rev }
    val localIds = local.map { it.id }.toSet()
    return SongChanges(
        changed = local.filter { it.id in serverRevs && serverRevs[it.id] != it.rev }.map { it.title.ifBlank { it.id } },
        added = server.count { it.id !in localIds },
        removed = local.filter { it.id !in serverRevs }.map { it.title.ifBlank { it.id } },
    )
}

/** "just now", "5 min ago", "3 h ago", "2 days ago". */
fun agoText(nowSec: Long, thenSec: Long): String {
    val d = (nowSec - thenSec).coerceAtLeast(0)
    return when {
        d < 60 -> "just now"
        d < 3600 -> "${d / 60} min ago"
        d < 86_400 -> "${d / 3600} h ago"
        d < 2 * 86_400 -> "1 day ago"
        else -> "${d / 86_400} days ago"
    }
}

/**
 * THE row state (the A91 table, precedence top-down — the first rule that matches wins). [server] is the concert as
 * last seen on the server (null = not listed there); [reach] says whether that view is live; [frozen] is "Freeze this
 * version" (the old FROZEN / pin, one concept); [inFlight] an operation running on it. Returns null for a concert that
 * has no row (archived with no copy here, or neither here nor on the server).
 */
fun rowState(local: LocalBundle?, server: ServerConcert?, reach: Reach, frozen: Boolean, inFlight: RowOp?, nowSec: Long): ConcertRow? {
    if (local == null && server == null) return null
    val online = reach is Reach.Online
    val checked = when (reach) { is Reach.Online -> reach.checkedAt; is Reach.Offline -> reach.lastCheckedAt }
    val lastChecked = checked?.let { "last checked " + agoText(nowSec, it) } ?: "not checked yet"
    val admin = online && server != null && server.isAdmin
    fun menu(vararg items: RowMenuItem?) = items.filterNotNull()
    val adminItems = if (admin) listOf(RowMenuItem.REBAKE, if (server!!.live) RowMenuItem.LIVE_OFF else RowMenuItem.LIVE_ON) else emptyList()
    fun row(section: RowSection, state: RowStateKind, line: String, primary: RowAction, enabled: Boolean, menu: List<RowMenuItem>, changes: SongChanges? = null) =
        ConcertRow(
            id = local?.id ?: server!!.id,
            name = server?.name?.ifBlank { null } ?: local?.name?.ifBlank { null } ?: (local?.id ?: server!!.id),
            bandId = server?.bandId?.ifBlank { null } ?: local?.bandId ?: "",
            bandName = server?.bandName?.ifBlank { null } ?: local?.bandName ?: "",
            eventDate = server?.eventDate ?: "", venue = server?.venue ?: "",
            section = section, state = state, line = line, primary = primary, primaryEnabled = enabled, menu = menu, changes = changes,
        )
    val deleteItem = if (local != null) RowMenuItem.DELETE else null

    // 1 — an operation in progress
    if (inFlight != null) {
        val (line, action) = when (inFlight) {
            is RowOp.Downloading -> "Downloading…" + pct(inFlight.progress) to RowAction.CANCEL
            is RowOp.Updating -> "Updating to v${server?.rev ?: "?"}…" + pct(inFlight.progress) to RowAction.CANCEL
            is RowOp.Baking -> "Baking on the server… ${inFlight.label}".trimEnd() to RowAction.NONE
        }
        val open = if (local != null && !local.damaged && inFlight is RowOp.Baking) RowMenuItem.OPEN_THIS_VERSION else null
        return row(RowSection.MAIN, RowStateKind.BUSY, line, action, action != RowAction.NONE, menu(open))
    }
    // 2/3 — archived on the server: never offered; a copy here goes to the Archived section, else no row
    if (server != null && server.archived) {
        if (local == null) return null
        val line = "Archived on the server · v${local.rev}" + if (local.damaged) " · can't be opened" else ""
        return row(RowSection.ARCHIVED, RowStateKind.ARCHIVED, line, RowAction.OPEN, !local.damaged, menu(RowMenuItem.DELETE))
    }
    // 4 — damaged copy
    if (local != null && local.damaged) {
        return row(RowSection.MAIN, RowStateKind.DAMAGED, "Can't be opened · re-download", RowAction.DOWNLOAD, online && server != null, menu(RowMenuItem.DELETE))
    }
    // 5/6 — on the server, not on this tablet (deleted here once or never: the same row, never announced)
    if (local == null) {
        val s = server!!
        return if (online) row(RowSection.MAIN, RowStateKind.NOT_ON_TABLET, "On the server · v${s.rev}", RowAction.DOWNLOAD, true, adminItems)
        else row(RowSection.MAIN, RowStateKind.NOT_ON_TABLET, "On the server · v${s.rev} · $lastChecked", RowAction.DOWNLOAD, false, emptyList())
    }
    // 7 — only here
    if (server == null && online) {
        return row(RowSection.MAIN, RowStateKind.LOCAL_ONLY, "On this tablet only · v${local.rev}", RowAction.OPEN, true, menu(RowMenuItem.DELETE))
    }
    val freezeItem = if (frozen) RowMenuItem.UNFREEZE else RowMenuItem.FREEZE
    // 8 — offline: open what is here
    if (!online) {
        val seen = server?.takeIf { it.rev > local.rev }?.let { " · v${it.rev} seen on the server" } ?: ""
        return row(RowSection.MAIN, RowStateKind.OFFLINE, "v${local.rev} · $lastChecked$seen", RowAction.OPEN, true, menu(freezeItem, deleteItem))
    }
    val s = server!!
    // 9 — up to date
    if (s.rev <= local.rev) {
        val frozenNote = if (frozen) " · frozen" else ""
        return row(RowSection.MAIN, RowStateKind.UP_TO_DATE, "Up to date · v${local.rev} · baked ${agoText(nowSec, local.bakedAt)}$frozenNote", RowAction.OPEN, true,
            menu(freezeItem, *adminItems.toTypedArray(), deleteItem))
    }
    val changes = songChanges(local.songs, s.songs)
    val what = changes.summary().let { if (it.isEmpty()) "" else " · $it" }
    // 10 — frozen: the person chose this version
    if (frozen) {
        return row(RowSection.MAIN, RowStateKind.FROZEN, "Frozen at v${local.rev} · v${s.rev} on the server", RowAction.OPEN, true,
            menu(RowMenuItem.UNFREEZE, *adminItems.toTypedArray(), deleteItem), changes)
    }
    // 11 — the band marked the server's version final: offer it (a lock means no newer bakes, not "stay old")
    if (s.finalLocked) {
        return row(RowSection.MAIN, RowStateKind.FINAL, "Final version v${s.rev} (you have v${local.rev})$what", RowAction.UPDATE, true,
            menu(RowMenuItem.OPEN_THIS_VERSION, RowMenuItem.FREEZE, *adminItems.toTypedArray(), deleteItem), changes)
    }
    // 12 — a newer version: Update is always the person's tap
    return row(RowSection.MAIN, RowStateKind.UPDATE_AVAILABLE, "New version v${s.rev} (you have v${local.rev})$what", RowAction.UPDATE, true,
        menu(RowMenuItem.OPEN_THIS_VERSION, RowMenuItem.FREEZE, *adminItems.toTypedArray(), deleteItem), changes)
}

private fun pct(p: Float?): String = p?.let { " ${(it * 100).toInt()} %" } ?: ""

/**
 * The whole list: the union by concert id of [locals] and [servers] (the last server answer; empty when never
 * fetched), one [rowState] each, ordered — bands A→Z (concerts with no band last), and within a band upcoming concerts
 * by date ascending, then past ones by date descending, then undated by name ([today] is ISO yyyy-mm-dd).
 * Archived rows keep their place in that order; the screen shows the ARCHIVED section separately.
 */
fun concertRows(
    locals: List<LocalBundle>, servers: List<ServerConcert>, reach: Reach, frozenIds: Set<String>,
    inFlight: Map<String, RowOp>, nowSec: Long, today: String,
): List<ConcertRow> {
    val byLocal = locals.associateBy { it.id }
    val byServer = servers.associateBy { it.id }
    val ids = (servers.map { it.id } + locals.map { it.id }).distinct()
    val rows = ids.mapNotNull { rowState(byLocal[it], byServer[it], reach, it in frozenIds, inFlight[it], nowSec) }
    return rows.sortedWith(
        compareBy<ConcertRow>({ it.bandName.isBlank() }, { it.bandName.lowercase() }, { it.bandId })
            .thenBy { dateBucket(it.eventDate, today) }
            .thenComparator { a, b ->
                when (dateBucket(a.eventDate, today)) {
                    0 -> a.eventDate.compareTo(b.eventDate)  // upcoming: soonest first
                    1 -> b.eventDate.compareTo(a.eventDate)  // past: most recent first
                    else -> 0
                }
            }
            .thenBy { it.name.lowercase() },
    )
}

/** 0 = upcoming (today or later), 1 = past, 2 = undated. */
private fun dateBucket(date: String, today: String): Int = when {
    !Regex("""^\d{4}-\d{2}-\d{2}""").containsMatchIn(date) -> 2
    date.take(10) >= today -> 0
    else -> 1
}
