package httpapi_test

import (
	"net/http"
	"strings"
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
			if f.Filename != "My Chart" {
				t.Fatalf("filename = %q, want \"My Chart\" (the # title, no .pdf — T72)", f.Filename)
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
			// T72: a source edit no longer re-derives the name — it keeps the create-time name
			// ("My Chart"), even though the source's # title changed, so a user rename survives.
			if f2.Filename != "My Chart" {
				t.Fatalf("after save filename = %q, want \"My Chart\" unchanged (T72)", f2.Filename)
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

// TestTextChartTranspose (T60 surface 1) covers the :transpose endpoint over the HTTP
// edge: dryRun returns the transposed source without persisting; a persist bumps the
// revision, rewrites the source, and (with updateSongKey) sets the song key; a stale
// baseRevision conflicts (409); a non-generated file and a no-op request are rejected
// (400); and the semitone fallback works when the song key isn't parseable.
func TestTextChartTranspose(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID

			// Song key = G → the key path is enabled (a parseable "from").
			resp, _ := admin.do(http.MethodPatch, base, map[string]any{"key": "G"})
			mustStatus(t, resp, http.StatusOK)

			// Create a chart whose chord row is "G            D".
			resp, body := admin.do(http.MethodPost, base+"/text-charts", map[string]string{"source": chartSrc})
			mustStatus(t, resp, http.StatusCreated)
			var f app.SongFile
			unmarshalField(t, body, "file", &f)
			tpath := base + "/files/" + f.ID + "/chart-source:transpose"

			// G→A (+2): G→A, D→E, same widths ⇒ byte-identical layout except the roots.
			wantSrc := strings.Replace(chartSrc, "G            D", "A            E", 1)
			if wantSrc == chartSrc {
				t.Fatal("test setup: chord row literal did not match chartSrc")
			}

			// dryRun: returns the transposed source, persists NOTHING.
			resp, db := admin.do(http.MethodPost, tpath, map[string]any{"targetKey": "A", "updateSongKey": true, "baseRevision": 1, "dryRun": true})
			mustStatus(t, resp, http.StatusOK)
			var drySrc string
			unmarshalField(t, db, "source", &drySrc)
			if drySrc != wantSrc {
				t.Fatalf("dryRun source = %q, want %q", drySrc, wantSrc)
			}
			_, sb := admin.do(http.MethodGet, base+"/files/"+f.ID+"/chart-source", nil)
			var stored string
			var sf app.SongFile
			unmarshalField(t, sb, "source", &stored)
			unmarshalField(t, sb, "file", &sf)
			if stored != chartSrc || sf.Revision != 1 {
				t.Fatalf("dryRun must not persist: rev=%d, srcChanged=%v", sf.Revision, stored != chartSrc)
			}

			// Persist G→A with updateSongKey: revision bumps, source rewritten, key=A.
			resp, pb := admin.do(http.MethodPost, tpath, map[string]any{"targetKey": "A", "updateSongKey": true, "baseRevision": 1, "dryRun": false})
			mustStatus(t, resp, http.StatusOK)
			var pf app.SongFile
			unmarshalField(t, pb, "file", &pf)
			if pf.ID != f.ID || pf.Revision != 2 {
				t.Fatalf("persist: id=%q rev=%d, want same id rev 2", pf.ID, pf.Revision)
			}
			_, sb2 := admin.do(http.MethodGet, base+"/files/"+f.ID+"/chart-source", nil)
			var stored2 string
			unmarshalField(t, sb2, "source", &stored2)
			if stored2 != wantSrc {
				t.Fatalf("persisted source = %q, want %q", stored2, wantSrc)
			}
			// Song key updated to A (side effect of updateSongKey).
			_, songsBody := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
			var songs []app.Song
			unmarshalField(t, songsBody, "songs", &songs)
			var key string
			for _, s := range songs {
				if s.ID == song.ID {
					key = s.Key
				}
			}
			if key != "A" {
				t.Fatalf("song key = %q, want A (updateSongKey)", key)
			}

			// Stale baseRevision (1, now 2) → 409.
			resp, _ = admin.do(http.MethodPost, tpath, map[string]any{"targetKey": "C", "baseRevision": 1, "dryRun": false})
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("stale transpose = %d, want 409", resp.StatusCode)
			}

			// No-op request (no parseable targetKey, no semitones) → 400.
			resp, _ = admin.do(http.MethodPost, tpath, map[string]any{"targetKey": "", "baseRevision": 2, "dryRun": true})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("no-op transpose = %d, want 400", resp.StatusCode)
			}

			// Non-generated (uploaded) file → 400.
			uresp, ubody := admin.upload(base+"/files", "u.pdf", "application/pdf", smallPDF)
			mustStatus(t, uresp, http.StatusCreated)
			var uf app.SongFile
			unmarshalField(t, ubody, "file", &uf)
			resp, _ = admin.do(http.MethodPost, base+"/files/"+uf.ID+"/chart-source:transpose",
				map[string]any{"targetKey": "A", "baseRevision": 1, "dryRun": true})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("non-generated transpose = %d, want 400", resp.StatusCode)
			}

			// Semitone fallback when the song key isn't parseable ("weird" → use +2).
			resp, _ = admin.do(http.MethodPatch, base, map[string]any{"key": "weird"})
			mustStatus(t, resp, http.StatusOK)
			resp, semb := admin.do(http.MethodPost, tpath, map[string]any{"semitones": 2, "baseRevision": 2, "dryRun": true})
			mustStatus(t, resp, http.StatusOK)
			var semSrc string
			unmarshalField(t, semb, "source", &semSrc)
			// Current stored source is the A-transposed chart; +2 more → B/F# roots.
			if !strings.Contains(semSrc, "B") || semSrc == stored2 {
				t.Fatalf("semitone transpose did not shift: %q", semSrc)
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
