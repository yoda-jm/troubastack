package chartpdf

import (
	"strings"
	"testing"
	"troubastack/core/internal/domain"
)

func anchorsOf(t *testing.T, src string) []Anchor {
	t.Helper()
	_, a, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	return a
}

func findAnchor(anchors []Anchor, text string) (Anchor, bool) {
	for _, a := range anchors {
		if a.Text == text {
			return a, true
		}
	}
	return Anchor{}, false
}

// TestSourceAnchor_SurvivesReflow (T145): a mark anchored to a run stays on the SAME words after the chart
// reflows. Same source, two renders at different sizes → a run near the end lands on a different page. The
// SourceAnchor resolves to that run in both; a frozen (page, x/y) — the pre-T145 model — would land on
// different text (or a page that no longer exists) after the reflow. That contrast IS the bug this fixes,
// so the test asserts both halves.
func TestSourceAnchor_SurvivesReflow(t *testing.T) {
	words := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo", "sierra", "tango",
	}
	var body strings.Builder
	for _, w := range words {
		body.WriteString("C       G\nthe verse line called " + w + "\n\n")
	}
	small := anchorsOf(t, "# Reflow Test\nsize: 8\n\n"+body.String())
	big := anchorsOf(t, "# Reflow Test\nsize: 16\n\n"+body.String())

	// Find a lyric run that reflowed onto a different page between the two sizes.
	var target string
	var pSmall, pBig int
	for _, w := range words {
		text := "the verse line called " + w
		as, oks := findAnchor(small, text)
		ab, okb := findAnchor(big, text)
		if oks && okb && as.Page != ab.Page {
			target, pSmall, pBig = text, as.Page, ab.Page
			break
		}
	}
	if target == "" {
		t.Fatal("no run reflowed across a page boundary between the two sizes — fixture too short to exercise reflow")
	}

	// The mark: a highlight over the whole target run. SourceAnchor carries no page/coords.
	sa := domain.SourceAnchor{RunText: target, Occurrence: 1, CharStart: 0, CharEnd: len([]rune(target))}

	// It resolves to the SAME words in both renders — on the page each render put the run.
	if pg, _, _, _, _, ok := Project(sa, small); !ok || pg != pSmall {
		t.Fatalf("project into the small render: ok=%v page=%d, want page %d", ok, pg, pSmall)
	}
	if pg, _, _, _, _, ok := Project(sa, big); !ok || pg != pBig {
		t.Fatalf("project into the big render: ok=%v page=%d, want page %d", ok, pg, pBig)
	}
	if pSmall == pBig {
		t.Fatal("test setup: the target did not actually change page")
	}

	// TEETH: the frozen-coordinate model breaks. Take the run's box in the BIG render and read what sits at
	// those same coordinates in the SMALL render — a pre-T145 mark kept exactly those numbers. It must NOT
	// still be the target (either a different run, or that page does not exist in the small render).
	ab, _ := findAnchor(big, target)
	if frozen, ok := AnchorAt(small, ab.Page, ab.X0, ab.Y0, ab.X1, ab.Y1); ok && frozen.RunText == target {
		t.Fatalf("frozen coords still hit %q after reflow — the fixture does not actually move the words, so it guards nothing", target)
	}
}
