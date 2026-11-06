package com.troubastack.desktop.studio

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.RGBLuminanceSource
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader
import com.troubastack.app.HttpTransport
import com.troubastack.desktop.DesktopAppPlatform
import com.troubastack.desktop.WIN09_SABOTAGE_PROPERTY
import com.troubastack.desktop.windowKey
import com.troubastack.shared.seams.SESSION_COOKIE_KEY
import com.troubastack.shared.seams.SESSION_ORIGIN_KEY
import com.troubastack.shared.seams.Storage
import kotlinx.coroutines.runBlocking
import java.awt.image.BufferedImage
import java.io.File
import java.net.InetAddress
import java.net.ServerSocket
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import javax.swing.SwingUtilities

/**
 * WIN09 — the band-QR smoke (see [Win09SmokeRunner]). A loopback fake core serves what the app asks for: `/api/me`,
 * the bands, each band's role (`myRole`), its invite links (listed, and a counted create), and a Studio stand-in page
 * that handshakes the bridge. The app is signed in as Connect leaves it (the server + its session cookie for that
 * origin), and the window shows the app's own TroubaStudio screen, `DesktopAppPlatform.studio.Browse`, re-opened per
 * phase so each phase fetches the roles afresh:
 *  1. both bands member → the ⋮ menu has no "Show band QR…";
 *  2. band A admin (its links list an expiring one FIRST, then the standing one) → the item opens A's QR directly; the
 *     capture decodes to the standing link; nothing was created; the display request is held only while it shows; the
 *     overlay takes the focus from a focused Browser, a real Alt+← closes it without walking Studio's history, and
 *     Back leaves the same controller on the same page, full size, with the focus back in Studio;
 *  3. band B admin (only a single-use link) → the screen offers "Create standing QR", creates nothing, shows no QR;
 *     the counter's positive control is the app's own create call.
 * The window then closes; the process exits 0 unless a check failed (UNMEASURED is reported, not failed).
 */
class Win09SmokeRunnerImpl : Win09SmokeRunner {
    @Volatile private var roles: Map<String, String> = mapOf(BAND_A to "member", BAND_B to "member")
    private val roleGets = AtomicInteger()   // GET /api/bands/{id}: the admin list's per-band role reads
    private val listGets = ConcurrentCounter()
    private val creates = AtomicInteger()    // POST /api/bands/{id}/invite-links
    private val pageKeys = java.util.concurrent.CopyOnWriteArrayList<String>()   // the stand-in page's keydowns (/win09-key)
    private val failBandsOnce = AtomicBoolean(false)   // phase 4: answer the next /api/bands with a 500
    private val failServed = AtomicBoolean(false)      // ...and that 500 was served (the stays check's control)
    @Volatile private var origin = ""

    private fun line(s: String) = win08log("win09: $s")

    override fun run(outDir: File): Int {
        outDir.mkdirs(); enableWin08FileLog()
        line("run start; sabotage=[${System.getProperty(WIN09_SABOTAGE_PROPERTY).orEmpty()}] log=$win08LogFile")
        Thread { try { Thread.sleep(190_000) } catch (_: InterruptedException) { return@Thread }
            line("BACKSTOP firing (>190s)"); Runtime.getRuntime().halt(1) }.apply { isDaemon = true; start() }
        try {
            val server = ServerSocket(0, 0, InetAddress.getLoopbackAddress())
            origin = "http://127.0.0.1:${server.localPort}"
            Thread { acceptLoop(server) }.apply { isDaemon = true; start() }
            line("fake core on $origin")
            // Signed in as Connect leaves it: the server, and the session cookie bound to that origin.
            val storage = Storage()
            storage.putSecret("coreUrl", origin)
            storage.putSecret(SESSION_COOKIE_KEY, "win09sess=smoke")
            storage.putSecret(SESSION_ORIGIN_KEY, origin)
            val transport = HttpTransport(storage, defaultServerUrl = null) { _, _ -> }
            val phase = mutableStateOf(1)
            val leftStudio = AtomicBoolean(false)
            val frame = AtomicReference<java.awt.Window?>()
            application {
                val windowState = rememberWindowState(width = 1200.dp, height = 820.dp, position = WindowPosition(0.dp, 0.dp))
                val platform = remember { DesktopAppPlatform(storage, windowState) }
                Window(onCloseRequest = ::exitApplication, title = "TroubaStage", state = windowState, onKeyEvent = { windowKey(it, platform) }) {
                    LaunchedEffect(Unit) {
                        platform.awtWindow = window; frame.set(window)
                        Thread { flow(outDir, platform, transport, phase, leftStudio, frame) }.apply { isDaemon = true; start() }
                    }
                    MaterialTheme(colorScheme = lightColorScheme()) {
                        key(phase.value) {
                            platform.studio.Browse(transport, onOpen = { _, _, _ -> }, onShowQr = { _, _ -> }, onBack = { leftStudio.set(true) })
                        }
                    }
                }
            }
        } catch (e: Throwable) {
            line("run FAILED: $e\n${e.stackTraceToString()}")
            Runtime.getRuntime().halt(1)
        }
        return 0
    }

