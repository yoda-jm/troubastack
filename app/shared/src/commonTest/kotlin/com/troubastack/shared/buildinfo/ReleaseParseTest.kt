package com.troubastack.shared.buildinfo

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * A88/WIN04 — reading the GitHub release JSON for a platform's asset (moved here from androidApp's AppUpdaterParseTest
 * when Windows got its own updater). The fixture is the real `latest` response (2026-10-12), trimmed to the fields read
 * plus a neighbour asset, so a renamed or reordered field shows up here, not on a tablet or a PC.
 */
class ReleaseParseTest {
    private val real = """
        {"tag_name":"latest","target_commitish":"cad74786de57d53a8f2f79176167b72cd9feb600",
         "published_at":"2026-10-12T01:05:12Z",
         "body":"Debug-signed Android APK — rolling build of `main`.\n\nCommit: cad74786de57d53a8f2f79176167b72cd9feb600\nBuilt:  2026-10-12T01:05:08Z\n",
         "assets":[{"name":"notes.txt","size":3,"browser_download_url":"https://example.invalid/notes.txt"},
                   {"name":"troubastage-debug.apk","size":15753501,
                    "browser_download_url":"https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk"}]}
    """.trimIndent()

    /** The release as it is published since WIN03 D8: the MSI next to the APK, each with GitHub's sha256 digest. */
    private val twoAssets = real.replace(
        "\"assets\":[",
        "\"assets\":[{\"name\":\"troubastage-windows.msi\",\"size\":68879068,\"digest\":\"sha256:D7926292D3263BFCC25FB231C3A1CCF1D766F1CA59A48696D52237BB834E991B\"," +
            "\"browser_download_url\":\"https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-windows.msi\"},",
    )

    @Test
    fun the_real_release_parses_to_its_commit_time_and_apk() {
        assertEquals(
            LatestRelease(
                commit = "cad74786de57d53a8f2f79176167b72cd9feb600", publishedAt = "2026-10-12T01:05:12Z",
                assetUrl = "https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk", assetSize = 15_753_501,
            ),
            parseRelease(real, UPDATE_ASSET_APK),
        )
    }

    @Test
    fun a_branch_name_as_target_falls_back_to_the_body_commit() {
        val r = parseRelease(real.replace("\"target_commitish\":\"cad74786de57d53a8f2f79176167b72cd9feb600\"", "\"target_commitish\":\"main\""), UPDATE_ASSET_APK)
        assertEquals("cad74786de57d53a8f2f79176167b72cd9feb600", r?.commit)
    }

    @Test
    fun with_both_assets_each_platform_gets_its_own() {
        // TEETH: a parser that takes the first asset offers the MSI (listed first here) to Android.
        assertEquals(true, twoAssets.indexOf(UPDATE_ASSET_MSI) < twoAssets.indexOf(UPDATE_ASSET_APK), "fixture: the MSI comes first")
        val apk = parseRelease(twoAssets, UPDATE_ASSET_APK)
        assertEquals("https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk", apk?.assetUrl)
        assertEquals(15_753_501, apk?.assetSize)
        val msi = parseRelease(twoAssets, UPDATE_ASSET_MSI)
        assertEquals("https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-windows.msi", msi?.assetUrl)
        assertEquals(68_879_068, msi?.assetSize)
        assertEquals("d7926292d3263bfcc25fb231c3a1ccf1d766f1ca59a48696d52237bb834e991b", msi?.assetSha256, "the digest, lowercase hex")
        assertEquals(apk?.commit, msi?.commit)
    }

    @Test
    fun with_every_platforms_asset_each_platform_gets_its_own() {
        // DESK01 D4 / DESK02 — the release with the AppImage and the DMG: each OS picks its asset by name.
        val all = twoAssets.replace(
            "\"assets\":[",
            "\"assets\":[{\"name\":\"$UPDATE_ASSET_DMG\",\"size\":115000000,\"browser_download_url\":\"https://github.com/yoda-jm/troubastack/releases/download/latest/$UPDATE_ASSET_DMG\"}," +
                "{\"name\":\"$UPDATE_ASSET_APPIMAGE\",\"size\":120000000,\"digest\":\"sha256:${"a".repeat(64)}\"," +
                "\"browser_download_url\":\"https://github.com/yoda-jm/troubastack/releases/download/latest/$UPDATE_ASSET_APPIMAGE\"},",
        )
        val sizes = listOf(UPDATE_ASSET_APK, UPDATE_ASSET_MSI, UPDATE_ASSET_APPIMAGE, UPDATE_ASSET_DMG).map { parseRelease(all, it)?.assetSize }
        assertEquals(listOf(15_753_501L, 68_879_068L, 120_000_000L, 115_000_000L), sizes)
        assertEquals("a".repeat(64), parseRelease(all, UPDATE_ASSET_APPIMAGE)?.assetSha256)
        assertNull(parseRelease(twoAssets, UPDATE_ASSET_DMG), "no DMG on latest (until a person has tested it): no update for macOS")
        assertNull(parseRelease(twoAssets, UPDATE_ASSET_APPIMAGE))
    }

    @Test
    fun a_release_with_only_the_apk_is_no_update_for_windows_not_an_error() {
        assertNull(parseRelease(real, UPDATE_ASSET_MSI))
    }

    @Test
    fun no_apk_asset_means_no_release_for_android() {
        assertNull(parseRelease(real.replace("troubastage-debug.apk", "something-else.apk"), UPDATE_ASSET_APK))
    }

    @Test
    fun an_asset_without_a_digest_has_none() {
        assertNull(parseRelease(real, UPDATE_ASSET_APK)?.assetSha256)
    }
}
