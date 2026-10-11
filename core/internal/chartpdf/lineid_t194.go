package chartpdf

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"unicode"
)

// T194 — anchoring stage 1: LINE IDENTITY (docs/design/anchoring/critique.md §7, anchoring-spec.md P14,
// P15, P19, P20).
//
// Option G attaches a mark to one source LINE by a stable id, never by text + occurrence. This file is the
// pure half of that: a generated file's list of line ids (LineIdentity), how the ids are carried across a
// source edit by a line diff (CarryLineIdentity), the per-line character map that lets a mark shift sideways
// when its own line is edited (CharMap), and the lookup stage 2 calls (LocateLine). Nothing here projects a
// mark: the T193 freeze stays on, and this only adds data.
//
// The pairing rule, in one place:
//  1. LINES are aligned by a longest common subsequence of their exact text. On a tie, the LATER old line
//     wins: inserting a copy of a line just above it gives the COPY the new id (P14), and deleting one of
//     two identical adjacent lines retires the FIRST. The two rules are inverses, so undoing an insert
//     restores every id.
//  2. Inside a hunk (the old lines and new lines between two aligned lines), a deleted and an inserted line
//     are paired as a MODIFIED line when their character similarity 2·LCS/(len a + len b) is at least
//     modifiedPairMin; the pairs are chosen monotonically to maximise total similarity, so a typo fix keeps
//     its id even when the same hunk also inserts a line. A line rewritten beyond that threshold is a new
//     line: its old id is retired (the mark on it becomes lost, P20), the new text gets a fresh id.
//  3. Every other new line gets a fresh id; every other old id is retired.
//
// A modified line also gets a CharMap (old rune offset → new rune offset). It is built from a word-level
// alignment first, then a character-level one inside the unaligned words, so a word inserted before a
// circled word moves the circled word's characters by exactly the insertion (P19), and a typo inside a
// word only blurs that word.

// modifiedPairMin is the similarity at or above which a deleted and an inserted line in one hunk are taken
// to be the same line, edited. 0.6 keeps a typo (≥0.9 on any line of a few words), an inserted word
// (≈0.8) and an appended chord (≈0.7) paired, and leaves a rewritten lyric (≈0.3–0.45 by chance letter
// matches) unpaired.
const modifiedPairMin = 0.6

// The history keeps, for every render a LIVE link still references, an unbroken chain of steps from that
// render to the current one (T195 review: a step a live link needs must never be dropped — dropping it made
// a mark untouched over 32 saves lose its character map, 4 characters off and "check").
//
//   - recentLineHistory: the newest steps are always kept exactly as recorded. A link written now is
//     measured on the current render, which is the newest step's ToRender, so it is always covered.
//   - Compact (called at serve, where the song's live links are known: httpapi projectSnapshot →
//     app.Service.CompactChartLines) folds the older steps into ONE cumulative step per still-referenced
//     render — from that render to the next referenced one — and drops the steps no link needs. After it,
//     len(History) ≤ recentLineHistory + the number of distinct renders the file's links reference.
//   - maxLineHistory is the hard cap for a chart saved many times with no serve in between (the save path
//     never reads the annotation store, renders nothing and takes no extra lock): past it the oldest steps
//     are folded into one cumulative step from the oldest render. Only a link on a render INSIDE that
//     folded run loses its map (check, never a wrong guess), and only after 512 saves without a single read.
//
// A step holds the character maps of the lines it modified only, so a step is a few dozen bytes.
const (
	recentLineHistory = 32
	maxLineHistory    = 512
)

// LineIdentity is one generated file's line identity: IDs[i] is the stable id of source line i of the
// source whose SourceHash it carries (chartLines(source), so len(IDs) == the line count, blank lines and
// directives included).
type LineIdentity struct {
	// SourceHash is SourceHash(source) of the source these ids describe. Anything that holds ids and a
	// source separately (an export folder, a stale record) checks it before trusting the ids.
	SourceHash string   `json:"sourceHash"`
	IDs        []string `json:"ids"`
	// Rederived is set when the ids were NOT carried by a save's diff: a backfill of a chart that had none,
	// an import whose ids could not be trusted, or a write that bypassed SaveChartSource. Stage 2 marks the
	// linked marks on such a file "to check" and then clears it (ClearRederived). nil = carried by diff.
	Rederived *Rederivation `json:"rederived,omitempty"`
	// History is the recent saves, oldest first, each keyed by the renders (PDF blob hashes) before and
	// after, with the character maps of the lines it modified. CharMapSince composes them.
	History []LineStep `json:"history,omitempty"`
}

