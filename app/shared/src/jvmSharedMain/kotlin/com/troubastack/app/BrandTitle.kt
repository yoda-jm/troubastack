package com.troubastack.app

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle

/** BRAND10 two-tone wordmark: "Trouba" in [ink], the product word in its [accent]. Shared by the
 *  native product launcher pages (the Studio browse, the Stage concert list). */
fun brandTitle(product: String, accent: Color, ink: Color) = buildAnnotatedString {
    withStyle(SpanStyle(color = ink)) { append("Trouba") }
    withStyle(SpanStyle(color = accent)) { append(product) }
}
