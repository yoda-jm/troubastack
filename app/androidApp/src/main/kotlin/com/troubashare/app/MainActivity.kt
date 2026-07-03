package com.troubashare.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.troubashare.shared.bundle.BundleLoader
import com.troubashare.shared.stage.ImageDecoder
import com.troubashare.shared.stage.StageScreen
import com.troubashare.shared.stage.StageViewModel

/**
 * The thin Android entrypoint (I15). It only navigates between a Concerts list and the shared
 * [StageScreen] and wires the platform bits Stage needs (image decoder, window behaviours) — no
 * presenter logic lives here; that is all in :shared. No account, no auth: Stage is reachable from
 * launch (I12). Import/downloads are A05; this ships fixture bundles from assets as a stopgap.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { MaterialTheme { App() } }
    }
}

private class OpenedBundle(val vm: StageViewModel, val decoder: ImageDecoder)

@Composable
private fun App() {
    var selected by remember { mutableStateOf<DemoBundle?>(null) }
    val current = selected

    if (current == null) {
        ConcertsList(onOpen = { selected = it })
        return
    }

    val context = LocalContext.current
    // Stopgap: copy + load on open. Fixtures are tiny; heavy page decoding happens off-thread in Stage.
    val opened = remember(current) {
        val dir = copyBundleToCache(context, current.assetPath)
        val result = BundleLoader().load(dir.path, FileBundleFiles())
        OpenedBundle(StageViewModel(result), AndroidImageDecoder(dir))
    }
    StageHost {
        StageScreen(opened.vm, opened.decoder, onExit = { selected = null })
    }
    BackHandler { selected = null }
}

@Composable
private fun ConcertsList(onOpen: (DemoBundle) -> Unit) {
    Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(Modifier.fillMaxSize().statusBarsPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("Concerts", style = MaterialTheme.typography.headlineMedium)
            LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                items(DEMO_BUNDLES) { bundle ->
                    Card(onClick = { onOpen(bundle) }, modifier = Modifier.fillMaxWidth()) {
                        Text(bundle.label, Modifier.padding(16.dp), style = MaterialTheme.typography.titleMedium)
                    }
                }
            }
        }
    }
}
