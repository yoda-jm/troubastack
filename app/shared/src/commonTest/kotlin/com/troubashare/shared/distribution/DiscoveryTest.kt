package com.troubashare.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals

/** B06 — the pure discovery core: URL/label derivation and dedup/stable-ordering of found servers. */
class DiscoveryTest {

    @Test
    fun url_andLabel_alwaysShowHostPort() {
        val s = DiscoveredServer(name = "Rehearsal Mac", host = "192.168.1.23", port = 8080)
        assertEquals("http://192.168.1.23:8080", s.url)
        assertEquals("Rehearsal Mac — 192.168.1.23:8080", s.label)
    }

    @Test
    fun sortedDiscovered_dedupsByUrl_keepingOne() {
        // The same instance resolved twice (e.g. a re-announce) must collapse to a single row.
        val a = DiscoveredServer("Mac", "192.168.1.23", 8080)
        val dup = DiscoveredServer("Mac", "192.168.1.23", 8080)
        assertEquals(listOf(a), sortedDiscovered(listOf(a, dup)))
    }

    @Test
    fun sortedDiscovered_ordersByNameThenHostThenPort_caseInsensitive() {
        val zed = DiscoveredServer("Zed", "192.168.1.9", 8080)
        val amy = DiscoveredServer("amy", "192.168.1.5", 8080)      // lowercase sorts with "A"
        val bobHi = DiscoveredServer("Bob", "192.168.1.7", 9000)
        val bobLo = DiscoveredServer("Bob", "192.168.1.7", 8080)    // same name/host, lower port first
        assertEquals(
            listOf(amy, bobLo, bobHi, zed),
            sortedDiscovered(listOf(zed, bobHi, amy, bobLo)),
        )
    }

    @Test
    fun sortedDiscovered_distinctHostsSameNameBothKept() {
        // Two real servers advertising the same friendly name are distinct rows (different url).
        val one = DiscoveredServer("core", "192.168.1.10", 8080)
        val two = DiscoveredServer("core", "192.168.1.11", 8080)
        assertEquals(listOf(one, two), sortedDiscovered(listOf(two, one)))
    }

    @Test
    fun sortedDiscovered_empty_isEmpty() {
        assertEquals(emptyList(), sortedDiscovered(emptyList()))
    }
}
