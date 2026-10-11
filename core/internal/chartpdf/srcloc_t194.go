package chartpdf

import (
	"strings"
	"unicode"
)

// T194 (anchoring stage 1) — small helpers that give each recorded run its source location (Anchor.SrcLine,
// SrcChar). They never change what is drawn: the PDF bytes are identical with or without them.

// leadingRunes counts the whitespace runes s starts with.
func leadingRunes(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			break
		}
		n++
	}
	return n
}

// lineAt is lines[i], or "" when i is out of range (a chart with no subtitle passes -1).
func lineAt(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return ""
	}
	return lines[i]
}

// titleLoc is where the drawn title starts: the first `# ` line (firstTitle's rule), after its indent, the
// "# " and the blanks before the words. A chart with no `# ` line draws "Chart", which comes from no line:
// it gets line -1.
func titleLoc(lines []string) srcLoc {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "# ") {
			return srcLoc{line: i, char: leadingRunes(l) + 2 + leadingRunes(t[2:])}
		}
	}
	return srcLoc{line: -1}
}

// annotAt is the rune offset of a chord row's "(…)" performance note in the line (chordRowParts' rule:
// the first "("), or 0 when it has none.
func annotAt(line string) int {
	t := strings.TrimRight(line, " \t")
	idx := strings.IndexByte(t, '(')
	if idx < 0 {
		return 0
	}
	return len([]rune(t[:idx]))
}
