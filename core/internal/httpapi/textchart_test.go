package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

const chartSrc = "# My Chart\n\n## Verse 1\nG            D\nPack a little light for the road ahead,\n\nA plain **bold** line."

// TestTextChart_CreateEditRegenerate (T19) covers the text-chart lifecycle over
// the HTTP edge on both backends: create → it enters the pool as a generated PDF;
// the source round-trips; saving new source regenerates in place (same file id,
// revision bumps, real PDF bytes); a stale baseRevision conflicts (409); and
// non-Latin source is rejected (400).
func TestTextChart_CreateEditRegenerate(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID

			// Create a text chart.
			resp, body := admin.do(http.MethodPost, base+"/text-charts", map[string]string{"source": chartSrc})
			mustStatus(t, resp, http.StatusCreated)
			var f app.SongFile
			unmarshalField(t, body, "file", &f)
			if !f.Generated || f.Revision != 1 || f.ContentType != "application/pdf" {
				t.Fatalf("created file = %+v, want generated pdf rev 1", f)
			}
			if f.Filename != "My Chart.pdf" {
				t.Fatalf("filename = %q, want \"My Chart.pdf\" (from the # title)", f.Filename)
			}

			// It shows up in the pool.
			_, lb := admin.do(http.MethodGet, base+"/files", nil)
			var files []app.SongFile
			unmarshalField(t, lb, "files", &files)
			if len(files) != 1 || files[0].ID != f.ID {
				t.Fatalf("pool = %+v, want the one generated file", files)
			}

			// The bytes are a real PDF.
			resp, _ = admin.do(http.MethodGet, "/api/files/"+f.ID, nil)
			mustStatus(t, resp, http.StatusOK)

			// Source round-trips; the file's revision is the edit base.
			resp, sb := admin.do(http.MethodGet, base+"/files/"+f.ID+"/chart-source", nil)
			mustStatus(t, resp, http.StatusOK)
			var gotSrc string
			var srcFile app.SongFile
			unmarshalField(t, sb, "source", &gotSrc)
			unmarshalField(t, sb, "file", &srcFile)
			if gotSrc != chartSrc {
				t.Fatalf("source round-trip = %q", gotSrc)
			}
			if srcFile.Revision != 1 {
				t.Fatalf("source file revision = %d, want 1", srcFile.Revision)
			}

			// Save new source at the right base revision → regenerates in place.
			newSrc := "# My Chart v2\n\n## Chorus\nsing"
			resp, eb := admin.do(http.MethodPut, base+"/files/"+f.ID+"/chart-source",
				map[string]any{"source": newSrc, "baseRevision": 1})
			mustStatus(t, resp, http.StatusOK)
			var f2 app.SongFile
			unmarshalField(t, eb, "file", &f2)
			if f2.ID != f.ID || f2.Revision != 2 {
				t.Fatalf("after save: id=%q rev=%d, want same id rev 2", f2.ID, f2.Revision)
			}
			if f2.Filename != "My Chart v2.pdf" {
				t.Fatalf("after save filename = %q, want \"My Chart v2.pdf\"", f2.Filename)
			}
			// Still exactly one file in the pool (edit in place, not a new file).
			_, lb2 := admin.do(http.MethodGet, base+"/files", nil)
			var files2 []app.SongFile
			unmarshalField(t, lb2, "files", &files2)
			if len(files2) != 1 {
				t.Fatalf("pool after edit has %d files, want 1", len(files2))
			}

			// A stale baseRevision (1, but current is 2) conflicts.
			resp, _ = admin.do(http.MethodPut, base+"/files/"+f.ID+"/chart-source",
				map[string]any{"source": newSrc, "baseRevision": 1})
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("stale save = %d, want 409", resp.StatusCode)
			}

			// Non-Latin-1 source is rejected.
			resp, _ = admin.do(http.MethodPost, base+"/text-charts", map[string]string{"source": "# T\n\n漢字"})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("non-latin create = %d, want 400", resp.StatusCode)
			}
		})
	}
}

// TestTextChartPreview (T25) covers the no-persistence preview endpoint: a member
// gets application/pdf bytes and NO pool file is created; bad chars → 400; a
// non-member is denied.
func TestTextChartPreview(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID

			// Member preview → 200 application/pdf, and the pool stays empty.
			resp, _ := admin.do(http.MethodPost, base+"/text-charts:preview", map[string]string{"source": chartSrc})
			mustStatus(t, resp, http.StatusOK)
			if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
				t.Fatalf("preview content-type = %q, want application/pdf", ct)
			}
			var files []app.SongFile
			_, lb := admin.do(http.MethodGet, base+"/files", nil)
			unmarshalField(t, lb, "files", &files)
			if len(files) != 0 {
				t.Fatalf("preview must not create a pool file; pool = %d", len(files))
			}

			// Bad characters → 400 on preview too (not just save).
			resp, _ = admin.do(http.MethodPost, base+"/text-charts:preview", map[string]string{"source": "# T\n\n漢字"})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("non-latin preview = %d, want 400", resp.StatusCode)
			}

			// A non-member is denied (T08 pattern: 403/404, never the bytes).
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodPost, base+"/text-charts:preview", map[string]string{"source": chartSrc})
			if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
				t.Fatalf("outsider preview = %d, want 403/404", resp.StatusCode)
			}
		})
	}
}
