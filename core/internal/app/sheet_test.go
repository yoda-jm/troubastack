package app_test

import (
	"errors"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// buildConcert sets up a 3-song concert with a voice change, a shared-bass handoff and an unamplified part, and
// returns the ids needed to assert the sheet.
func buildConcert(t *testing.T, st stack) (admin app.User, bandID, setlistID string, g1, g2 app.Musician, leadID, backID, bassID, shakerID string) {
	t.Helper()
	var song1 string
	admin, _, bandID, song1, _, _ = buildSourceBand(t, st)
	s2, _ := st.svc.CreateSong(admin, bandID, "Second", "")
	s3, _ := st.svc.CreateSong(admin, bandID, "Third", "")
	song2, song3 := s2.ID, s3.ID

	g1, _ = st.svc.AddMusician(admin, bandID, "Gee")
	g2, _ = st.svc.AddMusician(admin, bandID, "Vee")
	lead, _ := st.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	back, _ := st.svc.CreatePart(admin, bandID, "Backing vocal", app.PartSpec{Kind: app.PartVoice, Short: "B"})
	bass, _ := st.svc.CreatePart(admin, bandID, "Bass", app.PartSpec{Kind: app.PartInstrument, Short: "Bs", Shared: true})
	shaker, _ := st.svc.CreatePart(admin, bandID, "Shaker", app.PartSpec{Kind: app.PartInstrument, Input: app.InputNone})
	leadID, backID, bassID, shakerID = lead.ID, back.ID, bass.ID, shaker.ID

	setLineup(t, st, admin, bandID, song1, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{lead.ID, bass.ID}}})
	setLineup(t, st, admin, bandID, song2, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{back.ID}}, {MusicianID: g2.ID, PartIDs: []string{bass.ID}}})
	setLineup(t, st, admin, bandID, song3, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{lead.ID}}, {MusicianID: g2.ID, PartIDs: []string{shaker.ID}}})

	sl, _ := st.svc.CreateSetlist(admin, bandID, "Concert", "", "", "")
	for _, sid := range []string{song1, song2, song3} {
		if _, err := st.svc.AddSetlistItem(admin, bandID, sl.ID, sid); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	return admin, bandID, sl.ID, g1, g2, leadID, backID, bassID, shakerID
}

func TestBuildSheet_ProjectionChangesInput(t *testing.T) {
	st := newStack()
	admin, bandID, setlistID, g1, g2, _, _, bassID, shakerID := buildConcert(t, st)

	m, err := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	sub := map[string]app.SheetSubcol{}
	for _, sc := range m.Subcols {
		sub[sc.Key] = sc
	}
	voiceKey := app.VoiceChannelKey(g1.ID)
	bassKey := app.SharedChannelKey(bassID)
	shakerKey := app.InstChannelKey(g2.ID, shakerID)
	for _, k := range []string{voiceKey, bassKey, shakerKey} {
		if _, ok := sub[k]; !ok {
			t.Fatalf("missing subcol %q in %v", k, subcolKeys(m.Subcols))
		}
	}
	// the voice cell changes L (song1) → B (song2)
	if got := m.Rows[0].Cells[voiceKey].Text; got != "L" {
		t.Errorf("song1 voice = %q, want L", got)
	}
	if got := m.Rows[1].Cells[voiceKey].Text; got != "B" {
		t.Errorf("song2 voice = %q, want B", got)
	}
	// song2 changes: a voice change and a shared-channel handoff
	ch2 := strings.Join(m.Rows[1].Changes, " | ")
	if !strings.Contains(ch2, "Gee: L→B") {
		t.Errorf("want a voice change 'Gee: L→B' in %q", ch2)
	}
	if !strings.Contains(ch2, "Bs: Gee→Vee ("+app.SheetLabelSameChannel+")") {
		t.Errorf("want a shared handoff 'Bs: Gee→Vee (same channel)' in %q", ch2)
	}
	// input resolution before any patch: amplified bass "not patched", unamplified shaker "— acoustic"
	if sub[bassKey].Input != app.SheetLabelNotPatched {
		t.Errorf("bass input = %q, want %q", sub[bassKey].Input, app.SheetLabelNotPatched)
	}
	if sub[shakerKey].Input != app.SheetLabelAcoustic || sub[shakerKey].Amplified {
		t.Errorf("shaker should read %q and be unamplified, got %q amp=%v", app.SheetLabelAcoustic, sub[shakerKey].Input, sub[shakerKey].Amplified)
	}

	// after a patch, the bass channel reads its input + strip
	if _, err := st.svc.SetPatch(admin, bandID, setlistID, []app.ChannelPatch{{Channel: bassKey, Input: 3, Name: "BASS"}}); err != nil {
		t.Fatalf("SetPatch: %v", err)
	}
	m2, _ := st.svc.BuildSheet(admin, bandID, setlistID, app.SheetOrderStage)
	for _, sc := range m2.Subcols {
		if sc.Key == bassKey && sc.Input != "In 3 · BASS" {
			t.Errorf("patched bass input = %q, want 'In 3 · BASS'", sc.Input)
		}
	}
	if len(m2.InputList) != 1 || m2.InputList[0].Input != 3 {
		t.Errorf("input list = %+v, want one line at input 3", m2.InputList)
	}
	_ = g2
}

