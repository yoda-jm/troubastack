package chartpdf

import (
	"regexp"
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	cases := []struct {
		in    string
		root  int
		minor bool
		ok    bool
	}{
		{"C", 0, false, true},
		{"G", 7, false, true},
		{" A ", 9, false, true}, // trimmed
		{"F#", 6, false, true},
		{"Bb", 10, false, true},
		{"F#m", 6, true, true},
		{"Bbm", 10, true, true},
		{"Cb", 11, false, true}, // Cb = B
		{"B#", 0, false, true},  // B# = C
		{"", 0, false, false},
		{"H", 0, false, false},
		{"Gmaj", 0, false, false}, // not a bare key
		{"Gm7", 0, false, false},
		{"G ", 7, false, true}, // trailing space trimmed
	}
	for _, c := range cases {
		k, ok := ParseKey(c.in)
		if ok != c.ok {
			t.Errorf("ParseKey(%q) ok=%v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && (k.root != c.root || k.minor != c.minor) {
			t.Errorf("ParseKey(%q) = {root:%d minor:%v}, want {root:%d minor:%v}", c.in, k.root, k.minor, c.root, c.minor)
		}
	}
}

func mustKey(t *testing.T, s string) Key {
	t.Helper()
	k, ok := ParseKey(s)
	if !ok {
		t.Fatalf("ParseKey(%q) failed", s)
	}
	return k
}

func TestTransposeTokens(t *testing.T) {
	// G→A (+2). Roots shift, quality preserved, slash bass shifts, N.C. unchanged.
	from, to := mustKey(t, "G"), mustKey(t, "A")
	cases := []struct{ in, want string }{
		{"G", "A"},
		{"Em", "F#m"},
		{"C", "D"},
		{"D7", "E7"},
		{"Cmaj7", "Dmaj7"},
		{"Asus4", "Bsus4"},
		{"G/B", "A/C#"},
		{"D/F#", "E/G#"},
		{"N.C.", "N.C."},
		{"Cadd9", "Dadd9"},
	}
	for _, c := range cases {
		got, err := Transpose(c.in, from, to)
		if err != nil {
			t.Errorf("Transpose(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Transpose(%q) G→A = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTransposeSpelling(t *testing.T) {
	c := mustKey(t, "C")
	// FLAT target (Eb, +3 from C) spells accidentals with flats:
	if got, _ := Transpose("C", c, mustKey(t, "Eb")); got != "Eb" { // pc3
		t.Errorf("C→Eb of C = %q, want Eb", got)
	}
	if got, _ := Transpose("G", c, mustKey(t, "Eb")); got != "Bb" { // 7+3=10
		t.Errorf("G under C→Eb = %q, want Bb (flat)", got)
	}
	// SHARP target (A, +9 from C) spells accidentals with sharps:
	if got, _ := Transpose("B", c, mustKey(t, "A")); got != "G#" { // 11+9=8
		t.Errorf("B under C→A = %q, want G# (sharp)", got)
	}
	// A plain +2 (G→A) leaves naturals natural:
	if got, _ := Transpose("C", mustKey(t, "G"), mustKey(t, "A")); got != "D" {
		t.Errorf("C under G→A = %q, want D", got)
	}
}

func TestTransposeAlignment(t *testing.T) {
	// Chords over words: tokens anchored at their original columns; a grown token
	// (C→C#, +1 char) pushes the following token right to keep ≥1 space, never merges.
	// "C       G       Am" transposed C→Db (+1): each root gains a flat/sharp char.
	from, to := mustKey(t, "C"), mustKey(t, "Db")
	in := "C       G       Am"
	got, err := Transpose(in, from, to)
	if err != nil {
		t.Fatal(err)
	}
	// Every token grows by 1 (Db, Ab, Bbm). No two tokens may touch: at least one
	// space between each, and none reordered.
	fields := strings.Fields(got)
	if len(fields) != 3 {
		t.Fatalf("token count changed: %q → %q", in, got)
	}
	if fields[0] != "Db" || fields[1] != "Ab" || fields[2] != "Bbm" {
		t.Errorf("C→Db of %q = %q; tokens %v", in, got, fields)
	}
	// No double-collision: assert the rendered line has no token overlap (each token
	// separated by ≥1 space is guaranteed by Fields==3 above + monotonic columns).
	if strings.Contains(got, "DbAb") || strings.Contains(got, "AbBb") {
		t.Errorf("tokens merged: %q", got)
	}

	// A collision case: tokens one space apart, first grows → push right.
	in2 := "C G"
	got2, _ := Transpose(in2, from, to) // → "Db Ab"
	if got2 != "Db Ab" {
		t.Errorf("collision push: %q → %q, want %q", in2, got2, "Db Ab")
	}
}

// TestTransposeAlignmentAnchors is the "chords stay over their words" guard (VLL's
// concern): with realistic chord-over-word spacing the columns are preserved exactly,
// shrink never misaligns, and a growth-induced push never cascades past available slack.
func TestTransposeAlignmentAnchors(t *testing.T) {
	cases := []struct{ name, in, from, to, want string }{
		// Wide spacing (the normal case): every chord keeps its exact start column
		// even though all three roots grow by one char.
		{"over-words", "C       G       Am", "C", "Db", "Db      Ab      Bbm"},
		// Shrink: start columns preserved exactly; the inter-chord gap just widens.
		{"shrink", "Db      Ab", "Db", "C", "C       G"},
		// Recovery: the grown Db nudges Ab by one (they were 1 space apart), but Am has
		// slack so Bbm snaps back to its original column — the push does NOT cascade.
		{"recovery", "C G            Am", "C", "Db", "Db Ab          Bbm"},
	}
	for _, tc := range cases {
		from, ok1 := ParseKey(tc.from)
		to, ok2 := ParseKey(tc.to)
		if !ok1 || !ok2 {
			t.Fatalf("%s: bad key", tc.name)
		}
		got, err := Transpose(tc.in, from, to)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: Transpose(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestTransposeOnlyChordRows(t *testing.T) {
	src := "# My Song\n## Verse 1\nC       G\nWhen the night has come\n\nand the land is dark\nAm\nstand by me"
	got, err := Transpose(src, mustKey(t, "C"), mustKey(t, "D"))
	if err != nil {
		t.Fatal(err)
	}
	in := strings.Split(src, "\n")
	out := strings.Split(got, "\n")
	if len(in) != len(out) {
		t.Fatalf("line count changed: %d → %d", len(in), len(out))
	}
	// Non-chord lines byte-identical; classification preserved per line.
	for i := range in {
		wasChord := isChordRow(in[i])
		isChord := isChordRow(out[i])
		if wasChord != isChord {
			t.Errorf("line %d classification changed: %q → %q", i, in[i], out[i])
		}
		if !wasChord && in[i] != out[i] {
			t.Errorf("non-chord line %d changed: %q → %q", i, in[i], out[i])
		}
	}
	// The chord rows actually transposed (C→D, Am→Bm).
	if !strings.Contains(got, "D") || !strings.Contains(got, "Bm") {
		t.Errorf("chords not transposed: %q", got)
	}
}

func TestTransposeRoundTripPitchClass(t *testing.T) {
	src := "# Round\nC C#m F#7 Bb/D G/B N.C. Ebmaj7\n"
	up, dn := mustKey(t, "C"), mustKey(t, "E") // +4
	once, err := Transpose(src, up, dn)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Transpose(once, dn, up) // −4 (≡ +8)
	if err != nil {
		t.Fatal(err)
	}
	// String equality is NOT required (spelling may normalize); pitch classes must match.
	if !equalPitchClasses(t, src, back) {
		t.Errorf("round-trip pitch classes differ:\n orig: %q\n back: %q", src, back)
	}
}

func TestTransposeGeometryInvariant(t *testing.T) {
	// A multi-page fixture: enough chord/lyric pairs to force ≥2 pages. The rendered
	// page count AND the line-count/classification vector must be identical before and
	// after transposition — the invariant that keeps annotations anchored (rule 5).
	var b strings.Builder
	b.WriteString("# Long Chart\n")
	for s := 0; s < 6; s++ {
		b.WriteString("## Section\n")
		for i := 0; i < 8; i++ {
			b.WriteString("C       G       Am      F\n")
			b.WriteString("line of lyrics under the chords here\n")
			b.WriteString("\n")
		}
	}
	src := b.String()

	from, to := mustKey(t, "C"), mustKey(t, "Eb") // +3, flat target (roots grow)
	out, err := Transpose(src, from, to)
	if err != nil {
		t.Fatal(err)
	}

	inLines, outLines := strings.Split(src, "\n"), strings.Split(out, "\n")
	if len(inLines) != len(outLines) {
		t.Fatalf("line count changed: %d → %d", len(inLines), len(outLines))
	}
	for i := range inLines {
		if isChordRow(inLines[i]) != isChordRow(outLines[i]) {
			t.Fatalf("line %d classification changed", i)
		}
	}

	before, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Render(out)
	if err != nil {
		t.Fatal(err)
	}
	pb, pa := pdfPageCount(before), pdfPageCount(after)
	if pb < 2 {
		t.Fatalf("fixture did not span multiple pages (got %d) — not a real multi-page test", pb)
	}
	if pb != pa {
		t.Errorf("rendered page count changed: %d → %d", pb, pa)
	}
}

// --- test helpers ---------------------------------------------------------

var pageObjRe = regexp.MustCompile(`/Type\s*/Page[^s]`)

func pdfPageCount(pdf []byte) int {
	return len(pageObjRe.FindAll(pdf, -1))
}

// equalPitchClasses compares the pitch-class sequence of every chord token in two
// sources (ignores spelling — A# and Bb are equal).
func equalPitchClasses(t *testing.T, a, b string) bool {
	t.Helper()
	return strings.Join(pitchClasses(a), ",") == strings.Join(pitchClasses(b), ",")
}

func pitchClasses(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if !isChordRow(line) {
			continue
		}
		for _, tok := range strings.Fields(line) {
			if tok == "N.C." {
				out = append(out, "NC")
				continue
			}
			main := tok
			if i := strings.IndexByte(tok, '/'); i >= 0 {
				main = tok[:i]
			}
			pc := noteBasePC[main[0]]
			if len(main) > 1 && (main[1] == '#' || main[1] == 'b') {
				if main[1] == '#' {
					pc = (pc + 1) % 12
				} else {
					pc = (pc + 11) % 12
				}
			}
			out = append(out, sharpNames[pc]) // canonical spelling for comparison
		}
	}
	return out
}
