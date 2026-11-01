// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.seams

import com.sun.jna.platform.win32.Crypt32Util
import kotlinx.serialization.builtins.MapSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json
import java.io.File
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.nio.file.attribute.PosixFilePermission

/**
 * SEAM 3 `actual` (desktop) — storage (I15). One data root under the OS's per-user data dir; a single
 * secret file protected by DPAPI (current user) on Windows and `0600` elsewhere, healed to empty on
 * corruption (A54's open-or-heal, never a crash). `bundles/`, `tmp/`, `notes/` live under the root.
 *
 * The real app constructs `Storage()`; tests use the internal constructor to inject a throwaway [root]
 * and a [codec] (so the plaintext path and the heal path are driven on any OS).
 */
actual class Storage internal constructor(
    private val root: File,
    private val codec: SecretCodec,
) {
    /** The entrypoint (:desktopApp) constructor: root from the OS/env, DPAPI on Windows else plaintext. */
    constructor() : this(defaultDataRoot(), defaultSecretCodec())

    private val secretsFile: File get() = File(root, SECRETS_FILE)

    actual fun bundlesDir(): String = File(root, "bundles").apply { mkdirs() }.path

    actual fun tempDir(): String = File(root, "tmp").apply { mkdirs() }.path

    // Rehearsal notes live BESIDE bundles/, never inside a bundle (a re-import destroys bundles/<id>). (A70)
    actual fun notesDir(): String = File(root, "notes").apply { mkdirs() }.path

    // Read from the in-memory cache, loaded once (healed if the file is unreadable). Every preference goes
    // through this store and the Stage position is written on every page turn, so reads are a map lookup
    // and a write rewrites one small file.
    private val cache: MutableMap<String, String> by lazy { loadOrHeal().toMutableMap() }

    actual fun getSecret(key: String): String? = cache[key]

    actual fun putSecret(key: String, value: String) {
        cache[key] = value
        persist(cache)
    }

    /** A54 (desktop): read + decrypt + parse the store, or heal to empty — never throw. A missing file is
     *  simply empty. A file that cannot be decrypted or parsed (wrong DPAPI user after a restore, a truncated
     *  write, a format change) is deleted and the process-global reset signal is raised, so Home can show the
     *  "settings were reset" notice; what is lost is a session + settings, never a concert (those are files). */
    private fun loadOrHeal(): Map<String, String> {
        root.mkdirs()
        val f = secretsFile
        if (!f.isFile) return emptyMap()
        return try {
            val plain = codec.decrypt(f.readBytes())
            Json.decodeFromString(KV, String(plain, StandardCharsets.UTF_8))
        } catch (e: Exception) {
            settingsWereReset = true
            runCatching { f.delete() }
            emptyMap()
        }
    }

    private fun persist(map: Map<String, String>) {
        root.mkdirs()
        val bytes = codec.encrypt(Json.encodeToString(KV, map).toByteArray(StandardCharsets.UTF_8))
        // Atomic write: a temp beside the target, 0600 before any bytes land on a POSIX disk, then rename.
        val tmp = Files.createTempFile(root.toPath(), "secrets", ".tmp")
        try {
            restrictToOwner(tmp.toFile())
            Files.write(tmp, bytes)
            try {
                Files.move(tmp, secretsFile.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
            } catch (e: java.nio.file.AtomicMoveNotSupportedException) {
                Files.move(tmp, secretsFile.toPath(), StandardCopyOption.REPLACE_EXISTING)
            }
            restrictToOwner(secretsFile)
        } finally {
            Files.deleteIfExists(tmp)
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
 *  - Windows: `%LOCALAPPDATA%\TroubaStage` (fallback `<user.home>\AppData\Local\TroubaStage`);
 *  - elsewhere: `${XDG_DATA_HOME:-<user.home>/.local/share}/troubastage`.
 */
internal fun resolveDataRoot(
    home: String?,
    isWindows: Boolean,
    localAppData: String?,
    xdgDataHome: String?,
    userHome: String,
): File {
    home?.trim()?.takeIf { it.isNotEmpty() }?.let { return File(it) }
    return if (isWindows) {
        val base = localAppData?.takeIf { it.isNotBlank() } ?: "$userHome\\AppData\\Local"
        File(base, "TroubaStage")
    } else {
        val base = xdgDataHome?.takeIf { it.isNotBlank() } ?: "$userHome/.local/share"
        File(base, "troubastage")
    }
}

private fun defaultDataRoot(): File = resolveDataRoot(
    home = System.getenv("TROUBASTAGE_HOME"),
    isWindows = isWindows(),
    localAppData = System.getenv("LOCALAPPDATA"),
    xdgDataHome = System.getenv("XDG_DATA_HOME"),
    userHome = System.getProperty("user.home").orEmpty(),
)
