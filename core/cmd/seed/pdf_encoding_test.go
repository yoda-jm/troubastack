package main

import (
	"os"
	"testing"

	"github.com/go-pdf/fpdf"
)

// TestEmDashEncodesToCP1252 guards the mechanism the placeholder generator relies
// on: fpdf's default (cp1252) translator must map the em-dash to the single byte
// 0x97 that the core Helvetica font renders. If this regressed, "—" would be
// emitted as its three raw UTF-8 bytes and render as the "â€"" mojibake (T16).
func TestEmDashEncodesToCP1252(t *testing.T) {
	tr := fpdf.New("P", "mm", "A4", "").UnicodeTranslatorFromDescriptor("")
	for _, tc := range []struct {
		in, want string
	}{
		{"—", "\x97"}, // U+2014 em-dash → cp1252 0x97
		{"é", "\xe9"}, // sanity: a plain Latin-1 accent survives too
		{"page 1 / 2", "page 1 / 2"},
	} {
		if got := tr(tc.in); got != tc.want {
			t.Errorf("tr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestGeneratePlaceholderPDF is a smoke check: a title carrying an em-dash
// generates a well-formed, non-trivial PDF without error. When TROUBA_DUMP_PDF
// is set to a path, the bytes are written there so the render can be inspected
// (pdftoppm) — off in CI, so this stays a pure in-memory check by default.
func TestGeneratePlaceholderPDF(t *testing.T) {
	b, err := generatePlaceholderPDF(pdfSource{
		title:    "Wonderwall — Vocals",
		subtitle: "Oasis — lead vocal",
		pages:    1,
	})
	if err != nil {
		t.Fatalf("generatePlaceholderPDF: %v", err)
	}
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("output not a plausible PDF: %d bytes, head %q", len(b), b[:min(5, len(b))])
	}
	if p := os.Getenv("TROUBA_DUMP_PDF"); p != "" {
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatalf("dump: %v", err)
		}
	}
}
