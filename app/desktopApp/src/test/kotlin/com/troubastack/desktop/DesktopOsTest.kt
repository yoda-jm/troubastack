package com.troubastack.desktop

import androidx.compose.ui.input.key.Key
import com.sun.net.httpserver.HttpServer
import com.troubastack.shared.buildinfo.UPDATE_ASSET_APPIMAGE
import com.troubastack.shared.buildinfo.UPDATE_ASSET_DMG
import com.troubastack.shared.buildinfo.UPDATE_ASSET_MSI
import com.troubastack.shared.buildinfo.UpdateVerdict
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** DESK01 — what changes per desktop OS: the window's keys, the update's asset and mode, the macOS display hold. */
class DesktopOsTest {
    private fun action(os: HostOs, key: Key, ctrl: Boolean = false, alt: Boolean = false, meta: Boolean = false, inConcert: Boolean = false) =
        windowAction(os, key, ctrl, alt, meta, inConcert)

    @Test
    fun windowsAndLinuxKeepWin03sKeys() {
        for (os in listOf(HostOs.WINDOWS, HostOs.LINUX)) {
            assertEquals(WindowAction.TOGGLE_FULL_SCREEN, action(os, Key.F11))
            assertEquals(WindowAction.ESCAPE, action(os, Key.Escape))
            assertEquals(WindowAction.BACK, action(os, Key.DirectionLeft, alt = true))
            assertNull(action(os, Key.DirectionLeft, alt = true, inConcert = true), "never Back out of a concert")
            assertNull(action(os, Key.F, ctrl = true, meta = true))
        }
    }

    @Test
    fun macOsUsesItsOwnKeys() {
        // TEETH (D3): F11 is the system's on a Mac, full screen is Ctrl+Cmd+F, and Back is Cmd+[ / Cmd+Left.
        assertEquals(WindowAction.TOGGLE_FULL_SCREEN, action(HostOs.MACOS, Key.F, ctrl = true, meta = true))
        assertNull(action(HostOs.MACOS, Key.F11))
        assertNull(action(HostOs.MACOS, Key.F, meta = true), "Cmd+F alone is not full screen")
        assertEquals(WindowAction.ESCAPE, action(HostOs.MACOS, Key.Escape))
        assertEquals(WindowAction.BACK, action(HostOs.MACOS, Key.LeftBracket, meta = true))
        assertEquals(WindowAction.BACK, action(HostOs.MACOS, Key.DirectionLeft, meta = true))
        assertNull(action(HostOs.MACOS, Key.DirectionLeft, alt = true), "Alt+Left is not Back on a Mac")
        assertNull(action(HostOs.MACOS, Key.LeftBracket, meta = true, inConcert = true))
    }

    @Test
    fun eachOsUpdatesFromItsOwnAsset() {
        assertEquals(UPDATE_ASSET_MSI, updateAssetFor(HostOs.WINDOWS))
        assertEquals(UPDATE_ASSET_APPIMAGE, updateAssetFor(HostOs.LINUX))
        assertEquals(UPDATE_ASSET_DMG, updateAssetFor(HostOs.MACOS))
    }

