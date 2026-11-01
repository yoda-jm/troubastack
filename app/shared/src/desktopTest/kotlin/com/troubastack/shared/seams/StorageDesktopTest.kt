package com.troubastack.shared.seams

import java.io.File
import java.nio.file.Files
import java.nio.file.attribute.PosixFilePermission
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * WIN02 — the desktop Storage seam: where the data root resolves, the secret round trip, the heal-on-corrupt
 * path (A54), that the secret file is NOT plaintext at rest, and (B1) that every Storage over one root behaves
 * as one store and concurrent writes are safe. Runs on both JVMs; the DPAPI branch of
 * [secretFileIsNotPlaintextOnDisk] is the part the `windows` job proves, the `0600` branch the Linux one.
 */
class StorageDesktopTest {

    private lateinit var root: File

    @BeforeTest fun setup() {
        root = Files.createTempDirectory("tstage-store").toFile()
        Storage.settingsWereReset = false
    }

    @AfterTest fun teardown() {
        root.deleteRecursively()
        Storage.settingsWereReset = false
    }

    // ---- data root resolution (pure; each branch without touching the real env/OS) ----

    @Test fun resolveDataRoot_overrideWinsEverywhere() {
        val over = resolveDataRoot("/custom/home", isWindows = false, localAppData = "/x", xdgDataHome = "/y", userHome = "/home/u")
        assertEquals(File("/custom/home"), over)
        val overWin = resolveDataRoot("""D:\over""", isWindows = true, localAppData = """C:\la""", xdgDataHome = null, userHome = """C:\Users\u""")
        assertEquals(File("""D:\over"""), overWin)
    }

    @Test fun resolveDataRoot_windowsUsesLocalAppData_elseUserHome_andIsNotTheInstallDir() {
        // B2: the data folder is TroubaStageData, a SIBLING of the install dir %LOCALAPPDATA%\TroubaStage.
        val local = """C:\Users\u\AppData\Local"""
        val data = resolveDataRoot(null, isWindows = true, localAppData = local, xdgDataHome = null, userHome = """C:\Users\u""")
        assertEquals(File(local, "TroubaStageData"), data)
        assertFalse(data.path.startsWith(File(local, "TroubaStage").path + File.separator), "data root must not be inside the install dir")
        assertEquals(
            File("""C:\Users\u\AppData\Local""", "TroubaStageData"),
            resolveDataRoot(null, isWindows = true, localAppData = null, xdgDataHome = null, userHome = """C:\Users\u"""),
        )
    }

    @Test fun resolveDataRoot_elsewhereUsesXdg_elseDotLocalShare() {
        assertEquals(
            File("/xdg/data", "troubastage"),
            resolveDataRoot(null, isWindows = false, localAppData = null, xdgDataHome = "/xdg/data", userHome = "/home/u"),
        )
        assertEquals(
            File("/home/u/.local/share", "troubastage"),
            resolveDataRoot(null, isWindows = false, localAppData = null, xdgDataHome = null, userHome = "/home/u"),
        )
    }

    // ---- secret round trip ----

    @Test fun secretRoundTrips_andPersistsToDiskAcrossInstances() {
        Storage(root, PlainCodec).putSecret("token", "cafe-123")
        assertEquals("cafe-123", Storage(root, PlainCodec).getSecret("token"))
        assertNull(Storage(root, PlainCodec).getSecret("absent"))
        assertFalse(Storage.settingsWereReset, "a healthy store is never reset")
    }

    // ---- B1: two instances over one root are ONE store ----

    @Test fun twoInstancesOverOneRoot_areOneStore_noLostWrites() {
        // The field repro: MainActivity opens a theme store and a data store over the same root; interleaved
        // writes through the two must all survive (a per-instance cache drops whichever keys it did not load).
        val theme = Storage(root, PlainCodec)
        val data = Storage(root, PlainCodec)
        theme.putSecret("theme", "dark")
        data.putSecret("pos", "5")
        theme.putSecret("volume", "7")

        // Every instance — the two, and a fresh one reading from disk — sees all three.
        for (s in listOf(theme, data, Storage(root, PlainCodec))) {
            assertEquals("dark", s.getSecret("theme"))
            assertEquals("5", s.getSecret("pos"))
            assertEquals("7", s.getSecret("volume"))
        }
    }

