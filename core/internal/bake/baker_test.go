package bake

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// tinyPNG returns a valid w×h PNG (fake rasterizer/overlay output; the Baker
// decodes its dimensions via image.DecodeConfig, so it must be real PNG bytes).
func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

type fakeRaster struct {
	pages int
	png   []byte
}

func (f fakeRaster) Rasterize(context.Context, []byte) ([][]byte, error) {
	out := make([][]byte, f.pages)
	for i := range out {
		out[i] = f.png
	}
	return out, nil
}

// fakeOverlays returns one overlay per doc layer on page 0 (mimics web/bake).
type fakeOverlays struct{ png []byte }

func (f fakeOverlays) Render(_ context.Context, req cliRequest) ([]renderedOverlay, error) {
	var out []renderedOverlay
	for _, l := range req.Doc.Layers {
		out = append(out, renderedOverlay{
			Page: 0, LayerID: l.ID, Order: int32(l.Order), Mandatory: l.Mandatory,
			RoleTag: l.RoleTag, ContentHash: Sha256Hex(f.png), PNG: f.png,
		})
	}
	return out, nil
}

// seed builds a service + engine with one band/song/PDF/annotation-layer/setlist.
func seed(t *testing.T) (*app.Service, *engine.Engine, app.User, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	song, err := svc.CreateSong(u, band.ID, "Song", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload file: %v", err)
	}
	// One annotation layer + a rect object on it (the renderer doc source).
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", Name: "Marks", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
		AuthorID: u.ID,
	}); err != nil {
		t.Fatalf("layer create: %v", err)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind: domain.KindCreate, UUID: "o1", AuthorID: u.ID,
		Object: &domain.Object{
			UUID: "o1", LayerID: "L1", Type: domain.TypeRect, Page: 0, Version: 1,
			Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
			Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
		},
	}); err != nil {
		t.Fatalf("object create: %v", err)
	}
	sl, err := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("create setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	return svc, eng, u, band.ID, sl.ID
}

func TestBake_ProducesValidBundle_andBumpsRev(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	pngBytes := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: pngBytes},
		overlays: fakeOverlays{png: pngBytes},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	cb, err := b.Bake(context.Background(), bandID, setlistID, u)
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if cb.ConcertRev != 1 {
		t.Fatalf("first bake rev = %d, want 1", cb.ConcertRev)
	}
	if len(cb.Songs) != 1 || len(cb.Songs[0].Pages) != 1 {
		t.Fatalf("want 1 song × 1 page, got %d songs", len(cb.Songs))
	}
	if got := len(cb.Songs[0].Pages[0].Overlays); got != 1 {
		t.Fatalf("want 1 overlay (one layer), got %d", got)
	}
	if cb.Songs[0].SourceRevision == 0 {
		t.Fatalf("source_revision should be the song's head revision, got 0")
	}

	// bundle.json parses and every blob ref resolves on disk (the container contract).
	revDir := filepath.Join(b.bakesDir, setlistID, "1")
	data, err := os.ReadFile(filepath.Join(revDir, "bundle.json"))
	if err != nil {
		t.Fatalf("read bundle.json: %v", err)
	}
	var parsed ConcertBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("bundle.json does not parse: %v", err)
	}
	for _, song := range parsed.Songs {
		for _, page := range song.Pages {
			for _, ref := range append([]string{page.PageRasterRef}, refsOf(page.Overlays)...) {
				if _, err := os.Stat(filepath.Join(revDir, filepath.FromSlash(ref))); err != nil {
					t.Fatalf("blob ref %q missing: %v", ref, err)
				}
			}
		}
	}
	// The .tstage was written alongside the rev dir.
	if _, err := os.Stat(filepath.Join(b.bakesDir, setlistID, "1.tstage")); err != nil {
		t.Fatalf(".tstage missing: %v", err)
	}

	// Re-bake bumps concert_rev monotonically.
	cb2, err := b.Bake(context.Background(), bandID, setlistID, u)
	if err != nil {
		t.Fatalf("re-bake: %v", err)
	}
	if cb2.ConcertRev != 2 {
		t.Fatalf("re-bake rev = %d, want 2", cb2.ConcertRev)
	}
	if got := b.BundlePath(setlistID); got != filepath.Join(b.bakesDir, setlistID, "2.tstage") {
		t.Fatalf("BundlePath = %q, want the rev-2 tstage", got)
	}
	if concerts := b.ListConcerts(); len(concerts) != 1 || concerts[0].ConcertRev != 2 {
		t.Fatalf("ListConcerts should show the latest rev (2), got %+v", concerts)
	}
}

func refsOf(overlays []LayerImage) []string {
	out := make([]string, len(overlays))
	for i, o := range overlays {
		out[i] = o.ImageRef
	}
	return out
}

// TestPopplerRasterizer exercises the REAL pdftoppm on a minimal one-page PDF.
// Skips when the binary is absent (local dev without poppler); CI installs
// poppler-utils so it runs there.
func TestPopplerRasterizer(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed")
	}
	r := popplerRasterizer{bin: "pdftoppm", dpi: 72}
	pages, err := r.Rasterize(context.Background(), []byte(minimalPDF))
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("want 1 page, got %d", len(pages))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(pages[0])); err != nil || cfg.Width == 0 {
		t.Fatalf("page 0 is not a valid PNG: %v (w=%d)", err, cfg.Width)
	}
}

// A minimal valid single-page PDF (200×280pt, blank). poppler repairs the xref if
// needed, so this rasterizes to one blank page.
const minimalPDF = "%PDF-1.4\n" +
	"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 280]>>endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"
