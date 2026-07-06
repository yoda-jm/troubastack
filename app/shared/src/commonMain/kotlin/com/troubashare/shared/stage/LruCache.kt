// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubashare.shared.stage

/**
 * A tiny access-order LRU, portable to Kotlin/Native (the JVM-only access-order `LinkedHashMap(cap,
 * load, true)` constructor doesn't exist in the common stdlib). Access-order is emulated over a
 * plain insertion-ordered `LinkedHashMap` by re-inserting on `get`/`put`; on overflow the
 * least-recently-*used* entry (`keys.first()`) is evicted. Not thread-safe — [PageImageCache] uses it
 * only from the composition (main) thread. Generic + `internal` so the eviction/access-order logic is
 * unit-testable without a Compose `ImageBitmap`.
 */
internal class LruCache<K, V>(private val maxEntries: Int) {
    private val map = LinkedHashMap<K, V>()

    val size: Int get() = map.size

    /** Return the value and mark it most-recently-used; a miss leaves order untouched. */
    fun get(key: K): V? {
        val value = map.remove(key) ?: return null
        map[key] = value // move to most-recently-used
        return value
    }

    /** Insert/update [key] as most-recently-used, evicting the LRU entries past [maxEntries]. */
    fun put(key: K, value: V) {
        map.remove(key)
        map[key] = value
        while (map.size > maxEntries) {
            map.remove(map.keys.first()) // evict least-recently-used
        }
    }
}
