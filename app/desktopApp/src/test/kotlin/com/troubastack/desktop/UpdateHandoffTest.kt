package com.troubastack.desktop

import com.troubastack.shared.buildinfo.LatestRelease
import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** WIN04 — the Windows update's handoff rules (H1–H4) and the CI hook's limits (C1), as pure functions. */
class UpdateHandoffTest {
    @Test
    fun theHelpersResult_onlyZeroIsAReopenedUpdate() {
        // TEETH (H3): reading 3010 as success would reopen an app whose files Windows has not finished replacing.
        assertEquals(HandoffResult.Updated, handoffResult("0\r\n"))
        assertEquals(HandoffResult.RestartWindows, handoffResult("3010"))
        assertEquals(HandoffResult.NotClosed, handoffResult("notclosed"))
        assertEquals(HandoffResult.Failed("1603"), handoffResult("1603"))
        assertNull(handoffResult(null))
        assertNull(handoffResult("  "))
    }

    @Test
    fun theCiHook_acceptsLoopbackOnly() {
        // TEETH (C1): a hook that accepted any host could fetch an "update" from another machine.
        for (ok in listOf("http://127.0.0.1:8123/release.json", "http://localhost/r", "http://[::1]:9/r", "https://127.0.0.1/r")) {
            assertEquals(ok, loopbackReleaseUrl(ok))
        }
        for (bad in listOf("http://10.0.2.2:8080/r", "https://api.github.com/r", "http://127.0.0.1.example.com/r", "file:///C:/r.json", "ftp://127.0.0.1/r", "not a url")) {
            assertFailsWith<IllegalArgumentException>(bad) { loopbackReleaseUrl(bad) }
        }
    }

    @Test
    fun theInstallersAddress_isHttpsInNormalUse_andLoopbackForTheHook() {
        // TEETH (5a403976): checking only the release URL let a loopback release send the app to any host for the MSI,
        // and plain http with no digest would have been checked by size alone.
        checkAssetUrl("https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-windows.msi", hook = false)
        assertFailsWith<UpdateRefused>("plain http in normal use") { checkAssetUrl("http://github.com/x/troubastage-windows.msi", hook = false) }
        assertFailsWith<UpdateRefused>("no host") { checkAssetUrl("https:///x.msi", hook = false) }
        checkAssetUrl("http://127.0.0.1:18765/troubastage-windows.msi", hook = true)
        checkAssetUrl("http://[::1]:18765/troubastage-windows.msi", hook = true)
        for (bad in listOf("https://github.com/x/troubastage-windows.msi", "http://10.0.0.5/x.msi", "http://127.0.0.1.example.com/x.msi", "file:///C:/x.msi")) {
            assertFailsWith<UpdateRefused>("hook: $bad") { checkAssetUrl(bad, hook = true) }
        }
    }

    @Test
    fun theDownload_isRefusedOnSizeOrDigest_andTheHookRequiresADigest() {
        val f = Files.createTempFile("msi", ".msi").toFile()
        try {
            f.writeBytes(ByteArray(1000) { it.toByte() })
            val good = sha256Hex(f)
            fun rel(size: Long = 1000, sha: String? = good) = LatestRelease("c".repeat(40), "2026-11-02T00:00:00Z", "http://127.0.0.1/x.msi", size, sha)
            verifyDownload(f, rel(), requireDigest = true) // matches
            verifyDownload(f, rel(sha = good.uppercase()), requireDigest = true) // the digest compares case-blind
            verifyDownload(f, rel(sha = null), requireDigest = false) // GitHub without a digest: the size alone
            assertFailsWith<UpdateRefused>("wrong digest") { verifyDownload(f, rel(sha = "0".repeat(64)), requireDigest = false) }
            assertFailsWith<UpdateRefused>("wrong size") { verifyDownload(f, rel(size = 999), requireDigest = false) }
            assertFailsWith<UpdateRefused>("the hook needs a digest") { verifyDownload(f, rel(sha = null), requireDigest = true) }
        } finally {
            f.delete()
        }
    }

    @Test
    fun onlyTheExeTheMsiInstalled_canInstall() {
        val dir = Files.createTempDirectory("TroubaStage").toFile()
        try {
            val exe = File(dir, "TroubaStage.exe")
            assertTrue(isMsiInstalled(exe, listOf("Other" to "C:\\x", "TroubaStage" to dir.path)))
            assertFalse(isMsiInstalled(exe, listOf("TroubaStage" to "C:\\Somewhere\\Else")), "a copied folder")
            assertFalse(isMsiInstalled(exe, listOf("TroubaStage!" to dir.path)), "another product")
            assertFalse(isMsiInstalled(null, listOf("TroubaStage" to dir.path)), ":desktopApp:run has no launcher")
        } finally {
            dir.deleteRecursively()
        }
    }

    @Test
    fun theHelpersBody_isFixed_andUsesItsInputsOnlyInsideQuotes() {
        // TEETH (H1): an unquoted path expansion lets a "&" or ")" in a folder name run as a command.
        val lines = UPDATE_SCRIPT.split("\r\n")
        assertTrue(UPDATE_SCRIPT.endsWith("\r\n") && "\n" !in UPDATE_SCRIPT.replace("\r\n", ""), "CRLF lines")
        for (v in listOf(SCRIPT_ENV_MSI, SCRIPT_ENV_LOG, SCRIPT_ENV_EXE, SCRIPT_ENV_RESULT)) {
            val uses = Regex("""(.)%$v%(.)""").findAll(UPDATE_SCRIPT).toList()
            assertTrue(uses.isNotEmpty(), "$v is used")
            uses.forEach { m -> assertEquals("\"\"", m.groupValues[1] + m.groupValues[2], "$v only inside quotes: ${m.value}") }
        }
        assertTrue(lines.none { it.trimStart().startsWith("(") || it.trimEnd().endsWith("(") }, "no parenthesised blocks")
        assertTrue("DisableDelayedExpansion" in UPDATE_SCRIPT)
        val notClosed = lines.dropWhile { it != ":notclosed" }.takeWhile { it != ":closed" }
        assertTrue(notClosed.none { "msiexec" in it || "start" in it }, "H2: no install and no reopen when the app did not close")
        assertEquals(1, lines.count { "--after-update" in it })
        assertTrue(lines.single { "--after-update" in it }.startsWith("if \"%TS_CODE%\"==\"0\" "), "H3: only code 0 reopens the new build")
        // TEETH (5a403976): a failed install must not leave the performer without the app — any other code reopens the
        // unchanged build, AFTER the exits for 0 (already reopened) and 3010 (finishes at reboot).
        val closed = lines.dropWhile { it != ":closed" }
        val plainReopen = closed.indexOfFirst { it == "start \"\" \"%TS_EXE%\"" }
        assertTrue(plainReopen > 0, "a failed install reopens the unchanged build, with no flag")
        assertTrue(closed.indexOfFirst { it == "if \"%TS_CODE%\"==\"0\" exit /b 0" } in 0 until plainReopen, "code 0 exits before the plain reopen")
        assertTrue(closed.indexOfFirst { it == "if \"%TS_CODE%\"==\"3010\" exit /b 0" } in 0 until plainReopen, "3010 exits before the plain reopen")
        for (tool in listOf("tasklist", "find", "ping", "msiexec")) {
            assertTrue(lines.filter { Regex("""\b$tool(\.exe)?\b""").containsMatchIn(it) && !it.startsWith("rem ") }
                .all { "%SystemRoot%\\System32\\$tool.exe" in it }, "$tool by full path (cmd searches the working directory first)")
        }
    }
}
