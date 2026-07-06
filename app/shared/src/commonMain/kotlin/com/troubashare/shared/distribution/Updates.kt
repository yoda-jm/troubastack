// Generated proto types (AvailableConcerts, AvailableConcert, ConcertBundle, …) come from
// gen/ — single source of truth is proto/ (I1). Referenced here, never redefined.
package com.troubashare.shared.distribution

import com.troubashare.shared.bundle.AvailableConcert
import com.troubashare.shared.bundle.AvailableConcerts
import com.troubashare.shared.bundle.ImportResult
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * Distribution — SHARED Kotlin (commonMain). The downloader + revision/availability logic.
 * NOT a native seam (I15): the only platform touch is via [ManifestTransport] (ktor, an androidMain
 * dependency — not a seam) and via the injected persistence/import callbacks the app wires to the
 * Storage seam (3) + A05's BundleImporter. All the policy/diff logic lives here and is unit-tested
 * off-device with fakes.
 *
 * Responsibilities (docs/design/05-distribution.md, I13):
 *  - run a cheap metadata-only manifest diff → the "what's available to me" set,
 *  - apply updates EXPLICITLY (user action) — never automatically, never mid-performance,
 *  - ATOMIC SWAP on re-download: fetch to temp, then hand to BundleImporter (which keeps the old
 *    revision until the new one verifies — a half-download must never corrupt a working concert),
 *  - honor freeze/lock at the device tier (local pin + admin band-wide lock / server final_locked).
 */

/** Per-concert update policy (I13). Default is Prompt. `AUTO` is inert in B03 (P201 wires it). */
enum class UpdatePolicy { PROMPT, FROZEN, AUTO }

/** Two kinds of freeze (I13). */
sealed interface Freeze {
    /** Performer freezes their own copy; no chips; explicit unfreeze. */
    data class LocalPin(val atRev: ULong) : Freeze
    /** Leader marks a revision `final` band-wide; no new baked revisions emitted. */
    data class AdminLock(val atRev: ULong) : Freeze
}

/** Result of diffing local downloaded revisions against the server manifest. */
sealed interface Availability {
    /** Downloaded and server rev > local → "New version of Concert A available". */
    data class UpdateOffered(val concertId: String, val localRev: ULong, val serverRev: ULong) : Availability
    /** Per-song content change → "Song A changed — apply?" (granular re-bake + swap). B03 keeps the
     *  type but never emits it (out of scope — a follow-up when someone wants per-song apply). */
    data class SongChanged(val concertId: String, val songId: String, val serverRev: ULong) : Availability
    /** Not downloaded but shared to the band → "Concert B is available for your band". */
    data class NewlyAvailable(val concertId: String) : Availability
}

/**
 * The network surface the downloader needs. The real impl is ktor-client in androidMain (I15: a
 * *dependency*, not a new seam); commonTest supplies a fake. Both calls throw on failure — a failed
 * apply must leave installed bundles untouched (BundleImporter guarantees the swap side).
 */
interface ManifestTransport {
    /** Cheap metadata-only manifest call (a few KB); the server's `GET …/concerts` shape (I1). */
    suspend fun fetchManifest(): AvailableConcerts

    /** Stream a concert's `.tstage` to [destPath] (no full-file-in-memory). Throws on network/IO error. */
    suspend fun downloadBundle(concertId: String, destPath: String)
}

/**
 * Persisted per-concert policy + freeze, as one small JSON blob via the Storage seam KV (I13). Kept
 * out of `stage/` (I12). AdminLock is server-driven (`final_locked`), so only PROMPT/FROZEN/AUTO +
 * an optional LocalPin are stored per concert here.
 */
@Serializable
private data class PolicyRecord(val policy: UpdatePolicy = UpdatePolicy.PROMPT, val pinnedRev: ULong? = null)

@Serializable
private data class PolicyBook(val byConcert: Map<String, PolicyRecord> = emptyMap())

