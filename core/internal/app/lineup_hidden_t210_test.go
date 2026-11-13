package app_test

import (
	"errors"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// buildHideConcert makes a 2-song concert: A sings lead in both songs; B plays (amplified) guitar in song1 and
// is gone by song2 — so B owns a column and the changes column names B leaving. The ids let a test hide B (band
// scope) or mark B absent (concert scope) and watch every output drop them. Placeholder names only (T210).
func buildHideConcert(t *testing.T, st stack) (admin, member app.User, bandID, setlistID, song1, song2 string, a, b app.Musician, gtrID string) {
	t.Helper()
	admin, member, bandID, song1, _, _ = buildSourceBand(t, st)
	s2, _ := st.svc.CreateSong(admin, bandID, "Second", "")
	song2 = s2.ID

	a, _ = st.svc.AddMusician(admin, bandID, "Ann")
	b, _ = st.svc.AddMusician(admin, bandID, "Bo")
	lead, _ := st.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	gtr, _ := st.svc.CreatePart(admin, bandID, "Guitar", app.PartSpec{Kind: app.PartInstrument, Short: "Gtr", Input: app.InputMic})
	gtrID = gtr.ID

	setLineup(t, st, admin, bandID, song1, []app.LineupEntry{
		{MusicianID: a.ID, PartIDs: []string{lead.ID}},
		{MusicianID: b.ID, PartIDs: []string{gtr.ID}},
	})
	setLineup(t, st, admin, bandID, song2, []app.LineupEntry{
		{MusicianID: a.ID, PartIDs: []string{lead.ID}},
	})

	sl, _ := st.svc.CreateSetlist(admin, bandID, "Concert", "", "", "")
	for _, sid := range []string{song1, song2} {
		if _, err := st.svc.AddSetlistItem(admin, bandID, sl.ID, sid); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	return admin, member, bandID, sl.ID, song1, song2, a, b, gtrID
}

func hasSubcol(scs []app.SheetSubcol, key string) bool {
	for _, sc := range scs {
		if sc.Key == key {
			return true
		}
	}
	return false
}

func allChanges(m app.SheetModel) string {
	var all []string
	for _, r := range m.Rows {
		all = append(all, r.Changes...)
	}
	return strings.Join(all, " | ")
}

// T210: hiding a musician band-wide removes their columns AND stops the changes column naming them, from the
// ONE filter at the source — then clearing the flag restores everything (nothing was deleted).
func TestBuildSheet_HiddenMusician_VanishesAndNotNamed_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, setlistID, _, _, _, b, gtrID := buildHideConcert(t, st)
	gtrKey := app.PatchKey(b.ID, "p:"+gtrID)

	base, err := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if !hasSubcol(base.Subcols, gtrKey) {
		t.Fatalf("baseline should carry B's guitar column %q, got %v", gtrKey, subcolKeys(base.Subcols))
	}
	if !strings.Contains(allChanges(base), "Bo") {
		t.Fatalf("baseline changes should name Bo leaving, got %q", allChanges(base))
	}

	hid := true
	if _, err := st.svc.UpdateMusician(admin, bandID, b.ID, app.MusicianPatch{Hidden: &hid}); err != nil {
		t.Fatalf("hide: %v", err)
	}
	m, err := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet after hide: %v", err)
	}
	if hasSubcol(m.Subcols, gtrKey) {
		t.Errorf("hidden B's column must be gone, got %v", subcolKeys(m.Subcols))
	}
	for _, sc := range m.Subcols {
		if sc.OwnerID == b.ID {
			t.Errorf("no subcol may be owned by hidden B: %+v", sc)
		}
	}
	if strings.Contains(allChanges(m), "Bo") {
		t.Errorf("the changes column must not name hidden B, got %q", allChanges(m))
	}

	no := false
	if _, err := st.svc.UpdateMusician(admin, bandID, b.ID, app.MusicianPatch{Hidden: &no}); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	back, _ := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	if !hasSubcol(back.Subcols, gtrKey) {
		t.Errorf("clearing hidden must restore B's column, got %v", subcolKeys(back.Subcols))
	}
}

// T210: "Not at this concert" is per-concert — the player drops from THIS setlist's sheet but still shows in
// another concert of the same band.
func TestBuildSheet_ConcertAbsent_PerConcert_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, setlistID, song1, song2, _, b, gtrID := buildHideConcert(t, st)
	gtrKey := app.PatchKey(b.ID, "p:"+gtrID)

	absent := []string{b.ID}
	if _, err := st.svc.UpdateSetlist(admin, bandID, setlistID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("set absent: %v", err)
	}
	m, _ := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	if hasSubcol(m.Subcols, gtrKey) {
		t.Errorf("absent B's column must be gone in this concert, got %v", subcolKeys(m.Subcols))
	}

	// A second concert with the same songs is unaffected — B is present there.
	sl2, _ := st.svc.CreateSetlist(admin, bandID, "Other night", "", "", "")
	for _, sid := range []string{song1, song2} {
		if _, err := st.svc.AddSetlistItem(admin, bandID, sl2.ID, sid); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	m2, _ := st.svc.BuildSheet(admin, bandID, sl2.ID, app.SheetOrderStage)
	if !hasSubcol(m2.Subcols, gtrKey) {
		t.Errorf("B must still show in the OTHER concert, got %v", subcolKeys(m2.Subcols))
	}
}

