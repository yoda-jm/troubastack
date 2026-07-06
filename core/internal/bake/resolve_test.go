package bake

import (
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
)

func TestParseConcertID(t *testing.T) {
	if got := VariantConcertID("sl1", "u9"); got != "sl1~u9" {
		t.Fatalf("VariantConcertID = %q", got)
	}
	if base, user, v := ParseConcertID("sl1~u9"); !v || base != "sl1" || user != "u9" {
		t.Fatalf("ParseConcertID(variant) = %q,%q,%v", base, user, v)
	}
	if base, user, v := ParseConcertID("sl1"); v || base != "sl1" || user != "" {
		t.Fatalf("ParseConcertID(band) = %q,%q,%v", base, user, v)
	}
}

// TestResolveFile covers B07's per-song file resolution: a band bake picks the
// default (lowest-order PDF); a personal bake picks the first PDF in the
// member's my-files view; with no selection that view equals the default; and a
// PDF-less personal view falls back to the default so a personal bake never
// empties out.
func TestResolveFile(t *testing.T) {
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
	part, err := svc.UploadSongFile(admin, band.ID, song.ID, "part.pdf", "application/pdf", []byte("%PDF-1.4 part"))
	if err != nil {
		t.Fatalf("upload part: %v", err)
	}
	png, err := svc.UploadSongFile(admin, band.ID, song.ID, "notes.png", "image/png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	if err != nil {
		t.Fatalf("upload png: %v", err)
	}

	// Leo is a member (added directly — no invite flow needed here).
	leo, err := svc.Register("leo", "Leo", "password123", "")
	if err != nil {
		t.Fatalf("register leo: %v", err)
	}
	if err := repo.AddMembership(app.Membership{BandID: band.ID, UserID: leo.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("add leo: %v", err)
	}

	b := &Baker{svc: svc}
	mustResolve := func(who app.User, personal bool, wantID, label string) {
		t.Helper()
		f, ok, err := b.resolveFile(who, band.ID, song.ID, personal)
		if err != nil || !ok {
			t.Fatalf("%s: resolve ok=%v err=%v", label, ok, err)
		}
		if f.ID != wantID {
			t.Fatalf("%s: resolved %q, want %q", label, f.ID, wantID)
		}
	}

	// Band bake → default = lowest-order PDF (score).
	mustResolve(admin, false, score.ID, "band/default")

	// Personal, no selection → my-files view is the pool in order → first PDF = score.
	mustResolve(leo, true, score.ID, "personal/no-selection")

	// Personal, Leo curates [part, score] → first PDF = part (his tab).
	if _, err := svc.SetMyFileSelection(leo, band.ID, song.ID, []string{part.ID, score.ID}); err != nil {
		t.Fatalf("set selection: %v", err)
	}
	mustResolve(leo, true, part.ID, "personal/curated")

	// Personal, Leo curates a PDF-LESS view [png] → falls back to the default (score),
	// so a personal bake never ends up emptier than the band bake.
	if _, err := svc.SetMyFileSelection(leo, band.ID, song.ID, []string{png.ID}); err != nil {
		t.Fatalf("set png-only selection: %v", err)
	}
	mustResolve(leo, true, score.ID, "personal/pdf-less→fallback")
}