    private fun flow(
        outDir: File, platform: DesktopAppPlatform, transport: HttpTransport, phase: MutableState<Int>,
        leftStudio: AtomicBoolean, frame: AtomicReference<java.awt.Window?>,
    ) {
        val results = LinkedHashMap<String, Boolean?>()
        fun mark(name: String, ok: Boolean?, detail: String) {
            results[name] = ok
            line("WIN09-CHECK $name: ${when (ok) { true -> "PASS"; false -> "FAIL"; null -> "UNMEASURED" }} — $detail")
        }
        fun waitFor(ms: Long, cond: () -> Boolean): Boolean {
            val end = System.currentTimeMillis() + ms
            while (System.currentTimeMillis() < end) { if (cond()) return true; Thread.sleep(100) }
            return cond()
        }
        fun live() = SwtEmbeddedStudioProvider.liveController
        // A phase re-opens TroubaStudio (a fresh controller) and waits for the admin list's role reads, plus a margin.
        fun openStudio(n: Int, prev: SwtBrowserController?, r: Map<String, String>): SwtBrowserController? {
            roles = r
            val since = roleGets.get()
            leftStudio.set(false)
            phase.value = n
            val c = if (waitFor(40_000) { live().let { it != null && it !== prev && it.bridgeReady } }) live() else null
            val answered = waitFor(15_000) { roleGets.get() >= since + r.size }
            Thread.sleep(1_500)
            line("phase $n: controller=${c != null} roleReads=${roleGets.get() - since} answered=$answered")
            return c
        }
        fun capture(name: String): BufferedImage? = runCatching {
            val b = frame.get()!!.bounds
            java.awt.Robot().createScreenCapture(java.awt.Rectangle(b.x, b.y, b.width, b.height))
                .also { javax.imageio.ImageIO.write(it, "png", File(outDir, "$name.png")) }
        }.getOrNull()
        fun decodeNow(name: String): String? = capture(name)?.let(::decodeQr)
        // The panel is full size (and the Shell refit to it) only while no host overlay collapses it to 0.dp.
        fun panelFull(c: SwtBrowserController): Boolean { c.measureDpi(); return c.rectsMatch }
        fun decodeWithin(ms: Long, name: String, want: (String?) -> Boolean): String? {
            var last: String? = null
            waitFor(ms) { last = decodeNow(name); want(last) }
            return last
        }

        try {
            // ---- Phase 1: a member of both bands. The ⋮ menu has no "Show band QR…" (Reload proves the read works).
            val c1 = openStudio(1, null, mapOf(BAND_A to "member", BAND_B to "member"))
                ?: return noStudio(1)
            val items1 = menuItems(c1)
            closeMenu(c1)
            mark("item-absent-for-non-admin", items1 != null && "Reload" in items1 && ITEM !in items1, "items=$items1")

            // ---- Phase 2: admin of band A only → the item opens A's QR directly.
            val c2 = openStudio(2, c1, mapOf(BAND_A to "admin", BAND_B to "member"))
                ?: return noStudio(2)
            // Studio history, so a stray Alt+← into the Browser would show (it walks back from page2).
            c2.smokeInPageNav("$origin/page2")
            waitFor(10_000) { c2.lastUrl == "$origin/page2" && c2.bridgeReady }
            // The Browser holds the focus first, as after typing in Studio — the control for the focus checks.
            c2.focusBrowserForSmoke()
            val focusControl = waitFor(5_000) { c2.browserFocused }
            line("before overlay: browserFocused=${c2.browserFocused} ${awtFocus(frame.get())}")
            val heldBefore = displayRequestHeld()
            val items2 = menuItems(c2)
            mark("item-present-for-admin", items2?.contains(ITEM) == true, "items=$items2")
            val urlBefore = c2.lastUrl
            val menuWasOpen = c2.lastMenuWindow != null
            val clicked = clickMenuItem(c2, ITEM)
            val menuGone = waitFor(3_000) { c2.lastMenuWindow == null }
            val collapsed = waitFor(5_000) { !panelFull(c2) }
            line("overlay open: panel collapsed=$collapsed canvas=${c2.canvasClient}")
            mark("menu-dismissed-on-open", menuWasOpen && clicked && menuGone, "menuWasOpen=$menuWasOpen clicked=$clicked gone=$menuGone")
            val decodedA = decodeWithin(8_000, "qr-band-a") { it == LINK_A_STANDING }
            mark("qr-decodes-to-served-link", decodedA == LINK_A_STANDING, "decoded=[$decodedA] want=[$LINK_A_STANDING]")
            val createsA = creates.get()
            val focusLeft = !c2.browserFocused   // informational: WebView2 does not report losing the focus
            line("overlay focus: browserFocused=${c2.browserFocused} ${awtFocus(frame.get())}")
            val heldDuring = displayRequestHeld()
            // Back through the app's Back (what Esc, Alt+← and the mouse Back button dispatch).
            SwingUtilities.invokeAndWait { platform.back.dispatch() }
            val closed = waitFor(6_000) { panelFull(c2) }
            decodeNow("after-back")
            Thread.sleep(1_000)
            val heldAfter = displayRequestHeld()
            line("display request before=${heldBefore.first} during=${heldDuring.first} after=${heldAfter.first}; during: ${heldDuring.second}")
            val awake = if (heldBefore.first == null || heldDuring.first == null || heldAfter.first == null) null
                else heldBefore.first == false && heldDuring.first == true && heldAfter.first == false
            mark("keep-awake-while-shown", awake,
                if (awake == null) "powercfg /requests could not be read: ${listOf(heldBefore, heldDuring, heldAfter).firstOrNull { it.first == null }?.second}"
                else "DISPLAY held before=${heldBefore.first} during=${heldDuring.first} after=${heldAfter.first}")
            // Where it was: the same controller, on the same page, never walked or left, panel full size again.
            val canvasNonZero = c2.canvasClient.split('x').mapNotNull { it.toIntOrNull() }.let { it.size == 2 && it.all { v -> v > 0 } }
            val same = live() === c2 && c2.bridgeReady
            val where = collapsed && closed && same && c2.lastUrl == urlBefore && c2.wentBack == null && !leftStudio.get() && c2.rectsMatch && canvasNonZero
            mark("back-returns-where-it-was", where,
                "collapsedWhileShown=$collapsed fullAfter=$closed sameController=$same lastUrl=[${c2.lastUrl}] want=[$urlBefore] wentBack=${c2.wentBack} left=${leftStudio.get()} canvas=${c2.canvasClient} shell=${c2.shellClient} rectsMatch=${c2.rectsMatch}")
            // The focus is back in Studio: a real Esc now goes to the page and does nothing; had the focus stayed with the
            // app, Esc would be the app's Back and walk Studio's history from page2. (SWT's focus flag is logged only:
            // WebView2 does not report losing the focus to the frame, so it can't tell either way.)
            val fg = runCatching { frame.get()?.let { Win08Native.foregroundIsFrame(it) } }.getOrNull() == true
            Thread.sleep(800)
            val navE = c2.navCount; val urlE = c2.lastUrl
            if (fg) { val r = java.awt.Robot(); r.keyPress(java.awt.event.KeyEvent.VK_ESCAPE); r.keyRelease(java.awt.event.KeyEvent.VK_ESCAPE) }
            Thread.sleep(2_000)
            val escWalked = c2.navCount != navE || c2.lastUrl != urlE || leftStudio.get()
            // ...and the keys reach the PAGE, not just leave the app: a typed "q" must arrive as the page's keydown.
            val keysBefore = pageKeys.size
            if (fg) { val r = java.awt.Robot(); r.keyPress(java.awt.event.KeyEvent.VK_Q); r.keyRelease(java.awt.event.KeyEvent.VK_Q) }
            val qArrived = waitFor(4_000) { pageKeys.drop(keysBefore).contains("q") }
            mark("focus-returns-to-studio", if (!focusControl || !fg) null else !escWalked && qArrived,
                if (!fg) "the frame was not the foreground window, so a key could not be sent"
                else "Esc after Back walked Studio=$escWalked (navigations=${c2.navCount - navE} lastUrl=[${c2.lastUrl}] want=[$urlE]); " +
                    "typed q reached the page=$qArrived (page keys=${pageKeys.toList()}) swtFocusFlag=${c2.browserFocused}")

            // The overlay holds the keys: re-open it with the Browser focused, then a REAL Alt+← (Robot) must close the
            // overlay, not walk Studio's history behind it.
            val c2b = live() ?: c2
            c2b.focusBrowserForSmoke(); waitFor(3_000) { c2b.browserFocused }
            val urlB = c2b.lastUrl; val navB = c2b.navCount
            menuItems(c2b); clickMenuItem(c2b, ITEM)
            val shownB = waitFor(5_000) { !panelFull(c2b) }
            decodeNow("qr-band-a-again")
            val focusLeftB = !c2b.browserFocused
            runCatching { frame.get()?.let { Win08Native.setForeground(it) } }
            Thread.sleep(300)
            val robot = java.awt.Robot()
            robot.keyPress(java.awt.event.KeyEvent.VK_ALT); robot.keyPress(java.awt.event.KeyEvent.VK_LEFT)
            robot.keyRelease(java.awt.event.KeyEvent.VK_LEFT); robot.keyRelease(java.awt.event.KeyEvent.VK_ALT)
            val closedByKey = waitFor(5_000) { panelFull(c2b) }
            decodeNow("after-alt-left")
            Thread.sleep(1_000)
            val walked = c2b.navCount != navB || c2b.lastUrl != urlB || leftStudio.get()
            if (!closedByKey) SwingUtilities.invokeAndWait { platform.back.dispatch() }   // close it, so phase 3 starts clean
            mark("overlay-takes-focus", if (!focusControl) null else (shownB && closedByKey && !walked),
                "shown=$shownB swtFocusFlag(1st,2nd)=${!focusLeft},${!focusLeftB} altLeftClosed=$closedByKey walkedStudio=$walked (navigations=${c2b.navCount - navB} lastUrl=[${c2b.lastUrl}] want=[$urlB])")

            // "Manage / revoke in Studio" loads /bands/<id>/settings IN PLACE: the overlay closes, the same controller lands
            // on the settings page. The button is pressed through the window's accessibility tree (Compose exposes its
            // semantics there); if the runner gives no tree, this is UNMEASURED rather than claimed.
            val c2c = live() ?: c2
            menuItems(c2c); clickMenuItem(c2c, ITEM)
            val shownC = waitFor(5_000) { !panelFull(c2c) }
            Thread.sleep(1_000)
            val pressed = pressAccessible(frame.get(), "Manage / revoke in Studio")
            val settings = "$origin/bands/$BAND_A/settings"
            val landed = pressed != null && waitFor(8_000) { c2c.lastUrl == settings } && waitFor(5_000) { panelFull(c2c) }
            if (pressed == null) SwingUtilities.invokeAndWait { platform.back.dispatch() }
            mark("manage-opens-settings-in-place", if (pressed == null) null else (shownC && landed && live() === c2c),
                if (pressed == null) "no accessible node named \"Manage / revoke in Studio\" was found"
                else "pressed=$pressed shown=$shownC lastUrl=[${c2c.lastUrl}] want=[$settings] sameController=${live() === c2c}")

            // ---- Phase 3: admin of band B only, whose one link is single-use → "Create standing QR", nothing created.
            val c3 = openStudio(3, live(), mapOf(BAND_A to "member", BAND_B to "admin"))
                ?: return noStudio(3)
            val listsB = listGets.get(BAND_B)
            menuItems(c3); val clickedB = clickMenuItem(c3, ITEM)
            val listed = waitFor(8_000) { listGets.get(BAND_B) > listsB }
            Thread.sleep(4_000)
            val decodedB = decodeNow("qr-band-b")
            val createsB = creates.get() - createsA
            mark("no-create-on-open", clickedB && listed && createsA == 0 && createsB == 0,
                "creates: band A=$createsA band B=$createsB (clicked=$clickedB listed=$listed)")
            mark("unsuitable-link-not-shown", clickedB && listed && decodedB == null, "decoded=[$decodedB]")
            // The counter's positive control: the app's own create call (the button's) reaches it.
            val c0 = creates.get()
            val made = runBlocking { transport.createStandingInviteLink(BAND_B) }
            mark("create-counter-control", made != null && creates.get() == c0 + 1, "made=${made?.url} creates=${creates.get() - c0}")
            SwingUtilities.invokeAndWait { platform.back.dispatch() }

            // ---- Phase 4: the item follows the roles while TroubaStudio stays open — one controller, no reopen. Each
            // press is the REAL ⋮ (through the accessibility tree), which refreshes the list (≤ 800 ms) before the menu.
            val c4 = openStudio(4, live(), mapOf(BAND_A to "admin", BAND_B to "member")) ?: return noStudio(4)
            val (atOpen, latOpen) = menuAfterPress(c4, frame.get())   // the open's own fetch: A is admin (the precondition)
            // A failed refresh keeps the list: the next /api/bands answers 500 while the roles say member.
            roles = mapOf(BAND_A to "member", BAND_B to "member")
            failServed.set(false); failBandsOnce.set(true)
            val (onFail, latFail) = menuAfterPress(c4, frame.get())
            failBandsOnce.set(false)
            // Measurable only when the press made its fetch and got the 500 (its control); a press that fetched nothing is
            // the follows-roles check's red, not this one's.
            mark("item-stays-on-failed-refresh",
                if (!failServed.get()) null else atOpen?.contains(ITEM) == true && onFail?.contains(ITEM) == true && live() === c4,
                if (!failServed.get()) "the press made no fetch, so no 500 was served (atOpen=$atOpen afterPress=$onFail)"
                else "atOpen=$atOpen 500served=true afterFailedRefresh=$onFail sameController=${live() === c4}")
            // The roles flip, no reopen: member → the item goes; admin again → it comes back.
            val (asMember, latMember) = menuAfterPress(c4, frame.get())
            roles = mapOf(BAND_A to "admin", BAND_B to "member")
            val (asAdmin, latAdmin) = menuAfterPress(c4, frame.get())
            mark("item-follows-roles", asMember != null && ITEM !in asMember && asAdmin?.contains(ITEM) == true && live() === c4,
                "member=$asMember admin=$asAdmin sameController=${live() === c4}")
            line("menu press→open latency ms: open=$latOpen failedRefresh=$latFail member=$latMember admin=$latAdmin (the wait is ≤ 800)")
        } catch (e: Throwable) {
            mark("flow", false, "threw: $e\n${e.stackTraceToString()}")
        }
        val failed = results.filterValues { it == false }.keys
        line("win09-qr: ${results.count { it.value == true }} pass, ${failed.size} fail, ${results.count { it.value == null }} unmeasured; failed=$failed")
        if (failed.isNotEmpty()) { Runtime.getRuntime().halt(1); return }
        val hwnd = frame.get()?.let { Win08Native.hwndOf(it) }
        if (hwnd == null) { Runtime.getRuntime().halt(0); return }
        Win08Native.postClose(hwnd)   // the window closes the way a person closes it; the process then exits on its own
    }

