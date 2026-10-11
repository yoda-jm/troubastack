package chartpdf

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// T194 — line identity (anchoring stage 1): the diff that carries line ids across a source edit, the
// character map of an edited line, and the lookup stage 2 will call. All synthetic text.

// idGen is a deterministic id generator for tests: n1, n2, … .
func idGen() func() string {
	n := 0
	return func() string { n++; return fmt.Sprintf("n%d", n) }
}

const lineFixture = `# The Open Road
The Wayfarers

## Verse 1
G       C        D
We drive the road until the evening
Em      C
Oh the opn road is calling
Under the long grey sky

## Chorus
Keep on rolling, keep on rolling
`

// carrier is the identity function under test, so the teeth tests can swap in a naive one.
type carrier func(prev LineIdentity, oldSrc, newSrc string, newID func() string) LineIdentity

func realCarry(prev LineIdentity, oldSrc, newSrc string, newID func() string) LineIdentity {
	return CarryLineIdentity(prev, oldSrc, newSrc, newID, CarryOptions{})
}

// naiveByIndex is the identity a careless implementation would have: line i keeps id i, extra lines get
// fresh ids. It is the "ids reassigned by index" the teeth tests must catch.
func naiveByIndex(prev LineIdentity, oldSrc, newSrc string, newID func() string) LineIdentity {
	n := len(chartLines(newSrc))
	li := LineIdentity{SourceHash: SourceHash(newSrc), IDs: make([]string, n)}
	for i := range li.IDs {
		if i < len(prev.IDs) {
			li.IDs[i] = prev.IDs[i]
		} else {
			li.IDs[i] = newID()
		}
	}
	return li
}

// naiveByText is the other wrong identity: the id of the k-th line with this exact text (today's
// text+occurrence, applied to lines).
func naiveByText(prev LineIdentity, oldSrc, newSrc string, newID func() string) LineIdentity {
	type key struct {
		text string
		occ  int
	}
	oldIDs := map[key]string{}
	seen := map[string]int{}
	for i, l := range chartLines(oldSrc) {
		seen[l]++
		oldIDs[key{l, seen[l]}] = prev.IDs[i]
	}
	seen = map[string]int{}
	lines := chartLines(newSrc)
	li := LineIdentity{SourceHash: SourceHash(newSrc), IDs: make([]string, len(lines))}
	for i, l := range lines {
		seen[l]++
		if id, ok := oldIDs[key{l, seen[l]}]; ok {
			li.IDs[i] = id
		} else {
			li.IDs[i] = newID()
		}
	}
	return li
}

func findLine(src, text string) int {
	for i, l := range chartLines(src) {
		if l == text {
			return i
		}
	}
	return -1
}

func lineIndex(t *testing.T, src, text string) int {
	t.Helper()
	i := findLine(src, text)
	if i < 0 {
		t.Fatalf("fixture: no line %q", text)
	}
	return i
}

func insertLine(src string, at int, text string) string {
	lines := chartLines(src)
	out := append(append(append([]string{}, lines[:at]...), text), lines[at:]...)
	return strings.Join(out, "\n")
}

func deleteLine(src string, at int) string {
	lines := chartLines(src)
	out := append(append([]string{}, lines[:at]...), lines[at+1:]...)
	return strings.Join(out, "\n")
}

func replaceLine(src string, at int, text string) string {
	lines := chartLines(src)
	lines[at] = text
	return strings.Join(lines, "\n")
}

// checkInsertAbove: inserting a line above keeps every old id, in order, and the new line has a new id.
func checkInsertAbove(c carrier) error {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	at := 5 // above "We drive the road…"
	next := c(prev, lineFixture, insertLine(lineFixture, at, "A brand new line of words"), gen)
	want := append(append(append([]string{}, prev.IDs[:at]...), "?"), prev.IDs[at:]...)
	for i, id := range next.IDs {
		if i == at {
			for _, old := range prev.IDs {
				if id == old {
					return fmt.Errorf("inserted line got an OLD id %s", id)
				}
			}
			continue
		}
		if id != want[i] {
			return fmt.Errorf("line %d: id %s, want %s (old ids must follow their lines)", i, id, want[i])
		}
	}
	return nil
}

