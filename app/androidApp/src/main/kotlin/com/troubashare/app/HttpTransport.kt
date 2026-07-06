package com.troubashare.app

import com.troubashare.shared.bundle.AvailableConcert
import com.troubashare.shared.bundle.AvailableConcerts
import com.troubashare.shared.distribution.ManifestTransport
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
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File

private const val CORE_URL_KEY = "coreUrl"                       // shared with A06's Edit screen
private const val DEFAULT_CORE_URL = "http://10.0.2.2:8080"     // emulator → host
private const val SESSION_COOKIE_KEY = "sessionCookie"

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
        @Serializable data class Band(val id: String = "")
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
        return null
    }

    /** Clear the persisted session (the Storage seam is add/overwrite-only, so empty == signed out). */
    fun signOut() = storage.putSecret(SESSION_COOKIE_KEY, "")

    private fun cookie(): String? = storage.getSecret(SESSION_COOKIE_KEY)?.takeIf { it.isNotEmpty() }

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
