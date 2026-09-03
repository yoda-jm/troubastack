package com.troubastack.shared.stage

import androidx.compose.ui.graphics.ImageBitmap

/**
 * A64 part 2 — apply the chroma-gated annotation rule ([transformOverlayPixel]) to every pixel of a
 * decoded overlay bitmap, producing a new bitmap. This is what lets an overlay's ink be transformed as
 * a *code* (a red cue stays red) instead of by the page's colour matrix (which turned it teal). The
 * raster still takes the page matrix at draw time; only overlays go through this.
 *
 * Platform-specific because there is no common pixel-write in Compose: Android uses
 * `android.graphics.Bitmap`, iOS uses Skia. Pure logic lives in [transformOverlayPixel] (commonMain,
 * unit-tested), so both actuals are just the pixel plumbing and cannot diverge on the rule.
 *
 * Called at most once per (overlay, scheme) and the result is cached (see `decodeOverlayCached`).
 * `NORMAL` is the identity scheme and never calls this.
 */
expect fun transformOverlayBitmap(src: ImageBitmap, scheme: StageColorMode): ImageBitmap