// checkDuplicateBefore: a copy of line X inserted before it, adjacent or far, gets the new id; the
// original keeps its own (P14).
func checkDuplicateBefore(c carrier, adjacent bool) error {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	orig := "Under the long grey sky"
	oi := findLine(lineFixture, orig)
	at := oi // directly above
	if !adjacent {
		at = 5 // well above, before the first lyric
	}
	next := c(prev, lineFixture, insertLine(lineFixture, at, orig), gen)
	if got := next.IDs[oi+1]; got != prev.IDs[oi] {
		return fmt.Errorf("the original (now line %d) has id %s, want its own %s", oi+1, got, prev.IDs[oi])
	}
	for _, old := range prev.IDs {
		if next.IDs[at] == old {
			return fmt.Errorf("the copy (line %d) took an old id %s", at, old)
		}
	}
	return nil
}

func TestLineID_InsertAboveKeepsEveryID(t *testing.T) {
	if err := checkInsertAbove(realCarry); err != nil {
		t.Fatal(err)
	}
}

// Teeth for the key diff tests: the same checks against the two naive identities must go red.
func TestLineID_TeethNaiveIdentitiesFail(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    carrier
	}{{"by-index", naiveByIndex}, {"by-text+occurrence", naiveByText}} {
		errs := []error{checkInsertAbove(tc.c), checkDuplicateBefore(tc.c, true), checkDuplicateBefore(tc.c, false)}
		red := 0
		for _, e := range errs {
			if e != nil {
				red++
				t.Logf("teeth %s: %v", tc.name, e)
			}
		}
		if red == 0 {
			t.Errorf("%s identity passed every check: the checks have no teeth", tc.name)
		}
	}
	// by-index fails insert-above; by-text fails the duplicate cases. Pin which, so a weakened check shows.
	if checkInsertAbove(naiveByIndex) == nil {
		t.Error("insert-above did not catch the by-index identity")
	}
	if checkDuplicateBefore(naiveByText, false) == nil || checkDuplicateBefore(naiveByText, true) == nil {
		t.Error("duplicate-before did not catch the text+occurrence identity")
	}
}

func TestLineID_TypoKeepsIDWithShiftingCharMap(t *testing.T) {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	i := lineIndex(t, lineFixture, "Oh the opn road is calling")
	newSrc := replaceLine(lineFixture, i, "Oh the open road is calling")
	next := CarryLineIdentity(prev, lineFixture, newSrc, gen, CarryOptions{FromRender: "r1", ToRender: "r2"})
	if next.IDs[i] != prev.IDs[i] {
		t.Fatalf("typo fix changed the line's id: %s → %s", prev.IDs[i], next.IDs[i])
	}
	for k := range prev.IDs {
		if next.IDs[k] != prev.IDs[k] {
			t.Fatalf("line %d id changed by an edit elsewhere", k)
		}
	}
	cm, ok := next.History[len(next.History)-1].Maps[prev.IDs[i]]
	if !ok {
		t.Fatal("no char map recorded for the modified line")
	}
	// Before the edit: unchanged. "road" at old 11 → new 12, "calling" old 19 → new 20. All exact.
	for _, c := range []struct{ old, new int }{{0, 0}, {7, 7}, {11, 12}, {19, 20}, {25, 26}} {
		if got, exact := cm.Map(c.old); got != c.new || !exact {
			t.Errorf("Map(%d) = %d,%v; want %d,exact", c.old, got, exact, c.new)
		}
	}
	// Unchanged lines carry no map.
	if len(next.History[len(next.History)-1].Maps) != 1 {
		t.Errorf("maps for %d lines, want 1 (only the edited line)", len(next.History[0].Maps))
	}
}

