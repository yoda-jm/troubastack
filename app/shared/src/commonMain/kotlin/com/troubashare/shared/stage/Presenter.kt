// Generated proto types (ConcertBundle, BakedSong, PageImages, …) come from gen/ — single
// source of truth is proto/ (I1). This file references them, never redefines them.
package com.troubashare.shared.stage

/**
 * TroubaStage presenter — SHARED Kotlin (commonMain). This is NOT a native seam: it runs the
 * same everywhere because it does almost nothing (I12, I15).
 *
 * A baked concert is **flattened images** (per page: a PDF page raster + transparent annotation
 * overlay(s)). The presenter is a **pure image compositor + pager** — scroll / swipe / goto.
 * At performance time it depends on NOTHING server-side and contains:
 *  - NO `PdfRenderer` / stroke drawing (the smartness happened at bake time, on the server),
 *  - NO annotation-model logic,
 *  - NO access-control logic.
 *
 * It is fully decoupled from the editing data model. Freeze/lock is honored at this tier (I13)
 * but enforced upstream — the presenter just performs whatever bundle it was handed.
 *
 * See docs/design/05-distribution.md.
 */

/**
 * One performable page: a raster plus zero or more transparent overlays, composited top-down.
 * These are blob references resolved against local storage (seam 3) — mirrors proto `PageImages`.
 */
class Page(
    val rasterRef: String,          // PDF page raster (WebP); from proto PageImages.page_raster_ref
    val overlayRefs: List<String>,  // transparent overlay(s); one per layer-group (I12)
)

/**
 * The loaded concert to perform. Backed by the generated proto `ConcertBundle` (I1); the
 * presenter only reads its flattened pages, never any object/annotation model.
 */
class ConcertBundle(
    // TODO(scaffold): wrap gen `troubastack.v1.ConcertBundle`; expose ordered List<Page> only.
    val pages: List<Page>,
)

/** Pager position + mode. Pure view state; no domain logic (I12). */
class PagerState(
    val pageCount: Int,
    val current: Int = 0,
    val mode: PageMode = PageMode.SWIPE,
) {
    enum class PageMode { SWIPE, SCROLL }
}

/** The whole presenter surface: load a bundle, then page through it. */
interface Presenter {
    fun load(bundle: ConcertBundle): PagerState
    fun goto(page: Int): PagerState        // direct jump
    fun next(): PagerState                  // swipe / scroll forward
    fun previous(): PagerState
    // Composite = draw raster, then overlays in order. No stroke logic — bytes already baked (I12).
    // TODO(scaffold): rendering binds to Compose Multiplatform Image; bodies derived later.
}
