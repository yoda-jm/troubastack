package httpapi

import "testing"

// T79 — the download Content-Disposition name: a clean (extension-less) stored name gains the
// extension implied by its content type, while a pre-T79 name that already carries one is left
// alone (never doubled).
func TestDownloadFilename(t *testing.T) {
	cases := []struct {
		name, contentType, want string
	}{
		{"Riverside Waltz", "application/pdf", "Riverside Waltz.pdf"}, // clean name → gains .pdf
		{"scan_001", "application/pdf", "scan_001.pdf"},
		{"x.pdf", "application/pdf", "x.pdf"},            // already has it → no double
		{"x.PDF", "application/pdf", "x.PDF"},            // case-insensitive: still no double
		{"Cover", "image/png", "Cover.png"},              // png
		{"Cover", "image/jpeg", "Cover.jpg"},             // jpeg → .jpg
		{"photo.jpeg", "image/jpeg", "photo.jpeg"},       // existing .jpeg not doubled
		{"weird", "text/plain", "weird"},                 // unknown type → unchanged
		{"a.pdf", "application/pdf; charset=x", "a.pdf"}, // parameters ignored
	}
	for _, c := range cases {
		if got := downloadFilename(c.name, c.contentType); got != c.want {
			t.Errorf("downloadFilename(%q, %q) = %q, want %q", c.name, c.contentType, got, c.want)
		}
	}
}
