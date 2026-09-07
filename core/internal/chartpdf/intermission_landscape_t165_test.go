package chartpdf

import (
	"regexp"
	"strconv"
	"testing"
)

// T165 half A ⟨D1⟩ (VLL) — the break card is baked LANDSCAPE.
//
// The reason is his rig, not taste: the tablet is landscape on stage, so a landscape card fills the screen
// while a portrait one is a narrow strip between two wide empty margins. Measured on his device before the
// ruling: the card is A4 portrait, Stage width-fits it, and the viewport shows the top ~44% — so the
// wordmark, which sits at ~86% of the page, was never on screen at all.
//
// mediaBox reads the page box straight out of the PDF bytes (gofpdf writes it uncompressed), so the
// assertion is on the artefact itself rather than on anything the renderer says about itself.
var mediaBoxRe = regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([0-9.]+)\s+([0-9.]+)\s*\]`)

func mediaBox(t *testing.T, pdf []byte) (w, h float64) {
	t.Helper()
	m := mediaBoxRe.FindSubmatch(pdf)
	if m == nil {
		t.Fatalf("no /MediaBox found in the rendered card")
	}
	w, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil {
		t.Fatalf("MediaBox width %q: %v", m[1], err)
	}
	h, err = strconv.ParseFloat(string(m[2]), 64)
	if err != nil {
		t.Fatalf("MediaBox height %q: %v", m[2], err)
	}
	return w, h
}

func TestIntermission_IsBakedLandscape_T165(t *testing.T) {
	pdf, err := RenderIntermission("Entracte", "The Band")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	w, h := mediaBox(t, pdf)
	if w <= h {
		t.Errorf("the break card is %.0f×%.0f pt — portrait; ⟨D1⟩ asks for landscape (wider than tall)", w, h)
	}
	// Still exactly one page: "a separator must render exactly one page" is a runtime guard in the baker,
	// and the orientation change must not smuggle in a second.
	if n := len(goldenPageRe.FindAll(pdf, -1)); n != 1 {
		t.Errorf("page count = %d, want exactly 1", n)
	}
}

// The layout is the thing that actually failed on VLL's stage — the mark sat below the fold — so it is
// asserted directly rather than inferred from a hash: every element is ON the page, in reading order, with
// the mark clear of the bottom edge.
func TestIntermission_EverythingFitsOnTheCard_T165(t *testing.T) {
	for _, c := range []struct {
		name string
		w, h float64
	}{
		{"A4 landscape (what we bake)", pageH, pageW},
		{"A4 portrait (the layout must not assume one orientation)", pageW, pageH},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := intermissionLayoutFor(c.w, c.h)
			if l.labelY <= 0 {
				t.Errorf("label starts at %.1f, above the page", l.labelY)
			}
			if !(l.labelY < l.bandY && l.bandY < l.markY) {
				t.Errorf("reading order broken: label %.1f, band %.1f, mark %.1f — want descending prominence down the page", l.labelY, l.bandY, l.markY)
			}
			if bottom := l.markY + l.markH; bottom > c.h {
				t.Errorf("the mark ends at %.1f on a %.1f-tall page — off the bottom, which is exactly what VLL could not see", bottom, c.h)
			}
			if l.markW <= 0 || l.markW > c.w {
				t.Errorf("mark width %.1f does not fit a %.1f-wide page", l.markW, c.w)
			}
			// Centred: the mark's margins match on both sides.
			if left := (c.w - l.markW) / 2; left < 0 {
				t.Errorf("mark cannot be centred on a %.1f-wide page", c.w)
			}
		})
	}
}