// Rederivation says why and when a file's ids were re-derived rather than carried.
type Rederivation struct {
	// Reason is one of the Rederive* constants.
	Reason string `json:"reason"`
	// Render is the render (PDF blob hash) current when the ids were re-derived, when the caller knew it.
	Render string `json:"render,omitempty"`
}

// The reasons a file's ids are re-derived. Each is a write that does not go through SaveChartSource's diff.
const (
	// RederiveBackfill: the chart predates line identity (T194); its ids were minted from its current source.
	RederiveBackfill = "backfill"
	// RederiveImport: a band import (v1 or v2 folder) carried no ids, or ids for a different source.
	RederiveImport = "import"
	// RederiveImportChanged: an import replaced the source of a file that already had ids; the ids were
	// carried by the same diff as a save, but the edit was not made in Studio, so nobody saw it happen.
	RederiveImportChanged = "import-changed"
	// RederiveMismatch: the stored ids did not describe the stored source (some writer changed one without
	// the other); fresh ids were minted.
	RederiveMismatch = "mismatch"
)

// LineStep is one source change: the renders it went from and to, and the character map of every line it
// MODIFIED (paired but with different text), by line id. A line carried with its text unchanged has no
// entry: its map is the identity.
type LineStep struct {
	FromRender string             `json:"fromRender"`
	ToRender   string             `json:"toRender"`
	Maps       map[string]CharMap `json:"maps,omitempty"`
}

// CharMap maps rune offsets of a modified line's OLD text to its NEW text. Keep lists the aligned runs:
// each [3]int is {old offset, new offset, length}, in increasing order; characters outside every run were
// deleted or replaced.
type CharMap struct {
	OldLen int      `json:"oldLen"`
	NewLen int      `json:"newLen"`
	Keep   [][3]int `json:"keep,omitempty"`
	// Lands (T195 re-review) is set only on a map COMPOSED from several steps (composeCharMap): for an old
	// offset whose character did not survive, {old offset, new offset} where the step-by-step mapping lands
	// it, when that differs from the single-map default below. With it, a composed map gives exactly the
	// result of applying its steps one by one — so a serve-time compaction (a read) never moves a mark.
	Lands [][2]int `json:"lands,omitempty"`
}

// Map returns the new offset of the character at old offset off, and whether that character survived the
// edit (exact). A character that was deleted or replaced maps to the position just after the last kept
// character before it (exact=false): stage 2 still moves the mark there and flags it "to check" (P19).
// off == OldLen (the end of the line) maps to NewLen, exactly.
func (m CharMap) Map(off int) (int, bool) {
	if off >= m.OldLen {
		return m.NewLen + (off - m.OldLen), off == m.OldLen
	}
	after := 0
	for _, k := range m.Keep {
		o, n, l := k[0], k[1], k[2]
		if off >= o && off < o+l {
			return n + (off - o), true
		}
		if o+l <= off {
			after = n + l
		}
	}
	for _, ld := range m.Lands {
		if ld[0] == off {
			return ld[1], false
		}
	}
	return after, false
}

// CharPath is the composition of the character maps of several consecutive steps (oldest first). nil or
// empty is the identity.
type CharPath []CharMap

// Map runs off through every map in order; exact only if every step kept the character.
func (p CharPath) Map(off int) (int, bool) {
	exact := true
	for _, m := range p {
		var e bool
		off, e = m.Map(off)
		exact = exact && e
	}
	return off, exact
}

// SourceHash is the hex sha256 of a chart source, as stored in LineIdentity.SourceHash.
func SourceHash(source string) string {
	h := sha256.Sum256([]byte(source))
	return hex.EncodeToString(h[:])
}

