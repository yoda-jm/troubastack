package app

import (
	"strconv"
	"strings"
	"unicode"
)

// meterDenoms is the set of note-value denominators a metre may use.
var meterDenoms = map[int]bool{1: true, 2: true, 4: true, 8: true, 16: true}

// ParseMeter resolves a metre string to its GROUP LENGTHS in metric units — the one
// primitive the whole beat is built on (T86). "4/4"→[1,1,1,1], "6/8"→[3,3],
// additive "3+4/8"→[3,4]. Derivation:
//
//   - additive numerator ("3+4"): the grouping is taken literally;
//   - compound (n%3==0 && n>3): n/3 groups of 3;
//   - simple: n groups of 1.
//
// It is deliberately LENIENT: ok=false for anything malformed, and every caller treats
// that as unset (= today's 4/4) rather than rejecting a save — a typo in the metre must
// never fail a save or break a gig. See docs/tasks/T86-song-meter.md.
func ParseMeter(s string) (groups []int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	num, den, found := strings.Cut(s, "/")
	if !found {
		return nil, false
	}
	d, err := strconv.Atoi(strings.TrimSpace(den))
	if err != nil || !meterDenoms[d] {
		return nil, false
	}
	num = strings.TrimSpace(num)
	if strings.Contains(num, "+") {
		// Additive: the numerator IS the grouping, taken literally.
		for _, part := range strings.Split(num, "+") {
			g, gerr := strconv.Atoi(strings.TrimSpace(part))
			if gerr != nil {
				return nil, false
			}
			groups = append(groups, g)
		}
	} else {
		n, nerr := strconv.Atoi(num)
		if nerr != nil || n < 1 || n > 32 {
			return nil, false
		}
		switch {
		case n%3 == 0 && n > 3: // compound → groups of 3
			for i := 0; i < n/3; i++ {
				groups = append(groups, 3)
			}
		default: // simple → groups of 1
			for i := 0; i < n; i++ {
				groups = append(groups, 1)
			}
		}
	}
	if !validMeterGroups(groups) {
		return nil, false
	}
	return groups, true
}

// validMeterGroups enforces the lenient bounds: at least one group, at most 16, each
// 1–32, summing to no more than 64 metric units.
func validMeterGroups(groups []int) bool {
	if len(groups) == 0 || len(groups) > 16 {
		return false
	}
	sum := 0
	for _, g := range groups {
		if g < 1 || g > 32 {
			return false
		}
		sum += g
	}
	return sum <= 64
}

// NormalizeMeter returns the metre to store: a canonical form with ALL whitespace
// removed ("6 / 8" → "6/8") if it parses, else "" (unset, read as 4/4). It does NOT
// rebuild the string from the groups — that would rewrite a musician's "3+3/8" into
// "6/8": the same grid, but not the same intent. Only whitespace is canonicalised.
func NormalizeMeter(s string) string {
	c := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if _, ok := ParseMeter(c); ok {
		return c
	}
	return ""
}
