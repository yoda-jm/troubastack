package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// B13 containment test: every anchor-bound "cover" object (highlight / ring / rect / ellipse)
// must actually contain its target run, and every freehand stroke must read hand-drawn (not
// axis-parallel). Red-first is guaranteed by TestContainment_catchesDrift (the checker has
// teeth); the real builders must then be green.

const coverTol = 0.005 // the target may poke out of the object by at most this (page fraction)

// objBox is the axis-aligned bounding box of an object's points.
func objBox(o wireObject) anchorBox {
	bb := anchorBox{Page: o.Page, X0: math.MaxFloat64, Y0: math.MaxFloat64, X1: -math.MaxFloat64, Y1: -math.MaxFloat64}
	for _, p := range o.Points {
		bb.X0, bb.Y0 = math.Min(bb.X0, p.X), math.Min(bb.Y0, p.Y)
		bb.X1, bb.Y1 = math.Max(bb.X1, p.X), math.Max(bb.Y1, p.Y)
	}
	return bb
}

// covers reports whether box a contains box t within tolerance.
func covers(a, t anchorBox, tol float64) bool {
	return a.X0 <= t.X0+tol && a.Y0 <= t.Y0+tol && a.X1 >= t.X1-tol && a.Y1 >= t.Y1-tol
}

// checkContainment verifies every "cover" placement in im is covered by its object, and every
// freehand stroke is non-axis-parallel. Returns the list of problems (empty = OK).
func checkContainment(im annotationsImport) []string {
	var probs []string
	byUUID := map[string]wireObject{}
	for _, o := range im.Objects {
		byUUID[o.UUID] = o
	}
	for _, pl := range im.placements {
		o, ok := byUUID[pl.uuid]
		if !ok {
			probs = append(probs, "placement "+pl.uuid+" has no object")
			continue
		}
		bb := objBox(o)
		// A stroked mark (highlighter swipe / ring) covers a band of ±width/2 around its
		// polyline, so expand by the half stroke-width before the containment check.
		if hw := o.Style.Width / 2; hw > 0 {
			bb.X0 -= hw
			bb.Y0 -= hw
			bb.X1 += hw
			bb.Y1 += hw
		}
		if pl.kind == "cover" && !covers(bb, pl.tgt, coverTol) {
			probs = append(probs, pl.uuid+": object ["+f(bb.X0)+","+f(bb.Y0)+"→"+f(bb.X1)+","+f(bb.Y1)+
				"] does not cover target ["+f(pl.tgt.X0)+","+f(pl.tgt.Y0)+"→"+f(pl.tgt.X1)+","+f(pl.tgt.Y1)+"]")
		}
	}
	for _, o := range im.Objects {
		if o.Type != "freehand" || len(o.Points) < 3 {
			continue
		}
		bb := objBox(o)
		if bb.Y1-bb.Y0 < 0.002 || bb.X1-bb.X0 < 0.002 {
			probs = append(probs, o.UUID+": freehand looks axis-parallel/ruler-straight (yR="+
				f(bb.Y1-bb.Y0)+" xR="+f(bb.X1-bb.X0)+")")
		}
	}
	return probs
}

func f(v float64) string { return fmt.Sprintf("%.4f", v) }

func testUsers() map[string]string {
	return map[string]string{"marie": "u-marie", "leo": "u-leo", "sasha": "u-sasha", "ivan": "u-ivan", "cory": "u-cory", "flora": "u-flora"}
}

func TestContainment_openRoad(t *testing.T) {
	an := mustAnchors("open-road-leadsheet")
	im := buildOpenRoadAnnotations("song-or", "file-or", testUsers(), "u-leo", an)
	if len(im.placements) == 0 {
		t.Fatal("no anchor-bound placements recorded — builder not migrated")
	}
	if probs := checkContainment(im); len(probs) > 0 {
		for _, p := range probs {
			t.Error(p)
		}
	}
	t.Logf("open road: %d objects, %d anchored placements", len(im.Objects), len(im.placements))
}

func TestContainment_openRoadGuitar(t *testing.T) {
	im := buildOpenRoadGuitarAnnotations("s2", "f2", testUsers(), "u-leo", mustAnchors("open-road-guitar"))
	if probs := checkContainment(im); len(probs) > 0 {
		for _, p := range probs {
			t.Error(p)
		}
	}
	t.Logf("open road guitar: %d objects, %d anchored placements", len(im.Objects), len(im.placements))
}

