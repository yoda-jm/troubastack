package com.troubastack.app

import com.troubastack.shared.bundle.AvailableConcert
import com.troubastack.shared.bundle.AvailableConcerts
import com.troubastack.shared.distribution.BakeProgress
import com.troubastack.shared.distribution.ManifestTransport
import com.troubastack.shared.distribution.originOf
import com.troubastack.shared.home.bandLabel
import com.troubastack.shared.join.AcceptOutcome
import com.troubastack.shared.join.PreviewResult
import com.troubastack.shared.join.RegisterOutcome
import com.troubastack.shared.join.ServerIdentity
import com.troubastack.shared.join.acceptOutcome
import com.troubastack.shared.join.previewOutcome
import com.troubastack.shared.join.registerOutcome
import com.troubastack.shared.join.serverIdentity
import com.troubastack.shared.seams.SESSION_COOKIE_KEY
import com.troubastack.shared.seams.SESSION_ORIGIN_KEY
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.clearSession
import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.engine.okhttp.OkHttp
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.request.get
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.prepareGet
import io.ktor.client.request.setBody
import io.ktor.client.statement.bodyAsChannel
import io.ktor.http.ContentType
import io.ktor.http.HttpStatusCode
import io.ktor.http.contentType
import io.ktor.http.isSuccess
import io.ktor.serialization.kotlinx.json.json
import io.ktor.utils.io.readAvailable
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File

private const val CORE_URL_KEY = "coreUrl"                       // shared with A06's Edit screen
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080"     // emulator → host
// A41: the last username, remembered so Sign in after a Disconnect needs only a password. Like the
// server address it's not a secret and survives sign-out; but it belongs to a server, so it's cleared
// when the origin changes (below). Package-visible so ConnectScreen seeds + persists it.
internal const val LAST_USERNAME_KEY = "lastUsername"
// A38: SESSION_COOKIE_KEY / SESSION_ORIGIN_KEY + clearSession() moved to shared seams/Session.kt so the
// sign-out storage half is unit-tested (the I12 promise). The cookie is a bare name=value and the
// server URL is user-editable, so it stays BOUND to its issuing origin (SESSION_ORIGIN_KEY) and is only
// ever replayed to a matching one — otherwise logging into A then pointing at B would leak A's session.

/** The logged-in member as the app needs it for P205 identity (id → roster auto-match; name/band → Home). */
data class CurrentIdentity(val userId: String, val displayName: String, val band: String)

/** A31: the outcome of a live server probe — what Home renders its identity line from (never a cached flag). */
sealed interface Presence {
    /** Server answered and the session is valid — carries the member for "Performing as <name> · <band>". */
    data class Online(val userId: String, val displayName: String, val band: String) : Presence

    /** Couldn't reach the server (timeout / connection refused / 5xx) — Home shows "Offline" (reassurance). */
    data object Unreachable : Presence

    /** Server reachable but the session is missing/expired — Home shows the "Connect to your band" invite. */
    data object Unauthorized : Presence
}

/**
 * The persisted session cookie ("name=value") IFF it was issued by [url]'s origin; else null. The
 * single guard for BOTH the ktor transport and the Edit WebView seed — a session is never handed to a
 * server other than the one that issued it. An old install with no stored origin reads as null (⇒ one
 * re-login, which records the origin).
 *
 * A47: takes [getSecret] (usually `storage::getSecret`) rather than the `expect class Storage`, so this
 * cross-origin guard — whose wrong answer silently LEAKS a session to another server — is unit-testable
 * off the device with a fake secret map.
 */
internal fun sessionCookieFor(getSecret: (String) -> String?, url: String): String? {
    val cookie = getSecret(SESSION_COOKIE_KEY)?.takeIf { it.isNotEmpty() } ?: return null
    val origin = getSecret(SESSION_ORIGIN_KEY).orEmpty()
    return if (origin.isNotEmpty() && origin == originOf(url)) cookie else null
}

/**
 * Call when the user changes the server URL: if the new origin differs from the session's, drop the
 * stored session AND the WebView's cookie jar so nothing from the old server survives against the new
 * one. Defense-in-depth alongside [sessionCookieFor] (which already refuses a cross-origin replay).
 */
