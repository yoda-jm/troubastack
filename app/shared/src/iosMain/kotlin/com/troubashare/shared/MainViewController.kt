// Generated proto types come from gen/ — single source of truth is proto/ (I1).
@file:OptIn(kotlinx.cinterop.ExperimentalForeignApi::class, kotlinx.cinterop.BetaInteropApi::class)

package com.troubashare.shared

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeContentPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.toComposeImageBitmap
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.ComposeUIViewController
import com.troubashare.shared.bundle.BundleFiles
import com.troubashare.shared.bundle.BundleLoader
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.stage.ImageDecoder
import com.troubashare.shared.stage.StageColorMode
import com.troubashare.shared.stage.StageScreen
import com.troubashare.shared.stage.StageViewModel
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.usePinned
import org.jetbrains.skia.Image
import platform.Foundation.NSDocumentDirectory
import platform.Foundation.NSData
import platform.Foundation.NSFileManager
import platform.Foundation.NSSearchPathForDirectoriesInDomains
import platform.Foundation.NSString
import platform.Foundation.NSUTF8StringEncoding
import platform.Foundation.NSUserDomainMask
import platform.Foundation.dataWithContentsOfFile
import platform.Foundation.stringWithContentsOfFile
import platform.Foundation.writeToFile
import platform.UIKit.UIApplication
import platform.posix.memcpy

/**
 * The thin iOS entrypoint (I15) — the iOS analog of `:androidApp`'s `MainActivity`. Exported in the
 * `Shared` framework and called from Swift (`SharedMainViewControllerKt.MainViewController()`); the
 * Xcode `iosApp` holds no logic beyond mounting this. Like the Android entrypoint, the Concerts list
 * + Stage navigation is written here (not commonMain) — only [StageScreen] itself is shared UI. The
 * decoder + file access below are plain DI, NOT new expect/actual seams (mirrors `AndroidImageDecoder`
 * / `FileBundleFiles`). Stage-only for v1 (IOS02); the WebView editor is out of scope.
 */
fun MainViewController(): platform.UIKit.UIViewController = ComposeUIViewController {
    MaterialTheme { App() }
}

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

private data class ConcertEntry(val dir: String, val label: String, val damaged: Boolean)

@Composable
private fun App() {
    val storage = remember { Storage() }
    val entries = remember { listConcerts(storage) }
    // TROUBA_AUTOPEN (set by the simulator CI via SIMCTL_CHILD_TROUBA_AUTOPEN) opens the first healthy
    // concert straight to Stage, so the job can screenshot a Stage page without driving Compose taps.
    var selectedDir by remember {
        mutableStateOf(if (autopenEnabled()) entries.firstOrNull { !it.damaged }?.dir else null)
    }

    val dir = selectedDir
    if (dir == null) {
        ConcertsScreen(entries, onOpen = { selectedDir = it })
        return
    }

    val opened = remember(dir) {
        val load = BundleLoader().load(dir, IosBundleFiles())
        // Smoke marker for the simulator CI job: proves a bundle actually loaded (not just a launch).
        if (load is LoadResult.Loaded) writeMarker("loaded:${load.bundle.concertId}")
        OpenedBundle(StageViewModel(load), IosImageDecoder(dir))
    }
    KeepScreenAwake()  // performance resilience (I13) — iOS analog of Android StageHost's FLAG_KEEP_SCREEN_ON
    StageScreen(
        opened.vm, opened.decoder, onExit = { selectedDir = null },
        initialColorMode = StageColorMode.parse(storage.getSecret("stage.colorMode")),
        onColorModeChange = { storage.putSecret("stage.colorMode", it.name) },
    )
}

/**
 * Hold the screen awake for the lifetime of the Stage screen — a stand-mounted iPad must not sleep
 * mid-song (I13). Scoped like Android's StageHost: set on enter, cleared on dispose (every exit path),
 * never app-wide. `idleTimerDisabled` is a main-thread UIApplication flag; Compose effects run there.
 */
