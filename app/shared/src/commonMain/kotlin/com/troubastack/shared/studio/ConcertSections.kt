package com.troubastack.shared.studio

/**
 * A83 ⟨D1⟩ — the Studio browse's Concerts tab, grouped by band. Pure, so the order is tested in commonTest.
 *
 * - One section per band (a band with no concerts has none), bands in NATURAL order of their name.
 * - Within a band: dated concerts newest first, then undated ones by name, natural order.
 * - Exactly one band ⇒ one section with [ConcertSection.showHeader] false: a header naming the only band
 *   adds nothing, so the list looks as it did before A83.
 */
data class ConcertSection<T>(val bandId: String, val bandName: String, val showHeader: Boolean, val concerts: List<T>)

fun <T> groupConcerts(
    concerts: List<T>,
    bandId: (T) -> String,
    bandName: (T) -> String,
    name: (T) -> String,
    eventDate: (T) -> String, // ISO yyyy-mm-dd, or "" when undated
): List<ConcertSection<T>> {
    val byBand = LinkedHashMap<String, MutableList<T>>()
    for (c in concerts) byBand.getOrPut(bandId(c)) { mutableListOf() } += c
    val within = Comparator<T> { a, b ->
        val da = eventDate(a)
        val db = eventDate(b)
        when {
            da.isNotEmpty() && db.isNotEmpty() -> db.compareTo(da).takeIf { it != 0 } ?: naturalCompare(name(a), name(b))
            da.isNotEmpty() -> -1                       // dated before undated
            db.isNotEmpty() -> 1
            else -> naturalCompare(name(a), name(b))    // undated: by name, natural
        }
    }
    val sections = byBand.map { (id, list) -> ConcertSection(id, bandName(list.first()), true, list.sortedWith(within)) }
        .sortedWith { a, b -> naturalCompare(a.bandName, b.bandName).takeIf { it != 0 } ?: a.bandId.compareTo(b.bandId) }
    return if (sections.size == 1) listOf(sections[0].copy(showHeader = false)) else sections
}

/**
 * Natural order for names: case and accents ignored, digit runs compared by VALUE ("Band 2" < "Band 10",
 * "Session 2" < "Session 10"), the same rule as Studio's tag order (T188). Ties fall back to the raw strings,
 * so the order is total and stable.
 */
fun naturalCompare(a: String, b: String): Int {
    val x = fold(a)
    val y = fold(b)
    var i = 0
    var j = 0
    while (i < x.length && j < y.length) {
        if (x[i].isDigit() && y[j].isDigit()) {
            val si = i; while (i < x.length && x[i].isDigit()) i++
            val sj = j; while (j < y.length && y[j].isDigit()) j++
            val nx = x.substring(si, i).trimStart('0')
            val ny = y.substring(sj, j).trimStart('0')
            if (nx.length != ny.length) return nx.length - ny.length   // more significant digits = bigger
            val c = nx.compareTo(ny)
            if (c != 0) return c
        } else {
            val c = x[i].compareTo(y[j])
            if (c != 0) return c
            i++; j++
        }
    }
    return ((x.length - i) - (y.length - j)).takeIf { it != 0 } ?: a.compareTo(b)
}

private val ACCENTS = mapOf(
    'à' to 'a', 'á' to 'a', 'â' to 'a', 'ã' to 'a', 'ä' to 'a', 'å' to 'a', 'ā' to 'a', 'ç' to 'c', 'č' to 'c',
    'è' to 'e', 'é' to 'e', 'ê' to 'e', 'ë' to 'e', 'ē' to 'e', 'ì' to 'i', 'í' to 'i', 'î' to 'i', 'ï' to 'i',
    'ñ' to 'n', 'ò' to 'o', 'ó' to 'o', 'ô' to 'o', 'õ' to 'o', 'ö' to 'o', 'ø' to 'o', 'ō' to 'o',
    'ù' to 'u', 'ú' to 'u', 'û' to 'u', 'ü' to 'u', 'ū' to 'u', 'ý' to 'y', 'ÿ' to 'y', 'š' to 's', 'ž' to 'z',
)

private fun fold(s: String): String = buildString(s.length) {
    for (ch in s.lowercase()) {
        when (ch) {
            'æ' -> append("ae")
            'œ' -> append("oe")
            'ß' -> append("ss")
            else -> append(ACCENTS[ch] ?: ch)
        }
    }
}
