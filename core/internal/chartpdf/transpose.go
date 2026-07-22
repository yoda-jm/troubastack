// Chord transposition for the chart dialect (T60, Part A). One engine, used by all
// three surfaces (editor apply, playlist preview, bake) — the client never transposes,
// it only displays server output, so there is no second implementation to drift.
//
// The load-bearing invariant: Transpose changes ONLY chord rows and returns the SAME
// number of lines it was given. Because the renderer paginates purely by line count
// (it never wraps — fpdf Cell(0,…) clips), a line-count-preserving transpose preserves
// pagination and page geometry exactly, which is what keeps existing layer annotations
// anchored (T60 spec rule 5).
package chartpdf

import (
	"fmt"
	"strings"
)

// Key is a parsed musical key: a pitch class (0=C … 11=B) plus major/minor. Mode is
// carried for flat/sharp spelling only — transposition shifts pitch, it never re-modes.
type Key struct {
	root  int // pitch class, 0..11 (C=0)
	minor bool
}

// noteBasePC maps a natural note letter to its pitch class.
var noteBasePC = map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}

// sharpNames / flatNames spell a pitch class. One table each, unit-tested.
var sharpNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
var flatNames = [12]string{"C", "Db", "D", "Eb", "E", "F", "Gb", "G", "Ab", "A", "Bb", "B"}

// flatMajorPC / flatMinorPC: the keys that conventionally spell with flats (T60 rule 3).
// Majors: F Bb Eb Ab Db Gb. Minors: Dm Gm Cm Fm Bbm Ebm. Everything else uses sharps.
var flatMajorPC = map[int]bool{5: true, 10: true, 3: true, 8: true, 1: true, 6: true}
var flatMinorPC = map[int]bool{2: true, 7: true, 0: true, 5: true, 10: true, 3: true}

// ParseKey parses a free-text key ("G", "F#m", "Bbm"). Strict: ^[A-G](#|b)?m?$ after
// TrimSpace. Returns ok=false otherwise (the caller falls back to a semitone stepper).
func ParseKey(s string) (Key, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Key{}, false
	}
	base, ok := noteBasePC[s[0]]
	if !ok {
		return Key{}, false
	}
	pc := base
	i := 1
	if i < len(s) && (s[i] == '#' || s[i] == 'b') {
		if s[i] == '#' {
			pc = (pc + 1) % 12
		} else {
			pc = (pc + 11) % 12
		}
		i++
	}
	minor := false
	if i < len(s) && s[i] == 'm' {
		minor = true
		i++
	}
	if i != len(s) {
		return Key{}, false // trailing junk → not a bare key
	}
	return Key{root: pc, minor: minor}, true
}

// String renders a Key back to text (for "also update the song key"). Uses the same
// flat/sharp preference as transposition so the stored key matches the printed chords.
func (k Key) String() string {
	name := sharpNames[k.root]
	if preferFlats(k) {
		name = flatNames[k.root]
	}
	if k.minor {
		name += "m"
	}
	return name
}

// preferFlats reports whether a key spells accidentals with flats.
func preferFlats(k Key) bool {
	if k.minor {
		return flatMinorPC[k.root]
	}
	return flatMajorPC[k.root]
}

// Transpose rewrites every chord row in source by the pitch-class interval from→to,
// leaving all other lines byte-identical. Line count is preserved exactly.
func Transpose(source string, from, to Key) (string, error) {
	interval := ((to.root-from.root)%12 + 12) % 12
	return transposeBy(source, interval, preferFlats(to))
}

// TransposeSemitones shifts every chord row by a raw semitone count — the well-defined
// fallback when the song's key is free text that ParseKey can't read (so there is no
// from/to to derive an interval from). Spelling defaults to sharps (no key context to
// prefer flats). Negative and >12 counts are normalized mod 12.
func TransposeSemitones(source string, semitones int) (string, error) {
	return transposeBy(source, semitones, false)
}

