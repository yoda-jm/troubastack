package com.troubashare.shared.stage

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.troubashare.shared.bundle.BundleMember

/**
 * P205 Stage 3a-ii — resolving the viewer's identity for a concert (a LOCAL view preference, I12; no
 * account required). Precedence: a previously-stored pick (still in the roster) wins; else auto-match
 * the logged-in user against the roster; else none ("" ⇒ show the "Who are you?" picker). Pure.
 */
fun resolveIdentity(roster: List<BundleMember>, stored: String?, autoUserId: String = ""): String {
    if (!stored.isNullOrEmpty() && roster.any { it.memberId == stored }) return stored
    if (autoUserId.isNotEmpty() && roster.any { it.memberId == autoUserId }) return autoUserId
    return ""
}

/** The picker is needed only when the bundle HAS a roster (P205 band-wide) but no identity resolved.
 *  An old/-mine/anonymous bundle (no roster) never prompts — Perform stays one-tap (I12). */
fun needsIdentityPick(roster: List<BundleMember>, resolved: String): Boolean =
    roster.isNotEmpty() && resolved.isEmpty()

/**
 * "Who are you?" — one tap to pick your part in the roster so the stage shows YOUR layers + cues.
 * Remembered per concert by the host. Dismiss ("Not now") plays anonymous (shared/mandatory only).
 */
@Composable
fun WhoAreYouDialog(roster: List<BundleMember>, onPick: (String) -> Unit, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = {},
        dismissButton = { TextButton(onClick = onDismiss) { Text("Not now") } },
        title = { Text("Who are you?") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    "Pick your part so the stage shows your layers and cues. Remembered for this concert.",
                    style = MaterialTheme.typography.bodySmall,
                )
                roster.forEach { m ->
                    Surface(
                        onClick = { onPick(m.memberId) },
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.small,
                        color = MaterialTheme.colorScheme.secondaryContainer,
                    ) {
                        Text(
                            memberLabel(m),
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                        )
                    }
                }
            }
        },
    )
}

/** A roster member's display label: name, with the role appended for non-plain members (admin/conductor). */
internal fun memberLabel(m: BundleMember): String =
    if (m.role.isNotEmpty() && m.role != "member") "${m.displayName} · ${m.role}" else m.displayName
