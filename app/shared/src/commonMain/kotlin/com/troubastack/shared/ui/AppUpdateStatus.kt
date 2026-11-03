package com.troubastack.shared.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.troubastack.shared.buildinfo.UpdatePhase
import com.troubastack.shared.buildinfo.updatePhaseLine

/**
 * A96 — an app update in progress, the same on Home's banner and in Parameters → About: the phase's line, Cancel while
 * the app still owns the step (downloading, checking), and a bar under it — the download's progress, an indeterminate
 * one while checking, none while Android owns the step (we don't pretend to know how long its confirmation takes).
 */
@Composable
fun AppUpdateStatus(phase: UpdatePhase, onCancel: () -> Unit, textColor: Color, modifier: Modifier = Modifier) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(updatePhaseLine(phase), Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium, color = textColor)
            if (phase !is UpdatePhase.Confirming) TextButton(onClick = onCancel) { Text("Cancel") }
        }
        when (phase) {
            is UpdatePhase.Downloading -> LinearProgressIndicator(progress = { phase.fraction }, modifier = Modifier.fillMaxWidth())
            UpdatePhase.Checking -> LinearProgressIndicator(Modifier.fillMaxWidth())
            is UpdatePhase.Confirming -> {}
        }
    }
}
