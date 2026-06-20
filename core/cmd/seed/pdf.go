package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pdf/fpdf"
)

// assetsDir is where fetched/generated demo PDFs are cached so re-runs are fast.
// It is gitignored.
const assetsDir = "cmd/seed/assets"

// pdfSource describes how to obtain a song's PDF. A non-empty URL is tried first
// (public-domain sheet music); on any failure we fall back to a generated
// placeholder. Pages controls the generated fallback's length.
type pdfSource struct {
	cacheName string   // file name under assetsDir (stable per song)
	urls      []string // optional public-domain PDF URLs, tried in order
	title     string   // used in the generated fallback
	subtitle  string   // e.g. composer/artist
	pages     int      // generated fallback page count
}

// pdfResult is the outcome of resolving one source: the bytes plus how we got them.
type pdfResult struct {
	data    []byte
	origin  string // "cache", "fetched", or "generated"
	fetched bool   // true if a real PD PDF was downloaded (this run or cached-from-fetch)
}

// ensureAssetsDir creates the cache dir if missing.
func ensureAssetsDir() error { return os.MkdirAll(assetsDir, 0o755) }

// resolvePDF returns the PDF bytes for a source, using (in order): the on-disk
// cache, a network fetch (short timeout), then a generated placeholder. It never
// returns an error for a missing network — generation always succeeds.
func resolvePDF(src pdfSource) (pdfResult, error) {
	cachePath := filepath.Join(assetsDir, src.cacheName)
	metaPath := cachePath + ".origin"

	// 1. Cache hit.
	if b, err := os.ReadFile(cachePath); err == nil && len(b) > 0 {
		origin := "generated"
		if o, err := os.ReadFile(metaPath); err == nil && string(o) == "fetched" {
			origin = "fetched"
		}
		return pdfResult{data: b, origin: "cache", fetched: origin == "fetched"}, nil
	}

	// 2. Network fetch (best-effort, short timeout).
	for _, u := range src.urls {
		if b, ok := tryFetchPDF(u); ok {
			_ = os.WriteFile(cachePath, b, 0o644)
			_ = os.WriteFile(metaPath, []byte("fetched"), 0o644)
			return pdfResult{data: b, origin: "fetched", fetched: true}, nil
		}
	}

	// 3. Generated fallback (always works, incl. offline).
	b, err := generatePlaceholderPDF(src)
	if err != nil {
		return pdfResult{}, fmt.Errorf("generate placeholder for %q: %w", src.title, err)
	}
	_ = os.WriteFile(cachePath, b, 0o644)
	_ = os.WriteFile(metaPath, []byte("generated"), 0o644)
	return pdfResult{data: b, origin: "generated", fetched: false}, nil
}

// cachedFetched reports whether the cached PDF for a source is a real fetched
// public-domain file (unknown layout) vs a generated placeholder (known layout).
// Used on idempotent re-runs to place annotations with the right coordinates.
func cachedFetched(src pdfSource) bool {
	o, err := os.ReadFile(filepath.Join(assetsDir, src.cacheName) + ".origin")
	return err == nil && string(o) == "fetched"
}

// tryFetchPDF downloads url with a short timeout and validates it looks like a PDF.
func tryFetchPDF(url string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "troubastack-seed/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || len(b) < 5 || string(b[:5]) != "%PDF-" {
		return nil, false
	}
	return b, true
}

// generatePlaceholderPDF builds a simple multi-page PDF: a title, a "page N"
// marker, and a few staff-like horizontal lines per page. Pure Go (go-pdf/fpdf),
// so it works fully offline.
func generatePlaceholderPDF(src pdfSource) ([]byte, error) {
	pages := src.pages
	if pages < 1 {
		pages = 2
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(src.title, false)
	pageW, _ := pdf.GetPageSize()
	left, right := 20.0, pageW-20.0
	for n := 1; n <= pages; n++ {
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 24)
		pdf.SetXY(left, 25)
		pdf.Cell(0, 12, src.title)
		if src.subtitle != "" {
			pdf.SetFont("Helvetica", "I", 14)
			pdf.SetXY(left, 38)
			pdf.Cell(0, 10, src.subtitle)
		}
		pdf.SetFont("Helvetica", "", 12)
		pdf.SetXY(right-40, 25)
		pdf.CellFormat(40, 12, fmt.Sprintf("page %d / %d", n, pages), "", 0, "R", false, 0, "")

		// Staff-like systems: groups of 5 thin horizontal lines.
		pdf.SetLineWidth(0.2)
		y := 60.0
		for system := 0; system < 8; system++ {
			for line := 0; line < 5; line++ {
				pdf.Line(left, y, right, y)
				y += 2.2
			}
			y += 12 // gap between systems
			if y > 270 {
				break
			}
		}
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetXY(left, 285)
		pdf.Cell(0, 6, "TroubaStack demo placeholder — public-domain fetch unavailable, generated locally.")
	}
	var buf writerSink
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.b, nil
}

// writerSink collects PDF output bytes (fpdf wants an io.Writer).
type writerSink struct{ b []byte }

func (w *writerSink) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}
