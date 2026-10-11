package app_test

// The export/import column of the annotation-anchoring matrix (see chartpdf/anchor_matrix_test.go): a band
// folder round trip, per shape, on a GENERATED chart. P30: a linked mark comes back with the same link and
// render hash. P31: a mark that arrives WITHOUT a link (an old folder, coordinates only) is linked on
// import when it sits on or beside text, exactly as the HTTP import path does. Known-red cells skip unless
// ANCHOR_RED=1.

import (
	"reflect"
	"testing"

	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

const matrixChart = "# The Open Road\n\n## Verse 1\nG            D\nPack a little light for the road ahead\n" +
	"Em           C\nLeave the porch light on when the sun goes down\n"

// knownRedBandio: red on today's code (baseline 2026-10-11). Delete a line when its fix lands.
var knownRedBandio = map[string][2]string{
	"freehand/import_links_unlinked_mark":  {"P31", "import-link"},
	"line/import_links_unlinked_mark":      {"P31", "import-link"},
	"rect/import_links_unlinked_mark":      {"P31", "import-link"},
	"ellipse/import_links_unlinked_mark":   {"P31", "import-link"},
	"text/import_links_unlinked_mark":      {"P31", "import-link"},
	"highlight/import_links_unlinked_mark": {"P31", "import-link"},
	"icon/import_links_unlinked_mark":      {"P31", "import-link"},
	"jump_pair/import_links_unlinked_mark": {"P31", "import-link"},
}

func TestAnchorMatrixBandio(t *testing.T) {
	shapes := map[string]domain.ObjectType{
		"freehand": domain.TypeFreehand, "line": domain.TypeLine, "rect": domain.TypeRect,
		"ellipse": domain.TypeEllipse, "text": domain.TypeText, "highlight": domain.TypeHighlight,
		"icon": domain.TypeIcon, "jump_pair": domain.TypeIcon,
	}
	order := []string{"freehand", "line", "rect", "ellipse", "text", "highlight", "icon", "jump_pair"}
	for _, shape := range order {
		for _, linked := range []bool{true, false} {
			shape, linked := shape, linked
			name := shape + "/export_import_round_trip"
			if !linked {
				name = shape + "/import_links_unlinked_mark"
			}
			t.Run(name, func(t *testing.T) {
				if r, ok := knownRedBandio[name]; ok {
					testutil.AnchorKnownRed(t, r[0], r[1])
				}
				runBandioCell(t, shape, shapes[shape], linked)
			})
		}
	}
}

func runBandioCell(t *testing.T, shape string, typ domain.ObjectType, linked bool) {
	src := newStack()
	admin, err := src.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := src.svc.CreateBand(admin, "Demo Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := src.svc.CreateSong(admin, band.ID, "The Open Road", "Demo")
	if err != nil {
		t.Fatal(err)
	}
	f, err := src.svc.CreateTextChart(admin, band.ID, song.ID, matrixChart)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(m domain.Mutation) {
		t.Helper()
		if _, err := src.eng.Apply(song.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	apply(domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: f.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}})

	_, anchors, err := chartpdf.RenderWithAnchors(matrixChart)
	if err != nil {
		t.Fatal(err)
	}
	var line chartpdf.Anchor
	for _, a := range anchors {
		if a.Text == "Leave the porch light on when the sun goes down" {
			line = a
		}
	}
	h := line.Y1 - line.Y0
	box := []domain.Point{{X: line.X0 + 0.05, Y: line.Y0 + 0.1*h}, {X: line.X0 + 0.15, Y: line.Y1 - 0.1*h}}
	pts := box
	switch shape {
	case "text":
		pts = box[:1]
	case "freehand":
		pts = []domain.Point{box[0], {X: box[1].X, Y: box[0].Y}, box[1], {X: box[0].X, Y: box[1].Y}}
	}
	objs := []domain.Object{{UUID: "m1", LayerID: "L1", Type: typ, Page: 0, Version: 1, OwnerID: admin.ID,
		Points: pts, Text: map[bool]string{true: "star", false: ""}[typ == domain.TypeIcon],
		Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.003, FontSize: 0.02}}}
	if typ == domain.TypeText {
		objs[0].Text = "softly"
	}
	if shape == "jump_pair" {
		d := objs[0]
		d.UUID = "m2"
		objs[0].JumpTo = "m2"
		objs = append(objs, d)
	}
	for i := range objs {
		if linked {
			objs[i] = chartpdf.AnchorObject(objs[i], anchors, f.BlobHash)
			if objs[i].Anchor == nil {
				t.Fatal("fixture: the mark did not link at creation")
			}
		}
		o := objs[i]
		apply(domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: admin.ID, Object: &o})
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Object{}
	for _, o := range snap.LiveObjects() {
		got[o.UUID] = o
	}
	for _, want := range objs {
		g, ok := got[want.UUID]
		if !ok {
			t.Fatalf("%s did not survive the round trip", want.UUID)
		}
		if linked {
			// P30: the link and the render it was computed on come back unchanged.
			if !reflect.DeepEqual(g.Anchor, want.Anchor) || g.PointsRenderHash != want.PointsRenderHash {
				t.Errorf("%s: link %+v / %q after import, want %+v / %q", want.UUID, g.Anchor, g.PointsRenderHash, want.Anchor, want.PointsRenderHash)
			}
		} else if g.Anchor == nil {
			// P31: a coordinates-only mark on a generated chart, sitting on a line, is linked on import.
			t.Errorf("%s: imported onto a generated chart with no link, though it sits on a line of text", want.UUID)
		}
		if want.JumpTo != "" && g.JumpTo != want.JumpTo {
			t.Errorf("%s: jump pairing lost on import (%q, want %q)", want.UUID, g.JumpTo, want.JumpTo)
		}
	}
}
