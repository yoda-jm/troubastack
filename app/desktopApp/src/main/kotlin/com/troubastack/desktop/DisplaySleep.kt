package com.troubastack.desktop

import com.sun.jna.Library
import com.sun.jna.Memory
import com.sun.jna.Native
import com.sun.jna.Pointer
import com.sun.jna.Structure
import com.sun.jna.platform.win32.Kernel32
import com.sun.jna.platform.win32.WinBase
import com.sun.jna.ptr.IntByReference
import com.troubastack.shared.seams.HostOs
import com.troubastack.shared.seams.hostOs

/**
 * WIN03/DESK01 — keep the display on while a concert is performed, the way each OS offers it. [hold] is called on the
 * AWT event thread (Stage's DisposableEffect) and never throws: where the OS gives no way to hold the display, it does
 * nothing.
 */
internal interface DisplaySleep {
    fun hold(on: Boolean)
}

internal fun displaySleepForThisOs(): DisplaySleep = when (hostOs) {
    HostOs.WINDOWS -> WindowsDisplaySleep
    HostOs.LINUX -> FreedesktopDisplaySleep(LibDbusSession)
    HostOs.MACOS -> CaffeinateDisplaySleep { pid -> ProcessBuilder("/usr/bin/caffeinate", "-d", "-w", pid.toString()).start() }
}

internal fun isLinux(): Boolean = hostOs == HostOs.LINUX

/**
 * DESK01 D3 — macOS: `caffeinate -d` holds the display awake while it runs; `-w <pid>` ends it with the app, so a crash
 * never leaves the Mac unable to sleep. Released by ending it.
 */
internal class CaffeinateDisplaySleep(private val start: (pid: Long) -> Process) : DisplaySleep {
    private var running: Process? = null

    override fun hold(on: Boolean) {
        if (on && running?.isAlive != true) {
            running = runCatching { start(ProcessHandle.current().pid()) }.getOrNull()
        } else if (!on) {
            running?.destroy()
            running = null
        }
    }
}

@Structure.FieldOrder("tvSec", "tvNsec")
internal class Timespec : Structure() {
    @JvmField var tvSec: Long = 0
    @JvmField var tvNsec: Long = 0
}

@Suppress("FunctionName")
private interface LibC : Library {
    fun clock_gettime(clockId: Int, ts: Timespec): Int
}

@Suppress("FunctionName")
private interface LibSystem : Library {
    fun clock_gettime_nsec_np(clockId: Int): Long
}

/** DESK01 — Linux's CLOCK_BOOTTIME in ms (CLOCK_MONOTONIC plus the time spent suspended), or null if unavailable. */
internal fun linuxBootTimeMs(): Long? {
    val libc = linuxLibC ?: return null
    val ts = Timespec()
    return if (libc.clock_gettime(CLOCK_BOOTTIME, ts) == 0) ts.tvSec * 1000 + ts.tvNsec / 1_000_000 else null
}

/** DESK01 — macOS's CLOCK_MONOTONIC in ms, which Apple documents as continuing to count while the system sleeps (the
 *  JDK's System.nanoTime there does not); null if unavailable. */
internal fun macMonotonicMs(): Long? {
    val lib = macLibSystem ?: return null
    return lib.clock_gettime_nsec_np(MACOS_CLOCK_MONOTONIC).takeIf { it > 0 }?.let { it / 1_000_000 }
}

private const val CLOCK_BOOTTIME = 7
private const val MACOS_CLOCK_MONOTONIC = 6
private val linuxLibC: LibC? by lazy { if (isLinux()) runCatching { Native.load("c", LibC::class.java) }.getOrNull() else null }
private val macLibSystem: LibSystem? by lazy {
    if (hostOs == HostOs.MACOS) runCatching { Native.load("System", LibSystem::class.java) }.getOrNull() else null
}

internal object NoDisplaySleep : DisplaySleep {
    override fun hold(on: Boolean) {}
}

