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

// T207: one person sings lead AND backing. The voice cell joins in the channel's member order ("L+B", never
// "B+L") whatever order the line-up holds the parts in (binding 2), and the changes column compares the voice
// parts as a set, so the same two parts in opposite entry order print no change (binding 3).
func TestSheetVoice_LeadAndBacking_T207(t *testing.T) {
	st := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, st)

	g1, _ := st.svc.AddMusician(admin, bandID, "Gee")
	// Lead is created first, so it is the vocabulary-first voice part: the canonical channel order is lead
	// before backing. (These are the two voice parts of the band's one voice channel.)
	lead, _ := st.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	back, _ := st.svc.CreatePart(admin, bandID, "Backing vocal", app.PartSpec{Kind: app.PartVoice, Short: "B"})

	mk := func(title string, pids ...string) string {
		s, err := st.svc.CreateSong(admin, bandID, title, "")
		if err != nil {
			t.Fatal(err)
		}
		setLineup(t, st, admin, bandID, s.ID, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: pids}})
		return s.ID
	}
	s1 := mk("One", back.ID, lead.ID) // entry order {B,L} CONTRADICTS the channel order {L,B}
	s2 := mk("Two", lead.ID, back.ID) // same set as s1, opposite entry order
	s3 := mk("Three", lead.ID)        // lead only
	s4 := mk("Four", lead.ID, back.ID)

	sl, _ := st.svc.CreateSetlist(admin, bandID, "Set", "", "", "")
	for _, sid := range []string{s1, s2, s3, s4} {
		if _, err := st.svc.AddSetlistItem(admin, bandID, sl.ID, sid); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}

	m, err := st.svc.BuildSheet(admin, bandID, sl.ID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	voiceKey := app.PatchKey(g1.ID, "voice")

	// Binding 2 — canonical join: s1, entered backing-first, still reads "L+B" (never "B+L").
	if got := m.Rows[0].Cells[voiceKey].Text; got != "L+B" {
		t.Errorf("s1 voice cell = %q, want \"L+B\" (canonical order, though entered backing-first)", got)
	}
	// Binding 3 — set comparison: s2 {L,B} vs s1 {B,L} is the same set → NO voice change line.
	if ch := strings.Join(m.Rows[1].Changes, " | "); strings.Contains(ch, "Gee:") {
		t.Errorf("s2 should show no voice change (same two parts as s1, opposite order), got %q", ch)
	}
	// Control — a real change still prints, in canonical order both ways.
	if ch := strings.Join(m.Rows[2].Changes, " | "); !strings.Contains(ch, "Gee: L+B→L") {
		t.Errorf("s3 want voice change 'Gee: L+B→L' in %q", ch)
	}
	if ch := strings.Join(m.Rows[3].Changes, " | "); !strings.Contains(ch, "Gee: L→L+B") {
		t.Errorf("s4 want voice change 'Gee: L→L+B' in %q", ch)
	}
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
	voiceKey := app.PatchKey(g1.ID, "voice")
	bassKey := app.SharedChannelKey(bassID)
	shakerKey := app.PatchKey(g2.ID, "p:"+shakerID)
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
	admin, bandID, setlistID, g1, g2, _, _, bassID, shakerID := buildConcert(t, st)
	voiceKey := app.PatchKey(g1.ID, "voice")
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
	if _, ok := byKey[app.PatchKey(g2.ID, "p:"+shakerID)]; ok {
		t.Errorf("an unamplified channel must not be patched: %+v", sl.Patch)
	}
}

