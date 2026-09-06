// Package runningorder states, once for TroubaCore, the T158 running-order NUMBERING rule.
//
// THE RULE: a number belongs only to a MAIN-ORDER SONG — a song that is NOT on-call and NOT an
// intermission. An on-call (bench) song or an intermission carries no number and NEVER shifts the number of
// the entry after it. So "7" means the 7th main song on the printed export sheet, in the Stage drawer, and
// in the Studio editor alike.
//
// The rule cannot be SHARED as code across Go/Kotlin/TypeScript, so instead all three surfaces run the SAME
// vectors — docs/contracts/running-order-numbering.vectors.json — as a test (see runningorder_test.go),
// which is what keeps them from silently diverging (T158).
package runningorder

// Entry is one row of a setlist's running order, reduced to exactly what the numbering rule needs: its kind
// and (for a song) whether it is on the on-call bench.
type Entry struct {
	Kind   string `json:"kind"`   // "song" | "intermission"
	OnCall bool   `json:"onCall"` // applies only to a song
}

// KindSong / KindIntermission are the entry kinds. The intermission kind is forward-looking (T153): the
// numbering must already be correct when intermission instances exist.
const (
	KindSong         = "song"
	KindIntermission = "intermission"
)

// Numbers returns a 1-based running-order number per entry, or 0 for an entry that carries none (an on-call
// song or an intermission). The display number is DERIVED, never persisted — deriving it is what lets the
// three surfaces agree.
func Numbers(entries []Entry) []int {
	out := make([]int, len(entries))
	n := 0
	for i, e := range entries {
		if e.Kind == KindSong && !e.OnCall {
			n++
			out[i] = n
		}
	}
	return out
}
