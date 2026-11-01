package com.troubastack.desktop

import androidx.compose.ui.window.WindowState
import com.troubastack.shared.seams.Storage
import kotlin.test.Test
import kotlin.test.assertNull

/** WIN03 (Fable's ruling) — the desktop has no starting server: Connect starts at "http://", nothing is contacted. */
class DesktopDefaultsTest {
    @Test
    fun theDesktopPlatform_hasNoStartingServer() {
        // The test task points TROUBASTAGE_HOME at build/test-home, so this Storage never reads a real profile.
        assertNull(DesktopAppPlatform(Storage(), WindowState()).defaultServerUrl)
    }
}