// SourceLines splits a chart source into the lines LineIdentity numbers (CRLF-normalised, exactly what
// the renderer walks and what Anchor.SrcLine indexes).
func SourceLines(source string) []string { return chartLines(source) }

// NewLineIdentity mints a fresh id for every line of source. newID must return ids unique for the file's
// whole life (a retired id must never come back: a mark linked to it would land on another line).
func NewLineIdentity(source string, newID func() string) LineIdentity {
	lines := chartLines(source)
	ids := make([]string, len(lines))
	for i := range ids {
		ids[i] = newID()
	}
	return LineIdentity{SourceHash: SourceHash(source), IDs: ids}
}

// Describes reports whether li holds the ids of exactly this source.
func (li LineIdentity) Describes(source string) bool {
	return li.SourceHash == SourceHash(source) && len(li.IDs) == len(chartLines(source))
}

// Index returns the current source line index of the line with this id.
func (li LineIdentity) Index(id string) (int, bool) {
	for i, x := range li.IDs {
		if x == id {
			return i, true
		}
	}
	return -1, false
}

// CarryOptions tunes CarryLineIdentity.
type CarryOptions struct {
	// ByIndex pairs line i with line i, every line modified in place. Only for an edit KNOWN to preserve
	// the line structure — a transposition rewrites every chord row (G C D → A D E is not similar text) and
	// keeps every line where it was. Ignored (the diff runs) if the line counts differ.
	ByIndex bool
	// FromRender / ToRender are the PDF blob hashes before and after the edit, recorded on the History step
	// so stage 2 can compose maps from the render a mark was drawn on.
	FromRender, ToRender string
}

// LinePairs aligns old lines with new ones by the pairing rule above: pairs[j] is the old index paired
// with new line j, or -1 for an inserted line.
func LinePairs(oldLines, newLines []string) []int {
	pairs := lcsPairs(oldLines, newLines)
	// Hunks: runs of unpaired new lines between two aligned lines, with the unpaired old lines between the
	// same two aligned lines. Pair modified lines inside each.
	usedOld := make([]bool, len(oldLines))
	for _, o := range pairs {
		if o >= 0 {
			usedOld[o] = true
		}
	}
	j := 0
	prevOld := -1
	for j <= len(newLines) {
		if j < len(newLines) && pairs[j] >= 0 {
			prevOld = pairs[j]
			j++
			continue
		}
		// a hunk starts at new j (possibly empty on the new side)
		k := j
		for k < len(newLines) && pairs[k] < 0 {
			k++
		}
		nextOld := len(oldLines)
		if k < len(newLines) {
			nextOld = pairs[k]
		}
		var dels []int
		for o := prevOld + 1; o < nextOld; o++ {
			if !usedOld[o] {
				dels = append(dels, o)
			}
		}
		if len(dels) > 0 && k > j {
			ins := make([]int, 0, k-j)
			for x := j; x < k; x++ {
				ins = append(ins, x)
			}
			for _, pr := range pairModified(oldLines, newLines, dels, ins) {
				pairs[pr[1]] = pr[0]
			}
		}
		if k >= len(newLines) {
			break
		}
		j = k
	}
	return pairs
}

// pairModified chooses, monotonically, which deleted lines (dels, old indices) are the inserted lines
// (ins, new indices) edited: the alignment maximising total similarity over pairs at or above
// modifiedPairMin. Returns {old, new} pairs.
func pairModified(oldLines, newLines []string, dels, ins []int) [][2]int {
	n, m := len(dels), len(ins)
	sim := make([][]float64, n)
	for a := range sim {
		sim[a] = make([]float64, m)
		for b := range sim[a] {
			sim[a][b] = similarity(oldLines[dels[a]], newLines[ins[b]])
		}
	}
	// best[a][b]: best total over dels[a:], ins[b:].
	best := make([][]float64, n+1)
	for a := range best {
		best[a] = make([]float64, m+1)
	}
	for a := n - 1; a >= 0; a-- {
		for b := m - 1; b >= 0; b-- {
			v := best[a+1][b]
			if w := best[a][b+1]; w > v {
				v = w
			}
			if s := sim[a][b]; s >= modifiedPairMin && s+best[a+1][b+1] > v {
				v = s + best[a+1][b+1]
			}
			best[a][b] = v
		}
	}
	var out [][2]int
	for a, b := 0, 0; a < n && b < m; {
		if s := sim[a][b]; s >= modifiedPairMin && best[a][b] == s+best[a+1][b+1] {
			out = append(out, [2]int{dels[a], ins[b]})
			a, b = a+1, b+1
		} else if best[a][b] == best[a+1][b] {
			a++
		} else {
			b++
		}
	}
	return out
}