internal fun dropSessionIfOriginChanged(storage: Storage, newUrl: String) {
    if (originOf(newUrl) != storage.getSecret(SESSION_ORIGIN_KEY).orEmpty()) {
        clearSession(storage::putSecret)              // A38 review: consolidated onto the tested helper
        storage.putSecret(LAST_USERNAME_KEY, "")      // A41: the remembered username belongs to a server
        android.webkit.CookieManager.getInstance().removeAllCookies(null)
    }
}

/**
 * ktor-backed [ManifestTransport] + session login (B03, I13). App DI — the concrete transport
 * `:shared`'s commonMain `UpdatesManager` depends on through the interface; NOT a seam (I15). The
 * session cookie is persisted (encrypted) via the Storage seam and replayed on every request, so a
 * Connect survives relaunch. Offline-first: with no session, [fetchManifest] returns empty (the
 * concerts list just shows local bundles; Stage never needs an account, I12).
 */
class HttpTransport(private val storage: Storage) : ManifestTransport {
    private val json = Json { ignoreUnknownKeys = true }
    private val client = HttpClient(OkHttp) {
        install(ContentNegotiation) { json(json) }
        // A39: the client had NO timeouts, so a download GET that reused a stale/dropped keep-alive
        // connection (or hit a venue-wifi stall) blocked in `readAvailable` FOREVER — the intermittent
        // "Updating…" hang the instrumented repro traced to the download (apply/import complete in
        // ~180 ms once the bytes arrive). Bound every request so a stall becomes a retryable failure,
        // never an infinite spinner. socketTimeout covers a mid-stream stall (the actual hang);
        // connectTimeout a dead host; requestTimeout is a generous overall cap for a large bundle on
        // slow-but-alive wifi. Values are conservative — a real bake bundle is < ~1 MB.
        install(HttpTimeout) {
            connectTimeoutMillis = 15_000
            socketTimeoutMillis = 30_000
            requestTimeoutMillis = 120_000
        }
        expectSuccess = false
    }
    private val baseUrl: String get() = (storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL).trimEnd('/')
    private val concertBand = mutableMapOf<String, String>()  // concertId -> bandId (for download URLs)

    val isConnected: Boolean get() = cookie() != null

    // A65 — the Studio-browse launchers (LAUNCHERS ONLY, per Fable: id/name/date/venue, tap-to-open;
    // no create/rename/delete/search). Bands from GET /api/bands; concerts from GET
    // /api/bands/{id}/setlists (listSetlists → SetlistView; eventDate/venue are omitempty, so absent
    // fields default to ""). The rows carry bandId + setlistId so the tap can deep-link Studio to
    // /bands/{bandId}/setlists/{setlistId}.
    data class StudioBand(val id: String, val name: String, val isAdmin: Boolean = false)
    data class StudioConcert(
        val setlistId: String,
        val bandId: String,
        val bandName: String,
        val name: String,
        val eventDate: String, // ISO yyyy-mm-dd, or "" when the concert has none
        val venue: String,     // or "" when none
    )

    @Serializable private data class SetlistsResp(val setlists: List<SetlistRow> = emptyList())
    @Serializable private data class SetlistRow(
        val id: String = "",
        val name: String = "",
        val eventDate: String = "",
        val venue: String = "",
    )

    /** A65 — the user's bands (id + name + isAdmin), for the Bands tab. /api/bands has no role, so each
     *  band's admin flag is a follow-up GET /api/bands/{id} → myRole (a handful of bands; cheap enough).
     *  isAdmin gates the per-row "Show band QR" affordance. */
    suspend fun fetchStudioBands(): List<StudioBand> {
        val ck = cookie() ?: return emptyList()
        return runCatching {
            client.get("$baseUrl/api/bands") { header("Cookie", ck) }.body<Bands>().bands.map { b ->
                StudioBand(b.id, b.name, isAdmin = isAdminOfBand(b.id))
            }
        }.getOrDefault(emptyList())
    }

