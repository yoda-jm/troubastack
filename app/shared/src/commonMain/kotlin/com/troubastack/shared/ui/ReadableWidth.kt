package com.troubastack.shared.ui

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp

/**
 * WIN03 D7 — the widest a list screen's content grows. A desktop window can be 1920 px wide, where a list row with its
 * action at the far edge is hard to use; the desktop sets this, phones and tablets leave it null (full width, as
 * designed).
 */
val LocalMaxContentWidth = staticCompositionLocalOf<Dp?> { null }

/** At most [max] wide and centred; unchanged when [max] is null. Put it after any verticalScroll, so the whole window
 *  still scrolls with the wheel. */
fun Modifier.readableWidth(max: Dp?): Modifier =
    if (max == null) this else wrapContentWidth(Alignment.CenterHorizontally).widthIn(max = max).fillMaxWidth()