// similarity is 2·LCS/(len a + len b) over runes: 1 for equal text, 0 for nothing in common. Two empty
// lines are equal (1); an empty and a non-empty line share nothing (0).
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra)+len(rb) == 0 {
		return 1
	}
	l := 0
	for _, o := range lcsPairs(ra, rb) {
		if o >= 0 {
			l++
		}
	}
	return 2 * float64(l) / float64(len(ra)+len(rb))
}

// maxLCSCells bounds the quadratic table. A chart is a few hundred lines; past this the diff falls back to
// pairing only the common suffix (every other line gets a fresh id — visible as "lost", never wrong).
const maxLCSCells = 16 << 20

// lcsPairs aligns a and b by a longest common subsequence: out[j] is the index in a matched with b[j], or
// -1. Ties go to the LATEST match: the table is built over prefixes and walked back from the end, matching
// whenever the two elements are equal (always safe for an LCS), so equal elements pair as late as possible
// and an insertion is placed as early as possible.
func lcsPairs[T comparable](a, b []T) []int {
	out := make([]int, len(b))
	for j := range out {
		out[j] = -1
	}
	// A common SUFFIX is matched directly — consistent with late matching. (A common prefix is not: it
	// would match early.)
	n, m := len(a), len(b)
	for n > 0 && m > 0 && a[n-1] == b[m-1] {
		out[m-1] = n - 1
		n, m = n-1, m-1
	}
	if n == 0 || m == 0 || (n+1)*(m+1) > maxLCSCells {
		return out
	}
	w := m + 1
	t := make([]int32, (n+1)*w)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case a[i-1] == b[j-1]:
				t[i*w+j] = t[(i-1)*w+j-1] + 1
			case t[(i-1)*w+j] >= t[i*w+j-1]:
				t[i*w+j] = t[(i-1)*w+j]
			default:
				t[i*w+j] = t[i*w+j-1]
			}
		}
	}
	for i, j := n, m; i > 0 && j > 0; {
		switch {
		case a[i-1] == b[j-1]:
			out[j-1] = i - 1
			i, j = i-1, j-1
		case t[(i-1)*w+j] >= t[i*w+j-1]:
			i--
		default:
			j--
		}
	}
	return out
}

// tokens splits a line into maximal runs of blanks and of non-blanks, with each token's rune offset.
func tokens(rs []rune) (toks []string, offs []int) {
	start := 0
	for i := 1; i <= len(rs); i++ {
		if i == len(rs) || unicode.IsSpace(rs[i]) != unicode.IsSpace(rs[start]) {
			toks = append(toks, string(rs[start:i]))
			offs = append(offs, start)
			start = i
		}
	}
	return toks, offs
}