func TestContainment_house(t *testing.T) {
	im := buildBandChartAnnotations("s4", "f4", "House of the Rising Sun", testUsers(), "u-leo", mustAnchors("house-rising-sun-tab"))
	if probs := checkContainment(im); len(probs) > 0 {
		for _, p := range probs {
			t.Error(p)
		}
	}
	t.Logf("house: %d objects, %d placements", len(im.Objects), len(im.placements))
}

func TestContainment_amazingGrace(t *testing.T) {
	im := buildBandChartAnnotations("s3", "f3", "Amazing Grace", testUsers(), "u-leo", mustAnchors("amazing-grace"))
	if probs := checkContainment(im); len(probs) > 0 {
		for _, p := range probs {
			t.Error(p)
		}
	}
	t.Logf("amazing grace: %d objects, %d anchored placements", len(im.Objects), len(im.placements))
}

func TestContainment_engraved(t *testing.T) {
	u := testUsers()
	cases := map[string]annotationsImport{
		"greensleeves":           buildBandChartAnnotations("s5", "f5", "Greensleeves", u, "u-leo", mustAnchors("greensleeves")),
		"ek-violin1":             buildEineKleineAnnotations("s6", "f6", u, "u-flora", mustAnchors("ek-violin1")),
		"canon-violin1":          buildCanonAnnotations("s7", "f7", u, "u-flora", mustAnchors("canon-violin1")),
		"house-rising-sun-drums": buildDrumPartAnnotations("s10", "f10", mustAnchors("house-rising-sun-drums")),
	}
	for name, im := range cases {
		if probs := checkContainment(im); len(probs) > 0 {
			for _, p := range probs {
				t.Errorf("%s: %s", name, p)
			}
		}
		t.Logf("%s: %d objects, %d placements", name, len(im.Objects), len(im.placements))
	}
}

// TestDumpImports writes the built imports to $TROUBA_DUMP_DIR for offline visual calibration
// (composite over the chart raster). Skipped unless the env var is set.
func TestDumpImports(t *testing.T) {
	dir := os.Getenv("TROUBA_DUMP_DIR")
	if dir == "" {
		t.Skip("set TROUBA_DUMP_DIR to dump imports for calibration")
	}
	u := testUsers()
	dump := func(name string, im annotationsImport) {
		b, _ := json.MarshalIndent(im, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, name+".import.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dump("open-road-leadsheet", buildOpenRoadAnnotations("s", "f", u, "u-leo", mustAnchors("open-road-leadsheet")))
	dump("open-road-guitar", buildOpenRoadGuitarAnnotations("s2", "f2", u, "u-leo", mustAnchors("open-road-guitar")))
	dump("amazing-grace", buildBandChartAnnotations("s3", "f3", "Amazing Grace", u, "u-leo", mustAnchors("amazing-grace")))
	dump("house-rising-sun-tab", buildBandChartAnnotations("s4", "f4", "House of the Rising Sun", u, "u-leo", mustAnchors("house-rising-sun-tab")))
	dump("house-rising-sun-drums", buildDrumPartAnnotations("s4d", "f4d", mustAnchors("house-rising-sun-drums")))
	dump("greensleeves", buildBandChartAnnotations("s5", "f5", "Greensleeves", u, "u-leo", mustAnchors("greensleeves")))
	dump("ek-violin1", buildEineKleineAnnotations("s6", "f6", u, "u-flora", mustAnchors("ek-violin1")))
	dump("canon-violin1", buildCanonAnnotations("s7", "f7", u, "u-flora", mustAnchors("canon-violin1")))
	dump("ek-score", buildScoreConductorAnnotations("s8", "f8", "ek", "u-flora", mustAnchors("ek-score")))
	dump("canon-score", buildScoreConductorAnnotations("s9", "f9", "canon", "u-flora", mustAnchors("canon-score")))
}

// TestContainment_catchesDrift proves the checker fails on a mis-placed mark (red-first teeth).
func TestContainment_catchesDrift(t *testing.T) {
	im := annotationsImport{
		Objects: []wireObject{{
			UUID: "x", Type: "highlight",
			Points: []wirePoint{{X: 0.10, Y: 0.10}, {X: 0.20, Y: 0.12}},
		}},
		placements: []placement{{
			uuid: "x", kind: "cover",
			tgt: anchorBox{X0: 0.50, Y0: 0.50, X1: 0.60, Y1: 0.52}, // nowhere near the object
		}},
	}
	if probs := checkContainment(im); len(probs) == 0 {
		t.Fatal("checker did not catch a drifted mark — it has no teeth")
	}
}
