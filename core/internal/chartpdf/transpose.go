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
	"regexp"
	"strings"
	"unicode"
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

// reBareTabOpenerRewrite matches a bare tab opener (no attributes) so the transpose step can stamp the
// original key onto it — in place, so the line count (and thus pagination/geometry) is preserved.
var reBareTabOpenerRewrite = regexp.MustCompile(`(?i)^(\s*\{(?:start_of_tab|sot))\}(\s*)$`)

// annotateTabOpeners stamps `original=<key>` onto every bare tab opener when a non-zero transposition
// happened (T135). Tab frets are never transposed, so a baked transposed page would otherwise show the
// stave's chord names in the OLD key with nothing saying so; the renderer draws a zero-height marker
// from this attribute. In-place rewrite → line count preserved (the T60 anchoring invariant). No-op at
// a net-zero interval (nothing was transposed) or when there is no key to name.
func annotateTabOpeners(source string, interval int, originalKey string) string {
	if ((interval%12)+12)%12 == 0 || originalKey == "" {
		return source
	}
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		cr := ""
		body := line
		if strings.HasSuffix(body, "\r") {
			cr, body = "\r", body[:len(body)-1]
		}
		if reBareTabOpenerRewrite.MatchString(body) {
			lines[i] = reBareTabOpenerRewrite.ReplaceAllString(body, "${1} original="+originalKey+"}${2}") + cr
		}
	}
	return strings.Join(lines, "\n")
}

// Transpose rewrites every chord row in source by the pitch-class interval from→to,
// leaving all other lines byte-identical. Line count is preserved exactly.
func Transpose(source string, from, to Key) (string, error) {
	interval := ((to.root-from.root)%12 + 12) % 12
	out, err := transposeBy(source, interval, preferFlats(to))
	if err != nil {
		return "", err
	}
	return annotateTabOpeners(out, interval, from.String()), nil
}

// TransposeToKey is Transpose but spells the rewritten roots to match the accidental the
// user actually TYPED in rawTargetKey (D5): "F#" → sharps, "Gb" → flats — even though both
// parse to the same pitch class. This keeps the printed chords consistent with the key the
// user asked for (and, on the editor Apply path, with the key stored on the song). With no
// explicit accidental in rawTargetKey it falls back to the key's default spelling.
func TransposeToKey(source, rawTargetKey string, from, to Key) (string, error) {
	flat := preferFlats(to)
	switch {
	case strings.Contains(rawTargetKey, "#"):
		flat = false
	case strings.ContainsRune(rawTargetKey, 'b'): // lowercase only: the 'b' flat, not the 'B' note
		flat = true
	}
	interval := ((to.root-from.root)%12 + 12) % 12
	out, err := transposeBy(source, interval, flat)
	if err != nil {
		return "", err
	}
	return annotateTabOpeners(out, interval, from.String()), nil
}

// TransposeSemitones shifts every chord row by a raw semitone count — the well-defined
// fallback when the song's key is free text that ParseKey can't read (so there is no
// from/to to derive an interval from). Negative and >12 counts are normalized mod 12.
//
// D6: a net-zero shift is a TRUE no-op (returns the source unchanged, never respelling);
// otherwise the rewritten roots keep the source's existing accidental style (a flat-spelled
// chart stays flat) instead of being forced to sharps, so +1 then −1 round-trips.
func TransposeSemitones(source string, semitones int) (string, error) {
	if ((semitones%12)+12)%12 == 0 {
		return source, nil
	}
	return transposeBy(source, semitones, sourcePrefersFlats(source))
}

// sourcePrefersFlats reports whether the chart's chord rows spell accidentals with flats
// more than sharps — so the semitone stepper can preserve the existing style (D6). In a
// chord token 'b' is only ever a flat accidental and '#' only a sharp (qualities are
// maj/min/m/dim/aug/sus/add + digits + '/'), so a raw count is exact.
func sourcePrefersFlats(source string) bool {
	flats, sharps := 0, 0
	for _, line := range strings.Split(source, "\n") {
		body := strings.TrimSuffix(line, "\r")
		if !isChordRow(body) {
			continue
		}
		for _, r := range body {
			switch r {
			case 'b':
				flats++
			case '#':
				sharps++
			}
		}
	}
	return flats > sharps
}

// transposeBy is the interval-only core (also the well-defined semitone-stepper path).
// flat selects the spelling table for the rewritten roots.
func transposeBy(source string, interval int, flat bool) (string, error) {
	interval = ((interval % 12) + 12) % 12
	lines := strings.Split(source, "\n")
	// T135: a tab block is never transposed — chord names over an untouched stave would print a lie.
	// The block membership uses the SAME predicate the renderer draws with, so what is left un-
	// transposed is exactly what is drawn verbatim (and line count is preserved either way).
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = strings.TrimSuffix(l, "\r")
	}
	inTab := tabContentSet(stripped)
	for idx, line := range lines {
		if inTab[idx] {
			continue // inside a tab block: verbatim, including the chord names over the strings
		}
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
		// Capture the whitespace run before this token. Tokenize on ANY Unicode whitespace,
		// exactly as isChordRow's strings.Fields does (D2) — splitting only on ' '/'\t' let
		// an NBSP glue onto a token (only the first chord transposed; a leading NBSP hit the
		// byte-indexed shiftRoot with a multi-byte rune → "bad chord root").
		wsStart := i
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			i++
		}
		ws := runes[wsStart:i]
		if i >= len(runes) {
			b.WriteString(string(ws)) // trailing whitespace, verbatim
			break
		}
		start := i
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			i++
		}
		if runes[start] == '(' {
			// T73: the terminal "(…)" performance note — prose, never transposed. Emit its
			// leading whitespace + the rest of the line verbatim and stop.
			b.WriteString(string(ws))
			b.WriteString(string(runes[start:]))
			break
		}
		out, err := transposeToken(string(runes[start:i]), interval, flat)
		if err != nil {
			return "", err
		}
		if col == wsStart {
			// No accumulated drift up to here (no earlier token grew/shrank): reproduce the
			// ORIGINAL whitespace verbatim so tabs and exact spacing survive — the chord row
			// stays aligned over the (tab-keeping) lyric row (D7). Also preserves the leading
			// indent of the first token.
			b.WriteString(string(ws))
			col += len(ws)
		} else {
			// Drift: anchor at the original start column, but never overwrite the previous
			// token — keep at least one space, pushing right on collision (computed padding).
			target := start
			if target < col+1 {
				target = col + 1
			}
			for col < target {
				b.WriteByte(' ')
				col++
			}
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