// T210: auto-number numbers the remaining channels with NO gap when the middle owner is absent (1, 2, not 1, 3).
func TestAutoNumber_SkipsAbsentChannel_NoGap_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, song1, _, _ := buildSourceBand(t, st)

	a, _ := st.svc.AddMusician(admin, bandID, "Ann")
	b, _ := st.svc.AddMusician(admin, bandID, "Bo")
	c, _ := st.svc.AddMusician(admin, bandID, "Cy")
	o1, o2, o3 := 1, 2, 3
	st.svc.UpdateMusician(admin, bandID, a.ID, app.MusicianPatch{Order: &o1})
	st.svc.UpdateMusician(admin, bandID, b.ID, app.MusicianPatch{Order: &o2})
	st.svc.UpdateMusician(admin, bandID, c.ID, app.MusicianPatch{Order: &o3})
	gtr, _ := st.svc.CreatePart(admin, bandID, "Guitar", app.PartSpec{Kind: app.PartInstrument, Short: "Gtr", Input: app.InputMic})

	setLineup(t, st, admin, bandID, song1, []app.LineupEntry{
		{MusicianID: a.ID, PartIDs: []string{gtr.ID}},
		{MusicianID: b.ID, PartIDs: []string{gtr.ID}},
		{MusicianID: c.ID, PartIDs: []string{gtr.ID}},
	})
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	if _, err := st.svc.AddSetlistItem(admin, bandID, sl.ID, song1); err != nil {
		t.Fatalf("add item: %v", err)
	}
	absent := []string{b.ID} // the MIDDLE owner in column order A,B,C
	if _, err := st.svc.UpdateSetlist(admin, bandID, sl.ID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("set absent: %v", err)
	}

	out, err := st.svc.AutoNumber(admin, bandID, sl.ID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("AutoNumber: %v", err)
	}
	byKey := map[string]app.ChannelPatch{}
	for _, p := range out.Patch {
		byKey[p.Channel] = p
	}
	aKey := app.PatchKey(a.ID, "p:"+gtr.ID)
	bKey := app.PatchKey(b.ID, "p:"+gtr.ID)
	cKey := app.PatchKey(c.ID, "p:"+gtr.ID)
	if _, ok := byKey[bKey]; ok {
		t.Errorf("absent B must get no channel/number, got %+v", out.Patch)
	}
	if len(out.Patch) != 2 {
		t.Fatalf("want exactly two numbered channels, got %+v", out.Patch)
	}
	if byKey[aKey].Input != 1 || byKey[cKey].Input != 2 {
		t.Errorf("want A=1, C=2 with no gap, got A=%d C=%d", byKey[aKey].Input, byKey[cKey].Input)
	}
}

// T210: changing either scope is admin-only — a non-admin member's PATCH is forbidden.
func TestUpdateMusician_HiddenIsAdminOnly_T210(t *testing.T) {
	st := newStack()
	admin, member, bandID, _, _, _, _, b, _ := buildHideConcert(t, st)
	hid := true
	if _, err := st.svc.UpdateMusician(member, bandID, b.ID, app.MusicianPatch{Hidden: &hid}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("non-admin hidden PATCH: want ErrForbidden, got %v", err)
	}
	if _, err := st.svc.UpdateMusician(admin, bandID, b.ID, app.MusicianPatch{Hidden: &hid}); err != nil {
		t.Fatalf("admin hidden PATCH should succeed: %v", err)
	}
}

func TestUpdateSetlist_AbsentIsAdminOnly_T210(t *testing.T) {
	st := newStack()
	admin, member, bandID, setlistID, _, _, _, b, _ := buildHideConcert(t, st)
	absent := []string{b.ID}
	if _, err := st.svc.UpdateSetlist(member, bandID, setlistID, app.SetlistInput{AbsentMusicians: &absent}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("non-admin absent PATCH: want ErrForbidden, got %v", err)
	}
	if _, err := st.svc.UpdateSetlist(admin, bandID, setlistID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("admin absent PATCH should succeed: %v", err)
	}
}

// T210: a supplied absent id that is not a roster musician (and a duplicate) is dropped on write, never an error.
func TestUpdateSetlist_AbsentDropsStaleIDs_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, setlistID, _, _, _, b, _ := buildHideConcert(t, st)
	absent := []string{b.ID, "not-a-musician", b.ID}
	sl, err := st.svc.UpdateSetlist(admin, bandID, setlistID, app.SetlistInput{AbsentMusicians: &absent})
	if err != nil {
		t.Fatalf("UpdateSetlist: %v", err)
	}
	if len(sl.AbsentMusicianIDs) != 1 || sl.AbsentMusicianIDs[0] != b.ID {
		t.Errorf("stale + duplicate ids must be dropped, got %v", sl.AbsentMusicianIDs)
	}
}

