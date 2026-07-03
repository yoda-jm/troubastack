package com.troubashare.app

import android.content.Context
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.troubashare.shared.bundle.BundleImporter
import com.troubashare.shared.bundle.BundleLoader
import com.troubashare.shared.bundle.ImportResult
import com.troubashare.shared.bundle.LoadResult
import com.troubashare.shared.seams.Storage
import com.troubashare.shared.stage.ImageDecoder
import com.troubashare.shared.stage.StageScreen
import com.troubashare.shared.stage.StageViewModel
import java.io.File
import java.util.UUID

/**
 * The thin Android entrypoint (I15). Navigates between a Concerts list (backed by the Storage seam's
 * bundlesDir) and the shared [StageScreen]. No account, no auth, no network — Stage runs offline
 * straight from launch (I12). Bundles arrive by importing a `.tstage` (A05), not from assets.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { MaterialTheme { App() } }
    }
}

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

private data class ConcertEntry(val dir: String, val label: String, val damaged: Boolean)

@Composable
private fun App() {
    val context = LocalContext.current.applicationContext
    val storage = remember { Storage(context) }
    var selectedDir by remember { mutableStateOf<String?>(null) }
    var editing by remember { mutableStateOf(false) }

    if (editing) {
        EditScreen(storage, onBack = { editing = false })
        return
    }

    val dir = selectedDir
    if (dir == null) {
        ConcertsScreen(context, storage, onOpen = { selectedDir = it }, onEdit = { editing = true })
        return
    }

    val opened = remember(dir) {
        OpenedBundle(
            StageViewModel(BundleLoader().load(dir, FileBundleFiles())),
            AndroidImageDecoder(File(dir)),
        )
    }
    StageHost {
        StageScreen(opened.vm, opened.decoder, onExit = { selectedDir = null })
    }
    BackHandler { selectedDir = null }
}

@Composable
private fun ConcertsScreen(context: Context, storage: Storage, onOpen: (String) -> Unit, onEdit: () -> Unit) {
    var refresh by remember { mutableStateOf(0) }
    var message by remember { mutableStateOf<String?>(null) }

    val picker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        if (uri != null) {
            val temp = copyToTemp(context, storage, uri)
            message = when (val r = BundleImporter(AndroidImportFs(storage)).import(temp)) {
                is ImportResult.Imported -> "Imported ${r.concertId}"
                is ImportResult.Failed -> r.reason
            }
            File(temp).delete()
            refresh++
        }
    }

    val entries = remember(refresh) { listConcerts(storage) }

    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().statusBarsPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("Concerts", style = MaterialTheme.typography.headlineMedium, modifier = Modifier.weight(1f))
                TextButton(onClick = onEdit) { Text("Edit") }
                // ".tstage" has no registered MIME type, so accept zip + anything and validate on import.
                Button(onClick = { picker.launch(arrayOf("application/zip", "application/octet-stream", "*/*")) }) {
                    Text("Import")
                }
            }
            message?.let { Text(it, style = MaterialTheme.typography.bodyMedium) }
            if (entries.isEmpty()) {
                Text("No concerts yet. Import a .tstage to get started.", style = MaterialTheme.typography.bodyMedium)
            }
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(entries) { entry ->
                    Card(
                        onClick = { if (!entry.damaged) onOpen(entry.dir) },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Row(Modifier.fillMaxWidth().padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
                            Text(entry.label, style = MaterialTheme.typography.titleMedium, modifier = Modifier.weight(1f))
                            if (entry.damaged) {
                                TextButton(onClick = { File(entry.dir).deleteRecursively(); refresh++ }) { Text("Delete") }
                            }
                        }
                    }
                }
            }
        }
    }
}

/** List installed bundles; a directory that fails to load is shown as damaged (with a delete action). */
private fun listConcerts(storage: Storage): List<ConcertEntry> {
    val dirs = File(storage.bundlesDir()).listFiles()?.filter { it.isDirectory }?.sortedBy { it.name } ?: emptyList()
    return dirs.map { d ->
        when (val r = BundleLoader().load(d.path, FileBundleFiles())) {
            is LoadResult.Loaded -> ConcertEntry(d.path, r.bundle.name.ifEmpty { r.bundle.concertId.ifEmpty { d.name } }, damaged = false)
            is LoadResult.Failed -> ConcertEntry(d.path, "Damaged bundle (${d.name})", damaged = true)
        }
    }
}

/** Copy a picked document stream into a temp `.tstage` the importer can read. */
private fun copyToTemp(context: Context, storage: Storage, uri: Uri): String {
    val temp = File(storage.tempDir(), "pick-${UUID.randomUUID()}.tstage")
    context.contentResolver.openInputStream(uri)?.use { input -> temp.outputStream().use { input.copyTo(it) } }
    return temp.path
}