// T203-FIX: the sheet numbers by the T158 rule (runningorder), renders the intermission as a separator, and
// puts on-call songs in their own unnumbered section. Changes across a break compare with the last song before.
func TestBuildSheet_NumbersIntermissionOnCall(t *testing.T) {
	st := newStack()
	admin, _, bandID, song1, _, _ := buildSourceBand(t, st)
	s2, _ := st.svc.CreateSong(admin, bandID, "Two", "")
	s3, _ := st.svc.CreateSong(admin, bandID, "Three", "")
	oc, _ := st.svc.CreateSong(admin, bandID, "Bench", "")
	g1, _ := st.svc.AddMusician(admin, bandID, "Gee")
	lead, _ := st.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	back, _ := st.svc.CreatePart(admin, bandID, "Backing vocal", app.PartSpec{Kind: app.PartVoice, Short: "B"})
	gtr, _ := st.svc.CreatePart(admin, bandID, "Guitar", app.PartSpec{Kind: app.PartInstrument, Short: "Gtr"})
	setLineup(t, st, admin, bandID, song1, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{lead.ID}}})
	setLineup(t, st, admin, bandID, s2.ID, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{lead.ID}}})
	setLineup(t, st, admin, bandID, s3.ID, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{back.ID}}}) // L→B vs s2
	setLineup(t, st, admin, bandID, oc.ID, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{gtr.ID}}})

	sl, _ := st.svc.CreateSetlist(admin, bandID, "Concert", "", "", "")
	mustItem := func(id string) {
		if _, err := st.svc.AddSetlistItem(admin, bandID, sl.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	mustItem(song1)
	mustItem(s2.ID)
	if _, err := st.svc.AddSetlistIntermission(admin, bandID, sl.ID, "Break"); err != nil {
		t.Fatal(err)
	}
	mustItem(s3.ID)
	ocItem, _ := st.svc.AddSetlistItem(admin, bandID, sl.ID, oc.ID)
	if _, err := st.svc.UpdateSetlistItem(admin, bandID, sl.ID, ocItem.ID, app.SetlistItemPatch{OnCall: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}

	m, err := st.svc.BuildSheet(admin, bandID, sl.ID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if len(m.Rows) != 5 {
		t.Fatalf("want 5 rows (3 songs + break + on-call), got %d: %+v", len(m.Rows), m.Rows)
	}
	want := []struct {
		kind, section string
		number        int
	}{
		{"song", "main", 1},
		{"song", "main", 2},
		{"intermission", "main", 0},
		{"song", "main", 3},
		{"song", "oncall", 0},
	}
	for i, w := range want {
		r := m.Rows[i]
		if r.Kind != w.kind || r.Section != w.section || r.Number != w.number {
			t.Errorf("row %d = {kind=%q section=%q number=%d}, want {%q %q %d}", i, r.Kind, r.Section, r.Number, w.kind, w.section, w.number)
		}
	}
	if m.Rows[2].Label != "Break" {
		t.Errorf("intermission label = %q, want Break", m.Rows[2].Label)
	}
	// changes on the song AFTER the break compare to the last song BEFORE it (s2), skipping the intermission.
	if ch := strings.Join(m.Rows[3].Changes, " | "); !strings.Contains(ch, "Gee: L→B") {
		t.Errorf("row after the break should show the voice change vs s2, got %q", ch)
	}
}

func TestBuildSheet_26Items(t *testing.T) {
	st := newStack()
	admin, _, bandID, song1, _, _ := buildSourceBand(t, st)
	g1, _ := st.svc.AddMusician(admin, bandID, "Gee")
	lead, _ := st.svc.CreatePart(admin, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice, Short: "L"})
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Big concert", "", "", "")
	// 22 main songs + 1 intermission + 3 on-call = 26 items.
	addSong := func(title string, oncall bool) {
		var sid string
		if title == "song1" {
			sid = song1
		} else {
			sg, _ := st.svc.CreateSong(admin, bandID, title, "")
			sid = sg.ID
		}
		setLineup(t, st, admin, bandID, sid, []app.LineupEntry{{MusicianID: g1.ID, PartIDs: []string{lead.ID}}})
		it, _ := st.svc.AddSetlistItem(admin, bandID, sl.ID, sid)
		if oncall {
			_, _ = st.svc.UpdateSetlistItem(admin, bandID, sl.ID, it.ID, app.SetlistItemPatch{OnCall: boolPtr(true)})
		}
	}
	addSong("song1", false)
	for i := 2; i <= 11; i++ {
		addSong("song"+itoaT(i), false)
	}
	if _, err := st.svc.AddSetlistIntermission(admin, bandID, sl.ID, "Interval"); err != nil {
		t.Fatal(err)
	}
	for i := 12; i <= 22; i++ {
		addSong("song"+itoaT(i), false)
	}
	for i := 1; i <= 3; i++ {
		addSong("bench"+itoaT(i), true)
	}
	m, err := st.svc.BuildSheet(admin, bandID, sl.ID, app.SheetOrderStage)
	if err != nil {
		t.Fatalf("BuildSheet: %v", err)
	}
	if len(m.Rows) != 26 {
		t.Fatalf("want 26 rows, got %d", len(m.Rows))
	}
	numbered, oncalls, breaks, maxNum := 0, 0, 0, 0
	for _, r := range m.Rows {
		switch {
		case r.Kind == app.SheetRowIntermission:
			breaks++
			if r.Number != 0 {
				t.Errorf("intermission must have no number, got %d", r.Number)
			}
		case r.Section == app.SheetSectionOnCall:
			oncalls++
			if r.Number != 0 {
				t.Errorf("on-call song must have no number, got %d", r.Number)
			}
		default:
			numbered++
			if r.Number > maxNum {
				maxNum = r.Number
			}
		}
	}
	if numbered != 22 || maxNum != 22 || breaks != 1 || oncalls != 3 {
		t.Fatalf("want 22 numbered (max 22), 1 break, 3 on-call; got numbered=%d max=%d breaks=%d oncall=%d", numbered, maxNum, breaks, oncalls)
	}
}

func itoaT(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// T203-FIX2: a created PART gets a non-zero, increasing order (max+1), so the grid never falls back to a usage
// tie-break for the part vocabulary. (Musicians stay order-0 and sort by the stable roster index — see the note
// in AddMusician on why guests are not stamped.)
func TestCreatePart_DenseOrders(t *testing.T) {
	st := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, st)
	p1, _ := st.svc.CreatePart(admin, bandID, "Gtr", app.PartSpec{})
	p2, _ := st.svc.CreatePart(admin, bandID, "Bassoon", app.PartSpec{})
	if p1.Order == 0 || p2.Order <= p1.Order {
		t.Fatalf("part orders not increasing/non-zero: %d then %d", p1.Order, p2.Order)
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

func TestSetPatch_RejectsDuplicateInputs(t *testing.T) {
	st := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, st)
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	// two channels on input 4 — a console can't do that
	_, err := st.svc.SetPatch(admin, bandID, sl.ID, []app.ChannelPatch{
		{Channel: "voice:a", Input: 4, Name: "A V"},
		{Channel: "inst:b:g", Input: 4, Name: "B GTR"},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("duplicate input: got %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "voice:a") || !strings.Contains(err.Error(), "inst:b:g") {
		t.Errorf("error should name BOTH channels, got %q", err.Error())
	}
	// distinct inputs are fine
	if _, err := st.svc.SetPatch(admin, bandID, sl.ID, []app.ChannelPatch{
		{Channel: "voice:a", Input: 4}, {Channel: "inst:b:g", Input: 5},
	}); err != nil {
		t.Fatalf("distinct inputs: %v", err)
	}
}

func subcolKeys(scs []app.SheetSubcol) []string {
	out := make([]string, len(scs))
	for i, sc := range scs {
		out[i] = sc.Key
	}
	return out
}
