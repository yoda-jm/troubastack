package com.troubastack.shared.distribution

import com.troubastack.shared.bundle.AvailableConcert
import com.troubastack.shared.bundle.AvailableConcerts
import com.troubastack.shared.bundle.ImportResult
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
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
        override suspend fun downloadBundle(concertId: String, destPath: String, onBytes: (Long, Long) -> Unit) {
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
    fun neverDownloaded_alongsideStaleInstalled_bothSurface_a43() {
        // A43-fix — THE bug's shape: one concert installed-but-stale (an Update) and one NEVER downloaded
        // (a Download). diff must EMIT BOTH so the landing can rank them; the old bug was upstream in the
        // landing's ordered `when` (Update won, Download lost), but if diff ever dropped the never-downloaded
        // one the landing couldn't recover it. Both must be present.
        val m = manager(FakeTransport(), installed = mapOf("stale" to 1uL))
        val out = m.diff(AvailableConcerts(listOf(concert("stale", 2uL), concert("new", 1uL))))
        assertTrue(out.any { it is Availability.UpdateOffered && it.concertId == "stale" }, "stale → update, was $out")
        assertTrue(out.any { it is Availability.NewlyAvailable && it.concertId == "new" }, "never-downloaded → available, was $out")
    }

    @Test
    fun deletedConcert_staysQuiet_installedOnceMovesNotNagwareHere() = runTest {
        // A43-fix — the notNagware guarantee, now stated at diff level (it used to be inferred in the landing
        // from set arithmetic). A concert is installed, then DELETED: because a successful apply recorded
        // installedOnce, diff must NOT re-offer it as NewlyAvailable. A genuinely-new concert (no record)
        // still surfaces. Teeth: without installedOnce, the deleted one reappears every diff (nagware).
        val kv = FakeKV()
        var installed = mapOf<String, ULong>()
        val m = UpdatesManager(
            transport = FakeTransport(),
            tempDir = { "/tmp" },
            installedRevs = { installed },
            importBundle = { ImportResult.Imported("del") },
            readPolicies = kv.read,
            writePolicies = kv.write,
        )
        // Download it once (records installedOnce), then it's on the device and current.
        assertIs<ImportResult.Imported>(m.apply(Availability.NewlyAvailable("del")))
        installed = mapOf("del" to 2uL)
        assertTrue(m.diff(AvailableConcerts(listOf(concert("del", 2uL)))).isEmpty(), "current → nothing offered")
        // The performer DELETES it. It must stay QUIET (chosen), while a brand-new one still surfaces.
        installed = emptyMap()
        val out = m.diff(AvailableConcerts(listOf(concert("del", 2uL), concert("fresh", 1uL))))
        assertEquals(listOf<Availability>(Availability.NewlyAvailable("fresh")), out)
    }

    @Test
    fun preExistingInstall_backfilledOnDiff_thenDeletion_staysQuiet_a43followup() {
        // Fable 7d0e2d44 — installedOnce was written ONLY by apply(), so a concert already on the device before
        // this shipped has no record. Deleting it would then read as never-had-it and re-nag. diff() must
        // back-fill the flag when it sees the concert on disk, so a later deletion stays quiet — no migration.
        // Teeth: without the back-fill the SECOND diff returns NewlyAvailable("old") and this fails.
        val kv = FakeKV()
        var installed = mapOf("old" to 2uL)
        val m = UpdatesManager(
            transport = FakeTransport(),
            tempDir = { "/tmp" },
            installedRevs = { installed },
            importBundle = { ImportResult.Imported("old") },
            readPolicies = kv.read,
            writePolicies = kv.write,
        )
        // First manifest fetch after upgrade — the concert is on disk, current, with NO policy record yet.
        assertTrue(m.diff(AvailableConcerts(listOf(concert("old", 2uL)))).isEmpty(), "current install → nothing")
        // The performer now DELETES it. It must NOT be re-offered — the back-fill recorded it as ever-installed.
        installed = emptyMap()
        assertTrue(
            m.diff(AvailableConcerts(listOf(concert("old", 2uL)))).isEmpty(),
            "a pre-existing install, deleted after upgrade, must stay quiet (installedOnce back-filled by diff)",
        )
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
    fun apply_emitsDownloadBytesThenInstalling() = runTest {
        // A42 ①: apply forwards the transport's byte counts as Downloading, then exactly one Installing
        // right before the importer runs — the sequence the InFlight row renders.
        val transport = object : ManifestTransport {
            override suspend fun fetchManifest() = AvailableConcerts()
            override suspend fun downloadBundle(concertId: String, destPath: String, onBytes: (Long, Long) -> Unit) {
                onBytes(0, 100); onBytes(50, 100); onBytes(100, 100)
            }
        }
        val seen = mutableListOf<UpdateProgress>()
        val m = UpdatesManager(
            transport = transport, tempDir = { "/tmp" }, installedRevs = { emptyMap() },
            importBundle = { ImportResult.Imported("c") }, readPolicies = { null }, writePolicies = {},
        )
        assertIs<ImportResult.Imported>(m.apply(Availability.NewlyAvailable("c")) { seen += it })
        assertEquals(UpdateProgress.Downloading(0, 100), seen.first())
        assertEquals(UpdateProgress.Downloading(100, 100), seen[2])
        assertEquals(UpdateProgress.Installing, seen.last())
        assertEquals(1, seen.count { it == UpdateProgress.Installing }, "exactly one install phase")
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
    fun apply_cancellation_propagates_notSwallowed_andImportNeverCalled() = runTest {
        // A39: Home's Cancel is the first cancellable caller of apply(). A CancellationException from
        // the download must PROPAGATE (so the caller's per-offer loop stops) rather than be caught and
        // turned into Failed — and, like any interrupted download, must never reach the importer, so the
        // installed bundle is untouched (I12).
        val t = FakeTransport(onDownload = { _, _ -> throw kotlin.coroutines.cancellation.CancellationException("cancelled") })
        var imported = false
        val m = manager(t, onImport = { imported = true })
        assertFailsWith<kotlin.coroutines.cancellation.CancellationException> {
            m.apply(Availability.UpdateOffered("a", 1uL, 2uL))
        }
        assertTrue(!imported, "a cancelled download must never reach the importer")
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
