package com.troubastack.app

import java.io.File
import javax.xml.parsers.DocumentBuilderFactory
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A95 — the in-app update must never crash. Every build from A88 on crashed on Update: the metered check needs
 * ACCESS_NETWORK_STATE, which the manifest did not declare, and the SecurityException was not caught.
 */
class UpdatePathTest {
    private val android = "http://schemas.android.com/apk/res/android"

    private fun manifest() = DocumentBuilderFactory.newInstance().apply { isNamespaceAware = true }
        .newDocumentBuilder().parse(File("src/main/AndroidManifest.xml")).documentElement

    private fun declaredPermissions(): Set<String> {
        val nodes = manifest().getElementsByTagName("uses-permission")
        return (0 until nodes.length).map { (nodes.item(it) as org.w3c.dom.Element).getAttributeNS(android, "name") }.toSet()
    }

    @Test
    fun theManifest_declaresEverythingTheUpdatePathCalls() {
        // TEETH: drop ACCESS_NETWORK_STATE from the manifest and this is red (that was the crash).
        val declared = declaredPermissions()
        for (p in listOf(
            "android.permission.INTERNET",                 // fetch the release, download the APK
            "android.permission.ACCESS_NETWORK_STATE",     // A95: isActiveNetworkMetered
            "android.permission.REQUEST_INSTALL_PACKAGES", // canRequestPackageInstalls, the PackageInstaller session
        )) assertTrue(p in declared, "the manifest declares $p (declared: ${declared.sorted()})")
    }

    @Test
    fun theInstallStatusReceiver_isDeclared() {
        // Without it PackageInstaller's "needs the person's confirmation" never arrives and nothing happens on Update.
        val nodes = manifest().getElementsByTagName("receiver")
        val names = (0 until nodes.length).map { (nodes.item(it) as org.w3c.dom.Element).getAttributeNS(android, "name") }
        assertTrue(".UpdateInstallReceiver" in names, "receivers: $names")
    }

    @Test
    fun theMeteredCheck_neverThrows_andAnUnreadableNetworkCountsAsMetered() {
        // TEETH: without the catch, the SecurityException escapes, as it did on VLL's tablet.
        assertEquals(true, AppUpdater.onMeteredNetwork { throw SecurityException("Neither user 10356 nor current process has android.permission.ACCESS_NETWORK_STATE.") })
        assertEquals(true, AppUpdater.onMeteredNetwork { throw NullPointerException("no connectivity service") })
        assertEquals(true, AppUpdater.onMeteredNetwork { true })
        assertEquals(false, AppUpdater.onMeteredNetwork { false }, "a readable unmetered network is not asked about")
    }
}
