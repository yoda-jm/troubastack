package com.troubashare.shared.join

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class JoinLinkTest {

    // ---- parseTroubaLink: the accept rows -------------------------------------------------------

    @Test fun https_join_parses() {
        assertEquals(TroubaLink.Join("https://h", "tok_123"), parseTroubaLink("https://h/join/tok_123"))
    }

    @Test fun http_join_with_explicit_port_keeps_port() {
        assertEquals(TroubaLink.Join("http://h:8080", "abc"), parseTroubaLink("http://h:8080/join/abc"))
    }

    @Test fun trailing_slash_query_and_fragment_are_ignored() {
        val tok = "T-0k_en"
        assertEquals(TroubaLink.Join("https://h", tok), parseTroubaLink("https://h/join/$tok/"))
        assertEquals(TroubaLink.Join("https://h", tok), parseTroubaLink("https://h/join/$tok?x=1"))
        assertEquals(TroubaLink.Join("https://h", tok), parseTroubaLink("https://h/join/$tok#frag"))
    }

    @Test fun reset_password_link_parses_but_is_not_a_join() {
        assertEquals(TroubaLink.PasswordReset("https://h", "rtok"), parseTroubaLink("https://h/reset-password/rtok"))
    }

    // ---- parseTroubaLink: the refuse rows -------------------------------------------------------

    @Test fun no_scheme_is_unsupported() {
        assertTrue(parseTroubaLink("h/join/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("//h/join/tok") is TroubaLink.Unsupported)
    }

    @Test fun wrong_path_is_unsupported() {
        assertTrue(parseTroubaLink("https://h/") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("https://h/songs/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("https://h/join") is TroubaLink.Unsupported)          // no token segment
        assertTrue(parseTroubaLink("https://h/join/tok/extra") is TroubaLink.Unsupported)
    }

    @Test fun no_host_is_unsupported() {
        // A51 review nit: the parse table's "no host" row had no test. `http:///join/tok` has an empty
        // authority ⇒ null origin ⇒ Unsupported. (Covered here since A52 touches the join package.)
        assertTrue(parseTroubaLink("http:///join/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("https:///join/tok") is TroubaLink.Unsupported)
    }

    @Test fun empty_over_long_or_bad_charset_token_is_unsupported() {
        assertTrue(parseTroubaLink("https://h/join/") is TroubaLink.Unsupported)          // empty
        assertTrue(parseTroubaLink("https://h/join/${"a".repeat(513)}") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("https://h/join/tok%20en") is TroubaLink.Unsupported)  // '%' not allowed
        assertTrue(parseTroubaLink("https://h/join/tok.en") is TroubaLink.Unsupported)    // '.' not allowed
    }

    @Test fun a_512_char_token_is_accepted_length_is_not_pinned_to_32() {
        // The server may widen the token; the client must not break. 512 is the cap, not the size.
        val long = "a".repeat(512)
        assertEquals(TroubaLink.Join("https://h", long), parseTroubaLink("https://h/join/$long"))
    }

    // ---- the two hostile vectors, by name -------------------------------------------------------

    @Test fun userinfo_url_whose_prefix_looks_trusted_is_rejected() {
        // Host is 192.0.2.9, NOT trusted-looking-host. Reject rather than risk displaying the wrong host.
        val r = parseTroubaLink("http://trusted-looking-host@192.0.2.9/join/xyz")
        assertTrue(r is TroubaLink.Unsupported, "userinfo URL must be Unsupported, was $r")
    }

    @Test fun javascript_scheme_payload_is_rejected() {
        assertTrue(parseTroubaLink("javascript:alert(1)//h/join/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("file:///join/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("intent://h/join/tok") is TroubaLink.Unsupported)
        assertTrue(parseTroubaLink("content://h/join/tok") is TroubaLink.Unsupported)
    }

    // ---- origin normalisation -------------------------------------------------------------------

    @Test fun scheme_and_host_are_lowercased() {
        assertEquals(TroubaLink.Join("https://h.example", "t"), parseTroubaLink("HTTPS://H.Example/join/t"))
    }

    @Test fun default_port_is_dropped_explicit_port_is_kept() {
        assertEquals(TroubaLink.Join("http://h", "t"), parseTroubaLink("http://h:80/join/t"))
        assertEquals(TroubaLink.Join("https://h", "t"), parseTroubaLink("https://h:443/join/t"))
        assertEquals(TroubaLink.Join("http://h:443", "t"), parseTroubaLink("http://h:443/join/t")) // non-default for http
    }

    @Test fun ipv6_literal_survives_intact() {
        assertEquals(TroubaLink.Join("http://[::1]:8080", "t"), parseTroubaLink("http://[::1]:8080/join/t"))
        assertEquals(TroubaLink.Join("https://[::1]", "t"), parseTroubaLink("https://[::1]:443/join/t")) // default dropped
    }

    // ---- joinDecision: every row ----------------------------------------------------------------

    @Test fun same_origin_with_session_redeems() {
        val d = joinDecision(TroubaLink.Join("https://h", "t"), currentOrigin = "https://h", hasSession = true)
        assertEquals(JoinAction.Redeem("https://h", "t"), d)
    }

    @Test fun same_origin_without_session_signs_in() {
        val d = joinDecision(TroubaLink.Join("https://h", "t"), currentOrigin = "https://h", hasSession = false)
        assertEquals(JoinAction.SignIn("https://h", "t"), d)
    }

    @Test fun different_origin_confirms_server_regardless_of_session() {
        val link = TroubaLink.Join("https://target", "t")
        assertEquals(
            JoinAction.ConfirmServer(current = "https://current", target = "https://target", token = "t"),
            joinDecision(link, currentOrigin = "https://current", hasSession = true),
        )
        assertEquals(
            JoinAction.ConfirmServer(current = "https://current", target = "https://target", token = "t"),
            joinDecision(link, currentOrigin = "https://current", hasSession = false),
        )
    }

    @Test fun first_run_never_connected_still_confirms_server() {
        // A person who has never connected has no basis to trust the host either — they must see it.
        val d = joinDecision(TroubaLink.Join("https://h", "t"), currentOrigin = null, hasSession = false)
        assertEquals(JoinAction.ConfirmServer(current = null, target = "https://h", token = "t"), d)
    }

    @Test fun password_reset_is_blocked_with_browser_hint() {
        val d = joinDecision(TroubaLink.PasswordReset("https://h", "t"), currentOrigin = "https://h", hasSession = true)
        assertTrue(d is JoinAction.Blocked && d.reason.contains("browser"), "was $d")
    }

    @Test fun unsupported_is_blocked_carrying_its_reason() {
        val link = parseTroubaLink("javascript:alert(1)") as TroubaLink.Unsupported
        assertEquals(JoinAction.Blocked(link.reason), joinDecision(link, currentOrigin = "https://h", hasSession = true))
    }

    // ---- the normalisation-bug guard: :80 must NOT nag the user ---------------------------------

    @Test fun default_port_current_origin_still_redeems_not_confirms() {
        // http://h:80 and http://h are the SAME server. A case-sensitive/raw comparison would ConfirmServer
        // here — nagging the user on every scan. (This is teeth-check #2.)
        val d = joinDecision(TroubaLink.Join("http://h", "t"), currentOrigin = "http://h:80", hasSession = true)
        assertEquals(JoinAction.Redeem("http://h", "t"), d)
    }

    @Test fun differently_cased_current_origin_still_redeems() {
        val d = joinDecision(TroubaLink.Join("http://h", "t"), currentOrigin = "HTTP://H", hasSession = true)
        assertEquals(JoinAction.Redeem("http://h", "t"), d)
    }

    // ---- the foreign-host case A52's wildcard intent-filter makes reachable ----------------------

    @Test fun join_url_on_a_foreign_host_confirms_server() {
        // A plausible unrelated service prints a /join/ QR; the app is pointed at the band's server.
        // This MUST route to ConfirmServer — provably handled here, not left to the UI.
        val link = parseTroubaLink("https://evil.example.com/join/tok") as TroubaLink.Join
        val d = joinDecision(link, currentOrigin = "https://band.example.org", hasSession = true)
        assertEquals(
            JoinAction.ConfirmServer(current = "https://band.example.org", target = "https://evil.example.com", token = "tok"),
            d,
        )
    }
}
