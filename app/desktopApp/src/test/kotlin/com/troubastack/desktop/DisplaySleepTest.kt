package com.troubastack.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/** DESK01 D2 — the Linux display hold: which service it asks, what it releases, and that no bus is a no-op. */
class DisplaySleepTest {
    private class FakeBus(private val answers: Map<String, Int?>) : SessionBus {
        val calls = mutableListOf<String>()
        override fun callForCookie(dest: String, path: String, iface: String, method: String, args: List<Any>): Int? {
            calls += "$iface.$method$args"
            return answers[iface]
        }
        override fun call(dest: String, path: String, iface: String, method: String, args: List<Any>) {
            calls += "$iface.$method$args"
        }
    }

    @Test
    fun theFreedesktopServiceIsAskedFirst_andReleasedWithItsCookie() {
        // TEETH: a release with the wrong method or cookie leaves the screensaver inhibited after the concert.
        val bus = FakeBus(mapOf("org.freedesktop.ScreenSaver" to 42))
        val hold = FreedesktopDisplaySleep(bus)
        hold.hold(true)
        hold.hold(true) // a second hold does not inhibit twice
        hold.hold(false)
        hold.hold(false) // nor release twice
        assertEquals(
            listOf("org.freedesktop.ScreenSaver.Inhibit[TroubaStage, Performing a concert]", "org.freedesktop.ScreenSaver.UnInhibit[42]"),
            bus.calls,
        )
    }

    @Test
    fun gnomesSessionManagerIsTheFallback_withTheIdleFlag() {
        val bus = FakeBus(mapOf("org.freedesktop.ScreenSaver" to null, "org.gnome.SessionManager" to 7))
        val hold = FreedesktopDisplaySleep(bus)
        hold.hold(true)
        hold.hold(false)
        assertEquals(
            listOf(
                "org.freedesktop.ScreenSaver.Inhibit[TroubaStage, Performing a concert]",
                "org.gnome.SessionManager.Inhibit[TroubaStage, 0, Performing a concert, 8]",
                "org.gnome.SessionManager.Uninhibit[7]",
            ),
            bus.calls,
        )
    }

    @Test
    fun noServiceAndAThrowingBus_holdNothingAndNeverThrow() {
        val none = FakeBus(emptyMap())
        FreedesktopDisplaySleep(none).apply { hold(true); hold(false) }
        assertEquals(2, none.calls.size, "both services asked, nothing released")
        val throwing = object : SessionBus {
            override fun callForCookie(dest: String, path: String, iface: String, method: String, args: List<Any>): Int? = error("no bus")
            override fun call(dest: String, path: String, iface: String, method: String, args: List<Any>) = error("no bus")
        }
        FreedesktopDisplaySleep(throwing).apply { hold(true); hold(false) }
    }

    @Test
    fun linuxBootTime_countsAtLeastAsMuchAsTheMonotonicClock() {
        if (!isLinux()) return
        // CLOCK_BOOTTIME = CLOCK_MONOTONIC (System.nanoTime here) + time suspended, so read after it, it is never behind.
        val monotonic = System.nanoTime() / 1_000_000
        val boot = assertNotNull(linuxBootTimeMs(), "CLOCK_BOOTTIME through libc")
        assertTrue(boot >= monotonic, "boot $boot vs monotonic $monotonic (read first)")
    }
}

/**
 * DESK01 D2 — the real libdbus path, run by hand on a PRIVATE bus (never the desktop's): `dbus-run-session` with a
 * stand-in service, then this test with DESK01_DBUS_PROBE=1. Skipped otherwise.
 */
class DisplaySleepBusProbe {
    @Test
    fun holdAndReleaseOnTheSessionBus() {
        if (System.getenv("DESK01_DBUS_PROBE") != "1") return
        val hold = FreedesktopDisplaySleep(LibDbusSession)
        hold.hold(true)
        println("DESK01 probe: held")
        hold.hold(false)
        println("DESK01 probe: released")
    }
}