    /** A65 — every concert across the user's bands, newest-dated first, undated last (a missing date must
     *  not sort to the top of a date-ordered list — Fable's omitempty caution). */
    suspend fun fetchStudioConcerts(): List<StudioConcert> {
        val ck = cookie() ?: return emptyList()
        return runCatching {
            val bands = client.get("$baseUrl/api/bands") { header("Cookie", ck) }.body<Bands>().bands
            val out = ArrayList<StudioConcert>()
            for (b in bands) {
                val resp = client.get("$baseUrl/api/bands/${b.id}/setlists") { header("Cookie", ck) }
                if (!resp.status.isSuccess()) continue
                resp.body<SetlistsResp>().setlists.forEach {
                    out += StudioConcert(it.id, b.id, b.name, it.name, it.eventDate, it.venue)
                }
            }
            out.sortedWith(
                compareByDescending<StudioConcert> { it.eventDate.isNotEmpty() } // dated before undated
                    .thenByDescending { it.eventDate },                          // newest date first
            )
        }.getOrDefault(emptyList())
    }

    // A65 — invite links for the room QR. The invite LOGIC stays server-side (create/list/revoke via the
    // API); the app only lists+reuses a suitable standing link and draws its QR (Fable's ruling).
    data class InviteLink(
        val id: String,
        val url: String,        // the /join/{token} URL the QR encodes
        val role: String,
        val maxUses: Int,       // 0 = unlimited (a room link wants this)
        val expiresAt: String?, // null = no expiry (a room link wants this)
        val valid: Boolean,
    ) {
        /** A room-facing QR wants a still-valid, multi-use, no-expiry link (T122's opposite default). */
        val roomSuitable: Boolean get() = valid && maxUses == 0 && expiresAt == null
    }

    @Serializable private data class InviteLinksResp(val links: List<InviteLinkJson> = emptyList())
    @Serializable private data class InviteLinkJson(
        val id: String = "",
        val url: String = "",
        val role: String = "",
        val maxUses: Int = 0,
        val expiresAt: String? = null,
        val valid: Boolean = false,
    )

    private fun InviteLinkJson.toModel() = InviteLink(id, url, role, maxUses, expiresAt, valid)

    /** A65 — list a band's invite links (admin). LIST-FIRST so the QR view can REUSE one, per Fable. */
    suspend fun fetchInviteLinks(bandId: String): List<InviteLink> {
        val ck = cookie() ?: return emptyList()
        return runCatching {
            client.get("$baseUrl/api/bands/$bandId/invite-links") { header("Cookie", ck) }
                .body<InviteLinksResp>().links.map { it.toModel() }
        }.getOrDefault(emptyList())
    }

    @Serializable private data class CreateInviteReq(val role: String, val expiresInHours: Int, val maxUses: Int)

    /** A65 — mint a STANDING room link (multi-use, no expiry) — ONLY on an explicit user action, never as
     *  a side effect (Fable: else standing links pile up unaudited). Returns null on failure. */
    suspend fun createStandingInviteLink(bandId: String, role: String = "member"): InviteLink? {
        val ck = cookie() ?: return null
        return runCatching {
            val resp = client.post("$baseUrl/api/bands/$bandId/invite-links") {
                header("Cookie", ck)
                contentType(ContentType.Application.Json)
                setBody(CreateInviteReq(role = role, expiresInHours = 0, maxUses = 0))
            }
            if (!resp.status.isSuccess()) null else resp.body<InviteLinkJson>().toModel()
        }.getOrNull()
    }

    /** A65 — is the signed-in user an ADMIN of [bandId] (band id directly)? Gates "Show band QR".
     *  (Distinct from [isBandAdmin], which takes a CONCERT id and resolves the band via bandIdFor.) */
    suspend fun isAdminOfBand(bandId: String): Boolean {
        val ck = cookie() ?: return false
        return runCatching {
            val resp = client.get("$baseUrl/api/bands/$bandId") { header("Cookie", ck) }
            resp.status.isSuccess() && resp.body<BandDetail>().myRole == "admin"
        }.getOrDefault(false)
    }

    @Serializable private data class LoginReq(val username: String, val password: String)
    @Serializable private data class Bands(val bands: List<Band> = emptyList()) {
        @Serializable data class Band(val id: String = "", val name: String = "")
    }
    // /api/me returns the member WRAPPED: {"user":{"id":…,"displayName":…}}. Parsing the top level
    // (the old bug) left id + displayName empty — so Home read "Connected" not "Performing as <name>",
    // and P205 auto-match never fired (empty userId ⇒ the "Who are you?" picker always appeared). A31.
    @Serializable private data class MeResp(val user: Me = Me())
    @Serializable private data class Me(val id: String = "", val displayName: String = "")

