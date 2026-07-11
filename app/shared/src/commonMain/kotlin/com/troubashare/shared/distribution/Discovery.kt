package com.troubashare.shared.distribution

import kotlinx.coroutines.flow.Flow

/**
 * LAN server discovery (B06 — mDNS/DNS-SD `_troubacore._tcp`). SHARED Kotlin (commonMain): the common
 * layer sees only [DiscoveredServer] and [ServerDiscovery]; the platform backs discovery with
 * Android `NsdManager` / iOS `NWBrowser` as connectivity glue — NOT a new I15 seam (same status as
 * `HttpTransport`).
 *
 * Discovery is a convenience **prefill, never trust**: the user still taps a row (which shows
 * host:port) and still logs in — nothing auto-connects and no credential is sent without an explicit
 * tap. mDNS is unauthenticated, so a spoofed advertisement is possible; the risk equals mistyping a
 * URL, and the real mitigation is TLS (OPS01), not this discovery logic.
 */
data class DiscoveredServer(val name: String, val host: String, val port: Int) {
    /** The URL prefilled into the Connect screen's server field when the row is tapped. */
    val url: String get() = "http://$host:$port"

    /** The row label — always shows host:port so the user sees exactly where they'd connect. */
    val label: String get() = "$name — $host:$port"
}

/**
 * A source of the servers currently visible on the LAN. Discovery starts when [servers] is collected
 * and stops when collection ends, so tie it to the Connect screen's lifetime (don't scan in the
 * background). A platform without discovery returns a flow that emits an empty list.
 */
fun interface ServerDiscovery {
    fun servers(): Flow<List<DiscoveredServer>>
}

/**
 * Dedup + stable ordering of discovered servers (B06). A service can resolve more than once or on
 * several interfaces, so we dedup by [DiscoveredServer.url]; the order is by name (case-insensitive)
 * then host then port so the list stays stable as entries arrive and leave. Pure — this is the
 * UI-independent core the platform flows feed, and the part that's unit-tested (the platform
 * browse/resolve itself is verified manually per the spec).
 */
fun sortedDiscovered(servers: Collection<DiscoveredServer>): List<DiscoveredServer> =
    servers.distinctBy { it.url }
        .sortedWith(compareBy({ it.name.lowercase() }, { it.host }, { it.port }))
