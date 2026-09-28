package com.troubastack.app

import com.troubastack.shared.stage.ArmedSendVerdict
import com.troubastack.shared.stage.SendResultKind
import com.troubastack.shared.stage.armedSendOverwrite
import com.troubastack.shared.stage.armedSendVerdict
import com.troubastack.shared.stage.notes.NoteEntry
import com.troubastack.shared.stage.notes.NoteKey
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** After the last stroke of a burst, how long to wait before sending — coalesces a note that grows stroke by
 *  stroke into one upload (§7: "debounce after the last stroke"), not one per stroke. */
private const val ARMED_SEND_DEBOUNCE_MS = 1200L

/** After a failed send/delete, how long before trying again. The window stays armed (offline is the case A77
 *  lives in); the banner says "last send failed, retrying" until one succeeds. */
private const val ARMED_RETRY_MS = 15_000L

/**
 * A77 §7 — the host's "WHEN" of armed auto-upload. It debounces per-note commits and flushes on demand; the
 * "whether" (overwrite, the 409 verdict) is the shared, tested [armedSendOverwrite]/[armedSendVerdict], per
 * Fable's line. Not armed ⇒ [onCommitted] is a no-op.
 *
 * Reconcile reads the stored bytes back so a draw-then-erase in one burst resolves to the final state: present
 * ⇒ auto-send (the FIRST send of a note is non-overwrite so a note we did not put 409s; ours ⇒ overwrite),
 * gone ⇒ mirror the delete (§5). A CONFLICT disarms via [onConflict]; a network FAILED stays armed — the
 * offline case A77 is built around must not be punished as a conflict.
 */
class ArmedUploader(
    private val scope: CoroutineScope,
    private val transport: HttpTransport,
    private val notes: AndroidRehearsalNotes,
    private val concertId: String,
    private val isArmed: () -> Boolean,
    private val onConflict: (String) -> Unit,
    private val onStatus: (String) -> Unit,
    // A77 (Fable conditional GO) — each outcome, so the PERSISTENT banner reflects it: true = failed, retrying.
    private val onHealth: (Boolean) -> Unit,
) {
    // The keys this session has already auto-sent — so their next send overwrites OUR OWN prior send, and a
    // first send stays non-overwrite (a foreign note surfaces as a 409 instead of being clobbered).
    private val sentThisSession = HashSet<NoteKey>()
    private var pending: NoteEntry? = null
    private var debounce: Job? = null

    /** A pen-up commit OR a clear (§7). No-op unless armed. Coalesces a stroke burst into one reconcile. */
    fun onCommitted(entry: NoteEntry) {
        if (!isArmed()) return
        schedule(entry, ARMED_SEND_DEBOUNCE_MS)
    }

    private fun schedule(entry: NoteEntry, afterMs: Long) {
        pending = entry
        debounce?.cancel()
        debounce = scope.launch {
            delay(afterMs)
            reconcile()
        }
    }

    /** A failed attempt: say so on the banner and try again later — but only while still armed and only if no
     *  newer commit has superseded it (that one carries the current bytes anyway). After disarm the final
     *  flush was the last attempt; the note stays local and the Notes tab's Send is the fallback. */
    private fun retryLater(entry: NoteEntry) {
        onHealth(true)
        if (isArmed() && pending == null) schedule(entry, ARMED_RETRY_MS)
    }

    /** Send any debounced note NOW — the flush at a page turn / leaving note mode / disarm / expiry (§7). Sends
     *  even if [isArmed] has just gone false: this is the tail of the armed session, scheduled while armed. */
    fun flush() {
        debounce?.cancel(); debounce = null
        if (pending != null) scope.launch { reconcile() }
    }

    private suspend fun reconcile() {
        val entry = pending ?: return
        pending = null
        val png = withContext(Dispatchers.IO) { notes.pngBytes(concertId, entry.key) }
        if (png != null) {
            val overwrite = armedSendOverwrite(entry.key in sentThisSession)
            val kind = when (transport.sendRehearsalNote(concertId, entry, png, overwrite)) {
                is NoteSendResult.Ok -> SendResultKind.OK
                is NoteSendResult.Exists -> SendResultKind.CONFLICT
                is NoteSendResult.Failed -> SendResultKind.FAILED
            }
            when (armedSendVerdict(kind)) {
                ArmedSendVerdict.SENT -> { sentThisSession.add(entry.key); onHealth(false) }
                ArmedSendVerdict.CONFLICT_DISARM -> onConflict("A note is already in Studio for this page — auto-upload off")
                ArmedSendVerdict.FAILED -> retryLater(entry) // stay armed; the banner carries it (not a toast)
            }
        } else {
            // The commit cleared the note → mirror the delete (§5). In Stage a cleared page is always live, so
            // this only fires for the population §5 admits; an orphan clear happens outside Stage (disarmed).
            sentThisSession.remove(entry.key)
            val gone = withContext(Dispatchers.IO) { transport.deleteRehearsalNote(concertId, entry) }
            onStatus(if (gone) "Removed here and in Studio" else "Removed here — couldn't reach Studio")
            if (gone) onHealth(false) else retryLater(entry) // a surviving Studio copy is a broken promise too
        }
    }
}
