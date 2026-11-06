package com.troubastack.desktop

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** WIN08 S2.8 — About's licences come from the folder the Windows image ships, and only when it is there. */
class LicencesTest {
    private fun folder(notice: String? = "SWT\n  Version: 3.134.0\n  Bundle-Version: 3.134.0.v1\n") =
        Files.createTempDirectory("licences").toFile().apply {
            notice?.let { resolve(SWT_NOTICE).writeText(it) }
            resolve("EPL-2.0.txt").writeText("Eclipse Public License - v 2.0\n…")
            resolve("about_files").mkdirs()
            resolve("about_files/WebView2_LICENSE.txt").writeText("Copyright (C) Microsoft Corporation.")
            resolve("about_files/NEW_NOTICE.txt").writeText("a later SWT's extra notice")
        }

    @Test
    fun noFolderOrNoNotice_noLine() {
        // TEETH: a line without the shipped notice would show on Linux/macOS (no SWT there) — these go red.
        assertNull(shippedSwtLicences(null))
        assertNull(shippedSwtLicences(folder(notice = null)))
        assertNull(shippedSwtLicences(folder(notice = "no version line here")))
    }

    @Test
    fun theLineNamesTheNoticesVersion_andTheEntryListsTheShippedTexts() {
        val l = shippedSwtLicences(folder())!!
        assertEquals("Studio in this app is shown with SWT 3.134.0, open-source software under the Eclipse Public License 2.0.", l.line)
        val entries = l.entries()
        assertEquals(listOf("SWT 3.134.0"), entries.map { it.name })
        // In reading order; the absent files (SWT-about.txt, IJG_README, mpl-v11.txt) are left out; an unknown
        // about_files text is shown under its own name.
        assertEquals(
            listOf("SWT in this app", "Eclipse Public License 2.0", "Microsoft WebView2 loader licence", "NEW_NOTICE.txt"),
            entries.single().documents.map { it.title },
        )
    }

    @Test
    fun theVersionIsTheNoticesVersionLine() {
        assertEquals("3.134.0", swtNoticeVersion("x\n  Version: 3.134.0\n  Bundle-Version: 3.134.0.v20260515-1429\n"))
        assertNull(swtNoticeVersion("Bundle-Version: 3.134.0.v1"))
    }
}