func TestLineID_WordInsertedBeforeCircledWordMovesIt(t *testing.T) {
	oldL := "We drive the road until the evening"
	newL := "We drive the long road until the evening"
	cm := BuildCharMap(oldL, newL)
	circ := strings.Index(oldL, "road") // the circled word's left-edge character
	got, exact := cm.Map(circ)
	if want := strings.Index(newL, "road"); got != want || !exact {
		t.Fatalf("circled word at %d maps to %d (exact %v), want %d", circ, got, exact, want)
	}
	// Every character of the word follows, and so does what is after it.
	for k := 0; k < len(oldL)-circ; k++ {
		if g, _ := cm.Map(circ + k); g != circ+5+k {
			t.Fatalf("char %d maps to %d, want %d", circ+k, g, circ+5+k)
		}
	}
	// A word before the insertion does not move.
	if g, e := cm.Map(3); g != 3 || !e {
		t.Errorf("'drive' moved: %d", g)
	}
	// The same through a whole save, and a word inserted AFTER it does not move it.
	cm2 := BuildCharMap("the road", "the road ahead")
	if g, e := cm2.Map(4); g != 4 || !e {
		t.Errorf("a word appended after moved 'road' to %d", g)
	}
	// Teeth: a plain character LCS (no word pass) aligns "road"'s r with "red"'s r — the reason for words first.
	cm3 := BuildCharMap("the road", "the red road")
	if g, _ := cm3.Map(4); g != 8 {
		t.Errorf("'road' after inserting 'red ' maps to %d, want 8", g)
	}
}

func TestLineID_DuplicateInsertedBeforeKeepsOriginal(t *testing.T) {
	for _, adj := range []bool{true, false} {
		if err := checkDuplicateBefore(realCarry, adj); err != nil {
			t.Errorf("adjacent=%v: %v", adj, err)
		}
	}
}

func TestLineID_DeleteRetiresID(t *testing.T) {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	i := lineIndex(t, lineFixture, "Under the long grey sky")
	next := CarryLineIdentity(prev, lineFixture, deleteLine(lineFixture, i), gen, CarryOptions{})
	if _, ok := next.Index(prev.IDs[i]); ok {
		t.Fatal("deleted line's id is still present")
	}
	want := append(append([]string{}, prev.IDs[:i]...), prev.IDs[i+1:]...)
	if !reflect.DeepEqual(next.IDs, want) {
		t.Fatalf("ids after delete %v, want %v", next.IDs, want)
	}
	// A retired id is never handed out again.
	again := CarryLineIdentity(next, deleteLine(lineFixture, i), lineFixture, gen, CarryOptions{})
	if again.IDs[i] == prev.IDs[i] {
		t.Fatal("re-inserting the same text resurrected the retired id")
	}
}

// Two identical adjacent lines, one deleted: the LATER twin keeps its id and the first is retired. It is
// the inverse of "a copy inserted above gets the new id", so insert-then-delete restores every id.
func TestLineID_IdenticalAdjacentTwinsDeleteOne(t *testing.T) {
	src := "# The Open Road\n\nKeep on rolling\nKeep on rolling\nHome at last\n"
	gen := idGen()
	prev := NewLineIdentity(src, gen)
	next := CarryLineIdentity(prev, src, deleteLine(src, 2), gen, CarryOptions{})
	if next.IDs[2] != prev.IDs[3] {
		t.Fatalf("surviving twin has id %s, want the later twin's %s", next.IDs[2], prev.IDs[3])
	}
	if _, ok := next.Index(prev.IDs[2]); ok {
		t.Fatal("the first twin's id should be retired")
	}
	// Round trip: insert a copy above, then delete it — every id as before.
	base := "# The Open Road\n\nKeep on rolling\nHome at last\n"
	b := NewLineIdentity(base, gen)
	ins := CarryLineIdentity(b, base, insertLine(base, 2, "Keep on rolling"), gen, CarryOptions{})
	back := CarryLineIdentity(ins, insertLine(base, 2, "Keep on rolling"), base, gen, CarryOptions{})
	if !reflect.DeepEqual(back.IDs, b.IDs) {
		t.Fatalf("insert-then-delete of a copy changed ids: %v → %v", b.IDs, back.IDs)
	}
}