    private fun noStudio(phase: Int) {
        line("WIN09-CHECK studio-phase$phase: FAIL — no ready controller; halting 1")
        Runtime.getRuntime().halt(1)
    }

    /** Press the control named [name] through the window's accessibility tree (its first action, a click). Returns the
     *  node's class for the log, or null when no such node is exposed. */
    private fun pressAccessible(frame: java.awt.Window?, name: String): String? {
        frame ?: return null
        var found: String? = null
        SwingUtilities.invokeAndWait {
            val ctx = runCatching { findAccessible(frame, name, 0) }.getOrNull() ?: return@invokeAndWait
            val action = ctx.accessibleAction ?: return@invokeAndWait
            if (action.accessibleActionCount > 0 && action.doAccessibleAction(0)) found = ctx.javaClass.simpleName
        }
        line("accessible press [$name] -> $found")
        return found
    }

    private fun findAccessible(node: javax.accessibility.Accessible, name: String, depth: Int): javax.accessibility.AccessibleContext? {
        val ctx = node.accessibleContext ?: return null
        if (ctx.accessibleName == name) return ctx
        if (depth > 60) return null
        for (i in 0 until ctx.accessibleChildrenCount) {
            findAccessible(ctx.getAccessibleChild(i) ?: continue, name, depth + 1)?.let { return it }
        }
        return null
    }