    /** A31: LIVE connection state for the Home landing — resolved from an actual round-trip, never the
     *  cached cookie. [Presence.Unreachable] on timeout/IO (server down → Home reads "Offline");
     *  [Presence.Unauthorized] when the server answers but the session is gone/expired (→ "Connect");
     *  [Presence.Online] carries the name/band for "Performing as …". Short [timeoutMs] so a dead
     *  server can't hang Home on the "Checking…" state. */
    suspend fun probePresence(timeoutMs: Long = 3000): Presence {
        val ck = cookie() ?: return Presence.Unauthorized
        return runCatching {
            withTimeout(timeoutMs) {
                val me = client.get("$baseUrl/api/me") { header("Cookie", ck) }
                when {
                    me.status.isSuccess() -> {
                        val body = me.body<MeResp>().user
                        // A38 multi-band ruling: carry a LABEL, not an arbitrary firstOrNull() — the
                        // count is free (we already fetch the whole list). bandLabel: ""/name/"N bands".
                        val names = client.get("$baseUrl/api/bands") { header("Cookie", ck) }
                            .body<Bands>().bands.map { it.name }.filter { it.isNotEmpty() }
                        Presence.Online(body.id, body.displayName, bandLabel(names))
                    }
                    me.status == HttpStatusCode.Unauthorized -> Presence.Unauthorized
                    else -> Presence.Unreachable // 5xx / unexpected — treat as not-usable
                }
            }
        }.getOrDefault(Presence.Unreachable) // timeout / connection refused / DNS
    }

    /** Log in against the persisted server URL; on success store the session cookie. Returns a
     *  human-readable error message, or null on success. */
    suspend fun connect(username: String, password: String): String? {
        val resp = client.post("$baseUrl/api/auth/login") {
            contentType(ContentType.Application.Json)
            setBody(LoginReq(username, password))
        }
        if (!resp.status.isSuccess()) {
            return if (resp.status == HttpStatusCode.Unauthorized) "Wrong username or password"
            else "Couldn't sign in (${resp.status.value})"
        }
        val setCookie = resp.headers.getAll("Set-Cookie")?.firstOrNull()
            ?: return "The server didn't return a session"
        storage.putSecret(SESSION_COOKIE_KEY, setCookie.substringBefore(';')) // name=value only
        storage.putSecret(SESSION_ORIGIN_KEY, originOf(baseUrl))              // bind it to this server
        return null
    }

    // A52 — join-from-a-link. Both routes are `a.auth(...)`-wrapped, so a session for THIS server is
    // required; the flow arranges baseUrl == the link's origin (a Redeem is already there; a SignIn signs
    // in here; a ConfirmServer switchServer()s here first) before calling either. The token is a bearer
    // credential passed as an argument — NEVER stored, NEVER logged (no logging plugin is installed, and
    // we log the host not the token anywhere else).
    @Serializable private data class InviteResp(
        val band: BandRef = BandRef(), val role: String = "", val valid: Boolean = false,
        val reason: String = "", val error: String = "",
    ) { @Serializable data class BandRef(val id: String = "", val name: String = "") }

    @Serializable private data class RegisterReq(val username: String, val displayName: String, val password: String)

    /** A57: create an account (`POST /api/auth/register`) so an invited newcomer can join. The route is
     *  unauthenticated and open already, so this widens NO capability — it makes the supported path
     *  reachable from the app. On [RegisterOutcome.Created] the caller signs in and continues the join;
     *  409 ⇒ [RegisterOutcome.NameTaken] (recoverable). Never sends/needs a session. */
    suspend fun register(username: String, displayName: String, password: String): RegisterOutcome =
        runCatching {
            val resp = client.post("$baseUrl/api/auth/register") {
                contentType(ContentType.Application.Json)
                setBody(RegisterReq(username.trim(), displayName.trim(), password))
            }
            registerOutcome(resp.status.value)
        }.getOrElse { RegisterOutcome.Failed(0) }

