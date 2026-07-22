package bake

import (
	"context"
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

// recordRaster is a fake rasterizer that captures the PDF handed to it. Since the real
// raster is content-blind here, this is how we assert the bake fed the TRANSPOSED PDF.
type recordRaster struct {
	png     []byte
	lastPDF *[]byte
}

func (r recordRaster) Rasterize(_ context.Context, pdf []byte) ([][]byte, error) {
	*r.lastPDF = append([]byte(nil), pdf...)
	return [][]byte{r.png}, nil
}

const transposeChartSrc = "# Stand By Me\n## Verse\nC            G\nwhen the night has come\n"

// TestBake_TransposesChartOrWarns (T60 surface 2): a setlist item with transposeChords
// and eligible conditions bakes the chart TRANSPOSED to the key override (band-wide);
// a degraded item (unparseable override) bakes UNTRANSPOSED without failing.
func TestBake_TransposesChartOrWarns(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "Stand By Me", "")
	if err != nil {
		t.Fatal(err)
	}
	// Song key = C (a parseable "from").
	key := "C"
	if _, err := svc.UpdateSong(u, band.ID, song.ID, app.SongPatch{Key: &key}); err != nil {
		t.Fatal(err)
	}
	// The only file is a generated chart.
	if _, err := svc.CreateTextChart(u, band.ID, song.ID, transposeChartSrc); err != nil {
		t.Fatal(err)
	}
	// A minimal annotation layer so the engine snapshot is non-empty (bakeSong reads Head).
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", Name: "Marks", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
		AuthorID: u.ID,
	}); err != nil {
		t.Fatal(err)
	}
	sl, err := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	item, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Expected renders (chartpdf is deterministic — pinned dates).
	from, _ := chartpdf.ParseKey("C")
	to, _ := chartpdf.ParseKey("D")
	transposedSrc, _ := chartpdf.Transpose(transposeChartSrc, from, to)
	wantTransposed, _ := chartpdf.Render(transposedSrc)
	wantStored, _ := chartpdf.Render(transposeChartSrc)

	png := tinyPNG(t, 40, 56)
	var captured []byte
	newBaker := func() *Baker {
		return &Baker{
			svc:      svc,
			eng:      eng,
			raster:   recordRaster{png: png, lastPDF: &captured},
			overlays: fakeOverlays{png: png},
			bakesDir: t.TempDir(),
			now:      func() int64 { return 1700000000 },
		}
	}

	// Eligible: override D + transposeChords → the baked chart is transposed C→D.
	yes := true
	over := "D"
	if _, err := svc.UpdateSetlistItem(u, band.ID, sl.ID, item.ID, app.SetlistItemPatch{KeyOverride: &over, TransposeChords: &yes}); err != nil {
		t.Fatal(err)
	}
	if _, err := newBaker().Bake(context.Background(), band.ID, sl.ID, u, nil); err != nil {
		t.Fatalf("transposed bake failed: %v", err)
	}
	if string(captured) != string(wantTransposed) {
		t.Errorf("baked chart is not the transposed render")
	}
	if string(captured) == string(wantStored) {
		t.Errorf("baked chart equals the untransposed render — transpose did not run")
	}

	// Degraded: override edited to garbage → bake must NOT fail; chart is untransposed.
	garbage := "not-a-key"
	if _, err := svc.UpdateSetlistItem(u, band.ID, sl.ID, item.ID, app.SetlistItemPatch{KeyOverride: &garbage}); err != nil {
		t.Fatal(err)
	}
	captured = nil
	if _, err := newBaker().Bake(context.Background(), band.ID, sl.ID, u, nil); err != nil {
		t.Fatalf("degraded bake must not fail: %v", err)
	}
	if string(captured) != string(wantStored) {
		t.Errorf("degraded bake did not fall back to the untransposed render")
	}

	// TransposeEligible reasons (the single source of truth shared with the UI).
	if ok, _ := app.TransposeEligible("C", "D", true); !ok {
		t.Error("C→D with a chart should be eligible")
	}
	if _, reason := app.TransposeEligible("C", "D", false); reason != "no text chart on this song" {
		t.Errorf("no-chart reason = %q", reason)
	}
	if _, reason := app.TransposeEligible("weird", "D", true); reason != "song key not set or not parseable" {
		t.Errorf("bad-song-key reason = %q", reason)
	}
	if _, reason := app.TransposeEligible("C", "weird", true); reason != "override key not parseable" {
		t.Errorf("bad-override reason = %q", reason)
	}
}
