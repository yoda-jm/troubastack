package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// contentPNG is a white page with a centred black block, so the autodetect proposes a centred crop (and a
// far-corner mark falls outside it).
func contentPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for y := h * 3 / 10; y < h*7/10; y++ {
		for x := w * 3 / 10; x < w*7/10; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 0xff})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// oneContentPage is a Rasterizer that returns a single content page, so DetectCropForPages has something to
// detect without poppler.
type oneContentPage struct{}

func (oneContentPage) Rasterize(_ context.Context, _ []byte) ([][]byte, error) {
	return [][]byte{contentPNG(200, 280)}, nil
}

// detectFixture mounts crop-detect with a real baker (fake rasterizer) and a one-file song whose HEAD has a
// far-corner mark. caller is switchable so a non-member can be exercised.
type detectFixture struct {
	mux         *http.ServeMux
	svc         *app.Service
	caller      *app.User
	band, band2 string
	song, file  string
	chart       string
}

func newDetectFixture(t *testing.T, withBaker bool) *detectFixture {
	t.Helper()
	svc := app.NewService(memrepo.New()).WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, err := svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	band2, err := svc.CreateBand(admin, "Band2")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatal(err)
	}
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture"))
	if err != nil {
		t.Fatal(err)
	}
	chart, err := svc.CreateTextChart(admin, band.ID, song.ID, "# T\n\n## V\nline\n")
	if err != nil {
		t.Fatal(err)
	}
	// A mark in the far bottom-right corner, outside any centred crop.
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: file.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	o := domain.Object{UUID: "corner", LayerID: "L1", Page: 0, Type: domain.TypeRect, Version: 1,
		Points: []domain.Point{{X: 0.95, Y: 0.95}}, Style: domain.Style{Width: 0.01}}
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: "corner", Object: &o, AuthorID: admin.ID}); err != nil {
		t.Fatal(err)
	}

	var baker *bake.Baker
	if withBaker {
		baker = bake.New(svc, eng, bake.Config{Raster: oneContentPage{}})
	}
	caller := admin
	mux := http.NewServeMux()
	NewAnnotationsAPI(svc, eng, nil, baker).Mount(mux, func(h authedHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h(w, r, caller) }
	})
	return &detectFixture{mux: mux, svc: svc, caller: &caller, band: band.ID, band2: band2.ID, song: song.ID, file: file.ID, chart: chart.ID}
}

func (f *detectFixture) detect(t *testing.T, band, song, fileID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	url := "/api/bands/" + band + "/songs/" + song + "/files/" + fileID + "/crop-detect"
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, url, bytes.NewReader(b)))
	return rec
}

func TestCropDetect_ProposesAndReturnsHiddenMarks(t *testing.T) {
	f := newDetectFixture(t, true)
	rec := f.detect(t, f.band, f.song, f.file, map[string]any{"all": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("detect: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Pages       map[string]app.CropRect `json:"pages"`
		HiddenMarks []app.HiddenMark        `json:"hiddenMarks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Pages["0"].Source != "auto" || out.Pages["0"].W <= 0 {
		t.Fatalf("want an auto proposal for page 0, got %+v", out.Pages)
	}
	if len(out.HiddenMarks) != 1 || out.HiddenMarks[0].UUID != "corner" {
		t.Fatalf("the corner mark must be reported hidden, got %+v", out.HiddenMarks)
	}
	// Read-only: the file's crop is untouched (no save, no rev, no sidecar — detection never writes).
	files, _ := f.svc.SongFiles(*f.caller, f.band, f.song)
	for _, sf := range files {
		if sf.ID == f.file && sf.Crop != nil {
			t.Fatalf("crop-detect must not persist a crop, got %+v", sf.Crop)
		}
	}
}

func TestCropDetect_503WithoutBaker(t *testing.T) {
	f := newDetectFixture(t, false) // no baker wired
	if rec := f.detect(t, f.band, f.song, f.file, map[string]any{"all": true}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no baker: %d %s, want 503", rec.Code, rec.Body)
	}
}

func TestCropDetect_RefusesGeneratedChart(t *testing.T) {
	f := newDetectFixture(t, true)
	if rec := f.detect(t, f.band, f.song, f.chart, map[string]any{"all": true}); rec.Code != http.StatusBadRequest {
		t.Fatalf("generated chart: %d %s, want 400 (P4)", rec.Code, rec.Body)
	}
}

func TestCropDetect_CrossBandAndNonMember(t *testing.T) {
	f := newDetectFixture(t, true)
	// Cross-band: the file is not in band2 → 404.
	if rec := f.detect(t, f.band2, f.song, f.file, map[string]any{"all": true}); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-band: %d %s, want 404", rec.Code, rec.Body)
	}
	// Non-member: a stranger (not in the band) → 403.
	stranger, err := f.svc.Register("mallory", "Mallory", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	*f.caller = stranger
	if rec := f.detect(t, f.band, f.song, f.file, map[string]any{"all": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-member: %d %s, want 403", rec.Code, rec.Body)
	}
}