    /** Preview `GET /api/invite-links/{token}` before committing → band + role, or the server's reason for
     *  an unusable link. No local cookie ⇒ [PreviewResult.NeedsSignIn] without a round-trip; a network
     *  failure ⇒ [PreviewResult.Failed] with status 0 (the sheet says "couldn't reach the server"). */
    suspend fun previewInvite(token: String): PreviewResult {
        val ck = cookie() ?: return PreviewResult.NeedsSignIn
        return runCatching {
            val resp = client.get("$baseUrl/api/invite-links/$token") { header("Cookie", ck) }
            val body = if (resp.status == HttpStatusCode.OK) resp.body<InviteResp>() else null
            previewOutcome(resp.status.value, body?.band?.name, body?.role, body?.valid ?: false, body?.reason)
        }.getOrElse { PreviewResult.Failed(0) }
    }

    /** Accept `POST /api/invite-links/{token}/accept` → membership. 410 Gone carries the server's reason
     *  (expired/revoked/exhausted); a network failure ⇒ [AcceptOutcome.Failed] with status 0. */
    suspend fun acceptInvite(token: String): AcceptOutcome {
        val ck = cookie() ?: return AcceptOutcome.NeedsSignIn
        return runCatching {
            val resp = client.post("$baseUrl/api/invite-links/$token/accept") { header("Cookie", ck) }
            val band = if (resp.status.isSuccess()) runCatching { resp.body<InviteResp>().band.name }.getOrNull() else null
            val reason = if (resp.status == HttpStatusCode.Gone) runCatching { resp.body<InviteResp>().error }.getOrNull() else null
            acceptOutcome(resp.status.value, band, reason)
        }.getOrElse { AcceptOutcome.Failed(0) }
    }

    @Serializable private data class VersionResp(val product: String = "", val apiVersion: Int = 0)

    /** A52/T123: probe [url]'s `GET /api/version` (UNAUTHENTICATED, so it runs BEFORE any password field)
     *  and classify the host. The ConfirmServer path refuses to show a password unless this is
     *  [ServerIdentity.TroubaStack] — the real protection behind A53's scanner. A non-200/network failure
     *  ⇒ [ServerIdentity.Unreachable] (refuse, don't guess). Short timeout so a dead host can't hang the sheet. */
    suspend fun probeServerIdentity(url: String): ServerIdentity {
        val base = url.trim().trimEnd('/')
        return runCatching {
            withTimeout(8_000) {
                val resp = client.get("$base/api/version")
                val body = if (resp.status.isSuccess()) runCatching { resp.body<VersionResp>() }.getOrNull() else null
                serverIdentity(resp.status.value, body?.product, body?.apiVersion?.takeIf { it > 0 })
            }
        }.getOrElse { ServerIdentity.Unreachable }
    }

    /** A52: point the app at [url] for a join that named a different server, dropping any session bound to
     *  the OLD origin (defense-in-depth with [sessionCookieFor]). The ConfirmServer path calls this after
     *  the person confirms the host — so a subsequent sign-in + accept run against the chosen server. */
    fun switchServer(url: String) {
        dropSessionIfOriginChanged(storage, url.trim())
        storage.putSecret(CORE_URL_KEY, url.trim())
    }

    /** The origin the app currently points at (for A51's `joinDecision`). */
    val currentOrigin: String get() = baseUrl

    /** Clear the persisted session (Storage is overwrite-only, so empty == signed out) — and drop it
     *  from the Edit WebView's shared cookie jar too, so signing out signs out everywhere. */
    fun signOut() {
        clearSession(storage::putSecret) // the tested storage half — keeps CORE_URL_KEY, touches no files
        android.webkit.CookieManager.getInstance().removeAllCookies(null)
    }

    private fun cookie(): String? = sessionCookieFor(storage::getSecret, baseUrl)

    override suspend fun fetchManifest(): AvailableConcerts {
        val ck = cookie() ?: return AvailableConcerts()
        val bands = client.get("$baseUrl/api/bands") { header("Cookie", ck) }.body<Bands>()
        val all = ArrayList<AvailableConcert>()
        for (b in bands.bands) {
            val resp = client.get("$baseUrl/api/bands/${b.id}/concerts") { header("Cookie", ck) }
            if (!resp.status.isSuccess()) continue
            resp.body<AvailableConcerts>().concerts.forEach {
                concertBand[it.concertId] = b.id
                all += it
            }
        }
        return AvailableConcerts(all)
    }

