package bake

import (
	"fmt"
	"math"
	"sort"

	"troubastack/core/internal/domain"
)

// P206 Stage 3 — resolving authored jump marks into baked PageJumps.
//
// The authored form is deliberately free of pages and coordinates: a jump is a PAIR of placed icon
// landmarks, and the SOURCE carries only the destination object's uuid (the RESPEC — "nothing to
// drift"). A bundle, by contrast, is a snapshot of ONE render, so resolving the pair to a page + an
// anchor is correct HERE and only here.
//
// Everything below reads the SAME reprojected objects the overlay renderer drew (snapshotToDoc, T145),
// which is the property that matters: a hotspot that disagrees with its own ink is worse than no
// hotspot at all.

// jumpTarget is what a source resolved to, or why it did not.
type jumpTarget struct {
	page int // page index WITHIN THIS FILE
	y0   float64
}

// objectBox is an object's axis-aligned bbox in normalized [0,1] page space (I3).
type objectBox struct {
	x0, y0, x1, y1 float64
}

func boxOf(o docObject) objectBox {
	if len(o.Points) == 0 {
		return objectBox{}
	}
	b := objectBox{x0: o.Points[0].X, y0: o.Points[0].Y, x1: o.Points[0].X, y1: o.Points[0].Y}
	for _, p := range o.Points[1:] {
		b.x0 = math.Min(b.x0, p.X)
		b.y0 = math.Min(b.y0, p.Y)
		b.x1 = math.Max(b.x1, p.X)
		b.y1 = math.Max(b.y1, p.Y)
	}
	return b
}

// permille converts a normalized [0,1] coordinate to the wire's int32 permille, clamped. The proto uses
// permille rather than a float for the T149 reason: the Kotlin/TS mirror codegen has no float kind.
func permille(v float64) int32 {
	p := int32(math.Round(v * 1000))
	if p < 0 {
		return 0
	}
	if p > 1000 {
		return 1000
	}
	return p
}

// resolveJumps turns one pool FILE's objects into the PageJumps of that file's pages, keyed by the page
// index within the file. `pageBase` is the file's first page index in song.Pages (the pool space that
// member_pages also indexes — see PageJump.target_page), `pages` the file's rendered page count.
//
// Resolution is WITHIN THE FILE. A pair is placed on one chart in one gesture, and a song-level layer
// (empty FileID) is composited onto every pool file, so each file's copy of a source finds that file's
// copy of the destination — a member reading part B jumps inside part B rather than being thrown into
// part A.
//
// A jump that cannot be resolved is DROPPED with a warning naming the song; the source landmark's ink is
// untouched and the bake succeeds (Fable, ⟨GO⟩ ef24ec4c: two populations WILL arrive carrying dangling
// pointers — songs authored before the studio swept them, and survivors on a layer the deleting user was
// not allowed to edit — so this is a documented input, not defensive padding). `elsewhere` reports uuids
// that exist on ANOTHER pool file, purely so the warning can say which of the two things went wrong.
func resolveJumps(
	songTitle string,
	objects []docObject,
	pageBase, pages int,
	ownerByLayer map[string]string,
	elsewhere map[string]bool,
) (map[int][]PageJump, []string) {
	targets := map[string]jumpTarget{}
	for _, o := range objects {
		targets[o.UUID] = jumpTarget{page: o.Page, y0: boxOf(o).y0}
	}

	byPage := map[int][]PageJump{}
	var warnings []string
	for _, src := range objects {
		// ff28035b: the type string comes from the domain's own mapping, never a literal — a hand-typed
		// "icon" here would silently stop matching the day the enum's wire string changes.
		if src.Type != domain.ObjectTypeToString(domain.TypeIcon) || src.JumpTo == "" {
			continue
		}
		if src.Page < 0 || src.Page >= pages {
			// The source itself is off the rendered range. assembleSong's reflow-orphan guard fails the
			// bake for a drawn overlay in this state, so this is unreachable today; it stays because a
			// dropped jump must never become a jump onto the wrong page.
			warnings = append(warnings, fmt.Sprintf("%q: a jump mark sits on a page the chart no longer has — the jump was dropped.", songTitle))
			continue
		}
		if src.JumpTo == src.UUID {
			// A landmark pointing at ITSELF is not a jump — it is a mark that would send the reader to
			// where they already are. targets contains the source, so without this it resolves happily
			// (Fable's pairing-assertion hole, guarded in Studio and now here too).
			warnings = append(warnings, fmt.Sprintf("%q: a jump points at itself — the jump was dropped (its mark is still on the page).", songTitle))
			continue
		}
		dst, ok := targets[src.JumpTo]
		if !ok {
			if elsewhere[src.JumpTo] {
				warnings = append(warnings, fmt.Sprintf("%q: a jump's two landmarks are on different parts of this song — a jump can only point within one part, so it was dropped.", songTitle))
			} else {
				warnings = append(warnings, fmt.Sprintf("%q: a jump points at a landmark that no longer exists — the jump was dropped (its mark is still on the page).", songTitle))
			}
			continue
		}
		if dst.page < 0 || dst.page >= pages {
			warnings = append(warnings, fmt.Sprintf("%q: a jump points at page %d, which this chart no longer has — the jump was dropped.", songTitle, dst.page+1))
			continue
		}
		b := boxOf(src)
		byPage[src.Page] = append(byPage[src.Page], PageJump{
			X0Permille:            permille(b.x0),
			Y0Permille:            permille(b.y0),
			X1Permille:            permille(b.x1),
			Y1Permille:            permille(b.y1),
			TargetPage:            int32(pageBase + dst.page),
			TargetAnchorYPermille: permille(dst.y0),
			LayerID:               src.LayerID,
			Owner:                 ownerByLayer[src.LayerID], // P205: "" = shared; a member id = personal
		})
	}
	// Deterministic order — a re-bake of identical content must produce identical bytes (WriteTstage).
	for _, js := range byPage {
		sort.Slice(js, func(a, b int) bool {
			if js[a].Y0Permille != js[b].Y0Permille {
				return js[a].Y0Permille < js[b].Y0Permille
			}
			if js[a].X0Permille != js[b].X0Permille {
				return js[a].X0Permille < js[b].X0Permille
			}
			return js[a].LayerID < js[b].LayerID
		})
	}
	return byPage, warnings
}
