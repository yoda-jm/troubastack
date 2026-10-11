package chartpdf

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// T195 re-review (r4), kept as permanent tests: the line-id history's compaction is a READ-side operation,
// so it must never change what a projection gives — not a position, not a state.

// M1: a composed step maps every offset EXACTLY as its steps applied one by one (exact flag and landing
// position), on random edits of one line, over many seeds (the reviewer found 368 of 117,708 offsets off
// at seed 7 before composeCharMap recorded the landings).
func TestHistory_ComposeEqualsStepwise(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		trials := 600
		if seed == 7 {
			trials = 3000 // the reviewer's vector, in full
		}
		composeEqualsStepwise(t, seed, trials)
	}
}

func composeEqualsStepwise(t *testing.T, seed int64, trials int) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	words := []string{"the", "tide", "rolls", "in", "across", "stones", "a", "lantern", "swings", "quay"}
	exactMismatch, posMismatch, total := 0, 0, 0
	var example string
	for trial := 0; trial < trials; trial++ {
		line := strings.Join(words[:6+rng.Intn(4)], " ")
		var steps []LineStep
		cur := line
		for s := 0; s < 2+rng.Intn(3); s++ {
			r := []rune(cur)
			switch rng.Intn(3) {
			case 0: // delete a span
				i := rng.Intn(len(r))
				j := i + 1 + rng.Intn(3)
				if j > len(r) {
					j = len(r)
				}
				r = append(append([]rune{}, r[:i]...), r[j:]...)
			case 1: // insert
				i := rng.Intn(len(r) + 1)
				r = append(append(append([]rune{}, r[:i]...), []rune("XY")...), r[i:]...)
			case 2: // replace one
				i := rng.Intn(len(r))
				r[i] = 'Z'
			}
			nxt := string(r)
			steps = append(steps, LineStep{FromRender: fmt.Sprint(s), ToRender: fmt.Sprint(s + 1), Maps: map[string]CharMap{"L": BuildCharMap(cur, nxt)}})
			cur = nxt
		}
		var path CharPath
		for _, st := range steps {
			path = append(path, st.Maps["L"])
		}
		comp := composeSteps(steps).Maps["L"]
		for off := 0; off <= len([]rune(line)); off++ {
			total++
			p1, e1 := path.Map(off)
			p2, e2 := comp.Map(off)
			if e1 != e2 {
				exactMismatch++
			} else if p1 != p2 {
				posMismatch++
				if example == "" {
					example = fmt.Sprintf("line %q → %q, off %d: stepwise %d, composed %d (exact=%v)", line, cur, off, p1, p2, e1)
				}
			}
		}
	}
	if exactMismatch > 0 || posMismatch > 0 {
		t.Errorf("seed %d: of %d offsets, %d exact-flag and %d position mismatches between the composed map and step by step; e.g. %s",
			seed, total, exactMismatch, posMismatch, example)
	}
}

// H1: predatesRederive after Compact: the ids were re-derived (backfill) at R0; a mark is linked on R1, the
// next save; 40 saves later a serve compacts the history (keep = {R1}). The mark must not turn "check".
func TestHistory_CompactKeepsPostRederiveLinksLinked(t *testing.T) {
	src := "# T\n\n## Verse\nbeneath a paper moon\n\n## Notes\nnote 0\n"
	w := lw(t, src)
	w.li.Rederived = &Rederivation{Reason: RederiveImportChanged, Render: w.hash} // as a backfill/import leaves it
	w.setSrc(strings.Replace(w.src, "note 0", "note a", 1))
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "m", Type: domain.TypeIcon, Page: line.Page, Points: rectAround(line, 0, 1, 16, 18)}, w.chart())
	for i := 1; i <= 40; i++ {
		w.setSrc(strings.Replace(w.src, fmt.Sprintf("note %s", map[bool]string{true: "a", false: fmt.Sprint(i - 1)}[i == 1]), fmt.Sprintf("note %d", i), 1))
	}
	before := ProjectMark(o, w.chart()).Link.State
	c, changed := w.li.Compact(map[string]bool{o.Link.Render: true})
	w.li = c
	after := ProjectMark(o, w.chart()).Link.State
	if !changed {
		t.Fatal("fixture: nothing was compacted; the cell would prove nothing")
	}
	if before != domain.LinkLinked || before != after {
		t.Errorf("a GET's compaction changed a mark's state: %s -> %s", before, after)
	}
}

// M1: a read must not move a mark: project before and after Compact with NO edit in between.
func TestHistory_CompactDoesNotMoveMarks(t *testing.T) {
	src := "# T\n\n## Verse\nwe sail at dawn and dusk\n\n## Notes\nnote 0\n"
	w := lw(t, src)
	line := lineRun(t, w, "we sail at dawn and dusk")
	// left edge on 'd' of "dawn" (char 11)
	o := LinkMark(domain.Object{UUID: "m", Type: domain.TypeIcon, Page: line.Page, Points: rectAround(line, 0, 1, 11.2, 13)}, w.chart())
	w.setSrc(strings.Replace(w.src, "at dawn", "at awn", 1))   // delete the mark's character
	w.setSrc(strings.Replace(w.src, "at awn", "at XYZawn", 1)) // insert just where it was
	for i := 1; i <= 40; i++ {
		w.setSrc(strings.Replace(w.src, fmt.Sprintf("note %d", i-1), fmt.Sprintf("note %d", i), 1))
	}
	p1 := ProjectMark(o, w.chart())
	c, changed := w.li.Compact(map[string]bool{o.Link.Render: true})
	w.li = c
	p2 := ProjectMark(o, w.chart())
	if !changed {
		t.Fatal("fixture: nothing was compacted; the cell would prove nothing")
	}
	if p1.Points[0] != p2.Points[0] || p1.Link.State != p2.Link.State {
		t.Errorf("compaction (a read) moved the mark or changed its state: x %.5f (%s) → %.5f (%s)",
			p1.Points[0].X, p1.Link.State, p2.Points[0].X, p2.Link.State)
	}
}
