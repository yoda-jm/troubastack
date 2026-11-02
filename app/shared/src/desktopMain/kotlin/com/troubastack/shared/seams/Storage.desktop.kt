// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import com.sun.jna.platform.win32.Crypt32Util
import kotlinx.serialization.builtins.MapSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import java.io.File
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.nio.file.attribute.PosixFilePermission
import java.util.concurrent.ConcurrentHashMap

/**
 * SEAM 3 `actual` (desktop) — storage (I15). One data root under the OS's per-user data dir; a single
 * secret file protected by DPAPI (current user) on Windows and `0600` elsewhere, healed to empty on
 * corruption (A54's open-or-heal, never a crash). `bundles/`, `tmp/`, `notes/` live under the root.
 *
 * The real app constructs `Storage()`; tests use the internal constructor to inject a throwaway [root]
 * and a [codec] (so the plaintext path and the heal path are driven on any OS).
 *
 * B1 — every Storage over ONE root is ONE store, like Android's EncryptedSharedPreferences (which the
 * framework caches per file). MainActivity opens two instances (theme + storage); a per-instance cache
 * made each blind to the other's writes, so one instance's persist overwrote the file with its stale
 * snapshot and dropped the other's keys. So there is NO in-memory cache: every read loads the file, every
 * write loads-merges-writes, and both happen under a process-global per-root lock, so concurrent
 * `putSecret`s serialize (no lost update, no `ConcurrentModificationException`). The file is tiny.
 */