func TestAutoNumber_ProposesStripsNeverOverwrites(t *testing.T) {
	st := newStack()
	admin, bandID, setlistID, g1, _, _, _, bassID, _ := buildConcert(t, st)
	voiceKey := app.VoiceChannelKey(g1.ID)
	bassKey := app.SharedChannelKey(bassID)

	// Pre-set one strip name; auto-number must keep it and propose the rest.
	if _, err := st.svc.SetPatch(admin, bandID, setlistID, []app.ChannelPatch{{Channel: voiceKey, Input: 9, Name: "KEEP ME"}}); err != nil {
		t.Fatalf("SetPatch: %v", err)
	}
	sl, err := st.svc.AutoNumber(admin, bandID, setlistID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("AutoNumber: %v", err)
	}
	byKey := map[string]app.ChannelPatch{}
	for _, c := range sl.Patch {
		byKey[c.Channel] = c
	}
	// inputs are 1..N in order; the kept name survives; the bass gets a proposed "BASS"; voice an "X V"
	if byKey[voiceKey].Name != "KEEP ME" {
		t.Errorf("auto-number overwrote a name: %q", byKey[voiceKey].Name)
	}
	if byKey[bassKey].Name != "BASS" {
		t.Errorf("bass proposed strip = %q, want BASS", byKey[bassKey].Name)
	}
	if byKey[voiceKey].Input < 1 || byKey[bassKey].Input < 1 {
		t.Errorf("auto-number left an input unset: %+v", sl.Patch)
	}
	// the unamplified shaker never gets a patch entry
	if _, ok := byKey[app.InstChannelKey(g1.ID, "shaker")]; ok {
		t.Errorf("an unamplified channel must not be patched: %+v", sl.Patch)
	}
}

func TestSetPatch_AdminOnly(t *testing.T) {
	st := newStack()
	admin, member, bandID, _, _, _ := buildSourceBand(t, st)
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	if _, err := st.svc.SetPatch(member, bandID, sl.ID, []app.ChannelPatch{{Channel: "voice:x", Input: 1}}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member SetPatch: got %v, want ErrForbidden", err)
	}
	if _, err := st.svc.SetPatch(admin, bandID, sl.ID, []app.ChannelPatch{{Channel: "voice:x", Input: 1}}); err != nil {
		t.Fatalf("admin SetPatch: %v", err)
	}
}

func subcolKeys(scs []app.SheetSubcol) []string {
	out := make([]string, len(scs))
	for i, sc := range scs {
		out[i] = sc.Key
	}
	return out
}
