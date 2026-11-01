package com.troubastack.desktop

/** A rectangle in the window system's units (AWT's, which a Compose window's dp map to one for one). */
data class Bounds(val x: Int, val y: Int, val width: Int, val height: Int) {
    fun contains(px: Int, py: Int): Boolean = px >= x && px < x + width && py >= y && py < y + height
}

/** WIN03 D2 — what the window remembers across launches: its restored bounds, and whether it was maximized. */
data class SavedPlacement(val bounds: Bounds, val maximized: Boolean) {
    fun encode(): String = with(bounds) { "$x,$y,$width,$height,$maximized" }
}

const val MIN_WINDOW_WIDTH = 800
const val MIN_WINDOW_HEIGHT = 600
private const val DEFAULT_WIDTH = 1280
private const val DEFAULT_HEIGHT = 800

/** How far below the window's top edge its title bar is grabbed: the point that must be on a screen to move it. */
private const val TITLE_BAR_GRIP = 16

/** The stored form back, or null when it is missing or not five well-formed fields (the default placement then). */
fun parsePlacement(s: String?): SavedPlacement? {
    val f = s?.split(',') ?: return null
    if (f.size != 5) return null
    val n = f.take(4).map { it.trim().toIntOrNull() ?: return null }
    val max = f[4].trim().toBooleanStrictOrNull() ?: return null
    return SavedPlacement(Bounds(n[0], n[1], n[2], n[3]), max)
}

/**
 * WIN03 D2 — where the window opens. The saved placement, if its title bar is still on one of the connected
 * [screens] (so the window can be grabbed and moved). Otherwise (nothing saved, or the monitor it was on is
 * unplugged) it opens centred on the [primary] screen, at the saved size or the default. Either way the size is
 * never smaller than 800×600 and never bigger than the screen it opens on.
 */
fun openingPlacement(saved: SavedPlacement?, screens: List<Bounds>, primary: Bounds): SavedPlacement {
    val home = saved?.let { s ->
        val gripX = s.bounds.x + s.bounds.width / 2
        val gripY = s.bounds.y + TITLE_BAR_GRIP
        screens.firstOrNull { it.contains(gripX, gripY) }
    }
    if (saved != null && home != null) return saved.copy(bounds = saved.bounds.fitSize(home))
    val w = saved?.bounds?.width ?: DEFAULT_WIDTH
    val h = saved?.bounds?.height ?: DEFAULT_HEIGHT
    val size = Bounds(0, 0, w, h).fitSize(primary)
    val centred = Bounds(
        primary.x + (primary.width - size.width) / 2,
        primary.y + (primary.height - size.height) / 2,
        size.width,
        size.height,
    )
    return SavedPlacement(centred, maximized = saved?.maximized ?: false)
}

/** The same rectangle, its size kept within [MIN_WINDOW_WIDTH]×[MIN_WINDOW_HEIGHT] and [screen]'s size. */
private fun Bounds.fitSize(screen: Bounds): Bounds = copy(
    width = width.coerceAtLeast(MIN_WINDOW_WIDTH).coerceAtMost(maxOf(screen.width, MIN_WINDOW_WIDTH)),
    height = height.coerceAtLeast(MIN_WINDOW_HEIGHT).coerceAtMost(maxOf(screen.height, MIN_WINDOW_HEIGHT)),
)