    @Test
    fun theLinkMode_opensTheHttpsDownload_saysSo_andRefusesPlainHttp() {
        // TEETH (D4): Linux and macOS must not show WIN04's "install it from the MSI" line, and Update opens the asset.
        val opened = mutableListOf<String>()
        fun linkUpdate(releaseUrl: String) = DesktopAppUpdate(
            Storage(), CoroutineScope(Dispatchers.Unconfined), Files.createTempDirectory("desk01").toFile(), installedExe = null,
            releaseUrl = releaseUrl, assetName = UPDATE_ASSET_APPIMAGE, openInBrowser = { opened += it }, exitForInstall = {},
        )
        withRelease("https://github.com/yoda-jm/troubastack/releases/download/latest/$UPDATE_ASSET_APPIMAGE") { url ->
            val u = linkUpdate(url)
            assertNull(u.installNote)
            assertEquals(OPENS_DOWNLOAD_LINE, u.actionNote)
            assertIs<UpdateVerdict.Available>(runBlocking { u.checkNow(manual = true) })
            u.update()
            assertEquals(listOf("https://github.com/yoda-jm/troubastack/releases/download/latest/$UPDATE_ASSET_APPIMAGE"), opened)
            assertNull(u.message)
        }
        withRelease("http://example.invalid/$UPDATE_ASSET_APPIMAGE") { url ->
            val u = linkUpdate(url)
            runBlocking { u.checkNow(manual = true) }
            u.update()
            assertEquals(1, opened.size, "plain http is never opened")
            assertTrue(u.message.orEmpty().contains("does not download from"), u.message)
        }
        // A release without this OS's asset is no update, not a failure (e.g. an older release predating it).
        withRelease("https://github.com/x/$UPDATE_ASSET_APPIMAGE") { url ->
            val mac = DesktopAppUpdate(
                Storage(), CoroutineScope(Dispatchers.Unconfined), Files.createTempDirectory("desk01").toFile(), installedExe = null,
                releaseUrl = url, assetName = UPDATE_ASSET_DMG, openInBrowser = { opened += it }, exitForInstall = {},
            )
            assertNull(runBlocking { mac.checkNow(manual = true) })
            assertEquals(false, mac.failed)
        }
    }

    /** A loopback `latest` release whose only asset is the AppImage at [assetUrl], from an older-looking commit. */
    private fun withRelease(assetUrl: String, body: (releaseUrl: String) -> Unit) {
        val json = """{"target_commitish":"${"e".repeat(40)}","published_at":"2099-01-01T00:00:00Z","body":"",
            "assets":[{"name":"$UPDATE_ASSET_APPIMAGE","size":10,"browser_download_url":"$assetUrl"}]}"""
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply {
            createContext("/release.json") { ex ->
                val b = json.toByteArray()
                ex.sendResponseHeaders(200, b.size.toLong()); ex.responseBody.use { it.write(b) }
            }
            start()
        }
        try { body("http://127.0.0.1:${server.address.port}/release.json") } finally { server.stop(0) }
    }

    @Test
    fun theWindowIcon_isTheBrandMarkOnTheClasspath() {
        // The window's icon (Main.kt's appIcon) is read from the app's resources; a missing file would fall back to
        // Java's cup silently.
        val png = WindowAction::class.java.getResourceAsStream("/troubastage.png")?.use { javax.imageio.ImageIO.read(it) }
        assertEquals(512 to 512, png?.let { it.width to it.height })
    }

    private class FakeProcess : Process() {
        var destroyed = false
        override fun getOutputStream(): OutputStream = OutputStream.nullOutputStream()
        override fun getInputStream(): InputStream = InputStream.nullInputStream()
        override fun getErrorStream(): InputStream = InputStream.nullInputStream()
        override fun waitFor(): Int = 0
        override fun exitValue(): Int = if (destroyed) 0 else throw IllegalThreadStateException()
        override fun destroy() { destroyed = true }
    }

    @Test
    fun macOsHoldsTheDisplayWithOneCaffeinate_tiedToThisPid_andEndsIt() {
        val started = mutableListOf<Pair<Long, FakeProcess>>()
        val hold = CaffeinateDisplaySleep { pid -> FakeProcess().also { started += pid to it } }
        hold.hold(true)
        hold.hold(true) // still running: not a second one
        assertEquals(1, started.size)
        assertEquals(ProcessHandle.current().pid(), started[0].first, "-w <this pid>: it ends with the app")
        hold.hold(false)
        assertTrue(started[0].second.destroyed)
        hold.hold(false)
        hold.hold(true)
        assertEquals(2, started.size, "a new concert holds again")
        val failing = CaffeinateDisplaySleep { error("no caffeinate") }
        failing.hold(true); failing.hold(false) // never throws
    }
}