    /** AWT's view of the focus (for the log): is the frame focused, and which component owns the focus. */
    private fun awtFocus(frame: java.awt.Window?): String {
        var s = ""
        SwingUtilities.invokeAndWait {
            val kfm = java.awt.KeyboardFocusManager.getCurrentKeyboardFocusManager()
            s = "awt: frameFocused=${frame?.isFocused} focusedWindow=${kfm.focusedWindow?.javaClass?.simpleName} owner=${kfm.focusOwner?.javaClass?.simpleName}"
        }
        return s
    }

    /** Press the REAL ⋮ button (through the accessibility tree, so its onClick runs: the refresh, then the menu), read
     *  the menu's labels, close it. Returns the labels (null if no menu opened) and the press→menu time in ms. */
    private fun menuAfterPress(c: SwtBrowserController, frame: java.awt.Window?): Pair<List<String>?, Long> {
        closeMenu(c)
        val t0 = System.currentTimeMillis()
        if (pressAccessible(frame, "⋮") == null) return null to -1
        val end = t0 + 4_000
        while (c.lastMenuWindow == null && System.currentTimeMillis() < end) Thread.sleep(20)
        val took = System.currentTimeMillis() - t0
        val w = c.lastMenuWindow ?: return null to took
        var labels: List<String> = emptyList()
        SwingUtilities.invokeAndWait { labels = menuItemsOf(w).map { it.text } }
        closeMenu(c)
        return labels to took
    }

