package bake

import (
	"strings"
	"testing"
)

// P206 Stage 3 — the pure resolution of an authored jump PAIR into a baked PageJump. The authored form
// carries no page and no coordinate (only the destination's uuid), so everything a reader needs is
// decided here; and per Fable's ⟨GO⟩ on ef24ec4c, an unresolvable jump is a KNOWN input (songs authored
// before the studio swept dangling pointers, and survivors on a layer the deleting user could not edit).
// The rule those two populations meet: drop the jump, keep the ink, do not fail the bake.

func icon(uuid string, page int, x0, y0, x1, y1 float64, jumpTo string) docObject {
	return docObject{
		UUID: uuid, LayerID: "L1", Type: "icon", Page: page, Text: "segno", JumpTo: jumpTo,
		Points: []docPoint{{X: x0, Y: y0}, {X: x1, Y: y1}},
	}
}

func TestResolveJumps_PairResolvesToPageAndAnchor(t *testing.T) {
	objs := []docObject{
		icon("dest", 2, 0.40, 0.250, 0.48, 0.31, ""),
		icon("src", 0, 0.10, 0.600, 0.18, 0.66, "dest"),
	}
	// pageBase 5: this file's page 0 is the pool's entry 5 (T137 — target_page indexes the POOL, the
	// same space member_pages does).
	byPage, warns := resolveJumps("Song", objs, 5, 3, map[string]string{"L1": "m-1"}, nil)
	if len(warns) != 0 {
		t.Fatalf("a resolvable pair must warn about nothing, got %v", warns)
	}
	if len(byPage) != 1 || len(byPage[0]) != 1 {
		t.Fatalf("want one jump on the SOURCE's page 0, got %v", byPage)
	}
	j := byPage[0][0]
	if j.X0Permille != 100 || j.Y0Permille != 600 || j.X1Permille != 180 || j.Y1Permille != 660 {
		t.Fatalf("hotspot = %d,%d..%d,%d, want the SOURCE's bbox in permille (100,600..180,660)", j.X0Permille, j.Y0Permille, j.X1Permille, j.Y1Permille)
	}
	if j.TargetPage != 7 { // pageBase 5 + the destination's page 2
		t.Fatalf("target page = %d, want 7 (pool base 5 + destination page 2)", j.TargetPage)
	}
	if j.TargetAnchorYPermille != 250 {
		t.Fatalf("anchor = %d, want the DESTINATION's top (250)", j.TargetAnchorYPermille)
	}
	if j.LayerID != "L1" || j.Owner != "m-1" {
		t.Fatalf("layer/owner = %q/%q — a PageJump without both cannot be visibility-filtered (A1/P205)", j.LayerID, j.Owner)
	}
	// Only the SOURCE is a jump; the destination is an ordinary landmark and emits nothing.
	if len(byPage[2]) != 0 {
		t.Fatalf("the destination page must carry no jump of its own, got %v", byPage[2])
	}
}

func TestResolveJumps_DanglingTargetIsDroppedNotFatal(t *testing.T) {
	// The documented input: a source whose destination was deleted (pre-sweep corpus, or a survivor on a
	// layer the deleting user could not edit).
	objs := []docObject{icon("src", 0, 0.1, 0.6, 0.18, 0.66, "gone-uuid")}
	byPage, warns := resolveJumps("Ballad", objs, 0, 1, nil, nil)
	if len(byPage[0]) != 0 {
		t.Fatalf("a jump pointing at nothing must NOT be baked, got %v", byPage[0])
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "Ballad") {
		t.Fatalf("want one warning naming the song, got %v", warns)
	}
	if !strings.Contains(warns[0], "no longer exists") {
		t.Fatalf("the warning must say WHY it was dropped, got %q", warns[0])
	}
}

func TestResolveJumps_CrossPartPairSaysSo(t *testing.T) {
	// The destination lives on another pool file (part B). Same outcome — dropped — but the admin is told
	// the real reason instead of "it no longer exists", which would send them looking for a deletion.
	objs := []docObject{icon("src", 0, 0.1, 0.6, 0.18, 0.66, "dest-in-part-b")}
	byPage, warns := resolveJumps("Suite", objs, 0, 1, nil, map[string]bool{"dest-in-part-b": true})
	if len(byPage[0]) != 0 {
		t.Fatalf("a cross-part jump must not be baked, got %v", byPage[0])
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "different parts") {
		t.Fatalf("want the cross-part wording, got %v", warns)
	}
}

func TestResolveJumps_TargetPastTheLastPageIsDropped(t *testing.T) {
	// A shorter re-bake (a generated chart reflows: T75/T76/T77) can leave the destination on a page the
	// render no longer has. Never clamp onto a neighbouring page — a jump to the wrong page is worse on
	// stage than a jump that is not there.
	objs := []docObject{
		icon("dest", 4, 0.4, 0.25, 0.48, 0.31, ""),
		icon("src", 0, 0.1, 0.60, 0.18, 0.66, "dest"),
	}
	byPage, warns := resolveJumps("Reel", objs, 0, 2, nil, nil)
	if len(byPage[0]) != 0 {
		t.Fatalf("want no jump when the target page is past the render, got %v", byPage[0])
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "page 5") {
		t.Fatalf("the warning must name the page (1-based), got %v", warns)
	}
}

func TestResolveJumps_IgnoresNonIconsAndPlainIcons(t *testing.T) {
	objs := []docObject{
		icon("plain", 0, 0.1, 0.1, 0.2, 0.2, ""), // an ordinary cue stamp
		{UUID: "r1", LayerID: "L1", Type: "rect", Page: 0, JumpTo: "plain", // a jumpTo on a non-icon is not a jump
			Points: []docPoint{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}}},
	}
	byPage, warns := resolveJumps("Song", objs, 0, 1, nil, nil)
	if len(byPage) != 0 || len(warns) != 0 {
		t.Fatalf("neither a plain icon nor a rect is a jump source: %v / %v", byPage, warns)
	}
}

func TestResolveJumps_OrderIsDeterministic(t *testing.T) {
	// A re-bake of identical content must produce identical bytes (WriteTstage), so the per-page order
	// cannot depend on the object order the snapshot happened to yield.
	dest := icon("dest", 0, 0.4, 0.9, 0.48, 0.95, "")
	lower := icon("lower", 0, 0.10, 0.600, 0.18, 0.66, "dest")
	upper := icon("upper", 0, 0.10, 0.200, 0.18, 0.26, "dest")
	a, _ := resolveJumps("Song", []docObject{dest, lower, upper}, 0, 1, nil, nil)
	b, _ := resolveJumps("Song", []docObject{upper, dest, lower}, 0, 1, nil, nil)
	if len(a[0]) != 2 || len(b[0]) != 2 {
		t.Fatalf("want both jumps on page 0, got %d / %d", len(a[0]), len(b[0]))
	}
	if a[0][0].Y0Permille != 200 || a[0][1].Y0Permille != 600 {
		t.Fatalf("jumps must sort top-down, got %d then %d", a[0][0].Y0Permille, a[0][1].Y0Permille)
	}
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatalf("resolution must not depend on input order: %+v vs %+v", a[0][i], b[0][i])
		}
	}
}

func TestPermille_RoundsAndClamps(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want int32
	}{{0, 0}, {0.5, 500}, {0.1234, 123}, {0.1235, 124}, {1, 1000}, {-0.2, 0}, {1.4, 1000}} {
		if got := permille(c.in); got != c.want {
			t.Fatalf("permille(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