// T210: both scopes survive a .tband (bandio v2) export → import round trip; nothing is deleted. The band id
// and the musician/setlist ids are declared verbatim, so the absent list (stored by id) still resolves.
func TestBandV2_RoundTrip_HiddenAndAbsent_T210(t *testing.T) {
	src := newStack()
	admin, _, bandID, song1, _, _ := buildSourceBand(t, src)

	hidden, _ := src.svc.AddMusician(admin, bandID, "Hidden One")
	away, _ := src.svc.AddMusician(admin, bandID, "Away One")
	hid := true
	if _, err := src.svc.UpdateMusician(admin, bandID, hidden.ID, app.MusicianPatch{Hidden: &hid}); err != nil {
		t.Fatalf("hide: %v", err)
	}
	lead, _ := src.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	// Both play something, so the entries exist to be preserved (hidden/absent ≠ deleted).
	setLineup(t, src, admin, bandID, song1, []app.LineupEntry{
		{MusicianID: hidden.ID, PartIDs: []string{lead.ID}},
		{MusicianID: away.ID, PartIDs: []string{lead.ID}},
	})
	sl, _ := src.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	if _, err := src.svc.AddSetlistItem(admin, bandID, sl.ID, song1); err != nil {
		t.Fatalf("add item: %v", err)
	}
	absent := []string{away.ID}
	if _, err := src.svc.UpdateSetlist(admin, bandID, sl.ID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("set absent: %v", err)
	}

	zip, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zip, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	ms, _ := tgt.svc.Musicians(importer, rep.Band.ID)
	var sawHidden bool
	for _, m := range ms {
		if m.DisplayName == "Hidden One" {
			sawHidden = true
			if !m.Hidden {
				t.Error("the hidden flag was lost on the round trip")
			}
		}
		if m.DisplayName == "Away One" && m.Hidden {
			t.Error("the away musician must NOT be band-hidden after import")
		}
	}
	if !sawHidden {
		t.Fatal("imported roster is missing the hidden guest")
	}

	detail, err := tgt.svc.Setlist(importer, rep.Band.ID, sl.ID)
	if err != nil {
		t.Fatalf("read imported setlist: %v", err)
	}
	if len(detail.Setlist.AbsentMusicianIDs) != 1 || detail.Setlist.AbsentMusicianIDs[0] != away.ID {
		t.Errorf("the concert-absent list was lost on the round trip, got %v", detail.Setlist.AbsentMusicianIDs)
	}
}

// T210: a duplicated concert does NOT inherit the original's "Not at this concert" — a new date is a new
// attendance. (The original keeps its own.)
func TestDuplicateSetlist_DoesNotCopyAbsences_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, setlistID, _, _, _, b, _ := buildHideConcert(t, st)
	absent := []string{b.ID}
	if _, err := st.svc.UpdateSetlist(admin, bandID, setlistID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("set absent: %v", err)
	}
	dup, err := st.svc.DuplicateSetlist(admin, bandID, setlistID)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if len(dup.AbsentMusicianIDs) != 0 {
		t.Errorf("a duplicated concert must not copy absences, got %v", dup.AbsentMusicianIDs)
	}
	src, _ := st.svc.Setlist(admin, bandID, setlistID)
	if len(src.Setlist.AbsentMusicianIDs) != 1 {
		t.Errorf("the original must keep its own absences, got %v", src.Setlist.AbsentMusicianIDs)
	}
}

// T210: "Copy patch from…" copies only the console patch, never the other concert's absences.
func TestCopyPatchFrom_DoesNotCopyAbsences_T210(t *testing.T) {
	st := newStack()
	admin, _, bandID, fromID, song1, _, _, b, _ := buildHideConcert(t, st)
	absent := []string{b.ID}
	if _, err := st.svc.UpdateSetlist(admin, bandID, fromID, app.SetlistInput{AbsentMusicians: &absent}); err != nil {
		t.Fatalf("set absent: %v", err)
	}
	if _, err := st.svc.AutoNumber(admin, bandID, fromID, app.SheetOrderStage); err != nil {
		t.Fatalf("autonumber source (gives it a patch to copy): %v", err)
	}
	to, _ := st.svc.CreateSetlist(admin, bandID, "Target", "", "", "")
	if _, err := st.svc.AddSetlistItem(admin, bandID, to.ID, song1); err != nil {
		t.Fatalf("add item: %v", err)
	}
	out, err := st.svc.CopyPatchFrom(admin, bandID, to.ID, fromID)
	if err != nil {
		t.Fatalf("copy patch: %v", err)
	}
	if len(out.Patch) == 0 {
		t.Fatalf("precondition: the patch should have been copied, got none")
	}
	if len(out.AbsentMusicianIDs) != 0 {
		t.Errorf("copy-patch must not carry the source's absences, got %v", out.AbsentMusicianIDs)
	}
}