    @Test fun concurrentPuts_areSafe_noLostUpdatesOrExceptions() {
        val threads = 8
        val perThread = 40
        val pool = Executors.newFixedThreadPool(threads)
        val errors = ConcurrentLinkedQueue<Throwable>()
        val latch = CountDownLatch(threads)
        repeat(threads) { t ->
            pool.submit {
                try {
                    val s = Storage(root, PlainCodec) // its own instance, same root → shared lock
                    for (k in 0 until perThread) s.putSecret("k-$t-$k", "$t-$k")
                } catch (e: Throwable) {
                    errors += e
                } finally {
                    latch.countDown()
                }
            }
        }
        assertTrue(latch.await(60, TimeUnit.SECONDS), "threads did not finish")
        pool.shutdown()
        assertTrue(errors.isEmpty(), "concurrent putSecret threw: ${errors.firstOrNull()}")

        val fresh = Storage(root, PlainCodec)
        for (t in 0 until threads) for (k in 0 until perThread) {
            assertEquals("$t-$k", fresh.getSecret("k-$t-$k"), "lost update at k-$t-$k")
        }
    }

    // ---- heal on corrupt (A54) ----

    @Test fun healsToEmpty_whenTheStoreCannotBeDecrypted() {
        Storage(root, PlainCodec).putSecret("token", "will-be-lost")
        assertTrue(File(root, "secrets.dat").isFile)

        val healed = Storage(root, ThrowingDecryptCodec)
        assertNull(healed.getSecret("token"), "an unreadable store heals to empty, never crashes")
        assertTrue(Storage.settingsWereReset, "Home's 'settings were reset' signal must be raised")
        assertFalse(File(root, "secrets.dat").isFile, "the corrupt file must be removed")

        healed.putSecret("fresh", "ok")
        assertEquals("ok", Storage(root, PlainCodec).getSecret("fresh"))
    }

    @Test fun missingFile_isSimplyEmpty_notAReset() {
        assertNull(Storage(root, PlainCodec).getSecret("anything"))
        assertFalse(Storage.settingsWereReset)
    }

    // ---- protected at rest: DPAPI (Windows) / 0600 (elsewhere) ----
    // B4: construct through defaultSecretCodec() so a regression in defaultSecretCodec()/isWindows() that
    // shipped plaintext on Windows is caught here (not hidden by a test that hard-codes DpapiCodec).

    @Test fun secretFileIsNotPlaintextOnDisk() {
        val secret = "SUPER-SECRET-TOKEN-7f3a9c"
        Storage(root, defaultSecretCodec()).putSecret("token", secret)
        val file = File(root, "secrets.dat")
        if (isWindows()) {
            val raw = file.readBytes()
            assertFalse(
                raw.toString(Charsets.ISO_8859_1).contains(secret),
                "the DPAPI-protected file must NOT contain the plaintext secret",
            )
            assertEquals(secret, Storage(root, defaultSecretCodec()).getSecret("token"))
        } else {
            val perms = Files.getPosixFilePermissions(file.toPath())
            assertEquals(
                setOf(PosixFilePermission.OWNER_READ, PosixFilePermission.OWNER_WRITE),
                perms,
                "the plaintext secret file must be 0600",
            )
        }
    }

    /** Encrypts as identity but cannot decrypt — stands in for a DPAPI blob written by another user. */
    private object ThrowingDecryptCodec : SecretCodec {
        override fun encrypt(plain: ByteArray): ByteArray = plain
        override fun decrypt(stored: ByteArray): ByteArray = throw IllegalStateException("cannot decrypt (foreign user)")
    }
}
