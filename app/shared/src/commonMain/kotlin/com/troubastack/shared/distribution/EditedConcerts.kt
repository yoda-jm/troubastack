// A92 — "which concerts were edited since their last bake?" (VLL: "a list of modified non archived playlists,
// including 1 song changed"). The server (T201) says it per concert, honestly, for admins only: `editedSinceBake`
// true / false / absent (unknown — a bake from before T201, or not an admin). PURE: only an explicit TRUE on a
// non-archived concert counts; unknown is never shown as edited.
package com.troubastack.shared.distribution

import kotlinx.serialization.Serializable

/** One song that differs from the last bake. [what]: "added", "removed", "moved", "content". */
@Serializable
data class SongEdit(val songId: String = "", val title: String = "", val what: String = "")

/** What differs from the last bake: the songs, and whether the setlist itself changed. */
@Serializable
data class ConcertEdits(val songs: List<SongEdit> = emptyList(), val setlist: Boolean = false)

/** The concerts to list as edited: an explicit `editedSinceBake == true`, not archived. Unknown (null) never counts. */
fun editedConcerts(servers: List<ServerConcert>): List<ServerConcert> =
    servers.filter { it.editedSinceBake == true && !it.archived }

/** Home's line — "1 concert edited since its last bake →" / "3 concerts edited since their last bake →" — or null
 *  (nothing at all on Home) when there are none. */
fun editedHomeLine(count: Int): String? = when {
    count <= 0 -> null
    count == 1 -> "1 concert edited since its last bake →"
    else -> "$count concerts edited since their last bake →"
}

/** The changes in plain words: "Sail: content changed · Le Sud added · order changed". "" when the server gave none. */
fun editsLine(edits: ConcertEdits?): String {
    if (edits == null) return ""
    val parts = edits.songs.map { s ->
        val name = s.title.ifBlank { "a song" }
        when (s.what) {
            "content" -> "$name: content changed"
            "added" -> "$name added"
            "removed" -> "$name removed"
            "moved" -> "$name moved"
            else -> "$name changed"
        }
    } + listOfNotNull(if (edits.setlist) "setlist changed" else null)
    return parts.joinToString(" · ")
}
