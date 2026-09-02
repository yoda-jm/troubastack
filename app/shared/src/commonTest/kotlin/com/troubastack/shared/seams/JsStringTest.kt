package com.troubastack.shared.seams

import kotlin.test.Test
import kotlin.test.assertEquals

/** [jsQuote] — the shell->web bridge's JS-string-literal escaper (hoisted to commonMain to be testable). */
class JsStringTest {

    @Test
    fun wrapsInQuotes() = assertEquals("\"hi\"", jsQuote("hi"))

    @Test
    fun escapesQuoteAndBackslash() {
        assertEquals("\"a\\\"b\"", jsQuote("a\"b"))
        assertEquals("\"a\\\\b\"", jsQuote("a\\b"))
    }

    @Test
    fun escapesControlChars() {
        assertEquals("\"a\\nb\"", jsQuote("a\nb"))
        assertEquals("\"\\r\\t\"", jsQuote("\r\t"))
        // low control char -> \u escape (Char(0) avoids embedding a literal NUL in source)
        assertEquals("\"\\u0000\"", jsQuote(Char(0).toString()))
    }

    @Test
    fun handshakeJsonRoundTripsAsALiteral() {
        // The actual bridge payload shape — must be a safe single JS string literal.
        assertEquals("\"{\\\"type\\\":\\\"hello\\\"}\"", jsQuote("""{"type":"hello"}"""))
    }
}
