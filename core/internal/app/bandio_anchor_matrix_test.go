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

// knownRedBandio: cells known to fail. Empty since T195: the band-folder import links coordinates-only
// marks (P31), which turned the 8 import_links_unlinked_mark cells green.
var knownRedBandio = map[string][2]string{}

func TestAnchorMatrixBandio(t *testing.T) {
	shapes := map[string]domain.ObjectType{
		"freehand": domain.TypeFreehand, "line": domain.TypeLine, "rect": domain.TypeRect,
		"ellipse": domain.TypeEllipse, "text": domain.TypeText, "highlight": domain.TypeHighlight,
		"icon": domain.TypeIcon, "jump_pair": domain.TypeIcon,
	}
	order := []string{"freehand", "line", "rect", "ellipse", "text", "highlight", "icon", "jump_pair"}
	for _, shape := range order {
		for _, mode := range []string{"linked", "unlinked", "oldstyle"} {
			shape, mode := shape, mode
			name := map[string]string{
				"linked":   shape + "/export_import_round_trip",
				"unlinked": shape + "/import_links_unlinked_mark",
				"oldstyle": shape + "/old_style_anchor_round_trip",
			}[mode]
			t.Run(name, func(t *testing.T) {
				if r, ok := knownRedBandio[name]; ok {
					testutil.AnchorKnownRed(t, r[0], r[1])
				}
				runBandioCell(t, shape, shapes[shape], mode)
			})
		}
	}
}

func runBandioCell(t *testing.T, shape string, typ domain.ObjectType, mode string) {
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
	chart, err := src.svc.ChartRenderForFile(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	targetID := chart.Lines.IDs[line.SrcLine]
	for i := range objs {
		switch mode {
		case "linked": // the stage-2 create path
			objs[i] = chartpdf.LinkMark(objs[i], chart)
			if objs[i].Link == nil || objs[i].Link.Line != targetID {
				t.Fatalf("fixture: the mark did not link at creation: %+v", objs[i].Link)
			}
		case "oldstyle": // a T145-era mark: an old-style anchor, which stage 2 never interprets
			objs[i].Anchor = &domain.SourceAnchor{RunText: line.Text, Occurrence: 1, CharStart: 2, CharEnd: 9}
			objs[i].PointsRenderHash = "an-older-render"
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
		switch mode {
		case "linked", "oldstyle":
			// P30: the link (either style) and the render it was computed on come back unchanged.
			if !reflect.DeepEqual(g.Link, want.Link) || !reflect.DeepEqual(g.Anchor, want.Anchor) ||
				g.PointsRenderHash != want.PointsRenderHash || !reflect.DeepEqual(g.Points, want.Points) {
				t.Errorf("%s: link %+v anchor %+v / %q after import, want %+v %+v / %q", want.UUID, g.Link, g.Anchor,
					g.PointsRenderHash, want.Link, want.Anchor, want.PointsRenderHash)
			}
		case "unlinked":
			// P31: a coordinates-only mark on a generated chart, sitting on a line, is linked on import —
			// to THAT line (its id travels in the folder's lineIdentity) — and does not move.
			if g.Link == nil || g.Link.State != domain.LinkLinked || g.Link.Line != targetID {
				t.Errorf("%s: imported onto a generated chart with link %+v, want linked to the line it sits on", want.UUID, g.Link)
			}
			if !reflect.DeepEqual(g.Points, want.Points) {
				t.Errorf("%s: linking at import moved the mark", want.UUID)
			}
		}
		if want.JumpTo != "" && g.JumpTo != want.JumpTo {
			t.Errorf("%s: jump pairing lost on import (%q, want %q)", want.UUID, g.JumpTo, want.JumpTo)
		}
	}
}

// F2 (T195 review), the band-folder path: a folder whose mark carries a link the server could not have
// written (here a line-height of 5e-324: an infinite scale) is not trusted on import — the link is dropped
// and the mark re-linked by the rule (P31), so it can never break a serve or a bake.
func TestBandImport_ForgedLinkIsRelinked(t *testing.T) {
	src := newStack()
	admin, err := src.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, _ := src.svc.CreateBand(admin, "Demo Band")
	song, _ := src.svc.CreateSong(admin, band.ID, "A Test Song", "Demo")
	f, err := src.svc.CreateTextChart(admin, band.ID, song.ID, matrixChart)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: f.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	chart, err := src.svc.ChartRenderForFile(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	var line chartpdf.Anchor
	for _, a := range chart.Anchors {
		if a.Text == "Leave the porch light on when the sun goes down" {
			line = a
		}
	}
	id := chart.Lines.IDs[line.SrcLine]
	o := domain.Object{UUID: "m1", LayerID: "L1", Type: domain.TypeRect, Page: line.Page, Version: 1, OwnerID: admin.ID,
		Points: []domain.Point{{X: line.X0, Y: line.Y0}, {X: line.X0 + 0.1, Y: line.Y1}},
		Link:   &domain.MarkLink{State: domain.LinkLinked, Line: id, XRef: "start", LH: 5e-324, Render: "elsewhere"}, PointsRenderHash: "elsewhere"}
	if _, err := src.eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, band.ID)
	if err != nil {
		t.Fatal(err)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, _ := tgt.eng.Head(songs[0].ID)
	for _, g := range snap.LiveObjects() {
		if g.Link == nil || !chartpdf.ValidLink(g.Link) || g.Link.Line != id || g.Link.State != domain.LinkLinked {
			t.Errorf("%s: imported with link %+v, want the forged one dropped and a valid link to its line", g.UUID, g.Link)
		}
	}
}
