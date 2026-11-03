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
            "android.permission.POST_NOTIFICATIONS",       // A96: the notification that reopens the app (API 33+)
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

    @Test
    fun theReopenReceiver_listensForThisAppBeingReplaced() {
        // A96: without it nothing posts "TroubaStage was updated", and the app stays closed after its update.
        val nodes = manifest().getElementsByTagName("receiver")
        val receiver = (0 until nodes.length).map { nodes.item(it) as org.w3c.dom.Element }
            .single { it.getAttributeNS(android, "name") == ".AppUpdatedReceiver" }
        assertEquals("false", receiver.getAttributeNS(android, "exported"))
        val actions = receiver.getElementsByTagName("action")
        assertEquals(listOf("android.intent.action.MY_PACKAGE_REPLACED"), (0 until actions.length).map { (actions.item(it) as org.w3c.dom.Element).getAttributeNS(android, "name") })
    }

    @Test
    fun androidsSessionStatus_failureAndCancel_areNotInstalled() {
        // A96: a cancel in Android's dialog (STATUS_FAILURE_ABORTED) or any failure puts the status back to Update.
        assertEquals(SessionOutcome.CONFIRM, sessionOutcome(-1))      // STATUS_PENDING_USER_ACTION
        assertEquals(SessionOutcome.INSTALLED, sessionOutcome(0))     // STATUS_SUCCESS
        for (failure in 1..8) assertEquals(SessionOutcome.NOT_INSTALLED, sessionOutcome(failure), "status $failure")
    }
}
