package app_test

import (
	"encoding/json"
	"testing"
)

// TestImport_GeneratedChartSizeMatchesStoredBlob is the T141 regression: when a folder declares a
// generated chart's `size` as its SOURCE length (what a hand-written / migrated folder does), the importer
// renders the source and stores the PDF — so the SongFile.Size must become the RENDERED length, not the
// declared source length. When they disagreed, downloadFile emitted a Content-Length below the payload and
// the viewer got a truncated response ("failed to fetch"). Teeth: source and render differ in length, so
// reverting the fix (Size stays the declared source length) turns this red.
func TestImport_GeneratedChartSizeMatchesStoredBlob(t *testing.T) {
	source := []byte("# My Chart\n## Verse\nC       G       Am      F\nla la la la la la\n## Chorus\nF       C       G\noh oh oh oh oh oh\n")
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Gen Band",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	// declare size = the SOURCE length, and NO blobHash (the reader fills it from the render) — exactly a
	// hand-authored / migrated folder.
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{"slug": "s1", "title": "S1",
			"files": []any{map[string]any{"filename": "chart.txt", "contentType": "application/pdf", "generated": true, "size": len(source)}}}},
	})
	zipBytes := rezip(t, map[string][]byte{
		"band.json": band, "repertoire.json": rep, "s1/chart.txt": source,
	})

	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	report, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	songs, _ := tgt.repo.SongsOfBand(report.Band.ID)
	checked := 0
	for _, sng := range songs {
		fs, _ := tgt.repo.FilesOfSong(sng.ID)
		for _, f := range fs {
			if !f.Generated {
				continue
			}
			data, gerr := tgt.blobs.Get(f.BlobHash)
			if gerr != nil {
				t.Fatalf("stored blob for %q missing: %v", f.Filename, gerr)
			}
			if f.Size != int64(len(data)) {
				t.Fatalf("generated %q: Size=%d but stored blob (rendered PDF) is %d bytes — Content-Length would truncate the response",
					f.Filename, f.Size, len(data))
			}
			// teeth: the declared source length differs from the render, so this fixture discriminates —
			// reverting the fix leaves Size at the source length and fails the assertion above.
			if int64(len(source)) == f.Size {
				t.Fatalf("teeth: source (%d) and rendered blob (%d) are the same length — use content where they differ", len(source), f.Size)
			}
			checked++
		}
	}
	if checked != 1 {
		t.Fatalf("checked %d generated charts, want 1 — fixture regression", checked)
	}
}