// transposeBy is the interval-only core (also the well-defined semitone-stepper path).
// flat selects the spelling table for the rewritten roots.
func transposeBy(source string, interval int, flat bool) (string, error) {
	interval = ((interval % 12) + 12) % 12
	lines := strings.Split(source, "\n")
	for idx, line := range lines {
		// Preserve a trailing CR (a \r\n source stays \r\n) — we only touch chord rows.
		cr := ""
		body := line
		if strings.HasSuffix(body, "\r") {
			cr, body = "\r", body[:len(body)-1]
		}
		// Classify with the SAME predicate the renderer uses — agreement by construction.
		if !isChordRow(body) {
			continue
		}
		rewritten, err := transposeChordRow(body, interval, flat)
		if err != nil {
			return "", err
		}
		lines[idx] = rewritten + cr
	}
	return strings.Join(lines, "\n"), nil
}

// transposeChordRow rewrites each chord token in place, keeping every token anchored at
// its original start column; when a rewritten token grows into the next token's column
// the following tokens are pushed right just enough to keep ≥1 space (never merged or
// reordered). Padding between tokens is otherwise recomputed from the original columns.
func transposeChordRow(line string, interval int, flat bool) (string, error) {
	runes := []rune(line)
	var b strings.Builder
	col := 0 // current output column (rune count already emitted)
	i := 0
	for i < len(runes) {
		if runes[i] == ' ' || runes[i] == '\t' {
			i++
			continue
		}
		start := i
		for i < len(runes) && runes[i] != ' ' && runes[i] != '\t' {
			i++
		}
		tok := string(runes[start:i])
		out, err := transposeToken(tok, interval, flat)
		if err != nil {
			return "", err
		}
		// Anchor at the original start column, but never overwrite the previous token:
		// keep at least one space between tokens (push right on collision).
		target := start
		if target < col+1 && col > 0 {
			target = col + 1
		}
		if b.Len() == 0 && start > 0 {
			// leading indent of the first token is preserved verbatim
			target = start
		}
		for col < target {
			b.WriteByte(' ')
			col++
		}
		b.WriteString(out)
		col += len([]rune(out))
	}
	return b.String(), nil
}

// transposeToken transposes a single chord token: the root and the optional /bass shift
// by interval; the quality/extension (m, maj7, sus4, …) is preserved verbatim. "N.C." is
// unchanged. Assumes tok already matched chordToken (isChordRow guaranteed it).
func transposeToken(tok string, interval int, flat bool) (string, error) {
	if tok == "N.C." {
		return tok, nil
	}
	slash := strings.IndexByte(tok, '/')
	main := tok
	bass := ""
	if slash >= 0 {
		main, bass = tok[:slash], tok[slash+1:]
	}
	root, quality, err := shiftRoot(main, interval, flat)
	if err != nil {
		return "", err
	}
	out := root + quality
	if slash >= 0 {
		newBass, _, err := shiftRoot(bass, interval, flat)
		if err != nil {
			return "", err
		}
		out += "/" + newBass
	}
	return out, nil
}

// shiftRoot splits a chord body into its root note + trailing quality, shifts the root
// by interval, and returns (newRoot, quality). e.g. "C#m7" → ("Eb"|"D#", "m7").
func shiftRoot(s string, interval int, flat bool) (root, quality string, err error) {
	if s == "" {
		return "", "", fmt.Errorf("chartpdf: empty chord root")
	}
	base, ok := noteBasePC[s[0]]
	if !ok {
		return "", "", fmt.Errorf("chartpdf: bad chord root %q", s)
	}
	pc := base
	i := 1
	if i < len(s) && (s[i] == '#' || s[i] == 'b') {
		if s[i] == '#' {
			pc = (pc + 1) % 12
		} else {
			pc = (pc + 11) % 12
		}
		i++
	}
	quality = s[i:]
	pc = (pc + interval) % 12
	if flat {
		root = flatNames[pc]
	} else {
		root = sharpNames[pc]
	}
	return root, quality, nil
}
