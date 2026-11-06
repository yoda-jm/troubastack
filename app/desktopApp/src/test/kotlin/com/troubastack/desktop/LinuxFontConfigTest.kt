package com.troubastack.desktop

import java.io.File
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** A101 — which fontconfig file the Linux app runs with, and what it says. */
class LinuxFontConfigTest {
    private val dir = Files.createTempDirectory("fc").toFile()
    private val shipped = dir.resolve(LinuxFontConfig.SHIPPED).apply { writeText(LinuxFontConfig.wrapper(LinuxFontConfig.SYSTEM)) }
    private val written = ArrayList<String>()
    private val out = dir.resolve("out")
    private val write: (String) -> File = { include -> written += include; LinuxFontConfig.writeWrapper(include, out) }

    @Test
    fun noOwnConfig_runsWithTheShippedFile_writingNothing() {
        for (own in listOf(null, "")) {
            val (f, how) = LinuxFontConfig.choose(own = own, shipped = shipped, write = write).getOrThrow()
            assertEquals(shipped, f)
            assertTrue("FONTCONFIG_FILE unset" in how, how)
        }
        assertEquals(emptyList(), written)
    }

    @Test
    fun noOwnConfig_noShippedFile_wrapsTheSystemsOwn() {
        val (f, _) = LinuxFontConfig.choose(own = null, shipped = dir.resolve("absent.conf"), write = write).getOrThrow()
        assertEquals(listOf("fonts.conf"), written)
        assertTrue("<include ignore_missing=\"yes\">fonts.conf</include>" in f.readText())
    }

    @Test
    fun ownConfig_isWrapped_soTheirsStillApplies() {
        // TEETH: replacing their FONTCONFIG_FILE with the shipped one would drop the person's own font setup.
        val own = dir.resolve("fonts & more.conf").apply { writeText("<fontconfig><dir>/opt/my-fonts</dir></fontconfig>\n") }
        val (f, how) = LinuxFontConfig.choose(own = own.path, shipped = shipped, write = write).getOrThrow()
        val xml = f.readText()
        assertTrue("<include ignore_missing=\"yes\">${own.path.replace("&", "&amp;")}</include>" in xml, xml)
        assertTrue("<patelt name=\"scalable\"><bool>false</bool></patelt>" in xml, xml)
        assertTrue(own.path in how, how)
    }

    @Test
    fun aMissingAbsoluteFile_isNotWrapped_theImagesFileIsUsed() {
        // TEETH: wrapped, a missing file includes nothing — fontconfig then has no fonts at all (Fable measured 0, where
        // the missing FONTCONFIG_FILE alone falls back to fontconfig's defaults).
        val missing = dir.resolve("no-such-fonts.conf").path
        val (f, how) = LinuxFontConfig.choose(own = missing, shipped = shipped, write = write).getOrThrow()
        assertEquals(shipped, f)
        assertTrue("$missing is not a readable file" in how, how)
        assertEquals(emptyList(), written)
        // …and without the image's file, the system's configuration (not the missing one) is wrapped.
        LinuxFontConfig.choose(own = missing, shipped = dir.resolve("absent.conf"), write = write).getOrThrow()
        assertEquals(listOf(LinuxFontConfig.SYSTEM), written)
    }

    @Test
    fun aMissingPathToOneOfOurs_inheritedFromAnEarlierImage_usesThisImagesFile() {
        val gone = dir.resolve("unmounted/.mount_TroubaXYZ/lib/app/resources/${LinuxFontConfig.SHIPPED}").path
        assertEquals(shipped, LinuxFontConfig.choose(own = gone, shipped = shipped, write = write).getOrThrow().first)
        assertEquals(emptyList(), written)
    }

    @Test
    fun aRelativeName_isWrappedAsItIs() {
        // fontconfig resolves a relative FONTCONFIG_FILE through FONTCONFIG_PATH, so it can't be checked here.
        val (f, _) = LinuxFontConfig.choose(own = "my-fonts.conf", shipped = shipped, write = write).getOrThrow()
        assertEquals(listOf("my-fonts.conf"), written)
        assertTrue("<include ignore_missing=\"yes\">my-fonts.conf</include>" in f.readText())
    }

    @Test
    fun aProgramTheAppStarts_getsFontconfigFileAsAtLaunch() {
        // TEETH: a ProcessBuilder whose environment is left alone passes on the process's own, setenv included.
        val unset = LinuxFontConfig.childEnvironment(ProcessBuilder("true").apply { environment()["FONTCONFIG_FILE"] = "/ours" }, launch = null)
        assertTrue("FONTCONFIG_FILE" !in unset.environment(), "${unset.environment()["FONTCONFIG_FILE"]}")
        val theirs = LinuxFontConfig.childEnvironment(ProcessBuilder("true").apply { environment()["FONTCONFIG_FILE"] = "/ours" }, launch = "/theirs.conf")
        assertEquals("/theirs.conf", theirs.environment()["FONTCONFIG_FILE"])
    }

    @Test
    fun ourOwnFile_inheritedByARelaunch_isKept_notWrappedInItself() {
        val ours = LinuxFontConfig.writeWrapper("/somewhere/fonts.conf", out)
        assertEquals(ours, LinuxFontConfig.choose(own = ours.path, shipped = shipped, write = write).getOrThrow().first)
        assertEquals(shipped, LinuxFontConfig.choose(own = shipped.path, shipped = shipped, write = write).getOrThrow().first)
        assertEquals(emptyList(), written)
    }

    @Test
    fun theWrapper_isOneFile_replacedWhole_andMadeWhereItsFolderIsMissing() {
        // TEETH: an unset XDG_RUNTIME_DIR once sent the wrapper through Files.createTempFile(null, …), which threw; the
        // swallowed error left the fix off. The folder here doesn't exist yet (a first launch's data root).
        val a = LinuxFontConfig.writeWrapper("/home/someone/a.conf", out)
        val b = LinuxFontConfig.writeWrapper("/home/someone/b.conf", out)
        assertEquals(out.resolve(LinuxFontConfig.WRAPPER), a)
        assertEquals(a, b)
        assertEquals(LinuxFontConfig.wrapper("/home/someone/b.conf"), b.readText())
        assertEquals(listOf(LinuxFontConfig.WRAPPER), out.list()!!.sorted())   // no temp file left beside it
    }

    @Test
    fun noWrapperWritable_isAFailure_soTheAppSaysSo() {
        val notADir = dir.resolve("a-file").apply { writeText("x") }
        val own = dir.resolve("their-fonts.conf").apply { writeText("<fontconfig/>\n") }
        val r = LinuxFontConfig.choose(own = own.path, shipped = shipped) { LinuxFontConfig.writeWrapper(it, notADir) }
        assertTrue(r.isFailure, "$r")
    }

    @Test
    fun theShippedFileIsTheSystemWrapper() {
        // The committed desktopApp/fonts file is exactly the wrapper of the system's configuration.
        val committed = File(System.getProperty("user.dir"), "fonts/${LinuxFontConfig.SHIPPED}")
        assertTrue(committed.isFile, committed.path)
        // (A Windows checkout may turn its line ends into CRLF; it ships in the Linux image only.)
        assertEquals(LinuxFontConfig.wrapper(LinuxFontConfig.SYSTEM), committed.readText().replace("\r\n", "\n"))
    }
}
