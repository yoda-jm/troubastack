package com.troubastack.app

import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type

/**
 * WIN03 D4 — Enter (or the keypad's) in a single-line field submits its form, as everywhere on a desktop. A hardware key,
 * so Android's soft keyboard and its action key do not change.
 */
internal fun Modifier.submitOnEnter(enabled: Boolean, submit: () -> Unit): Modifier = onPreviewKeyEvent { e ->
    if (e.type == KeyEventType.KeyDown && (e.key == Key.Enter || e.key == Key.NumPadEnter) && enabled) {
        submit()
        true
    } else {
        false
    }
}