    /** Open the real ⋮ menu and read its item labels (null if it never opened). */
    private fun menuItems(c: SwtBrowserController): List<String>? {
        c.openStudioMenu()
        val end = System.currentTimeMillis() + 4_000
        while (c.lastMenuWindow == null && System.currentTimeMillis() < end) Thread.sleep(100)
        val w = c.lastMenuWindow ?: return null
        var labels: List<String> = emptyList()
        SwingUtilities.invokeAndWait { labels = menuItemsOf(w).map { it.text } }
        return labels
    }

    private fun closeMenu(c: SwtBrowserController) {
        SwingUtilities.invokeAndWait { c.lastMenuWindow?.dispose() }
        val end = System.currentTimeMillis() + 2_000
        while (c.lastMenuWindow != null && System.currentTimeMillis() < end) Thread.sleep(20)
    }

    /** Choose the open menu's item labelled [label], as a click does (JMenuItem.doClick → its action). */
    private fun clickMenuItem(c: SwtBrowserController, label: String): Boolean {
        val w = c.lastMenuWindow ?: return false
        var clicked = false
        SwingUtilities.invokeAndWait { menuItemsOf(w).firstOrNull { it.text == label }?.let { it.doClick(); clicked = true } }
        return clicked
    }

    private fun menuItemsOf(root: java.awt.Container): List<javax.swing.JMenuItem> =
        root.components.flatMap { c -> listOfNotNull(c as? javax.swing.JMenuItem) + ((c as? java.awt.Container)?.let(::menuItemsOf) ?: emptyList()) }

