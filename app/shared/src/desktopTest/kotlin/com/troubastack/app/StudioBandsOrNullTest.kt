package com.troubastack.app

import com.troubastack.shared.seams.PlainCodec
import com.troubastack.shared.seams.SESSION_COOKIE_KEY
import com.troubastack.shared.seams.SESSION_ORIGIN_KEY
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import java.net.InetAddress
import java.net.ServerSocket
import java.nio.file.Files
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * WIN09 — the Windows ⋮ item's refresh reads the bands through [HttpTransport.fetchStudioBandsOrNull]: signed out is an
 * empty list (the item goes), any network or HTTP error is null (the caller keeps the list it had), and a cancelled
 * call stays cancelled. Against a loopback core that answers per path.
 */
class StudioBandsOrNullTest {
    private val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
    private val origin = "http://127.0.0.1:${server.localPort}"
    @Volatile private var answers: Map<String, Pair<Int, String>> = emptyMap()
    @Volatile private var slowMs = 0L

    init {
        Thread {
            while (!server.isClosed) {
                val s = runCatching { server.accept() }.getOrNull() ?: break
                Thread {
                    runCatching {
                        s.use { sock ->
                            val r = sock.getInputStream().bufferedReader()
                            val path = (r.readLine() ?: "").substringAfter(' ').substringBefore(' ')
                            while (!r.readLine().isNullOrEmpty()) { }
                            if (slowMs > 0) Thread.sleep(slowMs)
                            val (code, body) = answers[path] ?: (404 to "{}")
                            val bytes = body.toByteArray()
                            sock.getOutputStream().apply {
                                write("HTTP/1.0 $code X\r\nContent-Type: application/json\r\nContent-Length: ${bytes.size}\r\n\r\n".toByteArray())
                                write(bytes); flush()
                            }
                        }
                    }
                }.apply { isDaemon = true; start() }
            }
        }.apply { isDaemon = true; start() }
    }

    @AfterTest fun close() = server.close()

    private fun transport(signedIn: Boolean): HttpTransport {
        val s = Storage(Files.createTempDirectory("bandsornull").toFile(), PlainCodec)
        s.putSecret("coreUrl", origin)
        if (signedIn) { s.putSecret(SESSION_COOKIE_KEY, "sess=x"); s.putSecret(SESSION_ORIGIN_KEY, origin) }
        return HttpTransport(s, defaultServerUrl = null) { _, _ -> }
    }

    private val twoBands = 200 to """{"bands":[{"id":"a","name":"Alpha"},{"id":"b","name":"Beta"}]}"""

    @Test
    fun theBandsWithTheirRoles() = runBlocking {
        answers = mapOf("/api/bands" to twoBands, "/api/bands/a" to (200 to """{"myRole":"admin"}"""), "/api/bands/b" to (200 to """{"myRole":"member"}"""))
        assertEquals(
            listOf(HttpTransport.StudioBand("a", "Alpha", isAdmin = true), HttpTransport.StudioBand("b", "Beta", isAdmin = false)),
            transport(signedIn = true).fetchStudioBandsOrNull(),
        )
    }

    @Test
    fun signedOut_isAnEmptyList_notNull() = runBlocking {
        answers = mapOf("/api/bands" to twoBands)
        assertEquals(emptyList(), transport(signedIn = false).fetchStudioBandsOrNull(), "no cookie: the item goes")
    }

    @Test
    fun anHttpError_onTheListOrOnARole_isNull() = runBlocking {
        // TEETH: returning emptyList on these (fetchStudioBands' shape) would blank the item on a failed refresh.
        answers = mapOf("/api/bands" to (500 to "{}"))
        assertNull(transport(signedIn = true).fetchStudioBandsOrNull(), "the list failed")
        answers = mapOf("/api/bands" to twoBands, "/api/bands/a" to (200 to """{"myRole":"admin"}"""), "/api/bands/b" to (503 to "{}"))
        assertNull(transport(signedIn = true).fetchStudioBandsOrNull(), "a role failed")
    }

    @Test
    fun anUnreachableServer_isNull() = runBlocking {
        server.close()
        assertNull(transport(signedIn = true).fetchStudioBandsOrNull())
    }

    @Test
    fun aCancelledCall_staysCancelled_neverAnAnswer() = runBlocking {
        // TEETH: catching the CancellationException (runCatching's shape) would complete it with null or an empty list.
        answers = mapOf("/api/bands" to twoBands)
        slowMs = 3_000
        var outcome: Result<List<HttpTransport.StudioBand>?>? = null
        val job = launch { outcome = runCatching { transport(signedIn = true).fetchStudioBandsOrNull() } }
        delay(300)
        job.cancel()
        job.join()
        assertTrue(outcome?.exceptionOrNull() is kotlinx.coroutines.CancellationException, "outcome=$outcome")
    }
}