// BuildCharMap aligns a modified line's old and new text: words first (so an inserted word shifts what
// follows it by exactly its length), then characters inside the words left unaligned between two aligned
// words (so a typo blurs only its own word).
func BuildCharMap(oldText, newText string) CharMap {
	ra, rb := []rune(oldText), []rune(newText)
	oldToNew := make([]int, len(ra))
	for i := range oldToNew {
		oldToNew[i] = -1
	}
	ta, oa := tokens(ra)
	tb, ob := tokens(rb)
	tp := lcsPairs(ta, tb)
	// charGap aligns old runes [a0,a1) with new runes [b0,b1) character by character.
	charGap := func(a0, a1, b0, b1 int) {
		if a0 >= a1 || b0 >= b1 {
			return
		}
		for j, o := range lcsPairs(ra[a0:a1], rb[b0:b1]) {
			if o >= 0 {
				oldToNew[a0+o] = b0 + j
			}
		}
	}
	end := func(offs []int, k int, total int) int { // rune offset where token k starts, or total
		if k < len(offs) {
			return offs[k]
		}
		return total
	}
	prevA, prevB := 0, 0 // first rune after the last aligned token, old and new
	for j, i := range tp {
		if i < 0 {
			continue
		}
		charGap(prevA, oa[i], prevB, ob[j])
		for k := 0; k < len([]rune(ta[i])); k++ {
			oldToNew[oa[i]+k] = ob[j] + k
		}
		prevA, prevB = end(oa, i+1, len(ra)), end(ob, j+1, len(rb))
	}
	charGap(prevA, len(ra), prevB, len(rb))

	cm := CharMap{OldLen: len(ra), NewLen: len(rb)}
	for i := 0; i < len(ra); {
		if oldToNew[i] < 0 {
			i++
			continue
		}
		s, n := i, oldToNew[i]
		for i < len(ra) && oldToNew[i] == n+(i-s) {
			i++
		}
		cm.Keep = append(cm.Keep, [3]int{s, n, i - s})
	}
	return cm
}

// CarryLineIdentity carries prev — the ids of oldSource — across an edit to newSource. A paired line keeps
// its id; an inserted line gets newID(); an unpaired old id is retired (it simply is not in the result).
// The result records a History step (opt.FromRender → opt.ToRender) with the character map of every line
// paired but modified. A Rederived flag on prev is KEPT: only stage 2 clears it (ClearRederived), once it
// has acted on it — a later save must not silently erase "these marks need checking".
//
// If prev does not describe oldSource, nothing can be carried: the result is fresh ids for newSource,
// flagged RederiveMismatch.
func CarryLineIdentity(prev LineIdentity, oldSource, newSource string, newID func() string, opt CarryOptions) LineIdentity {
	if !prev.Describes(oldSource) {
		li := NewLineIdentity(newSource, newID)
		li.Rederived = &Rederivation{Reason: RederiveMismatch, Render: opt.ToRender}
		return li
	}
	oldLines, newLines := chartLines(oldSource), chartLines(newSource)
	var pairs []int
	if opt.ByIndex && len(oldLines) == len(newLines) {
		pairs = make([]int, len(newLines))
		for i := range pairs {
			pairs[i] = i
		}
	} else {
		pairs = LinePairs(oldLines, newLines)
	}
	next := LineIdentity{SourceHash: SourceHash(newSource), IDs: make([]string, len(newLines)), Rederived: prev.Rederived}
	step := LineStep{FromRender: opt.FromRender, ToRender: opt.ToRender}
	for j, o := range pairs {
		if o < 0 {
			next.IDs[j] = newID()
			continue
		}
		id := prev.IDs[o]
		next.IDs[j] = id
		if oldLines[o] != newLines[j] {
			if step.Maps == nil {
				step.Maps = map[string]CharMap{}
			}
			step.Maps[id] = BuildCharMap(oldLines[o], newLines[j])
		}
	}
	next.History = capHistory(append(append([]LineStep(nil), prev.History...), step), rederivedRender(next.Rederived))
	return next
}

// RederiveLineIdentity is the defined behaviour of a source write that bypasses SaveChartSource (a band
// import over an existing file, a migration, a seed): the ids are carried by the same diff when prev
// describes oldSource, and the result is flagged with reason so stage 2 marks the file's linked marks "to
// check" — unless the source did not change at all, in which case prev is returned untouched. With no
// usable prev (nil, or ids for another source), fresh ids, flagged.
func RederiveLineIdentity(prev *LineIdentity, oldSource, newSource string, newID func() string, reason string, opt CarryOptions) LineIdentity {
	if prev != nil && prev.Describes(oldSource) {
		if oldSource == newSource {
			return *prev
		}
		li := CarryLineIdentity(*prev, oldSource, newSource, newID, opt)
		li.Rederived = &Rederivation{Reason: reason, Render: opt.ToRender}
		return li
	}
	li := NewLineIdentity(newSource, newID)
	li.Rederived = &Rederivation{Reason: reason, Render: opt.ToRender}
	return li
}