@Composable
private fun KeepScreenAwake() {
    DisposableEffect(Unit) {
        UIApplication.sharedApplication.idleTimerDisabled = true
        onDispose { UIApplication.sharedApplication.idleTimerDisabled = false }
    }
}

private fun autopenEnabled(): Boolean =
    platform.Foundation.NSProcessInfo.processInfo.environment["TROUBA_AUTOPEN"] != null

@Composable
private fun ConcertsScreen(entries: List<ConcertEntry>, onOpen: (String) -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            Modifier.fillMaxSize().safeContentPadding().padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text("Concerts", style = MaterialTheme.typography.headlineMedium)
            if (entries.isEmpty()) {
                Text("No concerts yet.", style = MaterialTheme.typography.bodyMedium)
            }
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(entries) { entry ->
                    Card(
                        onClick = { if (!entry.damaged) onOpen(entry.dir) },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(
                            entry.label,
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.fillMaxWidth().padding(16.dp),
                        )
                    }
                }
            }
        }
    }
}

/** List installed bundles; a directory that fails to load shows as damaged (mirrors Android). */
private fun listConcerts(storage: Storage): List<ConcertEntry> {
    val root = storage.bundlesDir()
    @Suppress("UNCHECKED_CAST")
    val names = (NSFileManager.defaultManager.contentsOfDirectoryAtPath(root, null) as? List<String>)
        ?.sorted() ?: emptyList()
    return names.map { name ->
        val dir = "$root/$name"
        when (val r = BundleLoader().load(dir, IosBundleFiles())) {
            is LoadResult.Loaded -> ConcertEntry(dir, r.bundle.name.ifEmpty { r.bundle.concertId.ifEmpty { name } }, false)
            is LoadResult.Failed -> ConcertEntry(dir, "Damaged bundle ($name)", true)
        }
    }
}

/**
 * iOS image decoder for Stage — plain DI (I15), not a seam. Decodes a blob PNG via Skia to a Compose
 * [ImageBitmap]. Unlike Android it does not downsample (iOS demo pages are small); a future pass can
 * add Skia subsampling if large scans OOM. Total: any failure returns a failed Result, never throws.
 */
private class IosImageDecoder(private val root: String) : ImageDecoder {
    override fun decode(ref: String, targetW: Int, targetH: Int): Result<ImageBitmap> = runCatching {
        val bytes = readBytes("$root/$ref") ?: error("could not read $ref")
        Image.makeFromEncoded(bytes).toComposeImageBitmap()
    }
}

/** File-backed [BundleFiles] over `NSFileManager` — the loader passes absolute paths, read as-is. */
private class IosBundleFiles : BundleFiles {
    override fun exists(path: String): Boolean = NSFileManager.defaultManager.fileExistsAtPath(path)

    override fun readText(path: String): String? =
        NSString.stringWithContentsOfFile(path, NSUTF8StringEncoding, null)

    override fun sizeOf(path: String): Long {
        val attrs = NSFileManager.defaultManager.attributesOfItemAtPath(path, null) ?: return 0L
        val size = attrs[platform.Foundation.NSFileSize] as? platform.Foundation.NSNumber
        return size?.longLongValue ?: 0L
    }
}

private fun readBytes(path: String): ByteArray? {
    val data = NSData.dataWithContentsOfFile(path) ?: return null
    val size = data.length.toInt()
    if (size == 0) return ByteArray(0)
    val out = ByteArray(size)
    out.usePinned { pinned -> memcpy(pinned.addressOf(0), data.bytes, data.length) }
    return out
}

/** Write a small marker into Documents so the simulator CI job can assert a real bundle load. */
private fun writeMarker(text: String) {
    val docs = (NSSearchPathForDirectoriesInDomains(NSDocumentDirectory, NSUserDomainMask, true).firstOrNull() as? String)
        ?: return
    (text as NSString).writeToFile("$docs/stage-loaded.marker", atomically = true, encoding = NSUTF8StringEncoding, error = null)
}
