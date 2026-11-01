package com.troubastack.app

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.runComposeUiTest
import com.troubastack.shared.seams.PlainCodec
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.runBlocking
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse

/**
 * WIN03 (Fable's ruling) — on the desktop, with no server saved, Connect starts at "http://" and nothing is contacted;
 * Android keeps its starting address.
 */
@OptIn(ExperimentalTestApi::class)
class ConnectDefaultUiTest {
    private fun emptyStorage() = Storage(Files.createTempDirectory("connect-default").toFile(), PlainCodec)

    @Test
    fun noDefault_connectStartsAtTheBareScheme_andTheTransportContactsNothing() = runComposeUiTest {
        // TEETH: the old shared default prefilled (and probed) http://10.0.2.2:8080 on the desktop too.
        val storage = emptyStorage()
        val transport = HttpTransport(storage, defaultServerUrl = null, clearWebCookies = {})
        setContent { ConnectDialog(storage, transport, onClose = {}) }
        onNodeWithText("http://", useUnmergedTree = true).assertExists()
        assertFalse(transport.isConnected)
        assertEquals("", transport.currentOrigin)
        assertEquals(Presence.Unauthorized, runBlocking { transport.probePresence() }, "no session, no server: no request")
    }

    @Test
    fun androidsDefault_isStillWhereConnectStarts() = runComposeUiTest {
        val storage = emptyStorage()
        val transport = HttpTransport(storage, defaultServerUrl = "http://10.0.2.2:8080", clearWebCookies = {})
        setContent { ConnectDialog(storage, transport, onClose = {}) }
        onNodeWithText("http://10.0.2.2:8080", useUnmergedTree = true).assertExists()
    }
}
