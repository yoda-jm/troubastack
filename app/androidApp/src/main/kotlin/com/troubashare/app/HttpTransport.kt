package com.troubashare.app

import com.troubashare.shared.bundle.AvailableConcert
import com.troubashare.shared.bundle.AvailableConcerts
import com.troubashare.shared.distribution.ManifestTransport
import com.troubashare.shared.distribution.originOf
import com.troubashare.shared.seams.Storage
import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.engine.okhttp.OkHttp
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
// Package-visible: EditScreen seeds this same session into the WebView's CookieManager so a Connect
// login flows into the web editor (one login, not two).
internal const val SESSION_COOKIE_KEY = "sessionCookie"
// The origin the session was issued by. The session cookie is a bare name=value and the server URL
// is user-editable, so we BIND the cookie to its origin and only ever replay it to a matching one —
// otherwise logging into A then pointing at B would leak A's session to B (ktor AND the Edit WebView).
internal const val SESSION_ORIGIN_KEY = "sessionOrigin"

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
 */
internal fun sessionCookieFor(storage: Storage, url: String): String? {
    val cookie = storage.getSecret(SESSION_COOKIE_KEY)?.takeIf { it.isNotEmpty() } ?: return null
    val origin = storage.getSecret(SESSION_ORIGIN_KEY).orEmpty()
    return if (origin.isNotEmpty() && origin == originOf(url)) cookie else null
}

/**
 * Call when the user changes the server URL: if the new origin differs from the session's, drop the
 * stored session AND the WebView's cookie jar so nothing from the old server survives against the new
 * one. Defense-in-depth alongside [sessionCookieFor] (which already refuses a cross-origin replay).
 */
internal fun dropSessionIfOriginChanged(storage: Storage, newUrl: String) {
    if (originOf(newUrl) != storage.getSecret(SESSION_ORIGIN_KEY).orEmpty()) {
        storage.putSecret(SESSION_COOKIE_KEY, "")
        storage.putSecret(SESSION_ORIGIN_KEY, "")
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
        expectSuccess = false
    }
    private val baseUrl: String get() = (storage.getSecret(CORE_URL_KEY) ?: DEFAULT_CORE_URL).trimEnd('/')
    private val concertBand = mutableMapOf<String, String>()  // concertId -> bandId (for download URLs)

    val isConnected: Boolean get() = cookie() != null

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
                        val band = client.get("$baseUrl/api/bands") { header("Cookie", ck) }
                            .body<Bands>().bands.firstOrNull()?.name ?: ""
                        Presence.Online(body.id, body.displayName, band)
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

    /** Clear the persisted session (Storage is overwrite-only, so empty == signed out) — and drop it
     *  from the Edit WebView's shared cookie jar too, so signing out signs out everywhere. */
    fun signOut() {
        storage.putSecret(SESSION_COOKIE_KEY, "")
        storage.putSecret(SESSION_ORIGIN_KEY, "")
        android.webkit.CookieManager.getInstance().removeAllCookies(null)
    }

    private fun cookie(): String? = sessionCookieFor(storage, baseUrl)

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

    override suspend fun downloadBundle(concertId: String, destPath: String) {
        val ck = cookie() ?: throw IllegalStateException("not connected")
        val bandId = concertBand[concertId]
            ?: run { fetchManifest(); concertBand[concertId] }
            ?: throw IllegalStateException("unknown concert $concertId")
        // Stream to disk — never hold the whole .tstage in memory (large bundles).
        client.prepareGet("$baseUrl/api/bands/$bandId/concerts/$concertId/bundle") {
            header("Cookie", ck)
        }.execute { resp ->
            if (!resp.status.isSuccess()) throw IllegalStateException("download failed (${resp.status.value})")
            val channel = resp.bodyAsChannel()
            File(destPath).outputStream().use { out ->
                val buf = ByteArray(64 * 1024)
                while (true) {
                    val n = channel.readAvailable(buf, 0, buf.size)
                    if (n < 0) break
                    if (n > 0) out.write(buf, 0, n)
                }
            }
        }
    }
}