// A line rewritten beyond recognition is a NEW line: the old id is retired (a mark on it becomes lost,
// P20) and the new text gets a fresh id. A line edited but still recognisable is paired. The boundary is
// modifiedPairMin; both sides are pinned, with the similarity printed.
func TestLineID_RewriteBeyondRecognitionGetsNewID(t *testing.T) {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	i := lineIndex(t, lineFixture, "Under the long grey sky")
	for _, c := range []struct {
		text   string
		paired bool
	}{
		{"Under the long blue sky", true},             // a word changed
		{"Under the long grey sky tonight", true},     // a word added
		{"Nothing ever stays the same for us", false}, // rewritten
		{"", false}, // blanked
	} {
		s := similarity("Under the long grey sky", c.text)
		next := CarryLineIdentity(prev, lineFixture, replaceLine(lineFixture, i, c.text), gen, CarryOptions{})
		if got := next.IDs[i] == prev.IDs[i]; got != c.paired {
			t.Errorf("%q (similarity %.2f): paired=%v, want %v", c.text, s, got, c.paired)
		}
		for k := range prev.IDs {
			if k != i && next.IDs[k] != prev.IDs[k] {
				t.Errorf("%q: line %d lost its id", c.text, k)
			}
		}
	}
}

// One hunk that both fixes a typo AND inserts a line: the typo'd line is still paired (the modified pairing
// is an alignment inside the hunk, not "equal-sized hunks only").
func TestLineID_TypoAndInsertInOneHunk(t *testing.T) {
	gen := idGen()
	prev := NewLineIdentity(lineFixture, gen)
	i := lineIndex(t, lineFixture, "Oh the opn road is calling")
	newSrc := insertLine(replaceLine(lineFixture, i, "Oh the open road is calling"), i, "Here comes the morning light")
	next := CarryLineIdentity(prev, lineFixture, newSrc, gen, CarryOptions{})
	if next.IDs[i+1] != prev.IDs[i] {
		t.Fatalf("typo'd line (now %d) has id %s, want %s", i+1, next.IDs[i+1], prev.IDs[i])
	}
	if next.IDs[i] == prev.IDs[i] || next.IDs[i] == prev.IDs[i-1] {
		t.Fatal("inserted line took an old id")
	}
}

// A transposition rewrites every chord row past the similarity threshold; ByIndex keeps them.
func TestLineID_TranspositionByIndexKeepsChordRows(t *testing.T) {
	// Dense chord rows: once transposed they share almost nothing but blanks with the original.
	src := "# The Open Road\n\nG C D\nWe drive the road\nEm C G D\nOh the road is calling\n"
	tr, err := TransposeSemitones(src, 2)
	if err != nil {
		t.Fatal(err)
	}
	gen := idGen()
	prev := NewLineIdentity(src, gen)
	ci := lineIndex(t, src, "G C D")
	if s := similarity(chartLines(src)[ci], chartLines(tr)[ci]); s >= modifiedPairMin {
		t.Fatalf("fixture: transposed chord row similarity %.2f is above the threshold; ByIndex would be untested", s)
	}
	byIdx := CarryLineIdentity(prev, src, tr, gen, CarryOptions{ByIndex: true})
	if !reflect.DeepEqual(byIdx.IDs, prev.IDs) {
		t.Fatalf("transposition by index changed ids: %v → %v", prev.IDs, byIdx.IDs)
	}
	// Teeth: by diff, the transposed chord row is a rewrite and loses its id.
	byDiff := CarryLineIdentity(prev, src, tr, gen, CarryOptions{})
	if byDiff.IDs[ci] == prev.IDs[ci] {
		t.Fatal("expected the plain diff to lose the transposed chord row's id (else ByIndex is untested)")
	}
	// ByIndex with a different line count falls back to the diff.
	more := CarryLineIdentity(prev, src, insertLine(tr, 2, "A new line"), gen, CarryOptions{ByIndex: true})
	if more.IDs[4] != prev.IDs[3] {
		t.Fatalf("ByIndex with an inserted line: lyric id %s, want %s (diff fallback)", more.IDs[4], prev.IDs[3])
	}
}

