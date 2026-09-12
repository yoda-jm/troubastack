package chartpdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// The body used by the size tests below. Chords over lyrics so both leadings are exercised; invented.
const sizeBody = "\n\n## Verse\nC       G       Am      F\nthe kettle hums a quiet tune\nF       C       G\nbeneath a paper moon\n"

// TestDefaultIsExactlyAnExplicitSize is the property that makes moving the default SAFE, and the one that
// breaks the moment somebody conflates the two constants again.
//
// `defaultBodyPt` and `scaleRefBodyPt` were a single constant until 2026-09-12, which hid the fact that
// they are different things: one is the size a chart gets when it asks for nothing, the other is the size
// every metric is calibrated at. Raising the single constant would have given a default chart 13 pt type
// over leading still calibrated for 11 — tight lines — AND silently rescaled every explicitly-sized chart
// in the opposite direction. Splitting them means the default is a VALUE INSIDE THE EXISTING MODEL:
//
//	a chart with no directive  ==  the same chart with `size: <defaultBodyPt>`, byte for byte.
//
// If that ever stops holding, the default has grown behaviour of its own and the two constants have been
// merged back by accident.
func TestDefaultIsExactlyAnExplicitSize(t *testing.T) {
	bare, err := Render("# T" + sizeBody)
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := Render("# T\nsize: 13" + sizeBody)
	if err != nil {
		t.Fatal(err)
	}
	if defaultBodyPt != 13 {
		t.Fatalf("this test hard-codes `size: 13` to stay readable; defaultBodyPt is %.0f — update both together", defaultBodyPt)
	}
	if !bytes.Equal(bare, explicit) {
		t.Errorf("a no-directive chart (%d B) is not byte-identical to the same chart at `size: %.0f` (%d B) — "+
			"the default has acquired behaviour an explicit size does not have", len(bare), defaultBodyPt, len(explicit))
	}
}

// TestExplicitSizeIsUnaffectedByTheDefault: `size:` is the author's own decision and must not move when
// the default does. `size: 11` is the calibration size, so it is also the chart that must render at
// exactly scale 1 — the case that would break first if scaleRefBodyPt were ever set to the default.
func TestExplicitSizeIsUnaffectedByTheDefault(t *testing.T) {
	if scaleRefBodyPt != 11 {
		t.Fatalf("scaleRefBodyPt moved to %.0f — every explicitly-sized chart just rescaled; that is a "+
			"layout change for every author who wrote a `size:`, not a default change", scaleRefBodyPt)
	}
	atRef, err := Render("# T\nsize: 11" + sizeBody)
	if err != nil {
		t.Fatal(err)
	}
	// Measured on `origin/main` BEFORE the default moved, when a no-directive chart and `size: 11` were
	// the same bytes. Pinning the digest rather than "it differs from the default" is what makes this a
	// guard for the author who already wrote `size: 11` in a chart that is sitting on VLL's server.
	sum := sha256.Sum256(atRef)
	if got := hex.EncodeToString(sum[:])[:16]; got != "1397f686ee53c03c" {
		t.Errorf("`size: 11` renders %s, want 1397f686ee53c03c — an explicitly-sized chart changed under "+
			"its author; only no-directive charts were supposed to move", got)
	}
	bare, err := Render("# T" + sizeBody)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(atRef, bare) {
		t.Error("`size: 11` and the no-directive default are identical — the default did not actually move")
	}
}
