package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseMeter_vectors runs ParseMeter against the SHARED contract table
// (docs/contracts/meter-groups.vectors.json) that the TS studio (meterGroups) and, from A35,
// Kotlin also run — so the three lenient copies of this parser cannot silently drift (T92). The
// in-code tables above stay as readable documentation; this file is the source of truth. A null
// `groups` means "the parser must treat this as unset": here that is ok=false (see the file's
// _comment for why each runtime asserts unset differently).
func TestParseMeter_vectors(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "contracts", "meter-groups.vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Cases []struct {
			Meter  string `json:"meter"`
			Groups []int  `json:"groups"` // null → nil → expect unset (ok=false)
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	valid, unset := 0, 0
	for _, c := range doc.Cases {
		got, ok := ParseMeter(c.Meter)
		if c.Groups == nil {
			unset++
			if ok {
				t.Errorf("ParseMeter(%q) = %v, ok=true; vectors say unset", c.Meter, got)
			}
			continue
		}
		valid++
		if !ok {
			t.Errorf("ParseMeter(%q): ok=false; vectors want %v", c.Meter, c.Groups)
			continue
		}
		if !reflect.DeepEqual(got, c.Groups) {
			t.Errorf("ParseMeter(%q) = %v, want %v", c.Meter, got, c.Groups)
		}
	}
	// Assert the counts so a truncated or half-written file fails loudly rather than passing fewer
	// cases in silence (the acceptance floor: ≥13 valid, ≥18 malformed).
	if valid < 13 {
		t.Errorf("vectors have %d valid cases, want >= 13 (truncated file?)", valid)
	}
	if unset < 18 {
		t.Errorf("vectors have %d malformed cases, want >= 18 (truncated file?)", unset)
	}
}

// The parser table from the T86 acceptance criteria, asserted as GROUPS.
func TestParseMeter_groups(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"4/4", []int{1, 1, 1, 1}},
		{"3/4", []int{1, 1, 1}}, // 3 is not > 3, so simple
		{"2/2", []int{1, 1}},    // denominator 2 is allowed
		{"6/8", []int{3, 3}},    // compound
		{"9/8", []int{3, 3, 3}}, // compound
		{"12/8", []int{3, 3, 3, 3}},
		{"5/4", []int{1, 1, 1, 1, 1}}, // no grouping assumed
		{"3/8", []int{1, 1, 1}},       // 3 not > 3 → simple
		{"3+2/8", []int{3, 2}},        // additive
		{"3+4/8", []int{3, 4}},        // additive
		{"2+2+3/8", []int{2, 2, 3}},   // additive
		{"3+3+1/4", []int{3, 3, 1}},   // additive
		{" 6 / 8 ", []int{3, 3}},      // whitespace tolerated
	}
	for _, c := range cases {
		got, ok := ParseMeter(c.in)
		if !ok {
			t.Errorf("ParseMeter(%q): ok=false, want groups %v", c.in, c.want)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseMeter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Everything malformed parses as unset (ok=false) rather than an error — leniency is
// the whole point (a typo must not fail a save).
func TestParseMeter_unset(t *testing.T) {
	bad := []string{
		"",
		"x/y",
		"4/5",                                 // denominator not in {1,2,4,8,16}
		"0/4",                                 // zero units → no group
		"33/4",                                // 33 groups of 1 → more than 16 groups
		"3+0/8",                               // a zero-length group
		"1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1+1/8", // 17 groups
		"4",                                   // no denominator
		"-3/4",                                // negative numerator
		"4/4/4",                               // trailing junk lands in the denominator
	}
	for _, s := range bad {
		if groups, ok := ParseMeter(s); ok {
			t.Errorf("ParseMeter(%q) = %v, ok=true; want unset", s, groups)
		}
	}
}

func TestNormalizeMeter(t *testing.T) {
	// Canonicalises ALL whitespace, including interior — not just a trim.
	for in, want := range map[string]string{
		" 6/8 ":    "6/8",
		"6 / 8":    "6/8",
		"3 + 3/8":  "3+3/8", // additive intent preserved, not collapsed to 6/8
		"nonsense": "",
		"4/5":      "",
	} {
		if got := NormalizeMeter(in); got != want {
			t.Errorf("NormalizeMeter(%q) = %q, want %q", in, got, want)
		}
	}
}
