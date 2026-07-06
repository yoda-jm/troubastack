package com.troubashare.shared.distribution

import com.troubashare.shared.bundle.AvailableConcert
import com.troubashare.shared.bundle.AvailableConcerts
import com.troubashare.shared.bundle.ImportResult
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

/**
 * Diff-matrix + apply behaviour for [UpdatesManager] — pure shared logic, fully faked (no network,
 * no fs). Covers I13's offered/newly/frozen/pinned/final-locked cases and the "a failed apply leaves
 * installed state untouched" guarantee.
 */
class UpdatesManagerTest {

    private class FakeTransport(
        val manifest: AvailableConcerts = AvailableConcerts(),
        val onDownload: (String, String) -> Unit = { _, _ -> },
    ) : ManifestTransport {
        var downloads = 0
        override suspend fun fetchManifest() = manifest
        override suspend fun downloadBundle(concertId: String, destPath: String) {
            downloads++; onDownload(concertId, destPath)
        }
    }

    /** In-memory Storage-KV for the policy JSON. */
    private class FakeKV {
        var value: String? = null
        val read: () -> String? = { value }
        val write: (String) -> Unit = { value = it }
    }

    private fun manager(
        transport: ManifestTransport,
        installed: Map<String, ULong> = emptyMap(),
        kv: FakeKV = FakeKV(),
        importResult: ImportResult = ImportResult.Imported("c"),
        onImport: (String) -> Unit = {},
    ) = UpdatesManager(
        transport = transport,
        tempDir = { "/tmp" },
        installedRevs = { installed },
        importBundle = { onImport(it); importResult },
        readPolicies = kv.read,
        writePolicies = kv.write,
    )

    private fun concert(id: String, rev: ULong, finalLocked: Boolean = false) =
        AvailableConcert(concertId = id, name = id, currentRev = rev, finalLocked = finalLocked)

    @Test
    fun notInstalled_isNewlyAvailable() {
        val m = manager(FakeTransport(), installed = emptyMap())
        val out = m.diff(AvailableConcerts(listOf(concert("a", 3uL))))
        assertEquals(listOf<Availability>(Availability.NewlyAvailable("a")), out)
    }

    @Test
    fun installedBehindServer_offersUpdate_andSameRevOffersNothing() {
        val m = manager(FakeTransport(), installed = mapOf("a" to 2uL, "b" to 5uL))
        val out = m.diff(AvailableConcerts(listOf(concert("a", 3uL), concert("b", 5uL))))
        assertEquals(listOf<Availability>(Availability.UpdateOffered("a", 2uL, 3uL)), out)
    }

    @Test
    fun frozenPolicy_suppressesOffer() {
        val kv = FakeKV()
        val m = manager(FakeTransport(), installed = mapOf("a" to 2uL), kv = kv)
        m.setPolicy("a", UpdatePolicy.FROZEN)
        assertTrue(m.diff(AvailableConcerts(listOf(concert("a", 3uL)))).isEmpty())
    }

    @Test
    fun localPin_suppressesOffer_andUnpinRestoresIt() {
        val kv = FakeKV()
        val m = manager(FakeTransport(), installed = mapOf("a" to 2uL), kv = kv)
        m.setFreeze("a", Freeze.LocalPin(atRev = 2uL))
        assertTrue(m.diff(AvailableConcerts(listOf(concert("a", 3uL)))).isEmpty())
        m.setFreeze("a", null)
        assertEquals(
            listOf<Availability>(Availability.UpdateOffered("a", 2uL, 3uL)),
            m.diff(AvailableConcerts(listOf(concert("a", 3uL)))),
        )
    }

    @Test
    fun serverFinalLocked_suppressesOffer() {
        val m = manager(FakeTransport(), installed = mapOf("a" to 2uL))
        val out = m.diff(AvailableConcerts(listOf(concert("a", 3uL, finalLocked = true))))
        assertTrue(out.isEmpty())
    }

    @Test
    fun policyBook_persistsAcrossManagerInstances() {
        val kv = FakeKV()
        manager(FakeTransport(), kv = kv).setPolicy("a", UpdatePolicy.FROZEN)
        // A fresh manager reading the same KV must still see the frozen policy.
        val fresh = manager(FakeTransport(), installed = mapOf("a" to 2uL), kv = kv)
        assertTrue(fresh.diff(AvailableConcerts(listOf(concert("a", 9uL)))).isEmpty())
    }

    @Test
    fun apply_downloadsThenImports() = runTest {
        val t = FakeTransport()
        val m = manager(t, importResult = ImportResult.Imported("a"))
        val r = m.apply(Availability.UpdateOffered("a", 1uL, 2uL))
        assertIs<ImportResult.Imported>(r)
        assertEquals(1, t.downloads)
    }

    @Test
    fun apply_downloadFailure_leavesStateIntact_importNeverCalled() = runTest {
        val t = FakeTransport(onDownload = { _, _ -> throw RuntimeException("offline") })
        var imported = false
        val m = manager(t, onImport = { imported = true })
        val r = m.apply(Availability.NewlyAvailable("a"))
        assertIs<ImportResult.Failed>(r)
        assertTrue(!imported, "a failed download must never reach the importer (old bundle untouched)")
    }

    @Test
    fun apply_importFailure_isReportedFailed() = runTest {
        val m = manager(FakeTransport(), importResult = ImportResult.Failed("bad zip"))
        assertIs<ImportResult.Failed>(m.apply(Availability.NewlyAvailable("a")))
    }

    @Test
    fun apply_songChanged_isUnsupported() = runTest {
        val m = manager(FakeTransport())
        assertIs<ImportResult.Failed>(m.apply(Availability.SongChanged("a", "s1", 3uL)))
    }
}
