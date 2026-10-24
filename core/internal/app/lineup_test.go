package app_test

import (
	"errors"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

func sp(s string) *string { return &s }

// setLineup PATCHes a song's line-up as the given caller.
func setLineup(t *testing.T, st stack, caller app.User, bandID, songID string, l []app.LineupEntry) {
	t.Helper()
	if _, err := st.svc.UpdateSong(caller, bandID, songID, app.SongPatch{Lineup: &l}); err != nil {
		t.Fatalf("set lineup: %v", err)
	}
}

func songByID(t *testing.T, st stack, caller app.User, bandID, songID string) app.Song {
	t.Helper()
	songs, err := st.svc.Songs(caller, bandID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range songs {
		if s.ID == songID {
			return s
		}
	}
	t.Fatalf("song %s not found", songID)
	return app.Song{}
}

// TestLineup_CRUDAndGates: the roster + part metadata are admin; a member may create a part + edit a line-up.
func TestLineup_CRUDAndGates(t *testing.T) {
	st := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, st)

	// A member cannot add a roster musician.
	if _, err := st.svc.AddMusician(member, bandID, "Guest Perc"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member AddMusician: got %v, want ErrForbidden", err)
	}
	guest, err := st.svc.AddMusician(admin, bandID, "Guest Perc")
	if err != nil {
		t.Fatalf("admin AddMusician: %v", err)
	}

	// A member MAY create a part (defaults to a personal instrument).
	part, err := st.svc.CreatePart(member, bandID, "Autoharp", app.PartSpec{})
	if err != nil {
		t.Fatalf("member CreatePart: %v", err)
	}
	if part.Kind != app.PartInstrument || part.Shared {
		t.Errorf("a member-created part should default to a personal instrument, got kind=%q shared=%v", part.Kind, part.Shared)
	}
	// ...but not edit its metadata.
	if _, err := st.svc.UpdatePart(member, bandID, part.ID, app.PartPatch{Kind: sp(app.PartVoice)}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member UpdatePart: got %v, want ErrForbidden", err)
	}
	if _, err := st.svc.UpdatePart(admin, bandID, part.ID, app.PartPatch{Input: sp(app.InputDI)}); err != nil {
		t.Fatalf("admin UpdatePart: %v", err)
	}

	// A member edits a line-up (like other song metadata).
	setLineup(t, st, member, bandID, songID, []app.LineupEntry{{MusicianID: guest.ID, PartIDs: []string{part.ID}}})
	if got := songByID(t, st, member, bandID, songID).Lineup; len(got) != 1 || got[0].MusicianID != guest.ID {
		t.Fatalf("lineup not saved: %+v", got)
	}
}

// TestParts_RenameIsMetadataOnly (M1): renaming a part touches only the Part; a line-up referencing its id is
// unchanged (no band-wide rewrite, so no rename-vs-PATCH race).
func TestParts_RenameIsMetadataOnly(t *testing.T) {
	st := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, st)
	guest, _ := st.svc.AddMusician(admin, bandID, "G")
	part, _ := st.svc.CreatePart(admin, bandID, "Gtr", app.PartSpec{})
	setLineup(t, st, admin, bandID, songID, []app.LineupEntry{{MusicianID: guest.ID, PartIDs: []string{part.ID}}})

	if _, err := st.svc.UpdatePart(admin, bandID, part.ID, app.PartPatch{Name: sp("Guitar")}); err != nil {
		t.Fatalf("rename: %v", err)
	}
	// The line-up still references the SAME id (unchanged), and the part's name changed.
	lu := songByID(t, st, admin, bandID, songID).Lineup
	if len(lu) != 1 || len(lu[0].PartIDs) != 1 || lu[0].PartIDs[0] != part.ID {
		t.Fatalf("rename rewrote the line-up (it must not): %+v", lu)
	}
	parts, _ := st.svc.Parts(admin, bandID)
	if len(parts) != 1 || parts[0].Name != "Guitar" {
		t.Fatalf("rename did not update the part name: %+v", parts)
	}
}

// TestParts_MergeUnions: merge rewrites line-up part ids from→to, unioned per entry, and deletes `from`.
func TestParts_MergeUnions(t *testing.T) {
	st := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, st)
	guest, _ := st.svc.AddMusician(admin, bandID, "G")
	a, _ := st.svc.CreatePart(admin, bandID, "Acoustic guitar", app.PartSpec{})
	b, _ := st.svc.CreatePart(admin, bandID, "Guitar", app.PartSpec{})
	// One entry already holds both — the merge must dedupe to one id.
	setLineup(t, st, admin, bandID, songID, []app.LineupEntry{{MusicianID: guest.ID, PartIDs: []string{a.ID, b.ID}}})

	if err := st.svc.MergeParts(admin, bandID, a.ID, b.ID); err != nil {
		t.Fatalf("merge: %v", err)
	}
	lu := songByID(t, st, admin, bandID, songID).Lineup
	if len(lu) != 1 || len(lu[0].PartIDs) != 1 || lu[0].PartIDs[0] != b.ID {
		t.Fatalf("merge did not union to a single id: %+v", lu)
	}
	if parts, _ := st.svc.Parts(admin, bandID); len(parts) != 1 || parts[0].ID != b.ID {
		t.Fatalf("merge did not delete the from-part: %+v", parts)
	}
}