// WithRerender records a render change that did not change the source (a renderer change re-rendered the
// stored PDF: cmd/rerender-charts) as an empty History step, so CharMapSince can still chain from a render
// before it. The ids are untouched. A no-op when from == to.
func (li LineIdentity) WithRerender(fromRender, toRender string) LineIdentity {
	if fromRender == toRender {
		return li
	}
	li.History = capHistory(append(append([]LineStep(nil), li.History...), LineStep{FromRender: fromRender, ToRender: toRender}), rederivedRender(li.Rederived))
	return li
}

// ClearRederived returns li without its Rederived flag — stage 2 calls it once it has marked the file's
// marks "to check".
func (li LineIdentity) ClearRederived() LineIdentity {
	li.Rederived = nil
	return li
}

// CharMapSince composes the character maps of line id from the render fromRender (the render a mark was
// drawn on, its PointsRenderHash) up to currentRender. ok is false when the history does not reach back to
// fromRender as one unbroken chain ending at currentRender (too old, or a render change no step recorded):
// stage 2 must then treat a sideways shift as unknown ("to check"), not guess.
func (li LineIdentity) CharMapSince(fromRender, currentRender, id string) (CharPath, bool) {
	if fromRender == currentRender {
		return nil, true
	}
	var rev []CharMap
	want := currentRender
	for k := len(li.History) - 1; k >= 0; k-- {
		st := li.History[k]
		if st.ToRender != want {
			return nil, false
		}
		if m, ok := st.Maps[id]; ok {
			rev = append(rev, m)
		}
		if st.FromRender == fromRender {
			path := make(CharPath, len(rev))
			for i := range rev {
				path[i] = rev[len(rev)-1-i]
			}
			return path, true
		}
		want = st.FromRender
	}
	return nil, false
}

// LineLocation is where a line is now: what stage 2 needs to move a mark linked to it.
type LineLocation struct {
	ID    string
	Index int    // current source line index
	Text  string // current source text of the line
	// Runs are the manifest runs drawn for the line, in draw order (SrcSeg 0, 1, …): page and box each.
	// Empty for a line that draws nothing (a blank line, a directive, a {np} marker).
	Runs []Anchor
	// CharMap maps the line's rune offsets from the render the mark was drawn on to now; CharMapKnown is
	// false when the history could not say (see CharMapSince).
	CharMap      CharPath
	CharMapKnown bool
}

// LocateLine resolves line id against the current source, its identity and its manifest (a fresh
// RenderWithAnchors of source). fromRender/currentRender scope the character map (CharMapSince). ok is
// false when the id is not a line of the current source: the line was deleted (P20: the mark is lost).
func LocateLine(li LineIdentity, source string, anchors []Anchor, id, fromRender, currentRender string) (LineLocation, bool) {
	idx, ok := li.Index(id)
	if !ok || !li.Describes(source) {
		return LineLocation{}, false
	}
	loc := LineLocation{ID: id, Index: idx, Text: chartLines(source)[idx]}
	for _, a := range anchors {
		if a.SrcLine == idx {
			loc.Runs = append(loc.Runs, a)
		}
	}
	sort.SliceStable(loc.Runs, func(i, j int) bool { return loc.Runs[i].SrcSeg < loc.Runs[j].SrcSeg })
	loc.CharMap, loc.CharMapKnown = li.CharMapSince(fromRender, currentRender, id)
	return loc, true
}

// capHistory enforces maxLineHistory by folding the oldest steps into one cumulative step (see above).
//
// The render a T194 re-derivation was recorded on (rederived) is never folded INTO a run: it stays a step
// boundary (T195 re-review H1), so predatesRederive can still tell which links came before it.
func capHistory(h []LineStep, rederived string) []LineStep {
	if len(h) <= maxLineHistory {
		return h
	}
	n := len(h) - maxLineHistory + 1 // fold this many oldest steps into one
	if !chained(h[:n]) {
		return h[n-1:] // a broken chain cannot be composed: those links had no map already
	}
	for j := 1; j < n; j++ {
		if rederived != "" && h[j].FromRender == rederived { // fold either side of the boundary
			return append([]LineStep{composeSteps(h[:j]), composeSteps(h[j:n])}, h[n:]...)
		}
	}
	return append([]LineStep{composeSteps(h[:n])}, h[n:]...)
}

