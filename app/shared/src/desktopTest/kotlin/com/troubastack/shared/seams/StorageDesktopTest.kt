package com.troubastack.shared.seams

import java.io.File
import java.nio.file.Files
import java.nio.file.attribute.PosixFilePermission
import kotlin.test.AfterTest
import kotlin.test.BeforeTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * WIN02 — the desktop Storage seam: where the data root resolves, the secret round trip, the heal-on-corrupt
 * path (A54), and that the secret file is NOT plaintext at rest. Runs on both JVMs; the DPAPI branch of
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

    @Test fun resolveDataRoot_windowsUsesLocalAppData_elseUserHome() {
        assertEquals(
            File("""C:\Users\u\AppData\Local""", "TroubaStage"),
            resolveDataRoot(null, isWindows = true, localAppData = """C:\Users\u\AppData\Local""", xdgDataHome = null, userHome = """C:\Users\u"""),
        )
        assertEquals(
            File("""C:\Users\u\AppData\Local""", "TroubaStage"),
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
        // A fresh instance reads it back from disk, not from the first instance's in-memory cache.
        assertEquals("cafe-123", Storage(root, PlainCodec).getSecret("token"))
        assertNull(Storage(root, PlainCodec).getSecret("absent"))
        assertFalse(Storage.settingsWereReset, "a healthy store is never reset")
    }

    // ---- heal on corrupt (A54) ----

    @Test fun healsToEmpty_whenTheStoreCannotBeDecrypted() {
        // Seed a real file, then open it through a codec that cannot decrypt it (a restored DPAPI blob on a
        // new user, or a format change). The read heals to empty, raises the reset signal, deletes the file.
        Storage(root, PlainCodec).putSecret("token", "will-be-lost")
        assertTrue(File(root, "secrets.dat").isFile)

        val healed = Storage(root, ThrowingDecryptCodec)
        assertNull(healed.getSecret("token"), "an unreadable store heals to empty, never crashes")
        assertTrue(Storage.settingsWereReset, "Home's 'settings were reset' signal must be raised")
        assertFalse(File(root, "secrets.dat").isFile, "the corrupt file must be removed")

        // And the healed store is writable again.
        healed.putSecret("fresh", "ok")
        assertEquals("ok", Storage(root, PlainCodec).getSecret("fresh"))
    }

    @Test fun missingFile_isSimplyEmpty_notAReset() {
        assertNull(Storage(root, PlainCodec).getSecret("anything"))
        assertFalse(Storage.settingsWereReset)
    }

    // ---- protected at rest: DPAPI (Windows) / 0600 (elsewhere) ----

    @Test fun secretFileIsNotPlaintextOnDisk() {
        val secret = "SUPER-SECRET-TOKEN-7f3a9c"
        if (isWindows()) {
            Storage(root, DpapiCodec).putSecret("token", secret)
            val raw = File(root, "secrets.dat").readBytes()
            assertFalse(
                raw.toString(Charsets.ISO_8859_1).contains(secret),
                "the DPAPI-protected file must NOT contain the plaintext secret",
            )
            // and it still round-trips for this user on this machine
            assertEquals(secret, Storage(root, DpapiCodec).getSecret("token"))
        } else {
            // Elsewhere the file is plaintext, protected by its mode: prove it is 0600 (owner read+write only).
            Storage(root, PlainCodec).putSecret("token", secret)
            val perms = Files.getPosixFilePermissions(File(root, "secrets.dat").toPath())
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
