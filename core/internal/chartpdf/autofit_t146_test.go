package chartpdf

import (
	"regexp"
	"strings"
	"testing"
)

var t146PageRe = regexp.MustCompile(`/Type\s*/Page[^s]`)

func t146Pages(t *testing.T, src string) int {
	t.Helper()
	pdf, err := Render(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return len(t146PageRe.FindAll(pdf, -1))
}

// TestAutoFitIsOptIn (T146 ⟨D1⟩, VLL): auto-fit is opt-in, never the default. A chart long enough to
// overflow one page at defaultBodyPt renders across MULTIPLE pages with no directive (one size across a
// setlist), and shrinks onto ONE page only when the source opts in with `fit: page`. A manual `size:`
// still overrides both.
//
// Red-first: before this change the default WAS auto-fit, so the no-directive case fit one page — the
// first assertion fails against today's code. And `fit: page` was not a directive, so there was nothing
// to opt into.
func TestAutoFitIsOptIn(t *testing.T) {
	body := strings.Repeat("## Section\nC       G       Am      F\na line of lyric that carries the tune along\n\n", 15)

	// No directive → defaultBodyPt, paginated.
	def := "# Long Chart\n\n" + body
	if p := t146Pages(t, def); p < 2 {
		t.Fatalf("no-directive render = %d page(s); ⟨D1⟩ requires the default to paginate at defaultBodyPt, not shrink", p)
	}

	// Opt in → auto-fit shrinks it onto one page.
	optin := "# Long Chart\nfit: page\n\n" + body
	if p := t146Pages(t, optin); p != 1 {
		t.Fatalf("`fit: page` render = %d page(s), want 1 (auto-fit should shrink it onto one page)", p)
	}

	// The point of the feature: opt-in fits fewer pages than the default (smaller type, one page).
	if t146Pages(t, optin) >= t146Pages(t, def) {
		t.Fatal("`fit: page` did not reduce the page count below the default — auto-fit is not engaging on opt-in")
	}

	// `fit: auto` is an accepted alias.
	if p := t146Pages(t, "# Long Chart\nfit: auto\n\n"+body); p != 1 {
		t.Fatalf("`fit: auto` render = %d page(s), want 1 (alias of fit: page)", p)
	}

	// A manual `size:` still disables auto-fit even with `fit: page` present — the author's explicit size wins.
	sized := "# Long Chart\nsize: 11\nfit: page\n\n" + body
	if p := t146Pages(t, sized); p < 2 {
		t.Fatalf("`size: 11` + `fit: page` = %d page(s); a manual size must override the fit opt-in and paginate", p)
	}
}
