package main

import (
	"regexp"
	"strconv"
	"testing"
)

// TestGeneratePlaceholderPDF_PageCount guards against the auto-page-break
// doubling bug: the footer near the page bottom must NOT spill onto extra pages.
// A request for N pages must produce exactly N.
func TestGeneratePlaceholderPDF_PageCount(t *testing.T) {
	countRe := regexp.MustCompile(`/Count (\d+)`)
	for _, pages := range []int{1, 2, 3, 4} {
		b, err := generatePlaceholderPDF(pdfSource{title: "T", subtitle: "S", pages: pages})
		if err != nil {
			t.Fatalf("pages=%d: %v", pages, err)
		}
		m := countRe.FindSubmatch(b)
		if m == nil {
			t.Fatalf("pages=%d: no /Count in PDF", pages)
		}
		got, _ := strconv.Atoi(string(m[1]))
		if got != pages {
			t.Fatalf("pages=%d: /Count = %d, want %d (auto-page-break doubling?)", pages, got, pages)
		}
	}
}