// TestMusicians_TwoGuestsSameName: a name-derived id would collide; guests get distinct random ids (M2).
func TestMusicians_TwoGuestsSameName(t *testing.T) {
	st := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, st)
	g1, _ := st.svc.AddMusician(admin, bandID, "Sam")
	g2, _ := st.svc.AddMusician(admin, bandID, "Sam")
	if g1.ID == g2.ID {
		t.Fatalf("two same-named guests must be two musicians, got the same id %s", g1.ID)
	}
}

// TestLinkMusician_BothBranches (M2): link a guest to an account — keep the guest's id when the account is
// unused; merge + delete the guest when the account already has line-ups.
func TestLinkMusician_BothBranches(t *testing.T) {
	// (a) account unused → the guest keeps its id and takes the userId.
	st := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, st)
	part, _ := st.svc.CreatePart(admin, bandID, "Bass", app.PartSpec{})
	guest, _ := st.svc.AddMusician(admin, bandID, "Dep")
	setLineup(t, st, admin, bandID, songID, []app.LineupEntry{{MusicianID: guest.ID, PartIDs: []string{part.ID}}})
	linked, err := st.svc.LinkMusician(admin, bandID, guest.ID, member.ID)
	if err != nil {
		t.Fatalf("link (unused): %v", err)
	}
	if linked.ID != guest.ID || linked.UserID != member.ID {
		t.Fatalf("link should keep the guest id + set userId, got %+v", linked)
	}
	if lu := songByID(t, st, admin, bandID, songID).Lineup; len(lu) != 1 || lu[0].MusicianID != guest.ID {
		t.Fatalf("the guest's line-up should be intact after linking: %+v", lu)
	}

	// (b) account already used → merge the guest's entries onto the account musician and drop the guest.
	st2 := newStack()
	admin2, member2, bandID2, songID2, _, _ := buildSourceBand(t, st2)
	// Same song, DIFFERENT parts + notes: the account sings backing (BV), the guest plays guitar (Gtr). The
	// merge must UNION the parts into one account entry and keep both notes — red if the union is dropped.
	bv, _ := st2.svc.CreatePart(admin2, bandID2, "Backing vocal", app.PartSpec{})
	gtr, _ := st2.svc.CreatePart(admin2, bandID2, "Gtr", app.PartSpec{})
	acctID := app.AccountMusicianID(bandID2, member2.ID)
	g2, _ := st2.svc.AddMusician(admin2, bandID2, "Dep2")
	setLineup(t, st2, admin2, bandID2, songID2, []app.LineupEntry{
		{MusicianID: acctID, PartIDs: []string{bv.ID}, Note: "harmonies"},
		{MusicianID: g2.ID, PartIDs: []string{gtr.ID}, Note: "sits in"},
	})
	if _, err := st2.svc.LinkMusician(admin2, bandID2, g2.ID, member2.ID); err != nil {
		t.Fatalf("link (used): %v", err)
	}
	lu := songByID(t, st2, admin2, bandID2, songID2).Lineup
	var acct *app.LineupEntry
	for i := range lu {
		if lu[i].MusicianID == g2.ID {
			t.Fatalf("the guest should be merged away, still present: %+v", lu)
		}
		if lu[i].MusicianID == acctID {
			acct = &lu[i]
		}
	}
	if acct == nil {
		t.Fatalf("the account entry vanished: %+v", lu)
	}
	if !containsID(acct.PartIDs, bv.ID) || !containsID(acct.PartIDs, gtr.ID) {
		t.Fatalf("the merge must UNION both parts into one entry, got %+v", acct.PartIDs)
	}
	if acct.Note != "harmonies / sits in" {
		t.Fatalf("the merge must join both notes, got %q", acct.Note)
	}
	ms, _ := st2.svc.Musicians(admin2, bandID2)
	for _, m := range ms {
		if m.ID == g2.ID {
			t.Fatalf("the guest record should be deleted after a merge: %+v", ms)
		}
	}
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestDepartedMemberLineupSurvives: a member referenced in a line-up is materialized as a stored record, so
// after they leave the band the record (and the line-up) survive.
func TestDepartedMemberLineupSurvives(t *testing.T) {
	st := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, st)
	part, _ := st.svc.CreatePart(admin, bandID, "Bass", app.PartSpec{})
	acctID := app.AccountMusicianID(bandID, member.ID)
	setLineup(t, st, admin, bandID, songID, []app.LineupEntry{{MusicianID: acctID, PartIDs: []string{part.ID}}})

	if err := st.svc.RemoveMember(admin, bandID, member.ID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	// The line-up entry survives on the song...
	if lu := songByID(t, st, admin, bandID, songID).Lineup; len(lu) != 1 || lu[0].MusicianID != acctID {
		t.Fatalf("a departed member's line-up entry must survive: %+v", lu)
	}
	// ...and the musician still resolves in the roster (as a former member, not a current one).
	ms, _ := st.svc.Musicians(admin, bandID)
	found := false
	for _, m := range ms {
		if m.ID == acctID {
			found = true
			if m.IsMember {
				t.Errorf("a departed member should not read as a current member")
			}
		}
	}
	if !found {
		t.Fatalf("the departed member's musician must still resolve: %+v", ms)
	}
}

