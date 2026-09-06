package httpapi

import (
	"net/url"
	"strings"
	"testing"
)

// decodeFilenameStar extracts and percent-decodes the RFC 5987 filename* value from a
// Content-Disposition header, so a test asserts the ROUND-TRIP rather than merely that the
// substring is present (a wrongly-encoded value would pass the weaker check).
func decodeFilenameStar(t *testing.T, header string) string {
	t.Helper()
	const marker = "filename*=UTF-8''"
	i := strings.Index(header, marker)
	if i < 0 {
		t.Fatalf("no filename* in %q", header)
	}
	val := header[i+len(marker):]
	if j := strings.IndexByte(val, ';'); j >= 0 {
		val = val[:j]
	}
	dec, err := url.PathUnescape(val) // %20 → space (PathUnescape does NOT turn '+' into space)
	if err != nil {
		t.Fatalf("decode filename* %q: %v", val, err)
	}
	return dec
}

func quotedFilename(t *testing.T, header string) string {
	t.Helper()
	const marker = `filename="`
	i := strings.Index(header, marker)
	if i < 0 {
		t.Fatalf("no quoted filename in %q", header)
	}
	rest := header[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("unterminated quoted filename in %q", header)
	}
	return rest[:j]
}

// TestContentDisposition_AccentedRoundTrips is VLL's case: the filename* decodes back to the exact
// accented name, and the ASCII fallback is present and non-empty.
func TestContentDisposition_AccentedRoundTrips(t *testing.T) {
	name := "Fête d'été — Bøîte.pdf"
	h := contentDisposition("attachment", name, "setlist.pdf")

	if got := decodeFilenameStar(t, h); got != name {
		t.Errorf("filename* round-trip = %q, want %q", got, name)
	}
	if fb := quotedFilename(t, h); fb == "" {
		t.Error("ASCII fallback filename is empty")
	}
	if !strings.HasPrefix(h, "attachment; ") {
		t.Errorf("disposition prefix wrong: %q", h)
	}
}

// TestContentDisposition_PureASCIIUnchanged: a plain name still parses the old way (quoted filename
// first), so a strict old client is unaffected.
func TestContentDisposition_PureASCIIUnchanged(t *testing.T) {
	h := contentDisposition("attachment", "setlist.pdf", "setlist.pdf")
	if fb := quotedFilename(t, h); fb != "setlist.pdf" {
		t.Errorf("ascii filename = %q, want setlist.pdf", fb)
	}
	if got := decodeFilenameStar(t, h); got != "setlist.pdf" {
		t.Errorf("filename* = %q, want setlist.pdf", got)
	}
}

// TestContentDisposition_NoInjection: a name with a quote, backslash and newline must not break out of
// the quoted-string or inject a header line, and the star value must percent-encode them.
func TestContentDisposition_NoInjection(t *testing.T) {
	h := contentDisposition("attachment", "a\"b\\c\r\nSet-Cookie: x.pdf", "setlist.pdf")
	fb := quotedFilename(t, h)
	if strings.ContainsAny(fb, "\"\\\r\n") {
		t.Errorf("ASCII fallback %q still contains a quoting/CRLF metacharacter", fb)
	}
	if strings.ContainsAny(h, "\r\n") {
		t.Errorf("header value contains a raw CR/LF: %q", h)
	}
	// The dangerous bytes survive intact in the (encoded) star value, so the name is not lost — just safe.
	if got := decodeFilenameStar(t, h); got != "a\"b\\c\r\nSet-Cookie: x.pdf" {
		t.Errorf("filename* round-trip lost data: %q", got)
	}
}

// TestContentDisposition_NonLatinFallback: a name with no ASCII at all still yields a usable fallback,
// never filename="".
func TestContentDisposition_NonLatinFallback(t *testing.T) {
	h := contentDisposition("attachment", "Группа", "band.tband")
	if fb := quotedFilename(t, h); fb != "band.tband" {
		t.Errorf("ascii fallback = %q, want band.tband (nothing ASCII survived the name)", fb)
	}
	if got := decodeFilenameStar(t, h); got != "Группа" {
		t.Errorf("filename* = %q, want the original non-Latin name", got)
	}
}

// TestRFC5987_SpaceIsPercent20: the encoder must use %20, not '+', for a space.
func TestRFC5987_SpaceIsPercent20(t *testing.T) {
	if got := rfc5987Encode("a b"); got != "a%20b" {
		t.Errorf("rfc5987Encode(\"a b\") = %q, want a%%20b", got)
	}
}
