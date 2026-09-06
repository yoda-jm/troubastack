package httpapi

import (
	"fmt"
	"strings"
)

// contentDisposition builds a Content-Disposition header value that carries a possibly-non-ASCII
// filename safely (T162). A bare `filename="…"` is an RFC 6266 quoted-string (Latin-1), so UTF-8 bytes
// dropped into it are undefined and browsers mangle accents differently. We emit BOTH: a quoted ASCII
// `filename` fallback for old clients AND an RFC 5987 `filename*=UTF-8”…` (percent-encoded) that modern
// clients prefer and decode correctly.
//
// disposition is "attachment" or "inline". Quote, backslash and control characters are stripped from the
// ASCII fallback so a name can never break out of the quoted-string or inject a header. When no ASCII
// survives (a fully non-Latin name), `fallback` is used so the header is never `filename=""`.
func contentDisposition(disposition, name, fallback string) string {
	ascii := asciiFilename(name)
	if ascii == "" {
		ascii = asciiFilename(fallback)
	}
	if ascii == "" {
		ascii = "download"
	}
	encoded := name
	if encoded == "" {
		encoded = fallback
	}
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, ascii, rfc5987Encode(encoded))
}

// asciiFilename keeps only printable ASCII, dropping the two quoted-string metacharacters (" and \) and
// all control characters — so the result is always safe to place inside a quoted-string unescaped. It
// strips (does not transliterate) non-ASCII, so "Bånd" becomes "Bnd"; that is the OLD-client fallback,
// while the filename* carries the exact name for everyone else.
func asciiFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 0x20 && r < 0x7f && r != '"' && r != '\\' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// rfc5987Encode percent-encodes a UTF-8 string for the RFC 5987 ext-value grammar: attr-char bytes pass
// through, everything else (including space, which must be %20 and NOT '+') is %XX. Encoding is byte-wise
// over the UTF-8 representation, which is exactly what "UTF-8”" declares.
func rfc5987Encode(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isAttrChar(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(upperhex[c>>4])
			b.WriteByte(upperhex[c&0x0f])
		}
	}
	return b.String()
}

// isAttrChar reports whether c is an RFC 5987 attr-char: ALPHA / DIGIT / one of "!#$&+-.^_`|~".
func isAttrChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