    /** Decode a QR anywhere in [img] with ZXing (null: none found). */
    private fun decodeQr(img: BufferedImage): String? = runCatching {
        val px = img.getRGB(0, 0, img.width, img.height, null, 0, img.width)
        QRCodeReader().decode(BinaryBitmap(HybridBinarizer(RGBLuminanceSource(img.width, img.height, px))), mapOf(DecodeHintType.TRY_HARDER to true)).text
    }.getOrNull()

    /**
     * Is a DISPLAY power request held by this process (`powercfg /requests`, which lists ES_DISPLAY_REQUIRED holders
     * per process and needs an elevated caller)? first = null when the output can't be read; second = the evidence.
     */
    private fun displayRequestHeld(): Pair<Boolean?, String> = runCatching {
        val p = ProcessBuilder("powercfg", "/requests").redirectErrorStream(true).start()
        val out = p.inputStream.bufferedReader().readText()
        if (!p.waitFor(30, TimeUnit.SECONDS)) { p.destroy(); return null to "powercfg timed out" }
        if (p.exitValue() != 0 || !out.contains("DISPLAY:")) return null to "exit=${p.exitValue()} out=[${out.trim().take(300)}]"
        val exe = ProcessHandle.current().info().command().map { File(it).name }.orElse("TroubaStage.exe")
        val display = out.substringAfter("DISPLAY:").substringBefore("SYSTEM:").trim()
        display.contains(exe, ignoreCase = true) to "exe=$exe DISPLAY=[${display.replace(Regex("\\s+"), " ")}]"
    }.getOrElse { null to "powercfg failed: $it" }

