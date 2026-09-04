package app_test

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
)

// TestAllFileSelections is the T137 guard for the band-wide bake's per-member SEQUENCE source: an ADMIN
// gathers EVERY member's ordered file selection for a song, keyed by member id, in deterministic Members
// order; members with no selection are omitted; a non-admin cannot read others' selections.
func TestAllFileSelections(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())

	admin, err := svc.Register("amy", "Amy", "pass1234", "")
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	band, err := svc.CreateBand(admin, "The Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	song, err := svc.CreateSong(admin, band.ID, "The Open Road", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	fileA, err := svc.UploadSongFile(admin, band.ID, song.ID, "a.pdf", "application/pdf", []byte("%PDF-1.4 A"))
	if err != nil {
		t.Fatalf("upload a: %v", err)
	}
	fileB, err := svc.UploadSongFile(admin, band.ID, song.ID, "b.pdf", "application/pdf", []byte("%PDF-1.4 B"))
	if err != nil {
		t.Fatalf("upload b: %v", err)
	}

	join := func(username string) app.User {
		u, err := svc.Register(username, username, "pass1234", "")
		if err != nil {
			t.Fatalf("register %s: %v", username, err)
		}
		inv, err := svc.Invite(admin, band.ID, u.Username, app.KindUsername)
		if err != nil {
			t.Fatalf("invite %s: %v", username, err)
		}
		if _, err := svc.AcceptInvite(u, inv.ID); err != nil {
			t.Fatalf("accept %s: %v", username, err)
		}
		return u
	}
	moe := join("moe") // will have a selection
	_ = join("kai")    // a member with NO selection → must be omitted

	if _, err := svc.SetMyFileSelection(admin, band.ID, song.ID, []string{fileB.ID, fileA.ID}); err != nil {
		t.Fatalf("admin selection: %v", err)
	}
	if _, err := svc.SetMyFileSelection(moe, band.ID, song.ID, []string{fileA.ID}); err != nil {
		t.Fatalf("moe selection: %v", err)
	}

	all, err := svc.AllFileSelections(admin, band.ID, song.ID)
	if err != nil {
		t.Fatalf("AllFileSelections: %v", err)
	}
	// Members order is oldest-join-first: admin, then moe, then kai. kai (no selection) omitted.
	if len(all) != 2 {
		t.Fatalf("got %d selections, want 2 (kai omitted)", len(all))
	}
	if all[0].MemberID != admin.ID || len(all[0].FileIDs) != 2 || all[0].FileIDs[0] != fileB.ID || all[0].FileIDs[1] != fileA.ID {
		t.Fatalf("admin entry wrong (order must be preserved): %+v", all[0])
	}
	if all[1].MemberID != moe.ID || len(all[1].FileIDs) != 1 || all[1].FileIDs[0] != fileA.ID {
		t.Fatalf("moe entry wrong: %+v", all[1])
	}

	// A non-admin cannot read other members' selections (admin-gated, like AllMemberCues).
	if _, err := svc.AllFileSelections(moe, band.ID, song.ID); err == nil {
		t.Fatalf("non-admin AllFileSelections should be refused")
	}
}
