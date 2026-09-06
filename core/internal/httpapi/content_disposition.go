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

// accentFolder maps the common accented Latin letters European band names use to their ASCII base, so the
// old-client fallback degrades "Café" to "Cafe" rather than "Caf" (T162 fix-forward — dropping the rune
// deleted the letter entirely). It only needs to be reasonable, not exhaustive: modern clients read the
// exact name from filename*; this is the legible fallback for the few that don't. Covers Latin-1 Supplement
// plus the common Latin Extended-A letters; ligatures/ß expand (æ→ae, ß→ss).
var accentFolder = strings.NewReplacer(
	"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A", "à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"Æ", "AE", "æ", "ae",
	"Ç", "C", "ç", "c", "Č", "C", "č", "c", "Ć", "C", "ć", "c",
	"È", "E", "É", "E", "Ê", "E", "Ë", "E", "è", "e", "é", "e", "ê", "e", "ë", "e",
	"Ì", "I", "Í", "I", "Î", "I", "Ï", "I", "ì", "i", "í", "i", "î", "i", "ï", "i",
	"Ñ", "N", "ñ", "n", "Ń", "N", "ń", "n",
	"Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O", "Ø", "O", "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o",
	"Œ", "OE", "œ", "oe",
	"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U", "ù", "u", "ú", "u", "û", "u", "ü", "u",
	"Ý", "Y", "ý", "y", "ÿ", "y",
	"ß", "ss",
	"Š", "S", "š", "s", "Ś", "S", "ś", "s", "Ž", "Z", "ž", "z", "Ź", "Z", "ź", "z", "Ż", "Z", "ż", "z",
	"Đ", "D", "đ", "d", "Ł", "L", "ł", "l", "Ř", "R", "ř", "r",
)

// asciiFilename folds accents to their ASCII base (accentFolder), then keeps only printable ASCII, dropping
// the two quoted-string metacharacters (" and \) and all control characters — so the result is always safe
// to place inside a quoted-string unescaped. "Café" becomes "Cafe"; a rune with no ASCII fold (e.g. a
// non-Latin script) is dropped, and the caller supplies a generic when nothing survives. This is the
// OLD-client fallback; the filename* carries the exact name for everyone else.
func asciiFilename(name string) string {
	var b strings.Builder
	for _, r := range accentFolder.Replace(name) {
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
