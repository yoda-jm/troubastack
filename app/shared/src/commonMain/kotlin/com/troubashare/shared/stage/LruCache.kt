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
    // B1: keys that eviction must NOT drop — the on-screen page pins its raster+overlays so a re-decode
    // can never evict the very entries it's about to reuse (which caused "same page, fewer annotations").
    private var pinned: Set<K> = emptySet()

    val size: Int get() = map.size

    /** Return the value and mark it most-recently-used; a miss leaves order untouched. */
    fun get(key: K): V? {
        val value = map.remove(key) ?: return null
        map[key] = value // move to most-recently-used
        return value
    }

    /** Insert/update [key] as most-recently-used, evicting the least-recently-used NON-pinned entries
     *  past [maxEntries]; if only pinned entries remain, stay (temporarily) over budget. */
    fun put(key: K, value: V) {
        map.remove(key)
        map[key] = value
        while (map.size > maxEntries) {
            val victim = map.keys.firstOrNull { it !in pinned } ?: break // all remaining pinned → keep
            map.remove(victim)
        }
    }

    /** Protect [keys] from eviction (B1 — the currently-displayed page's entries). Replaces the set. */
    fun pin(keys: Set<K>) {
        pinned = keys
    }
}