/** SetThreadExecutionState holds for the calling thread; Compose calls this on the AWT event thread, which lives as
 *  long as the app. */
internal object WindowsDisplaySleep : DisplaySleep {
    override fun hold(on: Boolean) {
        val flags = if (on) WinBase.ES_CONTINUOUS or WinBase.ES_DISPLAY_REQUIRED or WinBase.ES_SYSTEM_REQUIRED else WinBase.ES_CONTINUOUS
        Kernel32.INSTANCE.SetThreadExecutionState(flags)
    }
}

/**
 * DESK01 D2 — the session bus as the Linux hold needs it: one method call that returns a uint32 (the inhibit cookie),
 * or null when there is no bus, no such service, or any error. The connection stays open while held: the desktop
 * drops an inhibit when the connection that made it goes away.
 */
internal interface SessionBus {
    fun callForCookie(dest: String, path: String, iface: String, method: String, args: List<Any>): Int?
    fun call(dest: String, path: String, iface: String, method: String, args: List<Any>)
}

/**
 * DESK01 D2 — the freedesktop idle inhibit: `org.freedesktop.ScreenSaver.Inhibit` (KDE, and GNOME through its proxy),
 * else GNOME's own `org.gnome.SessionManager.Inhibit` with the "idle" flag. Releasing calls the matching UnInhibit with
 * the cookie. No bus or neither service: nothing is held, nothing fails (a documented no-op).
 */
internal class FreedesktopDisplaySleep(private val bus: SessionBus) : DisplaySleep {
    private var held: Pair<Service, Int>? = null

    private enum class Service(val dest: String, val path: String, val iface: String, val release: String) {
        FREEDESKTOP("org.freedesktop.ScreenSaver", "/org/freedesktop/ScreenSaver", "org.freedesktop.ScreenSaver", "UnInhibit"),
        GNOME("org.gnome.SessionManager", "/org/gnome/SessionManager", "org.gnome.SessionManager", "Uninhibit"),
    }

    override fun hold(on: Boolean) {
        if (on && held == null) {
            held = runCatching {
                bus.callForCookie(Service.FREEDESKTOP.dest, Service.FREEDESKTOP.path, Service.FREEDESKTOP.iface, "Inhibit", listOf(APP, REASON))
                    ?.let { Service.FREEDESKTOP to it }
                    ?: bus.callForCookie(Service.GNOME.dest, Service.GNOME.path, Service.GNOME.iface, "Inhibit", listOf(APP, 0, REASON, GNOME_INHIBIT_IDLE))
                        ?.let { Service.GNOME to it }
            }.getOrNull()
        } else if (!on) {
            val (service, cookie) = held ?: return
            held = null
            runCatching { bus.call(service.dest, service.path, service.iface, service.release, listOf(cookie)) }
        }
    }

    private companion object {
        const val APP = "TroubaStage"
        const val REASON = "Performing a concert"
        const val GNOME_INHIBIT_IDLE = 8
    }
}

/** DBusError as libdbus lays it out: name, message, a word of bit fields, a padding pointer. */
@Structure.FieldOrder("name", "message", "dummy", "padding1")
internal class DBusError : Structure() {
    @JvmField var name: Pointer? = null
    @JvmField var message: Pointer? = null
    @JvmField var dummy: Int = 0
    @JvmField var padding1: Pointer? = null
}

@Suppress("FunctionName")
private interface LibDbus : Library {
    fun dbus_error_init(error: DBusError)
    fun dbus_error_free(error: DBusError)
    fun dbus_bus_get_private(type: Int, error: DBusError): Pointer?
    fun dbus_connection_set_exit_on_disconnect(connection: Pointer, exitOnDisconnect: Int)
    fun dbus_message_new_method_call(dest: String, path: String, iface: String, method: String): Pointer?
    fun dbus_message_append_args(message: Pointer, firstType: Int, vararg rest: Any?): Int
    fun dbus_connection_send_with_reply_and_block(connection: Pointer, message: Pointer, timeoutMs: Int, error: DBusError): Pointer?
    fun dbus_message_get_args(message: Pointer, error: DBusError, firstType: Int, vararg rest: Any?): Int
    fun dbus_message_unref(message: Pointer)
}

