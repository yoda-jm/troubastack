package com.troubastack.app

import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.pressKey
import androidx.compose.ui.test.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

/** WIN03 D4 — Enter submits a single-line field's form (Connect, sign in, the invite link), only when it can. */
@OptIn(ExperimentalTestApi::class)
class SubmitOnEnterUiTest {
    @Test
    fun enter_submits_whenTheFormCan_andNotOtherwise() = runComposeUiTest {
        // TEETH: without the handler Enter does nothing in a single-line field (seen in the Connect dialog on Linux).
        var submitted = 0
        var text by mutableStateOf("")
        setContent {
            BasicTextField(text, { text = it }, singleLine = true, modifier = Modifier.testTag("f").submitOnEnter(text.isNotBlank()) { submitted++ })
        }
        onNodeWithTag("f").performClick()
        onNodeWithTag("f").performKeyInput { pressKey(Key.Enter) }
        assertEquals(0, submitted, "an empty form is not submitted")
        onNodeWithTag("f").performTextInput("demo")
        onNodeWithTag("f").performKeyInput { pressKey(Key.Enter) }
        assertEquals(1, submitted, "Enter submitted")
        onNodeWithTag("f").performKeyInput { pressKey(Key.NumPadEnter) }
        assertEquals(2, submitted, "the keypad's Enter too")
        assertEquals("demo", text, "and Enter typed nothing into the field")
    }
}
