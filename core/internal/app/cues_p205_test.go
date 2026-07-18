package app_test

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
)

// TestAllMemberCues is the P205 guard for the band-wide bake's cue source: an ADMIN
// gathers EVERY member's cues for a song, keyed by member id, in deterministic
// Members order; members with no cues are omitted; a non-admin cannot read others'
// cues (unlike self-only MyCues).
func TestAllMemberCues(t *testing.T) {
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
	song, err := svc.CreateSong(admin, band.ID, "Wonderwall", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
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
	moe := join("moe") // will have cues
	_ = join("kai")    // a member with NO cues → must be omitted

	if _, err := svc.SetMyCues(admin, band.ID, song.ID, []app.SongCue{{Icon: "baton"}}); err != nil {
		t.Fatalf("admin cues: %v", err)
	}
	if _, err := svc.SetMyCues(moe, band.ID, song.ID, []app.SongCue{{Icon: "mic"}, {Icon: "guitar-electric", Color: "#e11d48"}}); err != nil {
		t.Fatalf("moe cues: %v", err)
	}

	all, err := svc.AllMemberCues(admin, band.ID, song.ID)
	if err != nil {
		t.Fatalf("AllMemberCues: %v", err)
	}
	// Members order is oldest-join-first: admin, then moe, then kai. kai (no cues) omitted.
	if len(all) != 2 {
		t.Fatalf("want 2 members with cues (kai omitted), got %d: %+v", len(all), all)
	}
	if all[0].MemberID != admin.ID || len(all[0].Cues) != 1 || all[0].Cues[0].Icon != "baton" {
		t.Fatalf("entry[0] = %+v, want admin's [baton]", all[0])
	}
	if all[1].MemberID != moe.ID || len(all[1].Cues) != 2 ||
		all[1].Cues[0].Icon != "mic" || all[1].Cues[1].Color != "#e11d48" {
		t.Fatalf("entry[1] = %+v, want moe's [mic, guitar-electric#e11d48]", all[1])
	}

	// A non-admin cannot read the whole band's cues (privacy; admin-gated like the bake).
	if _, err := svc.AllMemberCues(moe, band.ID, song.ID); err != app.ErrForbidden {
		t.Fatalf("non-admin AllMemberCues err = %v, want ErrForbidden", err)
	}
}