// TestLineup_TbandRoundTrip: musicians, parts, a song's line-up + sound note survive a .tband export+import.
func TestLineup_TbandRoundTrip(t *testing.T) {
	src := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, src)
	guest, _ := src.svc.AddMusician(admin, bandID, "Percussionist")
	part, _ := src.svc.CreatePart(admin, bandID, "Bongos", app.PartSpec{})
	if _, err := src.svc.UpdatePart(admin, bandID, part.ID, app.PartPatch{Short: sp("Bgo"), Input: sp(app.InputMic), Shared: boolPtr(true)}); err != nil {
		t.Fatalf("part meta: %v", err)
	}
	// A personal instrument on the guest's roster record (ruling 1) must survive the round-trip (by id).
	gtr, _ := src.svc.CreatePart(member, bandID, "Guitar", app.PartSpec{})
	if _, err := src.svc.AddMusicianInstrument(member, bandID, guest.ID, gtr.ID); err != nil {
		t.Fatalf("add instrument: %v", err)
	}
	acctID := app.AccountMusicianID(bandID, member.ID) // materialize the member so the export carries them
	if _, err := src.svc.UpdateSong(admin, bandID, songID, app.SongPatch{
		Lineup: &[]app.LineupEntry{
			{MusicianID: guest.ID, PartIDs: []string{part.ID}, Note: "intro only"},
			{MusicianID: acctID, PartIDs: []string{part.ID}},
		},
		SoundNote: sp("percussion intro, then full band"),
	}); err != nil {
		t.Fatalf("set lineup: %v", err)
	}

	zip, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	// Binding 3: the export carries the account musician's USERNAME now (the import remap is 1b).
	if bj := string(unzip(t, zip)["band.json"]); !strings.Contains(bj, `"username": "`+member.Username+`"`) {
		t.Fatalf("band.json must export the account musician's username %q:\n%s", member.Username, bj)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zip, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	parts, _ := tgt.svc.Parts(importer, rep.Band.ID)
	var bongos *app.Part
	for i := range parts {
		if parts[i].Name == "Bongos" {
			bongos = &parts[i]
		}
	}
	if bongos == nil || bongos.Short != "Bgo" || !bongos.Shared || bongos.Input != app.InputMic {
		t.Fatalf("part did not round-trip: %+v", parts)
	}
	ms, _ := tgt.svc.Musicians(importer, rep.Band.ID)
	foundGuest := false
	for _, m := range ms {
		if m.ID == guest.ID && m.DisplayName == "Percussionist" {
			foundGuest = true
			if len(m.Instruments) != 1 || m.Instruments[0] != gtr.ID {
				t.Fatalf("the guest's instruments did not round-trip: %+v", m.Instruments)
			}
		}
	}
	if !foundGuest {
		t.Fatalf("guest musician did not round-trip: %+v", ms)
	}
	// The imported song gets a new id (re-derived for the new band), so find it by title.
	importedSongs, _ := tgt.svc.Songs(importer, rep.Band.ID)
	var gotSong app.Song
	for _, s := range importedSongs {
		if s.Title == "The Open Road" {
			gotSong = s
		}
	}
	if gotSong.SoundNote != "percussion intro, then full band" {
		t.Fatalf("soundNote did not round-trip: %q", gotSong.SoundNote)
	}
	var guestEntry *app.LineupEntry
	for i := range gotSong.Lineup {
		if gotSong.Lineup[i].MusicianID == guest.ID {
			guestEntry = &gotSong.Lineup[i]
		}
	}
	if guestEntry == nil || len(guestEntry.PartIDs) != 1 || guestEntry.PartIDs[0] != part.ID || guestEntry.Note != "intro only" {
		t.Fatalf("the guest's line-up entry did not round-trip: %+v", gotSong.Lineup)
	}
}
