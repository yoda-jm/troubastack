package com.troubastack.app

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import com.troubastack.shared.seams.PlainCodec
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.ui.assertGlyphsNotRead
import com.troubastack.shared.ui.assertNamedClickable
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * VLL ("« Close » pour les deux") — the ✕ of "Join a band" and of "Connect to your band" is one clickable named
 * "Close", and the ✕ itself is not read after the name. Connect's ✕ does what its Cancel does, but it is named Close.
 */
@OptIn(ExperimentalTestApi::class)
class DialogCloseNamesUiTest {
    private fun emptyStorage() = Storage(Files.createTempDirectory("close-names").toFile(), PlainCodec)
    private val close = hasContentDescription("Close") and hasClickAction()

    @Test
    fun connect_theCrossIsNamedClose_notCancel() = runComposeUiTest {
        val storage = emptyStorage()
        val transport = HttpTransport(storage, defaultServerUrl = null, clearWebCookies = { _, _ -> })
        var closed = 0
        setContent { ConnectDialog(storage, transport, onClose = { closed++ }) }
        assertNamedClickable("Close")
        assertGlyphsNotRead("✕")
        onAllNodes(hasContentDescription("Cancel"), useUnmergedTree = true).assertCountEquals(0)
        onNode(close, useUnmergedTree = true).performClick()
        assertEquals(1, closed, "the named ✕ closes Connect")
    }

    @Test
    fun join_theCrossIsNamedClose_onTheFirstStep_andBesideTheBlockedStepsCloseButton() = runComposeUiTest {
        val storage = emptyStorage()
        val transport = HttpTransport(storage, defaultServerUrl = null, clearWebCookies = { _, _ -> })
        var link by mutableStateOf("https://band.example/join/tok_123") // a first-ever server: Confirm, nothing contacted
        var closed = 0
        setContent { JoinDialog(link, storage, transport, onClose = { closed++ }, onJoined = {}) }
        onNodeWithText("This invite points at a server you haven't used before.", useUnmergedTree = true).assertExists()
        assertNamedClickable("Close")
        assertGlyphsNotRead("✕")

        link = "not an invite" // Blocked: the step's own "Close" button is a word, not a second name
        waitForIdle()
        onNodeWithText("Close", useUnmergedTree = true).assertExists()
        assertNamedClickable("Close")
        assertGlyphsNotRead("✕")
        onNode(close, useUnmergedTree = true).performClick()
        assertEquals(1, closed, "the named ✕ closes Join")
    }
}