/**
 * Downloader / update manager (I13). Pure shared logic; every platform touch is an injected lambda
 * so this is exhaustively unit-testable:
 *  - [transport]      network (ktor in prod, fake in tests),
 *  - [tempDir]        scratch dir for pending downloads (Storage.tempDir()),
 *  - [installedRevs]  concertId → installed concertRev, scanned from bundlesDir via the A02 loader,
 *  - [importBundle]   hands a downloaded `.tstage` to A05's BundleImporter (atomic swap),
 *  - [readPolicies]/[writePolicies]  the Storage KV round-trip for [PolicyBook] JSON.
 */
class UpdatesManager(
    private val transport: ManifestTransport,
    private val tempDir: () -> String,
    private val installedRevs: () -> Map<String, ULong>,
    private val importBundle: (zipPath: String) -> ImportResult,
    private val readPolicies: () -> String?,
    private val writePolicies: (String) -> Unit,
) {
    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    suspend fun fetchManifest(): AvailableConcerts = transport.fetchManifest()

    /**
     * Diff [manifest] against installed revisions → the chips to surface (I13). Offers are suppressed
     * for a FROZEN policy, a LocalPin, or server `finalLocked`. A concert not on disk is always
     * NewlyAvailable (nothing to freeze yet). `SongChanged` is intentionally never emitted (B03).
     */
    fun diff(manifest: AvailableConcerts): List<Availability> {
        val installed = installedRevs()
        val book = loadBook()
        val out = ArrayList<Availability>()
        for (c in manifest.concerts) {
            val local = installed[c.concertId]
            if (local == null) {
                out += Availability.NewlyAvailable(c.concertId)   // can't be frozen — not downloaded
                continue
            }
            if (c.currentRev > local && !offerSuppressed(c, book[c.concertId])) {
                out += Availability.UpdateOffered(c.concertId, local, c.currentRev)
            }
        }
        return out
    }

    private fun offerSuppressed(c: AvailableConcert, rec: PolicyRecord?): Boolean =
        c.finalLocked || rec?.policy == UpdatePolicy.FROZEN || rec?.pinnedRev != null

    /**
     * Apply an offered/available update — EXPLICIT only (the caller gates on not-in-performance, I13).
     * Downloads to a fixed per-concert path under [tempDir] (overwritten each attempt), then hands it
     * to [importBundle] (A05's atomic swap). On any failure returns [ImportResult.Failed] and the
     * installed bundle is left untouched. `SongChanged` is unsupported in B03.
     */
    suspend fun apply(offer: Availability): ImportResult {
        val concertId = when (offer) {
            is Availability.UpdateOffered -> offer.concertId
            is Availability.NewlyAvailable -> offer.concertId
            is Availability.SongChanged -> return ImportResult.Failed("per-song apply isn't supported yet")
        }
        val dest = "${tempDir().trimEnd('/')}/$concertId.tstage"
        return try {
            transport.downloadBundle(concertId, dest)
            importBundle(dest)
        } catch (e: Exception) {
            ImportResult.Failed("couldn't download the concert (${e.message ?: "network error"})")
        }
    }

    /** Set the per-concert update policy (PROMPT/FROZEN/AUTO). AUTO is inert in B03. */
    fun setPolicy(concertId: String, policy: UpdatePolicy) = mutate(concertId) { it.copy(policy = policy) }

    /**
     * Pin/unpin the local copy (I13). A [Freeze.LocalPin] records the pinned rev; [Freeze.AdminLock]
     * is server-driven (`final_locked`) and not stored here; `null` clears the local pin.
     */
    fun setFreeze(concertId: String, freeze: Freeze?) {
        when (freeze) {
            is Freeze.LocalPin -> mutate(concertId) { it.copy(pinnedRev = freeze.atRev) }
            null -> mutate(concertId) { it.copy(pinnedRev = null) }
            is Freeze.AdminLock -> Unit // band-wide lock rides the manifest's finalLocked, not local state
        }
    }

    // ---- policy persistence (Storage KV round-trip) ----

    private fun loadBook(): Map<String, PolicyRecord> =
        readPolicies()?.let { runCatching { json.decodeFromString<PolicyBook>(it).byConcert }.getOrNull() } ?: emptyMap()

    private fun mutate(concertId: String, f: (PolicyRecord) -> PolicyRecord) {
        val book = loadBook().toMutableMap()
        book[concertId] = f(book[concertId] ?: PolicyRecord())
        writePolicies(json.encodeToString(PolicyBook(book)))
    }
}
