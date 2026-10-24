package app_test

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
)

// T204: the channel-key scheme is id-based, so a part/group rename never re-keys.
func TestChannelKeys(t *testing.T) {
	voice := app.Part{ID: "v1", Kind: app.PartVoice}
	inst := app.Part{ID: "g1", Kind: app.PartInstrument}
	grouped := app.Part{ID: "g2", Kind: app.PartInstrument, ChannelID: "chA"}
	shared := app.Part{ID: "b1", Kind: app.PartInstrument, Shared: true}
	if k := app.PartChannelKey(voice); k != "voice" {
		t.Errorf("voice key = %q", k)
	}
	if k := app.PartChannelKey(inst); k != "p:g1" {
		t.Errorf("instrument key = %q", k)
	}
	if k := app.PartChannelKey(grouped); k != "g:chA" {
		t.Errorf("grouped key = %q", k)
	}
	if k := app.PartChannelKey(shared); k != "" {
		t.Errorf("shared key should be empty, got %q", k)
	}
	// renaming a part (Name change) does not touch its key — it is by id.
	renamed := inst
	renamed.Name = "Totally different"
	if app.PartChannelKey(renamed) != app.PartChannelKey(inst) {
		t.Error("a part rename must not change its channel key")
	}
}

func TestMigratePatchKey(t *testing.T) {
	cases := map[string]string{
		"voice:m1":    "ch:m1:voice",
		"inst:m1:p9":  "ch:m1:p:p9",
		"shared:b1":   "shared:b1",   // unchanged
		"ch:m1:voice": "ch:m1:voice", // idempotent
		"ch:m1:g:chA": "ch:m1:g:chA", // idempotent
		"weird":       "weird",       // unknown passes through
	}
	for in, want := range cases {
		if got := app.MigratePatchKey(in); got != want {
			t.Errorf("MigratePatchKey(%q) = %q, want %q", in, got, want)
		}
		// idempotent: applying twice == once
		if app.MigratePatchKey(app.MigratePatchKey(in)) != want {
			t.Errorf("MigratePatchKey not idempotent on %q", in)
		}
	}
}

func TestChannelCRUDAndGate(t *testing.T) {
	st := newStack()
	admin, member, bandID, _, _, _ := buildSourceBand(t, st)
	if _, err := st.svc.CreateChannel(member, bandID, "DI acoustique"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member create channel: got %v, want ErrForbidden", err)
	}
	ch, err := st.svc.CreateChannel(admin, bandID, "DI acoustique")
	if err != nil || ch.Order == 0 {
		t.Fatalf("create channel: %+v err=%v", ch, err)
	}
	// rename keeps the id (so patches keep resolving)
	ren, err := st.svc.RenameChannel(admin, bandID, ch.ID, "DI acoustic")
	if err != nil || ren.ID != ch.ID || ren.Name != "DI acoustic" {
		t.Fatalf("rename: %+v err=%v", ren, err)
	}
	// a part on the channel → delete is refused unless forced, which reverts the part.
	gtr, _ := st.svc.CreatePart(admin, bandID, "Acoustic guitar", app.PartSpec{})
	if _, err := st.svc.SetPartChannel(admin, bandID, gtr.ID, ch.ID); err != nil {
		t.Fatalf("set part channel: %v", err)
	}
	if n, _, err := st.svc.DeleteChannel(admin, bandID, ch.ID, false); n != 1 || !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("delete-while-used: n=%d err=%v, want 1 + ErrInvalidInput", n, err)
	}
	if _, _, err := st.svc.DeleteChannel(admin, bandID, ch.ID, true); err != nil {
		t.Fatalf("force delete: %v", err)
	}
	parts, _ := st.svc.Parts(admin, bandID)
	for _, p := range parts {
		if p.ID == gtr.ID && p.ChannelID != "" {
			t.Errorf("force delete should revert the part's channelId, got %q", p.ChannelID)
		}
	}
}

// Grouping two already-patched tracks merges their patches, keeping the lower input and reporting it.
func TestSetPartChannel_MergeReport(t *testing.T) {
	st := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, st)
	g, _ := st.svc.AddMusician(admin, bandID, "Gee")
	p1, _ := st.svc.CreatePart(admin, bandID, "Acoustic guitar", app.PartSpec{})
	p2, _ := st.svc.CreatePart(admin, bandID, "Autoharp", app.PartSpec{})
	ch, _ := st.svc.CreateChannel(admin, bandID, "DI")
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	// patch both personal tracks: p1 on input 5, p2 on input 3.
	if _, err := st.svc.SetPatch(admin, bandID, sl.ID, []app.ChannelPatch{
		{Channel: app.PatchKey(g.ID, "p:"+p1.ID), Input: 5, Name: "AG"},
		{Channel: app.PatchKey(g.ID, "p:"+p2.ID), Input: 3, Name: "AH"},
	}); err != nil {
		t.Fatalf("set patch: %v", err)
	}
	// group p1, then p2 — the second collides on ch:<g>:g:<ch> and merges, keeping input 3.
	if _, err := st.svc.SetPartChannel(admin, bandID, p1.ID, ch.ID); err != nil {
		t.Fatalf("group p1: %v", err)
	}
	notes, err := st.svc.SetPartChannel(admin, bandID, p2.ID, ch.ID)
	if err != nil {
		t.Fatalf("group p2: %v", err)
	}
	if len(notes) != 1 || notes[0].KeptInput != 3 || notes[0].DroppedInput != 5 {
		t.Fatalf("merge note = %+v, want keep 3 / drop 5", notes)
	}
	detail, _ := st.svc.Setlist(admin, bandID, sl.ID)
	want := app.PatchKey(g.ID, "g:"+ch.ID)
	found := false
	for _, c := range detail.Setlist.Patch {
		if c.Channel == want {
			found = true
			if c.Input != 3 {
				t.Errorf("merged input = %d, want 3", c.Input)
			}
		}
		if c.Channel == app.PatchKey(g.ID, "p:"+p1.ID) || c.Channel == app.PatchKey(g.ID, "p:"+p2.ID) {
			t.Errorf("old per-part key still present: %q", c.Channel)
		}
	}
	if !found {
		t.Fatalf("merged channel key %q not in patch %+v", want, detail.Setlist.Patch)
	}
}
