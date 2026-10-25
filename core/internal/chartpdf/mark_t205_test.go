package chartpdf

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// T205 structural guard (Fable binding): chartpdf must not even DEPEND on internal/pdfmark — a future caller
// can't accidentally wire the mark into generated charts. Asserted on the full transitive dependency set.
func TestChartpdf_DoesNotDependOnPdfmark_T205(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "troubastack/core/internal/chartpdf").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "troubastack/core/internal/pdfmark" {
			t.Fatal("chartpdf transitively depends on internal/pdfmark; generated charts must never carry the mark")
		}
	}
}

// T205 guard: a GENERATED chart must NEVER carry the "Made with TroubaStack" mark. It is the source the bake
// rasterizes for the tablets, so a mark would churn every page's raster hash and put branding on the stage
// display. chartpdf.Render takes no band setting, so no toggle can reach it — this pins that behaviourally.
func TestRender_NoMadeWithMark_T205(t *testing.T) {
	pdf, err := Render(sample)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := pdftotext(t, pdf); strings.Contains(got, "Made with TroubaStack") {
		t.Errorf("generated chart carries the PDF mark; it must not:\n%s", got)
	}
	// And still deterministic (the setting cannot vary it — there is no setting parameter here).
	again, err := Render(sample)
	if err != nil {
		t.Fatalf("render again: %v", err)
	}
	if !bytes.Equal(pdf, again) {
		t.Error("chartpdf.Render not deterministic")
	}
}
