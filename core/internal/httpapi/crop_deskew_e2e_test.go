package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// deskewE2EPDF is a non-square page (150×233 pt). pdftoppm renders it 313×486 px at 150 dpi (ceil rounding),
// so the cropped-raster ("pageSizes") ratio is 313/486 = 0.644033, while the MediaBox aspect pdfinfo reports —
// the one the deskew transform must use — is 150/233 = 0.643777. The two are sub-pixel apart BY CONSTRUCTION
// (ceil caps the gap at ≤1px), which is exactly why P2 below is caught through an INJECTED distinctive aspect,
// not geometrically: a mark can't be placed reliably inside a ~6e-6 boundary sliver.
func deskewE2EPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 150 233]/Contents 4 0 R>>endobj\n" +
		"4 0 obj<</Length 10>>stream\n0 0 1 1 re\nendstream endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
}

// TestCropDeskewDryRunEqualsBakeDrop is T199 B1: the REAL crop-save dry-run handler (→ cropAspects → the one
// aspect source) and the REAL bake (→ stageFile → the same source) name the SAME marks. A grid of marks
// straddles a rotated window; the dry-run's hidden set equals what the bake drops. It runs twice:
//   - REAL pdfinfo: proves the live path agrees (and prints both ratios, MediaBox vs pageSizes);
//   - an INJECTED distinctive aspect (1.6) through both real paths: this is the scenario where the two
//     production sabotages red — P1 (cropAspects returns asp×1.01) and P2 (stageFile takes the aspect from the
//     cropped pageSizes ≈0.644 instead of the source) — because 1.6 then diverges hugely on one side.
func TestCropDeskewDryRunEqualsBakeDrop(t *testing.T) {
	svc := app.NewService(memrepo.New()).WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, err := svc.Register("owner", "Owner", "password123", "")
	must(t, err)
	band, err := svc.CreateBand(admin, "Band")
	must(t, err)
	song, err := svc.CreateSong(admin, band.ID, "Crooked", "")
	must(t, err)
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "s.pdf", "application/pdf", deskewE2EPDF())
	must(t, err)
	_, err = eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: file.ID, Name: "M", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW}})
	must(t, err)

	x, y, w, h, angle := 0.22, 0.17, 0.5, 0.5, 10.0
	all := map[string]bool{}
	mark := func(uuid string, px, py float64) {
		o := domain.Object{UUID: uuid, LayerID: "L1", Page: 0, Type: domain.TypeRect, Version: 1,
			Points: []domain.Point{{X: px, Y: py}}, Style: domain.Style{Width: 0}}
		_, e := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: uuid, Object: &o, AuthorID: admin.ID})
		must(t, e)
		all[uuid] = true
	}
	for i := 1; i <= 60; i++ {
		for j := 1; j <= 60; j++ {
			mark(fmt.Sprintf("g%d_%d", i, j), float64(i)/61, float64(j)/61)
		}
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	must(t, err)
	_, err = svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID)
	must(t, err)
	_, err = svc.SetFileCrop(admin, band.ID, song.ID, file.ID, app.Crop{Pages: map[string]app.CropRect{"0": {X: x, Y: y, W: w, H: h, Angle: angle, Source: "manual"}}})
	must(t, err)

	// run builds a baker (real pdfinfo, or an injected fixed aspect), runs the real dry-run handler and the
	// real bake, and returns (dry-run hidden, bake dropped). The bogus node fails the overlay render AFTER
	// stageFile fires OnStagedMarks, so the drop is still observed.
	run := func(inject []float64) (hidden, dropped map[string]bool) {
		surviving := map[string]bool{}
		cfg := bake.Config{BakesDir: t.TempDir(), Pdftoppm: "pdftoppm", Pdfimages: "pdfimages", Pdfinfo: "pdfinfo",
			Node: "/nonexistent-node", BakeCLI: "/nonexistent-cli",
			OnStagedMarks: func(_ string, uuids []string) {
				for _, u := range uuids {
					surviving[u] = true
				}
			}}
		if inject != nil {
			cfg.Aspecter = bake.FixedAspecter{Aspects: inject}
		}
		b := bake.New(svc, eng, cfg)
		mux := http.NewServeMux()
		NewAnnotationsAPI(svc, eng, func(string) {}, b).Mount(mux, func(hdl authedHandler) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) { hdl(w, r, admin) }
		})
		body, _ := json.Marshal(map[string]any{"dryRun": true, "crop": map[string]any{
			"pages": map[string]any{"0": map[string]float64{"x": x, "y": y, "w": w, "h": h, "angle": angle}}}})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/bands/"+band.ID+"/songs/"+song.ID+"/files/"+file.ID+"/crop", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("dryRun: %d %s", rec.Code, rec.Body)
		}
		var out struct {
			HiddenMarks []app.HiddenMark `json:"hiddenMarks"`
		}
		must(t, json.Unmarshal(rec.Body.Bytes(), &out))
		hidden = map[string]bool{}
		for _, hm := range out.HiddenMarks {
			hidden[hm.UUID] = true
		}
		_, _, _ = b.Bake(context.Background(), band.ID, sl.ID, admin, nil, "")
		if len(surviving) == 0 {
			t.Fatal("OnStagedMarks never fired — can't observe the bake drop")
		}
		dropped = map[string]bool{}
		for u := range all {
			if !surviving[u] {
				dropped[u] = true
			}
		}
		return hidden, dropped
	}
	assertEqual := func(label string, hidden, dropped map[string]bool) {
		if ho, do := onlyIn(hidden, dropped), onlyIn(dropped, hidden); len(ho) != 0 || len(do) != 0 {
			t.Errorf("%s: dry run hidden != bake dropped — hidden-only=%v dropped-only=%v", label, ho, do)
		}
	}

	// Scenario A — real pdfinfo. Prove the live path agrees; print both ratios for the record.
	hA, dA := run(nil)
	t.Logf("real pdfinfo: MediaBox aspect=%.6f  pageSizes aspect=%.6f  hidden=%d dropped=%d", 150.0/233, 313.0/486, len(hA), len(dA))
	assertEqual("real pdfinfo", hA, dA)

	// Scenario B — an injected distinctive aspect (1.6) through both real pickers. This is where P1 and P2 red
	// (P2's pageSizes ≈0.644 then diverges hugely from 1.6; P1's ×1.01 shifts the grid). Both must agree here.
	hB, dB := run([]float64{1.6})
	t.Logf("injected aspect 1.6: hidden=%d dropped=%d", len(hB), len(dB))
	assertEqual("injected 1.6", hB, dB)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func onlyIn(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	return out
}