    /** HTTP/1.0 on loopback: the API the screens call, and the Studio stand-in page for everything else. */
    private fun acceptLoop(server: ServerSocket) {
        while (!server.isClosed) {
            val sock = try { server.accept() } catch (_: Throwable) { break }
            Thread {
                runCatching {
                    sock.use { s ->
                        val input = s.getInputStream()
                        val head = readHead(input)
                        val requestLine = head.lineSequence().firstOrNull().orEmpty()
                        val method = requestLine.substringBefore(' ')
                        val target = requestLine.substringAfter(' ', "").substringBefore(' ')
                        val path = target.substringBefore('?')
                        val query = target.substringAfter('?', "")
                        val length = Regex("(?im)^content-length:\\s*(\\d+)").find(head)?.groupValues?.get(1)?.toInt() ?: 0
                        if (length > 0) input.readNBytes(length)
                        val (status, type, body) = route(method, path, query)
                        val bytes = body.toByteArray(StandardCharsets.UTF_8)
                        val out = s.getOutputStream()
                        out.write("HTTP/1.0 $status\r\nContent-Type: $type\r\nContent-Length: ${bytes.size}\r\nConnection: close\r\n\r\n".toByteArray(StandardCharsets.US_ASCII))
                        out.write(bytes); out.flush()
                    }
                }
            }.apply { isDaemon = true; start() }
        }
    }

    private fun readHead(input: java.io.InputStream): String {
        val buf = java.io.ByteArrayOutputStream()
        var last4 = 0
        while (true) {
            val b = input.read()
            if (b < 0) break
            buf.write(b)
            last4 = (last4 shl 8) or b
            if (last4 == 0x0D0A0D0A) break
        }
        return buf.toString(StandardCharsets.US_ASCII)
    }

