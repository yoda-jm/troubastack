package com.troubastack.app

/**
 * A48 — the Stage reading-position string seam. A46 persists the position as one string on every page
 * move and reads it back at app open — on data written by a PREVIOUS install, which is the one place a
 * malformed value is not hypothetical (a truncated write, or a value from an older/newer encoding).
 *
 * Named + pure (the A47 pattern) so the round-trip and — above all — what the decode REJECTS are
 * unit-tested off-device: a stored value with no separator must degrade to null (→ start at the top),
 * never throw at composition time on launch. Behaviour is identical to A46's inline `"$s#$p"` /
 * `split('#', limit = 2).takeIf { it.size == 2 }`.
 */
internal fun encodeStagePosition(songId: String, pageInSong: Int): String = "$songId#$pageInSong"

/**
 * Decode a persisted "songId#pageInSong": the song and its 0-based page-in-song, or **null** to start at
 * the top. Null when [raw] is null or has no separator (`size == 2` guard — load-bearing: without it,
 * `[1]` throws on a `#`-less value, at launch, before the Stage renders). A non-numeric page degrades to
 * 0 (the song's first page) — a deliberate decision. A `#` inside the songId splits into a mangled
 * songId + a non-numeric page ⇒ page 0 + a songId that won't resolve ⇒ resolveStartPage lands at the top.
 */
internal fun decodeStagePosition(raw: String?): Pair<String, Int>? {
    val parts = raw?.split('#', limit = 2)?.takeIf { it.size == 2 } ?: return null
    return parts[0] to (parts[1].toIntOrNull() ?: 0)
}
