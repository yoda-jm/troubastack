package com.troubastack.shared.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/**
 * A36 — the app's brand theme, so it stops shipping Material 3's stock lavender/purple and instead
 * wears the studio's warm-paper + indigo identity. Values are the studio's real CSS tokens
 * (`web/studio/src/styles.css` `:root` and its `prefers-color-scheme: dark` block), copied exactly —
 * do NOT re-pick by eye. No dynamic colour / Material You (it would repaint from the wallpaper and
 * defeat the point).
 *
 * Only the roles the spec pins are brand-mapped; `secondary`/`tertiary` are folded onto the brand
 * family so nothing on any screen stays purple (there is one brand hue, not three). Stage keeps its
 * own performance palette + A34's fixed amber/aqua beat — those are deliberately NOT routed here.
 */
private val BrandLight = lightColorScheme(
    primary = Color(0xFF4F46E5), // --brand
    onPrimary = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFFEFEDFC), // --brand-tint
    onPrimaryContainer = Color(0xFF35309A), // --brand-ink
    // secondary/tertiary fold onto the brand so no screen keeps Material's purple.
    secondary = Color(0xFF4F46E5),
    onSecondary = Color(0xFFFFFFFF),
    secondaryContainer = Color(0xFFEFEDFC),
    onSecondaryContainer = Color(0xFF35309A),
    tertiary = Color(0xFF4F46E5),
    onTertiary = Color(0xFFFFFFFF),
    tertiaryContainer = Color(0xFFEFEDFC),
    onTertiaryContainer = Color(0xFF35309A),
    background = Color(0xFFF7F4EE), // --bg
    onBackground = Color(0xFF201D29), // --fg
    surface = Color(0xFFFFFDFA), // --surface
    onSurface = Color(0xFF201D29),
    surfaceVariant = Color(0xFFF2EEE6), // --surface-2
    onSurfaceVariant = Color(0xFF6D6979), // --muted
    outlineVariant = Color(0xFFE7E1D8), // --border
    outline = Color(0xFFD8D1C5), // --border-strong
    error = Color(0xFFB42318), // --error-fg
    onError = Color(0xFFFFFFFF),
    errorContainer = Color(0xFFFBECEB), // --error-bg
    onErrorContainer = Color(0xFFB42318),
)

private val BrandDark = darkColorScheme(
    primary = Color(0xFFA5B4FC),
    onPrimary = Color(0xFF201D33),
    primaryContainer = Color(0xFF201D33),
    onPrimaryContainer = Color(0xFFC3CCFF),
    secondary = Color(0xFFA5B4FC),
    onSecondary = Color(0xFF201D33),
    secondaryContainer = Color(0xFF201D33),
    onSecondaryContainer = Color(0xFFC3CCFF),
    tertiary = Color(0xFFA5B4FC),
    onTertiary = Color(0xFF201D33),
    tertiaryContainer = Color(0xFF201D33),
    onTertiaryContainer = Color(0xFFC3CCFF),
    background = Color(0xFF100E16),
    onBackground = Color(0xFFEFEAF6),
    surface = Color(0xFF191722),
    onSurface = Color(0xFFEFEAF6),
    surfaceVariant = Color(0xFF211E2C),
    onSurfaceVariant = Color(0xFF9C96AC), // --muted (dark)
    outlineVariant = Color(0xFF2B2836), // --border (dark)
    outline = Color(0xFF3A3648), // --border-strong (dark)
    error = Color(0xFFFCA5A5),
    onError = Color(0xFF201D33),
    errorContainer = Color(0xFF2A1414),
    onErrorContainer = Color(0xFFFCA5A5),
)

/**
 * The user's theme choice (A36 — the Parameters screen's Theme selector). SYSTEM follows the device
 * (like the studio's `prefers-color-scheme`); LIGHT/DARK force it. Persisted under [KEY].
 */
enum class ThemePref {
    SYSTEM,
    LIGHT,
    DARK,
    ;

    /** Resolve to a concrete dark flag, consulting the system only for [SYSTEM]. */
    @Composable
    fun resolveDark(): Boolean = when (this) {
        SYSTEM -> isSystemInDarkTheme()
        LIGHT -> false
        DARK -> true
    }

    companion object {
        /** Storage key for the persisted choice. */
        const val KEY = "app.theme"

        /** Parse a persisted value; anything unknown/absent → SYSTEM. */
        fun parse(raw: String?): ThemePref = entries.firstOrNull { it.name == raw } ?: SYSTEM
    }
}

/**
 * Wrap the app in the brand palette. Light/dark is driven by [dark] — pass a resolved [ThemePref] so
 * the Parameters screen can override the system setting. Use this at BOTH entrypoints in place of a
 * bare `MaterialTheme`.
 */
/**
 * BRAND09 — per-PRODUCT, per-GROUND brand accents, kept OUT of `colorScheme` (which is one indigo
 * chrome hue, A36) because these say "this is that product", not "act here". Provided by [TroubaTheme]
 * so light/dark is resolved once here, never as raw hex at a call site. Values from BRAND06's ACCENT
 * table; the [studio]/[stage] accents may carry a mark, border, icon, or heading (large text / 3:1) but
 * NOT small text on `--background` (measured: Studio dark is the strict case). [studioActive] is the
 * CONNECTED Studio tile's background — branded, so a connected tile looks connected — and [studioIdle]
 * is DERIVED from it (same family, desaturated) so "grey ⇒ disabled" stays a reliable signal.
 */
@Immutable
data class BrandAccents(
    val stage: Color,
    val studio: Color,
    val studioActive: Color,
    val studioIdle: Color,
)

private val BrandAccentsLight = BrandAccents(
    stage = Color(0xFF936B1F),        // BRAND06 paper — 4.74 on surface, 3:1 large on bg
    studio = Color(0xFFD62A8A),       // BRAND06 — 4.54 on surface, 3:1 large on bg
    studioActive = Color(0xFFF8E4F0), // a light studio tint: a connected tile reads active/branded
    studioIdle = Color(0xFFEDE7EA),   // derived from studio, desaturated → the "off" of the same family
)

private val BrandAccentsDark = BrandAccents(
    stage = Color(0xFFC8912A),        // BRAND06 dark — 6.88 on bg
    studio = Color(0xFFD62A8A),       // BRAND06 dark — 3:1 large on bg/surface
    studioActive = Color(0xFF2E1823), // a dark studio tint
    studioIdle = Color(0xFF221C20),   // derived, desaturated
)

/** BRAND09 accents for the current theme. Reads the value [TroubaTheme] provided for this ground. */
val LocalBrandAccents = staticCompositionLocalOf { BrandAccentsLight }

/**
 * Wrap the app in the brand palette. Light/dark is driven by [dark] — pass a resolved [ThemePref] so
 * the Parameters screen can override the system setting. Use this at BOTH entrypoints in place of a
 * bare `MaterialTheme`.
 */
@Composable
fun TroubaTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    CompositionLocalProvider(LocalBrandAccents provides if (dark) BrandAccentsDark else BrandAccentsLight) {
        MaterialTheme(colorScheme = if (dark) BrandDark else BrandLight, content = content)
    }
}
