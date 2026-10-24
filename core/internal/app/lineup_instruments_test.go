package app_test

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
)

// TestPartShort_EffectiveAndFolded pins the ONE short derivation (ruling 3): multi-word → initials, a single
// word → first-3, accents folded, and an explicit override wins. Studio reads this; it never derives.
func TestPartShort_EffectiveAndFolded(t *testing.T) {
	cases := []struct{ name, short, want string }{
		{"Lead vocal", "", "LV"},         // multi-word → upper-cased initials
		{"Backing vocal", "", "BV"},      //
		{"Guitar", "", "Gui"},            // single long word → first rune + next two
		{"Bg", "", "Bg"},                 // short single word (≤3 runes) → title-cased
		{"Électrique", "", "Ele"},        // accents folded before deriving (É→E, é→e), rune-safe
		{"Saxophône", "", "Sax"},         // folded, single long word
		{"Lead vocal", "LV!", "LV!"},     // explicit override is returned verbatim
		{"Percussion & perc", "", "P&P"}, // every whitespace field contributes its first rune
	}
	for _, c := range cases {
		got := app.PartShort(app.Part{Name: c.name, Short: c.short})
		if got != c.want {
			t.Errorf("PartShort(name=%q short=%q) = %q, want %q", c.name, c.short, got, c.want)
		}
	}
}

// TestParts_ServeEffectiveShort: the Parts() read fills the effective short into every part, so Studio never
// derives. An explicit short is left alone.
func TestParts_ServeEffectiveShort(t *testing.T) {
	st := newStack()
	admin, member, bandID, _, _, _ := buildSourceBand(t, st)
	g, _ := st.svc.CreatePart(member, bandID, "Guitar", app.PartSpec{})          // no short → derived
	v, _ := st.svc.CreatePart(member, bandID, "Lead vocal", app.PartSpec{})      // no short → derived "LV"
	_, _ = st.svc.UpdatePart(admin, bandID, v.ID, app.PartPatch{Short: sp("L")}) // override to "L"

	parts, err := st.svc.Parts(member, bandID)
	if err != nil {
		t.Fatal(err)
	}
	short := map[string]string{}
	for _, p := range parts {
		short[p.ID] = p.Short
	}
	if short[g.ID] != "Gui" {
		t.Errorf("derived short not served: got %q want Gui", short[g.ID])
	}
	if short[v.ID] != "L" {
		t.Errorf("override short not served: got %q want L", short[v.ID])
	}
}

// TestCreatePart_KindsAndShared: a MEMBER may create a voice part and a shared part in one call (ruling 2),
// not only a personal instrument. An unknown kind is rejected.
func TestCreatePart_KindsAndShared(t *testing.T) {
	st := newStack()
	_, member, bandID, _, _, _ := buildSourceBand(t, st)

	voice, err := st.svc.CreatePart(member, bandID, "Lead vocal", app.PartSpec{Kind: app.PartVoice})
	if err != nil || voice.Kind != app.PartVoice {
		t.Fatalf("member create voice part: %+v err=%v", voice, err)
	}
	bass, err := st.svc.CreatePart(member, bandID, "Bass", app.PartSpec{Kind: app.PartInstrument, Shared: true})
	if err != nil || !bass.Shared {
		t.Fatalf("member create shared part: %+v err=%v", bass, err)
	}
	if _, err := st.svc.CreatePart(member, bandID, "Weird", app.PartSpec{Kind: "nonsense"}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("unknown kind: got %v, want ErrInvalidInput", err)
	}
}

// TestMusicianInstruments: a member adds/removes a musician's personal instrument (ruling 1). A shared or voice
// part is rejected. Removal is refused while used (with the count) unless force, which clears the usages.
func TestMusicianInstruments(t *testing.T) {
	st := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, st)
	guest, _ := st.svc.AddMusician(admin, bandID, "G")
	gtr, _ := st.svc.CreatePart(member, bandID, "Guitar", app.PartSpec{})
	bass, _ := st.svc.CreatePart(member, bandID, "Bass", app.PartSpec{Shared: true})

	// A member adds the instrument (a line-up edit, not a roster edit).
	if _, err := st.svc.AddMusicianInstrument(member, bandID, guest.ID, gtr.ID); err != nil {
		t.Fatalf("member add instrument: %v", err)
	}
	// A shared part cannot be a personal instrument.
	if _, err := st.svc.AddMusicianInstrument(member, bandID, guest.ID, bass.ID); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("add shared as instrument: got %v, want ErrInvalidInput", err)
	}
	ms, _ := st.svc.Musicians(member, bandID)
	if !hasInstrument(ms, guest.ID, gtr.ID) {
		t.Fatalf("guitar not on the guest's instrument list: %+v", ms)
	}

	// Use the guitar in a song, then try to remove it: refused with the usage count, unless force.
	setLineup(t, st, member, bandID, songID, []app.LineupEntry{{MusicianID: guest.ID, PartIDs: []string{gtr.ID}}})
	n, err := st.svc.RemoveMusicianInstrument(member, bandID, guest.ID, gtr.ID, false)
	if n != 1 || !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("remove-while-used: got n=%d err=%v, want n=1 + ErrInvalidInput", n, err)
	}
	// Force clears the usage AND the instrument.
	if _, err := st.svc.RemoveMusicianInstrument(member, bandID, guest.ID, gtr.ID, true); err != nil {
		t.Fatalf("force remove: %v", err)
	}
	ms, _ = st.svc.Musicians(member, bandID)
	if hasInstrument(ms, guest.ID, gtr.ID) {
		t.Fatalf("guitar still listed after force remove: %+v", ms)
	}
	if got := songByID(t, st, member, bandID, songID).Lineup; len(got) != 0 {
		t.Fatalf("force should have cleared the usage, line-up: %+v", got)
	}
}

func hasInstrument(ms []app.MusicianView, musicianID, partID string) bool {
	for _, m := range ms {
		if m.ID == musicianID {
			for _, p := range m.Instruments {
				if p == partID {
					return true
				}
			}
		}
	}
	return false
}
