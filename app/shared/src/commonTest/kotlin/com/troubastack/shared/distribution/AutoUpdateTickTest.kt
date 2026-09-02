package com.troubastack.shared.distribution

import com.troubastack.shared.bundle.AvailableConcert
import com.troubastack.shared.bundle.AvailableConcerts
import com.troubastack.shared.bundle.ImportResult
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * P201 stage 3b — UpdatesManager.autoUpdateTick: one rehearsal auto-update poll for the
 * open concert. A new rev on the current concert imports and returns the rev; nothing to
 * do (or any failure) returns null and NEVER disrupts the performance (the current rev stays).
 */
class AutoUpdateTickTest {

    private class FakeTransport(
        val manifest: AvailableConcerts,
        val failDownload: Boolean = false,
    ) : ManifestTransport {
        var downloads = 0
        override suspend fun fetchManifest() = manifest
        override suspend fun downloadBundle(concertId: String, destPath: String, onBytes: (Long, Long) -> Unit) {
            downloads++
            if (failDownload) throw RuntimeException("network down")
        }
    }

    private fun manager(
        transport: ManifestTransport,
        installed: Map<String, ULong>,
        importResult: ImportResult = ImportResult.Imported("c1"),
        onImport: () -> Unit = {},
    ) = UpdatesManager(
        transport = transport,
        tempDir = { "/tmp" },
        installedRevs = { installed },
        importBundle = { onImport(); importResult },
        readPolicies = { null },
        writePolicies = {},
    )

    private fun concert(id: String, rev: ULong, finalLocked: Boolean = false) =
        AvailableConcert(concertId = id, name = id, currentRev = rev, finalLocked = finalLocked)

    @Test
    fun newerRev_importsAndReturnsIt() = runTest {
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 5uL))))
        var imported = false
        val m = manager(t, installed = mapOf("c1" to 3uL), onImport = { imported = true })
        assertEquals(5uL, m.autoUpdateTick("c1"))
        assertEquals(true, imported)
        assertEquals(1, t.downloads)
    }

    @Test
    fun sameRev_isNoOp() = runTest {
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 3uL))))
        val m = manager(t, installed = mapOf("c1" to 3uL))
        assertNull(m.autoUpdateTick("c1"))
        assertEquals(0, t.downloads, "no newer rev → no download")
    }

    @Test
    fun finalLocked_isNotAutoUpdated() = runTest {
        // A locked concert must not auto-update even with a newer rev (diff suppresses it).
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 5uL, finalLocked = true))))
        val m = manager(t, installed = mapOf("c1" to 3uL))
        assertNull(m.autoUpdateTick("c1"))
        assertEquals(0, t.downloads)
    }

    @Test
    fun otherConcertNewer_doesNotTouchTheOpenOne() = runTest {
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 3uL), concert("c2", 9uL))))
        val m = manager(t, installed = mapOf("c1" to 3uL, "c2" to 4uL))
        assertNull(m.autoUpdateTick("c1"), "only the OPEN concert (c1) is auto-updated")
    }

    @Test
    fun failedDownload_returnsNull_showKeepsPlaying() = runTest {
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 5uL))), failDownload = true)
        val m = manager(t, installed = mapOf("c1" to 3uL))
        assertNull(m.autoUpdateTick("c1"), "a failed download is a no-op — the current rev keeps performing")
    }

    @Test
    fun failedImport_returnsNull() = runTest {
        val t = FakeTransport(AvailableConcerts(listOf(concert("c1", 5uL))))
        val m = manager(t, installed = mapOf("c1" to 3uL), importResult = ImportResult.Failed("bad zip"))
        assertNull(m.autoUpdateTick("c1"))
    }
}