actual class Storage internal constructor(
    private val root: File,
    private val codec: SecretCodec,
    // F2: the file read is a seam so a test can make it throw — a transient read failure must NOT be treated
    // as corruption (it must not wipe the store). Default is a plain read.
    private val reader: (File) -> ByteArray = { it.readBytes() },
    // F2 binding: the temp write is a seam so a test can make a WRITE fail — putSecret must not throw on the
    // caller's (UI) thread. Default is a plain write.
    private val writeBytes: (java.nio.file.Path, ByteArray) -> Unit = { p, b -> Files.write(p, b) },
) {
    /** The entrypoint (:desktopApp) constructor: root from the OS/env, DPAPI on Windows else plaintext. */
    constructor() : this(defaultDataRoot(), defaultSecretCodec())

    private val secretsFile: File get() = File(root, SECRETS_FILE)

    actual fun bundlesDir(): String = File(root, "bundles").apply { mkdirs() }.path

    actual fun tempDir(): String = File(root, "tmp").apply { mkdirs() }.path

    // Rehearsal notes live BESIDE bundles/, never inside a bundle (a re-import destroys bundles/<id>). (A70)
    actual fun notesDir(): String = File(root, "notes").apply { mkdirs() }.path

    actual fun getSecret(key: String): String? = synchronized(lockFor(root)) {
        when (val r = loadOrHeal()) {
            is Load.Ok -> r.map[key]
            Load.Unavailable -> null // F2: a transient read failure reads as "no value", never a crash
        }
    }

    actual fun putSecret(key: String, value: String) {
        synchronized(lockFor(root)) {
            // F2: if the store could not be READ (a transient failure), skip the write rather than clobber a
            // store we cannot see — and never crash the caller (Stage writes the position on every page turn).
            val current = loadOrHeal()
            if (current is Load.Ok) {
                val m = current.map.toMutableMap()
                m[key] = value
                persist(m)
            }
        }
    }

    /** A54 + F2 (desktop): load the store, distinguishing a TRANSIENT READ failure from CORRUPTION — never throw.
     *  - missing file → an empty store;
     *  - the read fails (IOException: disk busy, a lock, a permission blip) → [Load.Unavailable]; the file is
     *    left ALONE (never deleted) and the caller degrades (get → null, put → skip), so a blip cannot wipe
     *    settings;
     *  - bytes read but cannot be DECRYPTED or PARSED (wrong DPAPI user after a restore, a truncated write, a
     *    format change) → genuine corruption: delete + raise the A54 reset signal, heal to an empty store.
     *  Called only under the per-root lock. */
    private fun loadOrHeal(): Load {
        root.mkdirs()
        val f = secretsFile
        if (!f.isFile) return Load.Ok(emptyMap())
        val bytes = try {
            reader(f)
        } catch (e: IOException) {
            return Load.Unavailable
        }
        return try {
            Load.Ok(Json.decodeFromString(KV, String(codec.decrypt(bytes), StandardCharsets.UTF_8)))
        } catch (e: Exception) {
            settingsWereReset = true
            runCatching { f.delete() }
            Load.Ok(emptyMap())
        }
    }

    /** Called only under the per-root lock. */
    /** Atomic write (temp beside the target, 0600 before any bytes land on a POSIX disk, then rename).
     *  F2 binding: a WRITE failure must NOT throw on the caller's thread — which is the UI thread on a page
     *  turn (Stage writes the position on every turn). On an IOException (full disk, a temp that cannot be
     *  created, an ATOMIC_MOVE onto a file an antivirus/indexer holds without delete-sharing) the on-disk store
     *  is left exactly as it was — only the temp is touched, and it is removed — so the next put simply retries. */
    private fun persist(map: Map<String, String>) {
        root.mkdirs()
        var tmp: java.nio.file.Path? = null
        try {
            val bytes = codec.encrypt(Json.encodeToString(KV, map).toByteArray(StandardCharsets.UTF_8))
            tmp = Files.createTempFile(root.toPath(), "secrets", ".tmp")
            restrictToOwner(tmp.toFile())
            writeBytes(tmp, bytes)
            try {
                Files.move(tmp, secretsFile.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
            } catch (e: java.nio.file.AtomicMoveNotSupportedException) {
                Files.move(tmp, secretsFile.toPath(), StandardCopyOption.REPLACE_EXISTING)
            }
            restrictToOwner(secretsFile)
        } catch (e: IOException) {
            // write failed — leave the store as it was; the next put retries. Never thrown to the caller.
        } finally {
            tmp?.let { runCatching { Files.deleteIfExists(it) } }
        }
    }

    companion object {
        private const val SECRETS_FILE = "secrets.dat"
        private val KV = MapSerializer(String.serializer(), String.serializer())

        /** A54: true (for THIS process) once the secret store had to be reset to recover from an unreadable
         *  file. Home reads it to show the after-the-fact notice; false again next launch once it opens clean. */
        @Volatile
        var settingsWereReset: Boolean = false
            internal set

        /** One monitor per data root (by canonical path), shared across every Storage instance over it — so
         *  two instances behave as one store and concurrent writes serialize (B1). */
        private val LOCKS = ConcurrentHashMap<String, Any>()

        private fun lockFor(root: File): Any {
            val key = runCatching { root.canonicalPath }.getOrDefault(root.absolutePath)
            return LOCKS.computeIfAbsent(key) { Any() }
        }

        /** Owner read+write only, where the filesystem is POSIX. On Windows (no POSIX view) DPAPI is the
         *  protection, so there is nothing to do and the UnsupportedOperationException is expected. */
        private fun restrictToOwner(file: File) {
            runCatching {
                Files.setPosixFilePermissions(
                    file.toPath(),
                    setOf(PosixFilePermission.OWNER_READ, PosixFilePermission.OWNER_WRITE),
                )
            }
        }
    }
}

/** F2 — the outcome of loading the secret store: read + usable, or a TRANSIENT read failure that must not heal. */
private sealed interface Load {
    data class Ok(val map: Map<String, String>) : Load
    data object Unavailable : Load
}

/** The desktop data root as the app resolves it (F2.3 nit — CI reads this instead of hard-coding the folder). */
fun desktopDataRoot(): String = defaultDataRoot().path

/** How the secret file's bytes are protected at rest. Injected so the plaintext and heal paths are testable
 *  on any OS, and so DPAPI is referenced but only INSTANTIATED on Windows. */
internal interface SecretCodec {
    fun encrypt(plain: ByteArray): ByteArray
    fun decrypt(stored: ByteArray): ByteArray
}

/** Elsewhere than Windows: the file is the plaintext JSON, protected by its `0600` mode. */
internal object PlainCodec : SecretCodec {
    override fun encrypt(plain: ByteArray): ByteArray = plain
    override fun decrypt(stored: ByteArray): ByteArray = stored
}

/** Windows: DPAPI, current-user scope (JNA's Crypt32Util). The bytes on disk are ciphertext only this user
 *  on this machine can read; a `cryptUnprotectData` on another user/machine throws → heals to empty. */
internal object DpapiCodec : SecretCodec {
    override fun encrypt(plain: ByteArray): ByteArray = Crypt32Util.cryptProtectData(plain)
    override fun decrypt(stored: ByteArray): ByteArray = Crypt32Util.cryptUnprotectData(stored)
}

internal fun isWindows(): Boolean =
    System.getProperty("os.name").orEmpty().startsWith("Windows", ignoreCase = true)

internal fun defaultSecretCodec(): SecretCodec = if (isWindows()) DpapiCodec else PlainCodec

/**
 * Where TroubaStage keeps its data on desktop. Pure (the environment is passed in) so each branch is
 * unit-tested without touching the real env or OS:
 *  - `TROUBASTAGE_HOME` wins everywhere (tests and agents run isolated);
 *  - Windows: `%LOCALAPPDATA%\TroubaStageData` (fallback `<user.home>\AppData\Local\TroubaStageData`);
 *  - macOS (DESK01): `<user.home>/Library/Application Support/TroubaStage`, outside the `.app` an update replaces;
 *  - Linux: `${XDG_DATA_HOME:-<user.home>/.local/share}/troubastage`.
 *
 * B2 — the Windows folder is `TroubaStageData`, a SIBLING of the per-user MSI install dir
 * (`%LOCALAPPDATA%\TroubaStage`), NOT the same folder: a data root inside the install dir would be wiped
 * when an upgrade replaces it. The `windows` CI job asserts the two are disjoint and that a marker in the
 * data root survives an upgrade.
 */
internal fun resolveDataRoot(
    home: String?,
    os: HostOs,
    localAppData: String?,
    xdgDataHome: String?,
    userHome: String,
): File {
    home?.trim()?.takeIf { it.isNotEmpty() }?.let { return File(it) }
    return when (os) {
        HostOs.WINDOWS -> File(localAppData?.takeIf { it.isNotBlank() } ?: "$userHome\\AppData\\Local", "TroubaStageData")
        HostOs.MACOS -> File("$userHome/Library/Application Support", "TroubaStage")
        HostOs.LINUX -> File(xdgDataHome?.takeIf { it.isNotBlank() } ?: "$userHome/.local/share", "troubastage")
    }
}

private fun defaultDataRoot(): File = resolveDataRoot(
    home = System.getenv("TROUBASTAGE_HOME"),
    os = hostOs,
    localAppData = System.getenv("LOCALAPPDATA"),
    xdgDataHome = System.getenv("XDG_DATA_HOME"),
    userHome = System.getProperty("user.home").orEmpty(),
)