func TestLineID_PrevForAnotherSourceIsRederived(t *testing.T) {
	gen := idGen()
	stale := NewLineIdentity("# Other\nline\n", gen)
	next := CarryLineIdentity(stale, lineFixture, lineFixture+"x\n", gen, CarryOptions{ToRender: "r9"})
	if next.Rederived == nil || next.Rederived.Reason != RederiveMismatch || next.Rederived.Render != "r9" {
		t.Fatalf("Rederived = %+v, want mismatch at r9", next.Rederived)
	}
	for _, id := range next.IDs {
		for _, o := range stale.IDs {
			if id == o {
				t.Fatal("a stale id was reused")
			}
		}
	}
	if !next.Describes(lineFixture + "x\n") {
		t.Fatal("re-derived ids do not describe the new source")
	}
}

func TestLineID_RederivedFlagSurvivesLaterSaves(t *testing.T) {
	gen := idGen()
	li := RederiveLineIdentity(nil, "", lineFixture, gen, RederiveImport, CarryOptions{ToRender: "r1"})
	next := CarryLineIdentity(li, lineFixture, lineFixture+"more\n", gen, CarryOptions{FromRender: "r1", ToRender: "r2"})
	if next.Rederived == nil || next.Rederived.Reason != RederiveImport {
		t.Fatalf("a save erased the pending Rederived flag: %+v", next.Rederived)
	}
	if next.ClearRederived().Rederived != nil {
		t.Fatal("ClearRederived did not clear")
	}
	// Unchanged source through a bypass write: nothing affected, prev returned as is (no flag added).
	same := RederiveLineIdentity(&next, lineFixture+"more\n", lineFixture+"more\n", gen, RederiveImportChanged, CarryOptions{})
	if !reflect.DeepEqual(same, next) {
		t.Fatal("an unchanged bypass write altered the identity")
	}
	// Changed source through a bypass write: ids carried by diff, flagged.
	prev := NewLineIdentity(lineFixture, gen)
	ch := RederiveLineIdentity(&prev, lineFixture, insertLine(lineFixture, 5, "new words here"), gen, RederiveImportChanged, CarryOptions{})
	if ch.Rederived == nil || ch.Rederived.Reason != RederiveImportChanged || ch.IDs[6] != prev.IDs[5] {
		t.Fatalf("bypass write: %+v ids %v", ch.Rederived, ch.IDs)
	}
}

func TestLineID_CharMapSinceComposesAndRefusesGaps(t *testing.T) {
	gen := idGen()
	s0 := "# The Open Road\n\nthe road home\n"
	s1 := "# The Open Road\n\nthe long road home\n"
	s2 := "# The Open Road\n\nwe take the long road home\n"
	li := NewLineIdentity(s0, gen)
	id := li.IDs[2]
	li = CarryLineIdentity(li, s0, s1, gen, CarryOptions{FromRender: "r0", ToRender: "r1"})
	li = CarryLineIdentity(li, s1, s2, gen, CarryOptions{FromRender: "r1", ToRender: "r2"})
	if li.IDs[2] != id {
		t.Fatal("line lost its id across two edits")
	}
	road0 := strings.Index("the road home", "road")
	road2 := strings.Index("we take the long road home", "road")
	p, ok := li.CharMapSince("r0", "r2", id)
	if !ok {
		t.Fatal("history r0→r2 should be known")
	}
	if g, e := p.Map(road0); g != road2 || !e {
		t.Fatalf("r0→r2: 'road' %d → %d (exact %v), want %d", road0, g, e, road2)
	}
	if p, ok := li.CharMapSince("r2", "r2", id); !ok || len(p) != 0 {
		t.Fatal("same render: identity path expected")
	}
	if _, ok := li.CharMapSince("r-unknown", "r2", id); ok {
		t.Fatal("an unknown render must not be reachable")
	}
	if _, ok := li.CharMapSince("r0", "r3", id); ok {
		t.Fatal("a current render no step reaches must be refused (gap)")
	}
	// A renderer change recorded with WithRerender keeps the chain.
	li2 := li.WithRerender("r2", "r3")
	if p, ok := li2.CharMapSince("r0", "r3", id); !ok {
		t.Fatal("WithRerender broke the chain")
	} else if g, _ := p.Map(road0); g != road2 {
		t.Fatalf("after rerender 'road' → %d, want %d", g, road2)
	}
	if !reflect.DeepEqual(li2.IDs, li.IDs) {
		t.Fatal("WithRerender changed ids")
	}
}

