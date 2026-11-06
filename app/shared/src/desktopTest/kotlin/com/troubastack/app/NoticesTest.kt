package com.troubastack.app

import kotlin.test.Test
import kotlin.test.assertEquals

/** A101 — the Licences screen's entries come from the build's notices index, in its order. */
class NoticesTest {
    private val index = """
        # generated
        entry	Ktor
        component	io.ktor:ktor-client-core-jvm:3.0.3	Apache-2.0	Apache-2.0	
        component	io.ktor:ktor-http-jvm:3.0.3	Apache-2.0	Apache-2.0	
        doc	Apache License 2.0	aaaa
        entry	OkHttp and Okio
        component	com.squareup.okhttp3:okhttp:4.12.0	Apache-2.0,MPL-2.0	Apache-2.0; publicsuffixes.gz (the Public Suffix List) MPL-2.0	nnnn
        doc	okhttp: okhttp3/internal/publicsuffix/NOTICE	nnnn
        doc	Mozilla Public License 2.0	missing
        entry	libffi (in JNA)
        carrier	net.java.dev.jna:jna:5.17.0
        doc	libffi	ffff
        delegated	org.eclipse.platform:org.eclipse.swt.win32.win32.x86_64:3.134.0	EPL-2.0	SWT
    """.trimIndent()

    @Test
    fun entries_withTheirComponentsAndTexts_inIndexOrder() {
        val texts = mapOf("aaaa" to "Apache License", "nnnn" to "MPL-2.0 data", "ffff" to "libffi licence")
        val entries = parseNotices(index) { texts[it] }
        assertEquals(listOf("Ktor", "OkHttp and Okio", "libffi (in JNA)"), entries.map { it.name })
        assertEquals(listOf("Components", "Apache License 2.0"), entries[0].documents.map { it.title })
        assertEquals("io.ktor:ktor-client-core-jvm 3.0.3 — Apache-2.0\nio.ktor:ktor-http-jvm 3.0.3 — Apache-2.0", entries[0].documents[0].text)
        // A text the build didn't ship is left out rather than shown empty; the delegated SWT line is no entry.
        assertEquals(listOf("Components", "okhttp: okhttp3/internal/publicsuffix/NOTICE"), entries[1].documents.map { it.title })
        assertEquals("com.squareup.okhttp3:okhttp 4.12.0 — Apache-2.0; publicsuffixes.gz (the Public Suffix List) MPL-2.0", entries[1].documents[0].text)
        assertEquals(listOf("Inside", "libffi"), entries[2].documents.map { it.title })
        assertEquals("net.java.dev.jna:jna 5.17.0", entries[2].documents[0].text)
    }
}