    private fun route(method: String, path: String, query: String): Triple<String, String, String> {
        val json = "application/json"
        val bandRole = Regex("^/api/bands/([^/]+)$").find(path)?.groupValues?.get(1)
        val bandLinks = Regex("^/api/bands/([^/]+)/invite-links$").find(path)?.groupValues?.get(1)
        return when {
            path == "/api/me" -> Triple("200 OK", json, """{"user":{"id":"u-win09","displayName":"Smoke Admin"}}""")
            path == "/win09-key" -> {
                val k = java.net.URLDecoder.decode(query.removePrefix("k="), StandardCharsets.UTF_8)
                pageKeys += k; line("fake core: page keydown [$k]")
                Triple("204 No Content", "text/plain", "")
            }
            path == "/api/bands" && failBandsOnce.compareAndSet(true, false) -> {
                failServed.set(true); line("fake core: /api/bands answered 500 (phase 4)")
                Triple("500 Internal Server Error", json, "{}")
            }
            path == "/api/bands" -> Triple("200 OK", json, """{"bands":[{"id":"$BAND_A","name":"Smoke Band A"},{"id":"$BAND_B","name":"Smoke Band B"}]}""")
            bandRole != null -> { roleGets.incrementAndGet(); Triple("200 OK", json, """{"myRole":"${roles[bandRole] ?: "member"}"}""") }
            bandLinks != null && method == "POST" -> {
                creates.incrementAndGet(); line("fake core: CREATE invite link for $bandLinks")
                Triple("201 Created", json, """{"id":"created","url":"$origin/join/win09-created","role":"member","maxUses":0,"valid":true}""")
            }
            bandLinks != null -> {
                listGets.inc(bandLinks)
                // Band A lists an expiring link FIRST, so a screen that took the first link would draw the wrong one.
                Triple("200 OK", json, if (bandLinks == BAND_A) """{"links":[""" +
                    """{"id":"exp","url":"$origin/join/win09-expiring-A","role":"member","maxUses":0,"expiresAt":"2026-12-01T00:00:00Z","valid":true},""" +
                    """{"id":"std","url":"$LINK_A_STANDING","role":"member","maxUses":0,"valid":true}]}"""
                    else """{"links":[{"id":"once","url":"$origin/join/win09-single-B","role":"member","maxUses":1,"valid":true}]}""")
            }
            else -> Triple("200 OK", "text/html; charset=utf-8", PAGE)
        }
    }

    private val LINK_A_STANDING: String get() = "$origin/join/win09-standing-A"

    /** Per-band request counts. */
    private class ConcurrentCounter {
        private val m = java.util.concurrent.ConcurrentHashMap<String, AtomicInteger>()
        fun inc(k: String) { m.computeIfAbsent(k) { AtomicInteger() }.incrementAndGet() }
        fun get(k: String): Int = m[k]?.get() ?: 0
    }

    private companion object {
        const val ITEM = "Show band QR…"
        const val BAND_A = "win09-band-a"
        const val BAND_B = "win09-band-b"

        // The Studio stand-in: handshake the bridge at the top of <head>, as WIN08's loopback page does.
        const val PAGE = """<!doctype html><html><head><meta charset="utf-8"><script>
(function(){
  if (typeof window.TroubaStageShellReceive === "function") {
    window.__troubastageShell = { deliver: function(j){ try { if (JSON.parse(j).type === "hello") window.TroubaStageShellReceive(JSON.stringify({type:"ready"})); } catch(e){} } };
    window.TroubaStageShellReceive(JSON.stringify({type:"ready"}));
  }
  // WIN09: report each keydown the PAGE receives, so the smoke can tell the keys reached it (not just left the app).
  document.addEventListener("keydown", function(e){ try { fetch("/win09-key?k=" + encodeURIComponent(e.key)); } catch(x){} }, true);
})();
</script></head><body><h1>win09 Studio stand-in</h1><p><input id="t" value="type here"></p></body></html>"""
    }
}