func TestLineID_HistoryIsBounded(t *testing.T) {
	gen := idGen()
	src := "# The Open Road\n\nline\n"
	li := NewLineIdentity(src, gen)
	for k := 0; k < maxLineHistory+10; k++ {
		ns := src + strings.Repeat("x", k+1) + "\n"
		li = CarryLineIdentity(li, src, ns, gen, CarryOptions{FromRender: fmt.Sprint(k), ToRender: fmt.Sprint(k + 1)})
		src = ns
	}
	// T195 re-review: the cap folds the oldest steps into one, so the chain still starts at render "0".
	if len(li.History) != maxLineHistory || li.History[0].FromRender != "0" {
		t.Fatalf("history has %d steps from %q, want %d from \"0\"", len(li.History), li.History[0].FromRender, maxLineHistory)
	}
}

func TestCharMap_DeletedCharIsInexact(t *testing.T) {
	cm := BuildCharMap("the grey road", "the road")
	if _, e := cm.Map(5); e { // inside "grey"
		t.Fatal("a deleted character must map inexactly")
	}
	if g, e := cm.Map(9); g != 4 || !e { // "road"
		t.Fatalf("'road' → %d,%v want 4,exact", g, e)
	}
	if g, e := cm.Map(13); g != 8 || !e { // end of line
		t.Fatalf("end → %d,%v", g, e)
	}
}

// LocateLine: the stage-2 lookup returns the line's current index, text, runs and char map — and nothing
// for a deleted line.
func TestLocateLine(t *testing.T) {
	gen := idGen()
	li := NewLineIdentity(lineFixture, gen)
	i := lineIndex(t, lineFixture, "Under the long grey sky")
	id := li.IDs[i]
	newSrc := insertLine(lineFixture, 3, "A new line above")
	li = CarryLineIdentity(li, lineFixture, newSrc, gen, CarryOptions{FromRender: "r1", ToRender: "r2"})
	_, anchors, err := RenderWithAnchors(newSrc)
	if err != nil {
		t.Fatal(err)
	}
	loc, ok := LocateLine(li, newSrc, anchors, id, "r1", "r2")
	if !ok || loc.Index != i+1 || loc.Text != "Under the long grey sky" || !loc.CharMapKnown {
		t.Fatalf("loc = %+v ok=%v", loc, ok)
	}
	if len(loc.Runs) != 1 || loc.Runs[0].Text != "Under the long grey sky" || loc.Runs[0].Page != 0 {
		t.Fatalf("runs = %+v", loc.Runs)
	}
	// Deleted → not found.
	gone := deleteLine(newSrc, i+1)
	li2 := CarryLineIdentity(li, newSrc, gone, gen, CarryOptions{FromRender: "r2", ToRender: "r3"})
	_, anchors2, _ := RenderWithAnchors(gone)
	if _, ok := LocateLine(li2, gone, anchors2, id, "r1", "r3"); ok {
		t.Fatal("a deleted line must not be located (P20: lost)")
	}
	// Ids for another source are refused rather than misread.
	if _, ok := LocateLine(li, lineFixture, anchors, id, "r1", "r2"); ok {
		t.Fatal("ids that do not describe the source must not resolve")
	}
}
