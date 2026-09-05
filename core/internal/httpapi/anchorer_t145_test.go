package httpapi

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// TestChartAnchorer_AnchorsMarkOnGeneratedChart is the adapter half of the T145 create-time hook: given a
// mark on a layer whose file is a generated chart, the adapter renders the source, finds the run under the
// mark, and stamps the source anchor + the current render hash. It is fully deterministic — it renders the
// chart itself, picks a REAL run, and centres the mark on it, so it never guesses coordinates.
func TestChartAnchorer_AnchorsMarkOnGeneratedChart(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Riverside Waltz", "The Riverside Trio")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := svc.CreateTextChart(admin, band.ID, song.ID,
		"# Riverside Waltz\nThe Riverside Trio\n\n## Verse\nAm\nrolling downstream slow\n")
	if err != nil {
		t.Fatal(err)
	}

	// Render the SAME source the adapter will, and pick a real run to place the mark on.
	_, src, err := svc.ChartSourceForFile(chart.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, anchors, err := chartpdf.RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	var target chartpdf.Anchor
	for _, a := range anchors {
		if a.Text != "" {
			target = a
			break
		}
	}
	if target.Text == "" {
		t.Fatal("no text run in the rendered chart to anchor to")
	}

	// Materialize a layer pointing at the generated chart file (the create's target layer in HEAD).
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", FileID: chart.ID, Zone: domain.ZonePersonal, OwnerID: admin.ID, Access: domain.AccessRW},
		AuthorID: admin.ID,
		Summary:  "layer",
	}); err != nil {
		t.Fatal(err)
	}

	anc := newChartAnchorer(svc, eng)
	mark := domain.Object{
		UUID:    "o1",
		LayerID: "L1",
		Type:    domain.TypeHighlight,
		Page:    target.Page,
		Points:  []domain.Point{{X: target.X0, Y: target.Y0}, {X: target.X1, Y: target.Y1}},
	}
	out := anc.AnchorMark(song.ID, mark)

	if out.Anchor == nil {
		t.Fatal("adapter did not anchor a mark placed on a real text run")
	}
	if out.Anchor.RunText != target.Text {
		t.Errorf("anchor RunText = %q, want %q", out.Anchor.RunText, target.Text)
	}
	if out.PointsRenderHash != chart.BlobHash {
		t.Errorf("PointsRenderHash = %q, want the chart's BlobHash %q", out.PointsRenderHash, chart.BlobHash)
	}

	// Cache hit: a second call resolves the same file without re-rendering, same result.
	if out2 := anc.AnchorMark(song.ID, mark); out2.Anchor == nil || out2.Anchor.RunText != target.Text {
		t.Errorf("cached second call anchor = %+v, want RunText %q", out2.Anchor, target.Text)
	}
}

// TestChartAnchorer_BestEffortDegradesToNoAnchor covers the safe-degradation paths: an unknown layer, an
// already-anchored mark, and a mark with no layer all return the object unchanged (never panic, never
// wrong). These are the pre-T145 behavior the hook must fall back to on any miss.
func TestChartAnchorer_BestEffortDegradesToNoAnchor(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	anc := newChartAnchorer(svc, eng)

	// Unknown layer → no anchor.
	if out := anc.AnchorMark("s1", domain.Object{UUID: "o1", LayerID: "nope"}); out.Anchor != nil {
		t.Errorf("unknown layer anchored a mark: %+v", out.Anchor)
	}
	// No layer id → no anchor.
	if out := anc.AnchorMark("s1", domain.Object{UUID: "o1"}); out.Anchor != nil {
		t.Errorf("layerless mark anchored: %+v", out.Anchor)
	}
	// Already anchored → returned untouched.
	pre := &domain.SourceAnchor{RunText: "kept", Occurrence: 1}
	out := anc.AnchorMark("s1", domain.Object{UUID: "o1", LayerID: "L1", Anchor: pre})
	if out.Anchor != pre {
		t.Errorf("an already-anchored mark was modified: %+v", out.Anchor)
	}
}
