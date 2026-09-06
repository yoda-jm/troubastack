// Package annrecover holds the pure logic of the T159 recovery: matching an archived annotation stream to
// its live song, and computing what to restore. The 2026-09-05 re-seed churned band→song ids, so three
// archived streams never re-attached; the songs still exist (by title), so each orphaned stream has a
// target. This restores the lost OBJECTS exactly — it never anchors them (T159 ⚠: anchoring is T145's
// separate decision; anchoring here would bind a frozen-coordinate mark to today's reflowed render, the
// corruption BLOCKER 2 exists to prevent).
package annrecover

import (
	"fmt"
	"strings"

	"troubastack/core/internal/domain"
)

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// TargetKey is the stable identity a stream is matched on — band name + song title — NOT ids, which are
// exactly what churned.
func TargetKey(bandName, title string) string { return norm(bandName) + "\x00" + norm(title) }

// MatchTarget resolves one archived (bandName, title) to a single live song id via a prebuilt index. It
// REFUSES to guess: zero or more-than-one match returns an error so that stream is aborted and reported,
// never mis-attached.
func MatchTarget(bandName, title string, index map[string][]string) (string, error) {
	ids := index[TargetKey(bandName, title)]
	switch len(ids) {
	case 1:
		return ids[0], nil
	case 0:
		return "", fmt.Errorf("no live song matches")
	default:
		return "", fmt.Errorf("ambiguous: %d live songs share this band+title", len(ids))
	}
}

// Plan is what to restore into a live target stream.
type Plan struct {
	LayersToCreate []domain.Layer  // archived layers an object needs but the live stream lacks (by id)
	ObjectsToCopy  []domain.Object // archived live objects absent from the live stream (by uuid), copied EXACTLY
}

// BuildPlan diffs an archived stream against its live target and returns only what is MISSING: absent
// objects (by uuid) and the layers they need that the live stream lacks (by id). It is idempotent — an
// object or layer already live is left out, so a second run copies nothing. It preserves each object
// exactly (points/page/style/layer/owner) and NEVER adds an Anchor or PointsRenderHash. Archive tombstones
// are not resurrected.
func BuildPlan(archObjs []domain.Object, archLayers []domain.Layer, liveObjs []domain.Object, liveLayers []domain.Layer) Plan {
	liveObj := make(map[string]bool, len(liveObjs))
	for _, o := range liveObjs {
		liveObj[o.UUID] = true
	}
	liveLayer := make(map[string]bool, len(liveLayers))
	for _, l := range liveLayers {
		liveLayer[l.ID] = true
	}

	var p Plan
	need := map[string]bool{}
	for _, o := range archObjs {
		if o.Deleted || liveObj[o.UUID] {
			continue
		}
		p.ObjectsToCopy = append(p.ObjectsToCopy, o) // exact copy; Anchor/PointsRenderHash untouched
		need[o.LayerID] = true
	}
	for _, l := range archLayers {
		if need[l.ID] && !liveLayer[l.ID] {
			p.LayersToCreate = append(p.LayersToCreate, l)
		}
	}
	return p
}
