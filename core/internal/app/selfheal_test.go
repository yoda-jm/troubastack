package app_test

import (
	"bytes"
	"testing"
)

// TestSelfHealGeneratedBlob (T69) covers the download-time recovery: a generated chart whose
// rendered PDF blob has vanished from the store (orphaned historical data) re-materializes
// from its stored source on the next download — no 404. An uploaded file whose blob is gone
// has no source to heal from, so it still errors (genuinely lost).
func TestSelfHealGeneratedBlob(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := st.svc.CreateSong(admin, band.ID, "Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}

	src := "# Heal Me\n\n## Verse 1\nG            D\na line to render\n"
	chart, err := st.svc.CreateTextChart(admin, band.ID, song.ID, src)
	if err != nil {
		t.Fatal(err)
	}
	// The good render, captured before we break the store.
	_, want, err := st.svc.DownloadSongFile(admin, chart.ID)
	if err != nil || len(want) == 0 {
		t.Fatalf("baseline download: err=%v len=%d", err, len(want))
	}

	// Simulate the orphan: delete the blob directly, leaving the file RECORD + chart source.
	if err := st.blobs.Delete(chart.BlobHash); err != nil {
		t.Fatalf("delete blob: %v", err)
	}
	if _, gerr := st.blobs.Get(chart.BlobHash); gerr == nil {
		t.Fatal("precondition: blob should be gone after Delete")
	}

	// Download now must SELF-HEAL (re-render from source), not 404.
	healedFile, healed, err := st.svc.DownloadSongFile(admin, chart.ID)
	if err != nil {
		t.Fatalf("download after orphan should self-heal, got err=%v", err)
	}
	if !bytes.Equal(healed, want) {
		t.Fatalf("healed bytes differ from the original render (deterministic Render expected)")
	}
	// The blob exists again under the record's (unchanged, deterministic) hash, and the
	// revision was NOT bumped (same logical content, just re-materialized).
	if _, gerr := st.blobs.Get(healedFile.BlobHash); gerr != nil {
		t.Fatalf("blob not restored: %v", gerr)
	}
	if healedFile.Revision != chart.Revision {
		t.Fatalf("revision bumped on heal: %d → %d (must be unchanged)", chart.Revision, healedFile.Revision)
	}
	// A second download now serves from the restored blob (no re-heal needed).
	if _, again, err := st.svc.DownloadSongFile(admin, chart.ID); err != nil || !bytes.Equal(again, want) {
		t.Fatalf("second download after heal: err=%v equal=%v", err, bytes.Equal(again, want))
	}

	// An UPLOADED file whose blob is gone has no source → still an error (nothing to heal).
	up, err := st.svc.UploadSongFile(admin, band.ID, song.ID, "u.pdf", "application/pdf", []byte("%PDF-1.4 uploaded"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.blobs.Delete(up.BlobHash); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.svc.DownloadSongFile(admin, up.ID); err == nil {
		t.Fatal("uploaded file with a missing blob must NOT heal (bytes are genuinely lost)")
	}
}

// TestRepairMissingBlobs (T69) covers the one-pass operator repair: a store with a healthy
// chart, an orphaned chart (re-rendered from source), and an orphaned uploaded file
// (unrecoverable) → RepairMissingBlobs heals the chart, leaves the healthy one, and reports
// the uploaded casualty; the healed chart downloads afterwards.
func TestRepairMissingBlobs(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := st.svc.CreateSong(admin, band.ID, "Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := st.svc.CreateTextChart(admin, band.ID, song.ID, "# Healthy\n\n## A\nkeep me\n")
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := st.svc.CreateTextChart(admin, band.ID, song.ID, "# Orphan\n\n## B\nheal me\n")
	if err != nil {
		t.Fatal(err)
	}
	up, err := st.svc.UploadSongFile(admin, band.ID, song.ID, "u.pdf", "application/pdf", []byte("%PDF-1.4 lost"))
	if err != nil {
		t.Fatal(err)
	}
	// Orphan the chart + the upload (distinct sources → distinct blobs; healthy untouched).
	_ = st.blobs.Delete(orphan.BlobHash)
	_ = st.blobs.Delete(up.BlobHash)

	rep, err := st.svc.RepairMissingBlobs()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Scanned != 3 || rep.Healthy != 1 {
		t.Fatalf("report: scanned=%d healthy=%d, want 3/1", rep.Scanned, rep.Healthy)
	}
	if len(rep.Healed) != 1 || rep.Healed[0] != orphan.ID {
		t.Fatalf("healed=%v, want [%s]", rep.Healed, orphan.ID)
	}
	if len(rep.Unfixable) != 1 || rep.Unfixable[0].ID != up.ID {
		t.Fatalf("unfixable=%v, want the uploaded file %s", rep.Unfixable, up.ID)
	}
	// The healed chart downloads now; the healthy one is undisturbed.
	if _, _, err := st.svc.DownloadSongFile(admin, orphan.ID); err != nil {
		t.Fatalf("healed chart not downloadable: %v", err)
	}
	if _, _, err := st.svc.DownloadSongFile(admin, healthy.ID); err != nil {
		t.Fatalf("healthy chart disturbed: %v", err)
	}
}
