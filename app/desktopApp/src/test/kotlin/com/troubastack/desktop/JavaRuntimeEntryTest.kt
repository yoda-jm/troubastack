package com.troubastack.desktop

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** A101 — the Java runtime entry reads the image's own legal/ in place: through its symlinks, each text once. */
class JavaRuntimeEntryTest {
    @Test
    fun symlinkedLegal_eachTextOnce_titledByItsFirstFile() {
        // Linux and macOS jlink images make most of legal/ symlinks to java.base's files.
        val home = Files.createTempDirectory("jre").toFile()
        val base = home.resolve("legal/java.base").apply { mkdirs() }
        val desktop = home.resolve("legal/java.desktop").apply { mkdirs() }
        base.resolve("LICENSE").writeText("The GNU General Public License (GPL)\n\"CLASSPATH\" EXCEPTION TO THE GPL\n")
        base.resolve("ASSEMBLY_EXCEPTION").writeText("OPENJDK ASSEMBLY EXCEPTION\n")
        Files.createSymbolicLink(desktop.resolve("LICENSE").toPath(), java.nio.file.Paths.get("../java.base/LICENSE"))
        Files.createSymbolicLink(desktop.resolve("ASSEMBLY_EXCEPTION").toPath(), java.nio.file.Paths.get("../java.base/ASSEMBLY_EXCEPTION"))
        desktop.resolve("libpng.md").writeText("## libpng v1.6\n")
        val entry = javaRuntimeEntry(home)!!
        // TEETH: keyed by path instead of content, the GPL and the assembly exception would each show twice.
        assertEquals(
            listOf("java.base/ASSEMBLY_EXCEPTION (and 1 more)", "java.base/LICENSE (and 1 more)", "java.desktop/libpng.md"),
            entry.documents.map { it.title },
        )
        assertEquals("The GNU General Public License (GPL)\n\"CLASSPATH\" EXCEPTION TO THE GPL\n", entry.documents[1].text)
        assert(entry.name.startsWith("Java runtime (OpenJDK ")) { entry.name }
    }

    @Test
    fun windowsPointerFiles_countAsTheFileTheyPointAt() {
        // On Windows jlink writes "Please see ..\\java.base\\LICENSE" (CRLF) where Linux makes a symlink.
        val home = Files.createTempDirectory("jre-win").toFile()
        val base = home.resolve("legal/java.base").apply { mkdirs() }
        val desktop = home.resolve("legal/java.desktop").apply { mkdirs() }
        base.resolve("LICENSE").writeText("The GNU General Public License (GPL)\n")
        desktop.resolve("LICENSE").writeText("Please see ..\\java.base\\LICENSE\r\n")
        desktop.resolve("ASSEMBLY_EXCEPTION").writeText("Please see ..\\java.base\\ASSEMBLY_EXCEPTION\r\n")   // target missing
        desktop.resolve("OUTSIDE").writeText("Please see ..\\..\\secret.txt\r\n")                      // leaves legal/
        home.resolve("secret.txt").writeText("not a licence\n")
        val entry = javaRuntimeEntry(home)!!
        // TEETH: read as it is, the pointer is a document of its own ("Please see …") and java.base/LICENSE stands alone.
        assertEquals(
            listOf("java.base/LICENSE (and 1 more)", "java.desktop/ASSEMBLY_EXCEPTION", "java.desktop/OUTSIDE"),
            entry.documents.map { it.title },
        )
        assertEquals("The GNU General Public License (GPL)\n", entry.documents[0].text)
        // Unresolvable pointers show as they are (the About smoke fails on any "Please see" document).
        assertEquals("Please see ..\\..\\secret.txt\r\n", entry.documents[2].text)
    }

    @Test
    fun noLegalFolder_noEntry() {
        assertNull(javaRuntimeEntry(null))
        assertNull(javaRuntimeEntry(Files.createTempDirectory("jre-without-legal").toFile()))
    }
}
