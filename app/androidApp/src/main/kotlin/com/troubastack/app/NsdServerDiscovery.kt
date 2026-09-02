package com.troubastack.app

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import com.troubastack.shared.distribution.DiscoveredServer
import com.troubastack.shared.distribution.ServerDiscovery
import com.troubastack.shared.distribution.sortedDiscovered
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlin.coroutines.resume

private const val SERVICE_TYPE = "_troubacore._tcp."

/**
 * B06 — Android LAN discovery of `_troubacore._tcp` via the built-in [NsdManager] (no new dependency,
 * no I15 seam — connectivity glue like `HttpTransport`). Discovery runs ONLY while [servers] is
 * collected, so tying it to the Connect screen's composition starts it on open and stops it on close
 * (no background scanning). Results are a convenience prefill only (see [ServerDiscovery]).
 *
 * A Wi-Fi multicast lock is held for the discovery's lifetime — many devices drop multicast (hence
 * mDNS) packets to save power unless a lock is held. All mutations of the found-set are serialized on
 * [resolveMutex] because NsdManager resolves one service at a time and its callbacks arrive off the
 * flow's coroutine.
 */
class NsdServerDiscovery(context: Context) : ServerDiscovery {
    private val appContext = context.applicationContext

    override fun servers(): Flow<List<DiscoveredServer>> = callbackFlow {
        val nsd = appContext.getSystemService(Context.NSD_SERVICE) as NsdManager
        val wifi = appContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        val lock = wifi.createMulticastLock("troubacore-mdns").apply {
            setReferenceCounted(false)
            runCatching { acquire() }
        }

        val found = LinkedHashMap<String, DiscoveredServer>() // key = service instance name
        val resolveMutex = Mutex()                            // NsdManager resolves one at a time

        val listener = object : NsdManager.DiscoveryListener {
            override fun onStartDiscoveryFailed(type: String, err: Int) { close() }
            override fun onStopDiscoveryFailed(type: String, err: Int) {}
            override fun onDiscoveryStarted(type: String) {}
            override fun onDiscoveryStopped(type: String) {}

            override fun onServiceFound(info: NsdServiceInfo) = launchLocked {
                val resolved = resolve(nsd, info) ?: return@launchLocked
                @Suppress("DEPRECATION") // host/resolveService: broad-compat path (minSdk 26)
                val host = resolved.host?.hostAddress ?: return@launchLocked
                found[resolved.serviceName] = DiscoveredServer(resolved.serviceName, host, resolved.port)
                trySend(sortedDiscovered(found.values))
            }

            override fun onServiceLost(info: NsdServiceInfo) = launchLocked {
                if (found.remove(info.serviceName) != null) trySend(sortedDiscovered(found.values))
            }

            private fun launchLocked(block: suspend () -> Unit) {
                launch { resolveMutex.withLock { block() } }
            }
        }

        runCatching { nsd.discoverServices(SERVICE_TYPE, NsdManager.PROTOCOL_DNS_SD, listener) }
            .onFailure { close(it) }

        awaitClose {
            runCatching { nsd.stopServiceDiscovery(listener) }
            runCatching { if (lock.isHeld) lock.release() }
        }
    }
}

/** Bridge NsdManager's callback-style resolve to a suspend call; null on resolve failure. */
private suspend fun resolve(nsd: NsdManager, info: NsdServiceInfo): NsdServiceInfo? =
    suspendCancellableCoroutine { cont ->
        @Suppress("DEPRECATION") // resolveService is the minSdk-26-compatible resolve path
        nsd.resolveService(info, object : NsdManager.ResolveListener {
            override fun onResolveFailed(si: NsdServiceInfo, err: Int) { if (cont.isActive) cont.resume(null) }
            override fun onServiceResolved(si: NsdServiceInfo) { if (cont.isActive) cont.resume(si) }
        })
    }