    override suspend fun downloadBundle(
        concertId: String,
        destPath: String,
        onBytes: (bytesRead: Long, contentLength: Long) -> Unit,
    ) {
        val ck = cookie() ?: throw IllegalStateException("not connected")
        val bandId = concertBand[concertId]
            ?: run { fetchManifest(); concertBand[concertId] }
            ?: throw IllegalStateException("unknown concert $concertId")
        // Stream to disk — never hold the whole .tstage in memory (large bundles).
        client.prepareGet("$baseUrl/api/bands/$bandId/concerts/$concertId/bundle") {
            header("Cookie", ck)
        }.execute { resp ->
            if (!resp.status.isSuccess()) throw IllegalStateException("download failed (${resp.status.value})")
            // A42 ①: Content-Length for a determinate bar; ≤0 (absent/unparseable/chunked) ⇒ the UI
            // stays indeterminate — never fabricate a fraction.
            val total = resp.headers["Content-Length"]?.toLongOrNull() ?: -1L
            val channel = resp.bodyAsChannel()
            var read = 0L
            onBytes(0L, total)
            File(destPath).outputStream().use { out ->
                val buf = ByteArray(64 * 1024)
                while (true) {
                    val n = channel.readAvailable(buf, 0, buf.size)
                    if (n < 0) break
                    if (n > 0) {
                        out.write(buf, 0, n)
                        read += n
                        onBytes(read, total)
                    }
                }
            }
        }
    }

    @Serializable private data class BandDetail(val myRole: String = "")

    /** The bandId that owns [concertId], populating the cache via [fetchManifest] once if needed. */
    private suspend fun bandIdFor(concertId: String): String? =
        concertBand[concertId] ?: run { runCatching { fetchManifest() }; concertBand[concertId] }

    /** A42②: does the signed-in user ADMIN the band that owns [concertId]? Gates the Home Re-bake
     *  affordance (the server also 403s a non-admin — this just hides a control that would only fail). */
    suspend fun isBandAdmin(concertId: String): Boolean {
        val ck = cookie() ?: return false
        val bandId = bandIdFor(concertId) ?: return false
        return runCatching {
            val resp = client.get("$baseUrl/api/bands/$bandId") { header("Cookie", ck) }
            resp.status.isSuccess() && resp.body<BandDetail>().myRole == "admin"
        }.getOrElse { false }
    }

    /** A42② / T103: KICK a re-bake of [concertId]'s setlist, sending [bakeId] so the caller can poll
     *  progress. The POST returns promptly (202 Accepted) — the bake runs on the SERVER's context, so a
     *  dropped client no longer cancels it. Returns null on a successful kick, else a human message. The
     *  OUTCOME is NOT here — the caller polls [bakeProgress] to a terminal state (the source of truth). */
    suspend fun reBake(concertId: String, bakeId: String): String? {
        val ck = cookie() ?: return "You're not connected"
        val bandId = bandIdFor(concertId) ?: return "Unknown concert"
        val resp = client.post("$baseUrl/api/bands/$bandId/setlists/$concertId/bake") {
            header("Cookie", ck)
            header("X-Trouba-Bake-Id", bakeId)
        }
        return when {
            resp.status.isSuccess() -> null // 202 Accepted — kicked
            resp.status == HttpStatusCode.Forbidden -> "Only a band admin can bake"
            else -> "Couldn't start the bake (${resp.status.value})"
        }
    }

    /** A42② / T99: poll a running bake's progress by [bakeId]; null on 404/expired/old-server/any failure
     *  so the caller degrades to a plain "Baking…" (T99 §4) — the bake still completes server-side. */
    suspend fun bakeProgress(concertId: String, bakeId: String): BakeProgress? {
        val ck = cookie() ?: return null
        val bandId = bandIdFor(concertId) ?: return null
        return runCatching {
            val resp = client.get("$baseUrl/api/bands/$bandId/setlists/$concertId/bakes/$bakeId/progress") {
                header("Cookie", ck)
            }
            if (resp.status.isSuccess()) resp.body<BakeProgress>() else null
        }.getOrNull()
    }
}
