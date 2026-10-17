package com.troubastack.app

import com.troubastack.shared.buildinfo.LatestRelease
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** A88 — reading the GitHub release JSON. The fixture is the real `latest` response (2026-10-12), trimmed to the
 *  fields read plus a neighbour asset, so a renamed or reordered field shows up here, not on a tablet. */
class AppUpdaterParseTest {
    private val real = """
        {"tag_name":"latest","target_commitish":"cad74786de57d53a8f2f79176167b72cd9feb600",
         "published_at":"2026-10-12T01:05:12Z",
         "body":"Debug-signed Android APK — rolling build of `main`.\n\nCommit: cad74786de57d53a8f2f79176167b72cd9feb600\nBuilt:  2026-10-12T01:05:08Z\n",
         "assets":[{"name":"notes.txt","size":3,"browser_download_url":"https://example.invalid/notes.txt"},
                   {"name":"troubastage-debug.apk","size":15753501,
                    "browser_download_url":"https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk"}]}
    """.trimIndent()

    @Test
    fun the_real_release_parses_to_its_commit_time_and_apk() {
        assertEquals(
            LatestRelease(
                commit = "cad74786de57d53a8f2f79176167b72cd9feb600", publishedAt = "2026-10-12T01:05:12Z",
                apkUrl = "https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk", apkSize = 15_753_501,
            ),
            AppUpdater.parseRelease(real),
        )
    }

    @Test
    fun a_branch_name_as_target_falls_back_to_the_body_commit() {
        val r = AppUpdater.parseRelease(real.replace("\"target_commitish\":\"cad74786de57d53a8f2f79176167b72cd9feb600\"", "\"target_commitish\":\"main\""))
        assertEquals("cad74786de57d53a8f2f79176167b72cd9feb600", r?.commit)
    }

    @Test
    fun no_apk_asset_means_no_release() {
        assertNull(AppUpdater.parseRelease(real.replace("troubastage-debug.apk", "something-else.apk")))
    }
}
