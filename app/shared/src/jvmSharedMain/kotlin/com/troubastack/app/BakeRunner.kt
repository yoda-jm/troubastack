// The one re-bake path for the app's lists (A91's ⋯, A92's edited list): kick the bake, then poll its progress to a
// terminal state (T103: the poll, not the POST, is the source of truth). Returns null on success, else the server's
// user-safe message. [onLabel] receives the live "Baking <song> — 3 of 12" line.
package com.troubastack.app

import com.troubastack.shared.distribution.BakeStatus
import com.troubastack.shared.distribution.bakePollStep
import kotlinx.coroutines.delay
import kotlin.coroutines.coroutineContext
import kotlinx.coroutines.isActive

internal suspend fun runReBake(transport: HttpTransport, concertId: String, onLabel: (String) -> Unit): String? {
    val bakeId = java.util.UUID.randomUUID().toString()
    val kickErr = runCatching { transport.reBake(concertId, bakeId) }.getOrElse { "Couldn't reach the server" }
    if (kickErr != null) return kickErr
    var step = bakePollStep(null)
    var polls = 0
    while (!step.done && coroutineContext.isActive && polls < 600) { // ~10 min backstop; real bakes end far sooner
        delay(1000); polls++
        step = bakePollStep(runCatching { transport.bakeProgress(concertId, bakeId) }.getOrNull())
        (step.status as? BakeStatus.Baking)?.let { onLabel(it.label) }
    }
    return (step.status as? BakeStatus.Failed)?.message
}
