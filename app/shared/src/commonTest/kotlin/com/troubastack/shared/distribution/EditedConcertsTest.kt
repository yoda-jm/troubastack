package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A92 §Tests — only an explicit editedSinceBake=true on a non-archived concert counts; unknown never; zero → no line. */
class EditedConcertsTest {
    private fun c(id: String, edited: Boolean?, archived: Boolean = false) = ServerConcert(id, 3uL, name = id, editedSinceBake = edited, archived = archived, isAdmin = edited != null)

    @Test fun only_explicit_true_on_non_archived_concerts_counts() {
        val list = listOf(c("edited", true), c("clean", false), c("unknown", null), c("archivedEdited", true, archived = true))
        assertEquals(listOf("edited"), editedConcerts(list).map { it.id })
    }
    @Test fun unknown_is_never_counted() {
        assertTrue(editedConcerts(listOf(c("a", null), c("b", null))).isEmpty())
    }
    @Test fun zero_means_no_home_line_and_the_wording_counts() {
        assertNull(editedHomeLine(0))
        assertEquals("1 concert edited since its last bake →", editedHomeLine(1))
        assertEquals("2 concerts edited since their last bake →", editedHomeLine(2))
    }
    @Test fun the_changes_in_plain_words() {
        val e = ConcertEdits(listOf(SongEdit("1", "Sail", "content"), SongEdit("2", "Le Sud", "added"), SongEdit("3", "", "removed")), setlist = true)
        assertEquals("Sail: content changed · Le Sud added · a song removed · setlist changed", editsLine(e))
        assertEquals("", editsLine(null))
    }
    @Test fun an_admins_row_says_edited_only_on_an_explicit_true() {
        val now = 1_800_000_000L
        val local = LocalBundle("a", 3uL, now - 60, "a", "b", "B")
        val on = rowState(local, ServerConcert("a", 3uL, editedSinceBake = true, isAdmin = true), Reach.Online(now), false, null, now)!!
        val unknown = rowState(local, ServerConcert("a", 3uL, editedSinceBake = null, isAdmin = true), Reach.Online(now), false, null, now)!!
        assertEquals("Up to date · v3 · baked 1 min ago · edited since its last bake", on.line)
        assertEquals("Up to date · v3 · baked 1 min ago", unknown.line)
    }
}
