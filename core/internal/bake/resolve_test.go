package bake

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
)

// TestParseConcertID: the band concert id is the setlist id; old `<setlist>~<user>`
// variant ids still parse (read-compat for concerts baked before the scope=mine
// retirement — no new ones are minted).
func TestParseConcertID(t *testing.T) {
	if base, user, v := ParseConcertID("sl1~u9"); !v || base != "sl1" || user != "u9" {
		t.Fatalf("ParseConcertID(variant) = %q,%q,%v", base, user, v)
	}
	if base, user, v := ParseConcertID("sl1"); v || base != "sl1" || user != "" {
		t.Fatalf("ParseConcertID(band) = %q,%q,%v", base, user, v)
	}
}

// TestDefaultFile covers the band bake's per-song file pick: the lowest-DisplayOrder
// viewable PDF (the same file Studio opens by default). Non-PDF files are skipped.
func TestDefaultFile(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo)
	svc.WithBlobStore(blob.NewMem())

	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatalf("band: %v", err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatalf("song: %v", err)
	}
	// Pool: score.pdf (order 0), part.pdf (order 1), notes.png (order 2).
	score, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 score"))
	if err != nil {
		t.Fatalf("upload score: %v", err)
	}
	if _, err := svc.UploadSongFile(admin, band.ID, song.ID, "part.pdf", "application/pdf", []byte("%PDF-1.4 part")); err != nil {
		t.Fatalf("upload part: %v", err)
	}
	if _, err := svc.UploadSongFile(admin, band.ID, song.ID, "notes.png", "image/png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")); err != nil {
		t.Fatalf("upload png: %v", err)
	}

	b := &Baker{svc: svc}
	f, ok, err := b.defaultFile(admin, band.ID, song.ID)
	if err != nil || !ok {
		t.Fatalf("defaultFile ok=%v err=%v", ok, err)
	}
	if f.ID != score.ID {
		t.Fatalf("defaultFile = %q, want the lowest-order PDF (score)", f.ID)
	}
}
