package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// cropFixture mounts the crop route with a notify spy and a one-file song whose engine HEAD carries two
// marks on page 0: one inside a centre crop, one in the far corner that any centre crop hides.
type cropFixture struct {
	svc         *app.Service
	admin       app.User
	mux         *http.ServeMux
	band, song  string
	file, chart string
	notified    *[]string
}

// fileCrop returns the stored crop of the fixture's uploaded file (nil when unset), for persistence checks.
func (f *cropFixture) fileCrop(t *testing.T) *app.Crop {
	t.Helper()
	files, err := f.svc.SongFiles(f.admin, f.band, f.song)
	if err != nil {
		t.Fatal(err)
	}
	for _, sf := range files {
		if sf.ID == f.file {
			return sf.Crop
		}
	}
	t.Fatal("uploaded file not found")
	return nil
}

func newCropFixture(t *testing.T) *cropFixture {
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
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: file.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	mark := func(uuid string, x, y float64) {
		o := domain.Object{UUID: uuid, LayerID: "L1", Page: 0, Type: domain.TypeRect, Version: 1,
			Points: []domain.Point{{X: x, Y: y}, {X: x + 0.02, Y: y + 0.02}}, Style: domain.Style{Width: 0.01}}
		if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: uuid, Object: &o, AuthorID: admin.ID}); err != nil {
			t.Fatal(err)
		}
	}
	mark("inside", 0.45, 0.45)
	mark("corner", 0.95, 0.95)

	notified := &[]string{}
	mux := http.NewServeMux()
	NewAnnotationsAPI(svc, eng, func(songID string) { *notified = append(*notified, songID) }).
		Mount(mux, func(h authedHandler) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) { h(w, r, admin) }
		})
	return &cropFixture{svc: svc, admin: admin, mux: mux, band: band.ID, song: song.ID, file: file.ID, chart: chart.ID, notified: notified}
}

func (f *cropFixture) put(t *testing.T, fileID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	url := "/api/bands/" + f.band + "/songs/" + f.song + "/files/" + fileID + "/crop"
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, url, bytes.NewReader(b)))
	return rec
}

// centreCrop hides the far corner but not the centre mark.
var centreCrop = map[string]any{"pages": map[string]any{"0": map[string]float64{"x": 0.25, "y": 0.25, "w": 0.5, "h": 0.5}}}

func TestCropEndpoint_DryRunListsHiddenMarksWithoutPersisting(t *testing.T) {
	f := newCropFixture(t)
	rec := f.put(t, f.file, map[string]any{"crop": centreCrop, "dryRun": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("dryRun: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		HiddenMarks []app.HiddenMark `json:"hiddenMarks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.HiddenMarks) != 1 || out.HiddenMarks[0].UUID != "corner" {
		t.Fatalf("hiddenMarks = %+v, want just [corner]", out.HiddenMarks)
	}
	if len(*f.notified) != 0 {
		t.Fatalf("a dry run must not re-bake, notified=%v", *f.notified)
	}
	if c := f.fileCrop(t); c != nil {
		t.Fatalf("a dry run must persist nothing, but the file now has a crop: %+v", c)
	}
}

func TestCropEndpoint_SavePersistsAndNotifies(t *testing.T) {
	f := newCropFixture(t)
	rec := f.put(t, f.file, map[string]any{"crop": centreCrop})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		File app.SongFile `json:"file"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.File.Crop == nil || !out.File.Crop.LockAR || out.File.Crop.Pages["0"].Source != "manual" {
		t.Fatalf("crop not persisted as expected: %+v", out.File.Crop)
	}
	if len(*f.notified) != 1 || (*f.notified)[0] != f.song {
		t.Fatalf("a save must re-bake the song once, notified=%v", *f.notified)
	}

	// Reset: an empty crop clears it and still fires the autobaker (the page reverts).
	rec = f.put(t, f.file, map[string]any{"crop": map[string]any{}})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	out.File = app.SongFile{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.File.Crop != nil {
		t.Fatalf("reset should clear the crop, got %+v", out.File.Crop)
	}
	if len(*f.notified) != 2 {
		t.Fatalf("reset must re-bake too, notified=%v", *f.notified)
	}
}

func TestCropEndpoint_RefusesGeneratedAndInvalid(t *testing.T) {
	f := newCropFixture(t)
	// P4: a generated chart has no crop — 400 on both the save and the dry run (they share the gate).
	for _, dry := range []bool{false, true} {
		rec := f.put(t, f.chart, map[string]any{"crop": centreCrop, "dryRun": dry})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("generated chart (dryRun=%v): %d %s, want 400", dry, rec.Code, rec.Body)
		}
	}
	// Out-of-page geometry is 400.
	bad := map[string]any{"pages": map[string]any{"0": map[string]float64{"x": 0.9, "y": 0, "w": 0.8, "h": 0.8}}}
	if rec := f.put(t, f.file, map[string]any{"crop": bad}); rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-page crop: %d %s, want 400", rec.Code, rec.Body)
	}
	if len(*f.notified) != 0 {
		t.Fatalf("no refused request may re-bake, notified=%v", *f.notified)
	}
}