/**
 * The session bus through libdbus-1 (on every Linux desktop), loaded on first use. A private connection, with
 * exit-on-disconnect turned OFF: libdbus would otherwise end the whole app if the bus went away.
 */
internal object LibDbusSession : SessionBus {
    private const val DBUS_BUS_SESSION = 0
    private const val TYPE_INVALID = 0
    private const val TYPE_STRING = 's'.code
    private const val TYPE_UINT32 = 'u'.code
    private const val TIMEOUT_MS = 2000

    // Only an existing session bus: with no address, libdbus would "autolaunch" one of its own on X11.
    private val lib: LibDbus? by lazy {
        val hasBus = !System.getenv("DBUS_SESSION_BUS_ADDRESS").isNullOrBlank() ||
            System.getenv("XDG_RUNTIME_DIR")?.let { java.io.File(it, "bus").exists() } == true
        if (hasBus) runCatching { Native.load("dbus-1", LibDbus::class.java) }.getOrNull() else null
    }
    private var connection: Pointer? = null

    private fun connection(lib: LibDbus): Pointer? = connection ?: run {
        val err = DBusError().also { lib.dbus_error_init(it) }
        lib.dbus_bus_get_private(DBUS_BUS_SESSION, err).also {
            lib.dbus_error_free(err)
            if (it != null) lib.dbus_connection_set_exit_on_disconnect(it, 0)
            connection = it
        }
    }

    private fun send(dest: String, path: String, iface: String, method: String, args: List<Any>, readCookie: Boolean): Int? {
        val lib = lib ?: return null
        val conn = connection(lib) ?: return null
        val msg = lib.dbus_message_new_method_call(dest, path, iface, method) ?: return null
        val keep = mutableListOf<Any>() // native memory the call reads; alive until it returns
        try {
            val varargs = mutableListOf<Any?>()
            for (a in args) when (a) {
                is String -> {
                    val s = Memory(a.toByteArray().size + 1L).apply { setString(0, a, "UTF-8") }
                    val ref = Memory(Native.POINTER_SIZE.toLong()).apply { setPointer(0, s) }
                    keep += s; keep += ref
                    varargs += TYPE_STRING; varargs += ref
                }
                is Int -> { varargs += TYPE_UINT32; varargs += IntByReference(a) }
                else -> error("unsupported D-Bus argument $a")
            }
            varargs += TYPE_INVALID
            if (lib.dbus_message_append_args(msg, varargs.first() as Int, *varargs.drop(1).toTypedArray()) == 0) return null
            val err = DBusError().also { lib.dbus_error_init(it) }
            val reply = lib.dbus_connection_send_with_reply_and_block(conn, msg, TIMEOUT_MS, err)
            lib.dbus_error_free(err)
            reply ?: return null
            try {
                if (!readCookie) return 0
                val cookie = IntByReference()
                val gerr = DBusError().also { lib.dbus_error_init(it) }
                val ok = lib.dbus_message_get_args(reply, gerr, TYPE_UINT32, cookie, TYPE_INVALID)
                lib.dbus_error_free(gerr)
                return if (ok != 0) cookie.value else null
            } finally {
                lib.dbus_message_unref(reply)
            }
        } finally {
            lib.dbus_message_unref(msg)
            keep.clear()
        }
    }

    override fun callForCookie(dest: String, path: String, iface: String, method: String, args: List<Any>): Int? =
        send(dest, path, iface, method, args, readCookie = true)

    override fun call(dest: String, path: String, iface: String, method: String, args: List<Any>) {
        send(dest, path, iface, method, args, readCookie = false)
    }
}
