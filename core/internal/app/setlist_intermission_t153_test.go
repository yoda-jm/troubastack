package app_test

import (
	"encoding/json"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// T153 slice 1 — an intermission is an entry with NO SongID. The risk is not the feature: an empty string
// is a valid string, so every reader that never questioned SongID now meets an entry with no song and can
// produce a plausible wrong answer instead of an error (T140's shape). These tests pin the three consumers
// the enumeration flagged as traps, plus the two it cleared, so "cleared" is asserted rather than assumed.

// t153Fixture: a band with one song in one setlist, plus the admin.
func t153Fixture(t *testing.T) (*app.Service, app.User, string, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	admin, err := svc.Register("dana", "Dana", "pass1234", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := svc.CreateBand(admin, "Break Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Opener", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("create setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add song item: %v", err)
	}
	return svc, admin, band.ID, sl.ID, song.ID
}

// TRAP 1 — Setlist() builds each row with `if err == nil` around the song lookup, so an intermission would
// silently get an EMPTY title: a blank row in Studio and on the printed sheet, with nothing failing.
// Teeth: the assertion is on the LABEL reaching the view, so restoring the plain song lookup reddens it.
func TestSetlistView_IntermissionCarriesItsLabelNotAnEmptyTitle_T153(t *testing.T) {
	svc, admin, bandID, setlistID, _ := t153Fixture(t)
	if _, err := svc.AddSetlistIntermission(admin, bandID, setlistID, "Entracte"); err != nil {
		t.Fatalf("add intermission: %v", err)
	}

	detail, err := svc.Setlist(admin, bandID, setlistID)
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("got %d items, want 2 (a song and a break)", len(detail.Items))
	}
	brk := detail.Items[1]
	if !brk.IsIntermission() {
		t.Fatalf("second entry is not an intermission: kind=%q", brk.Kind)
	}
	if brk.SongTitle != "Entracte" {
		t.Errorf("intermission row title = %q, want %q — a break must not render as a blank row", brk.SongTitle, "Entracte")
	}
	if brk.SongID != "" {
		t.Errorf("intermission carries SongID %q, want empty", brk.SongID)
	}
	if detail.Items[0].SongTitle != "Opener" {
		t.Errorf("song row title = %q, want %q", detail.Items[0].SongTitle, "Opener")
	}
}

// TRAP 2 — DuplicateSetlist() copies field by field, so a field it does not know is silently dropped:
// every break in the copy would become an empty song. The same test also pins TransposeChords, which the
// copier ALREADY dropped before T153 — found by the enumeration, fixed here.
func TestDuplicateSetlist_CopiesKindLabelAndTranspose_T153(t *testing.T) {
	svc, admin, bandID, setlistID, _ := t153Fixture(t)
	if _, err := svc.AddSetlistIntermission(admin, bandID, setlistID, "Pause"); err != nil {
		t.Fatalf("add intermission: %v", err)
	}
	detail, err := svc.Setlist(admin, bandID, setlistID)
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	yes := true
	if _, err := svc.UpdateSetlistItem(admin, bandID, setlistID, detail.Items[0].ID, app.SetlistItemPatch{TransposeChords: &yes}); err != nil {
		t.Fatalf("set transposeChords: %v", err)
	}

	dup, err := svc.DuplicateSetlist(admin, bandID, setlistID)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	copied, err := svc.Setlist(admin, bandID, dup.ID)
	if err != nil {
		t.Fatalf("setlist(copy): %v", err)
	}
	if len(copied.Items) != 2 {
		t.Fatalf("copy has %d items, want 2", len(copied.Items))
	}
	if !copied.Items[1].IsIntermission() {
		t.Errorf("the copied break is not an intermission (kind=%q) — duplicating turned it into an empty song", copied.Items[1].Kind)
	}
	if copied.Items[1].Label != "Pause" {
		t.Errorf("copied label = %q, want %q", copied.Items[1].Label, "Pause")
	}
	if !copied.Items[0].TransposeChords {
		t.Errorf("copied song lost TransposeChords — a pre-T153 drop the enumeration found")
	}
}

// TRAP 3 — the folder import resolves a song by `songMap[SongRef]`, and a map miss yields "" silently, so
// a typo'd or missing ref already creates a song-less song today. With kinds in play the guard is
// explicit: an entry that CLAIMS to be a song must resolve to a real one, or the import fails loudly.
// Teeth: the same folder with the ref corrected must import cleanly, so the guard rejects the defect and
// not the shape.
func TestImport_RefusesASongEntryWhoseRefDoesNotResolve_T153(t *testing.T) {
	mk := func(ref string) []byte {
		band, _ := json.Marshal(map[string]any{
			"formatVersion": 2, "name": "Ref Band",
			"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
		})
		rep, _ := json.Marshal(map[string]any{"songs": []any{map[string]any{"slug": "s01", "title": "Real Song"}}})
		setl, _ := json.Marshal(map[string]any{
			"setlists": []any{map[string]any{"name": "Gig", "items": []any{map[string]any{"song": ref}}}},
		})
		return rezip(t, map[string][]byte{"band.json": band, "repertoire.json": rep, "setlists.json": setl})
	}

	st := newStack()
	importer, _ := st.svc.Register("owner", "Owner", "password123", "")
	if _, err := st.svc.ImportBand(importer, st.eng, mk("nope-not-a-slug"), nil); err == nil {
		t.Errorf("import accepted a song entry whose ref does not resolve — it becomes an entry with no song, silently")
	}

	st2 := newStack()
	importer2, _ := st2.svc.Register("owner", "Owner", "password123", "")
	if _, err := st2.svc.ImportBand(importer2, st2.eng, mk("s01"), nil); err != nil {
		t.Errorf("import of the SAME folder with a resolvable ref failed: %v — the guard must reject the defect, not the shape", err)
	}
}

// CLEARED, and asserted rather than assumed — LiveSetlistsForSong matches `it.SongID == songID`, which an
// intermission's empty SongID can only hit if somebody ever passes an empty songID.
func TestLiveSetlistsForSong_NeverMatchesAnIntermission_T153(t *testing.T) {
	svc, admin, bandID, setlistID, songID := t153Fixture(t)
	if _, err := svc.AddSetlistIntermission(admin, bandID, setlistID, "Pause"); err != nil {
		t.Fatalf("add intermission: %v", err)
	}
	if _, err := svc.SetSetlistLive(admin, bandID, setlistID, true); err != nil {
		t.Fatalf("set live: %v", err)
	}
	live, err := svc.LiveSetlistsForSong(songID)
	if err != nil {
		t.Fatalf("live for song: %v", err)
	}
	if len(live) != 1 {
		t.Fatalf("live setlists for the real song = %d, want 1", len(live))
	}
	empty, err := svc.LiveSetlistsForSong("")
	if err != nil {
		t.Fatalf("live for empty song: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("an empty songID matched %d setlists via the intermission — a break must never stand in for a song", len(empty))
	}
}

// ADDITIVE — every entry written before T153 has no kind, and must read as a SONG. This is the band_id
// lesson (T143): absent must mean the OLD meaning, never a new third state.
func TestAbsentKindMeansSong_T153(t *testing.T) {
	if (app.SetlistItem{}).IsIntermission() {
		t.Errorf("an item with no kind reads as an intermission — every pre-T153 entry would change meaning")
	}
	if !(app.SetlistItem{Kind: app.SetlistKindIntermission}).IsIntermission() {
		t.Errorf("an explicit intermission does not read as one")
	}
	if (app.SetlistItem{Kind: app.SetlistKindSong}).IsIntermission() {
		t.Errorf("an explicit song reads as an intermission")
	}
}

// ROUND-TRIP — an intermission must survive export→import with its label AND its place in the running
// order. The v2 folder expresses order as array order (T140), so a break that exports without a `song`
// must still come back at the right index. Teeth: the assertion is on the ORDER (song, break, song), which
// a naive "skip entries with no song" on either side would break.
func TestExportImport_IntermissionSurvivesWithLabelAndPosition_T153(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("dana", "Dana", "pass1234", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := st.svc.CreateBand(admin, "Round Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	first, err := st.svc.CreateSong(admin, band.ID, "First", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	last, err := st.svc.CreateSong(admin, band.ID, "Last", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	sl, err := st.svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("create setlist: %v", err)
	}
	if _, err := st.svc.AddSetlistItem(admin, band.ID, sl.ID, first.ID); err != nil {
		t.Fatalf("add first: %v", err)
	}
	if _, err := st.svc.AddSetlistIntermission(admin, band.ID, sl.ID, "Entracte"); err != nil {
		t.Fatalf("add break: %v", err)
	}
	if _, err := st.svc.AddSetlistItem(admin, band.ID, sl.ID, last.ID); err != nil {
		t.Fatalf("add last: %v", err)
	}

	zipBytes, _, err := st.svc.ExportBand(admin, st.eng, band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	st2 := newStack()
	importer, _ := st2.svc.Register("owner", "Owner", "password123", "")
	rep, err := st2.svc.ImportBand(importer, st2.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	setlists, err := st2.svc.Setlists(importer, rep.Band.ID)
	if err != nil || len(setlists) != 1 {
		t.Fatalf("Setlists: got %d (err %v), want 1", len(setlists), err)
	}
	detail, err := st2.svc.Setlist(importer, rep.Band.ID, setlists[0].ID)
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	if len(detail.Items) != 3 {
		t.Fatalf("imported %d entries, want 3 (song, break, song) — a break must not be dropped in transit", len(detail.Items))
	}
	if detail.Items[0].SongTitle != "First" {
		t.Errorf("entry 0 = %q, want %q", detail.Items[0].SongTitle, "First")
	}
	if !detail.Items[1].IsIntermission() {
		t.Errorf("entry 1 is not an intermission (kind=%q) — it came back as a song", detail.Items[1].Kind)
	}
	if detail.Items[1].Label != "Entracte" {
		t.Errorf("entry 1 label = %q, want %q", detail.Items[1].Label, "Entracte")
	}
	if detail.Items[2].SongTitle != "Last" {
		t.Errorf("entry 2 = %q, want %q — the break shifted the running order", detail.Items[2].SongTitle, "Last")
	}
}
