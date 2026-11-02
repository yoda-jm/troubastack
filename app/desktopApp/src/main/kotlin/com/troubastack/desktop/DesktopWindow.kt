package com.troubastack.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.window.WindowPlacement
import androidx.compose.ui.window.WindowState
import com.troubastack.app.WindowControls
import com.troubastack.shared.seams.Storage
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.hostOs
import com.troubastack.shared.stage.DESKTOP_RESERVED_KEYS
import com.troubastack.shared.stage.MACOS_RESERVED_KEYS
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.nio.channels.FileChannel
import java.nio.channels.FileLock
import java.nio.file.StandardOpenOption

/** Parameters' "Full screen when performing"; absent means on (D2's default). */
internal const val FULL_SCREEN_WHEN_PERFORMING_KEY = "window.fullScreenWhenPerforming"

/** The window's remembered placement (D2), as [SavedPlacement.encode] writes it. */
internal const val WINDOW_PLACEMENT_KEY = "window.placement"

/**
 * WIN03 D2 — the window's modes. Full screen is Compose's borderless [WindowPlacement.Fullscreen] on the screen the
 * window is on; leaving it returns to the placement it had before (windowed or maximized).
 */
class DesktopWindowControls(private val state: WindowState, private val storage: Storage) : WindowControls {
    override val fullScreen: Boolean get() = state.placement == WindowPlacement.Fullscreen

    private var beforeFullScreen = WindowPlacement.Floating

    override fun toggleFullScreen() = setFullScreen(!fullScreen)

    private fun setFullScreen(on: Boolean) {
        if (on == fullScreen) return
        if (on) {
            beforeFullScreen = state.placement
            state.placement = WindowPlacement.Fullscreen
        } else {
            state.placement = beforeFullScreen
        }
    }

    private var whenPerforming by mutableStateOf(storage.getSecret(FULL_SCREEN_WHEN_PERFORMING_KEY) != "false")
    override val fullScreenWhenPerforming: Boolean get() = whenPerforming

    override fun chooseFullScreenWhenPerforming(on: Boolean) {
        whenPerforming = on
        storage.putSecret(FULL_SCREEN_WHEN_PERFORMING_KEY, on.toString())
    }

    override val reservedKeys: Set<Key> = if (hostOs == HostOs.MACOS) MACOS_RESERVED_KEYS else DESKTOP_RESERVED_KEYS

    /** Whether the window was full screen when the concert was entered; null outside a concert. */
    private var fullScreenOnEnter: Boolean? = null

    /** Entering a concert goes full screen when Parameters says so; leaving it restores the mode it was entered in. */
    fun enterConcert() {
        fullScreenOnEnter = fullScreen
        if (whenPerforming) setFullScreen(true)
    }

    fun leaveConcert() {
        fullScreenOnEnter?.let { setFullScreen(it) }
        fullScreenOnEnter = null
    }
}

/**
 * WIN03 D2 — one TroubaStage per data root. The first launch holds a lock file in the data root and listens on a
 * loopback port; a second launch finds the lock taken, asks the first to come to the front, and exits. If it cannot
 * reach the first, it says "TroubaStage is already open" and exits all the same: two apps writing one store is the
 * thing to prevent.
 */
class SingleInstance private constructor(private val channel: FileChannel, private val lock: FileLock, private val server: ServerSocket) {
    companion object {
        private const val FRONT = "front"

        /** This launch's claim, or null when another TroubaStage holds the root (it was asked to come forward). */
        fun claim(root: File, onFront: () -> Unit): SingleInstance? {
            root.mkdirs()
            val portFile = File(root, "instance.port")
            val channel = FileChannel.open(File(root, "instance.lock").toPath(), StandardOpenOption.CREATE, StandardOpenOption.WRITE)
            val lock = runCatching { channel.tryLock() }.getOrNull()
            if (lock == null) {
                channel.close()
                val reached = runCatching {
                    Socket(InetAddress.getLoopbackAddress(), portFile.readText().trim().toInt()).use { s ->
                        s.getOutputStream().write("$FRONT\n".toByteArray())
                    }
                }.isSuccess
                if (!reached) alreadyOpenNotice()
                return null
            }
            val server = ServerSocket(0, 4, InetAddress.getLoopbackAddress())
            portFile.writeText(server.localPort.toString())
            Thread({
                while (!server.isClosed) {
                    runCatching {
                        server.accept().use { s ->
                            s.soTimeout = 2_000
                            if (BufferedReader(InputStreamReader(s.getInputStream())).readLine() == FRONT) onFront()
                        }
                    }
                }
            }, "single-instance").apply { isDaemon = true; start() }
            return SingleInstance(channel, lock, server)
        }

        private fun alreadyOpenNotice() =
            javax.swing.JOptionPane.showMessageDialog(null, "TroubaStage is already open.", "TroubaStage", javax.swing.JOptionPane.INFORMATION_MESSAGE)
    }
}