func rederivedRender(r *Rederivation) string {
	if r == nil {
		return ""
	}
	return r.Render
}

// chained reports whether consecutive steps join (each step starts at the render the previous one ended).
func chained(h []LineStep) bool {
	for i := 1; i < len(h); i++ {
		if h[i].FromRender != h[i-1].ToRender {
			return false
		}
	}
	return true
}

// Compact returns li with its history reduced to what the links on its file still need: keep holds every
// render some link (live or tombstoned) references. The newest recentLineHistory steps stay as recorded;
// older steps are folded into one cumulative step per kept render (from it to the next kept render, or to
// the start of the recent window), and steps reachable from no kept render are dropped. A no-op (same
// value, false) when there is nothing to fold.
func (li LineIdentity) Compact(keep map[string]bool) (LineIdentity, bool) {
	n := len(li.History)
	if n <= recentLineHistory {
		return li, false
	}
	old, rest := li.History[:n-recentLineHistory], li.History[n-recentLineHistory:]
	if li.Rederived != nil && li.Rederived.Render != "" { // H1: the re-derivation stays a boundary
		k := make(map[string]bool, len(keep)+1)
		for r := range keep {
			k[r] = true
		}
		k[li.Rederived.Render] = true
		keep = k
	}
	var out, seg []LineStep
	flush := func() {
		if len(seg) > 0 {
			out = append(out, composeSteps(seg))
			seg = nil
		}
	}
	for i, st := range old {
		if i > 0 && st.FromRender != old[i-1].ToRender {
			flush() // a broken chain: nothing before it can be followed past it
		}
		switch {
		case keep[st.FromRender]:
			flush()
			seg = []LineStep{st}
		case len(seg) > 0:
			seg = append(seg, st)
		}
	}
	flush()
	if len(out) == len(old) {
		return li, false
	}
	li.History = append(out, rest...)
	return li, true
}

// composeSteps folds consecutive, chained steps into one: from the first's render to the last's, each line
// id's character maps composed in order (a line modified in several steps gets one map).
func composeSteps(steps []LineStep) LineStep {
	out := LineStep{FromRender: steps[0].FromRender, ToRender: steps[len(steps)-1].ToRender}
	for _, st := range steps {
		for id, m := range st.Maps {
			if out.Maps == nil {
				out.Maps = map[string]CharMap{}
			}
			if acc, ok := out.Maps[id]; ok {
				out.Maps[id] = composeCharMap(acc, m)
			} else {
				out.Maps[id] = m
			}
		}
	}
	return out
}

// composeCharMap is b∘a, EXACTLY as applying a then b (CharPath{a, b}.Map) for every offset: the same exact
// flag and the same landing position (T195 re-review M1). Characters kept by both maps form the Keep runs;
// a character that did not survive lands wherever the step-by-step mapping puts it, recorded in Lands
// when that differs from what Keep alone would give.
func composeCharMap(a, b CharMap) CharMap {
	type land struct {
		pos   int
		exact bool
	}
	step := make([]land, a.OldLen)
	for o := range step {
		pos, exact := CharPath{a, b}.Map(o)
		step[o] = land{pos, exact}
	}
	cm := CharMap{OldLen: a.OldLen, NewLen: b.NewLen}
	for i := 0; i < len(step); {
		if !step[i].exact {
			i++
			continue
		}
		st, n := i, step[i].pos
		for i < len(step) && step[i].exact && step[i].pos == n+(i-st) {
			i++
		}
		cm.Keep = append(cm.Keep, [3]int{st, n, i - st})
	}
	for o, ld := range step {
		if ld.exact {
			continue
		}
		if def, _ := cm.Map(o); def != ld.pos {
			cm.Lands = append(cm.Lands, [2]int{o, ld.pos})
		}
	}
	return cm
}
