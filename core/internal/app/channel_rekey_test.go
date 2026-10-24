package app_test

import (
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

func patchByChannel(t *testing.T, st stack, caller app.User, bandID, setlistID string) map[string]int {
	t.Helper()
	d, err := st.svc.Setlist(caller, bandID, setlistID)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]int{}
	for _, c := range d.Setlist.Patch {
		m[c.Channel] = c.Input
	}
	return m
}

// setup: a musician g, two instrument parts p1/p2, a setlist sl.
func rekeyFixture(t *testing.T, st stack) (admin app.User, bandID string, g app.Musician, p1, p2 app.Part, slID string) {
	t.Helper()
	admin, _, bandID, _, _, _ = buildSourceBand(t, st)
	g, _ = st.svc.AddMusician(admin, bandID, "Gee")
	p1, _ = st.svc.CreatePart(admin, bandID, "Acoustic guitar", app.PartSpec{})
	p2, _ = st.svc.CreatePart(admin, bandID, "Autoharp", app.PartSpec{})
	sl, _ := st.svc.CreateSetlist(admin, bandID, "Gig", "", "", "")
	return admin, bandID, g, p1, p2, sl.ID
}

// Binding 1: a part leaving a group that KEEPS other parts must not move the group's patch.
func TestSetPartChannel_LeaveGroupOthersRemain(t *testing.T) {
	st := newStack()
	admin, bandID, g, p1, p2, slID := rekeyFixture(t, st)
	ch, _ := st.svc.CreateChannel(admin, bandID, "DI")
	mustSetPartChannel(t, st, admin, bandID, p1.ID, ch.ID)
	mustSetPartChannel(t, st, admin, bandID, p2.ID, ch.ID)
	groupKey := app.PatchKey(g.ID, "g:"+ch.ID)
	if _, err := st.svc.SetPatch(admin, bandID, slID, []app.ChannelPatch{{Channel: groupKey, Input: 7}}); err != nil {
		t.Fatal(err)
	}
	// p1 leaves the group; p2 stays → the group patch must remain on input 7.
	if _, err := st.svc.SetPartChannel(admin, bandID, p1.ID, ""); err != nil {
		t.Fatal(err)
	}
	pm := patchByChannel(t, st, admin, bandID, slID)
	if pm[groupKey] != 7 {
		t.Fatalf("group patch must stay at 7, got %v", pm)
	}
	if _, stolen := pm[app.PatchKey(g.ID, "p:"+p1.ID)]; stolen {
		t.Fatalf("the departing part stole the group's input: %v", pm)
	}
}

// Binding 1: moving a part X→Y while others remain in X leaves both tracks' patches intact.
func TestSetPartChannel_MoveWithOthersRemaining(t *testing.T) {
	st := newStack()
	admin, bandID, g, p1, p2, slID := rekeyFixture(t, st)
	p3, _ := st.svc.CreatePart(admin, bandID, "Mandolin", app.PartSpec{})
	chX, _ := st.svc.CreateChannel(admin, bandID, "X")
	chY, _ := st.svc.CreateChannel(admin, bandID, "Y")
	mustSetPartChannel(t, st, admin, bandID, p1.ID, chX.ID)
	mustSetPartChannel(t, st, admin, bandID, p2.ID, chX.ID)
	mustSetPartChannel(t, st, admin, bandID, p3.ID, chY.ID)
	xKey, yKey := app.PatchKey(g.ID, "g:"+chX.ID), app.PatchKey(g.ID, "g:"+chY.ID)
	if _, err := st.svc.SetPatch(admin, bandID, slID, []app.ChannelPatch{{Channel: xKey, Input: 7}, {Channel: yKey, Input: 4}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.SetPartChannel(admin, bandID, p1.ID, chY.ID); err != nil { // p1 X→Y; p2 still in X
		t.Fatal(err)
	}
	pm := patchByChannel(t, st, admin, bandID, slID)
	if pm[xKey] != 7 || pm[yKey] != 4 {
		t.Fatalf("both tracks must keep their inputs (X=7, Y=4), got %v", pm)
	}
}

// Binding 1: when the LAST part leaves a group, the (now-empty) group track's patch re-keys to the part's own.
func TestSetPartChannel_LastPartLeaves(t *testing.T) {
	st := newStack()
	admin, bandID, g, p1, _, slID := rekeyFixture(t, st)
	ch, _ := st.svc.CreateChannel(admin, bandID, "Solo")
	mustSetPartChannel(t, st, admin, bandID, p1.ID, ch.ID)
	groupKey := app.PatchKey(g.ID, "g:"+ch.ID)
	if _, err := st.svc.SetPatch(admin, bandID, slID, []app.ChannelPatch{{Channel: groupKey, Input: 7}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.SetPartChannel(admin, bandID, p1.ID, ""); err != nil {
		t.Fatal(err)
	}
	pm := patchByChannel(t, st, admin, bandID, slID)
	if pm[app.PatchKey(g.ID, "p:"+p1.ID)] != 7 {
		t.Fatalf("empty group's patch should move to the part's own track, got %v", pm)
	}
	if _, stale := pm[groupKey]; stale {
		t.Fatalf("stale group key remains: %v", pm)
	}
}

// Binding 2: force-deleting a channel with ONE part moves its patch to the part's own track; with TWO it drops
// and reports. No stale g: key either way.
func TestDeleteChannel_OrphanPatches(t *testing.T) {
	// one part
	st := newStack()
	admin, bandID, g, p1, _, slID := rekeyFixture(t, st)
	ch, _ := st.svc.CreateChannel(admin, bandID, "DI")
	mustSetPartChannel(t, st, admin, bandID, p1.ID, ch.ID)
	if _, err := st.svc.SetPatch(admin, bandID, slID, []app.ChannelPatch{{Channel: app.PatchKey(g.ID, "g:"+ch.ID), Input: 5}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.svc.DeleteChannel(admin, bandID, ch.ID, true); err != nil {
		t.Fatal(err)
	}
	pm := patchByChannel(t, st, admin, bandID, slID)
	if pm[app.PatchKey(g.ID, "p:"+p1.ID)] != 5 || hasGroupKey(pm) {
		t.Fatalf("one-part force delete should move patch to p: and leave no g:, got %v", pm)
	}

	// two parts
	st2 := newStack()
	admin2, bandID2, g2, q1, q2, slID2 := rekeyFixture(t, st2)
	ch2, _ := st2.svc.CreateChannel(admin2, bandID2, "DI")
	mustSetPartChannel(t, st2, admin2, bandID2, q1.ID, ch2.ID)
	mustSetPartChannel(t, st2, admin2, bandID2, q2.ID, ch2.ID)
	if _, err := st2.svc.SetPatch(admin2, bandID2, slID2, []app.ChannelPatch{{Channel: app.PatchKey(g2.ID, "g:"+ch2.ID), Input: 5}}); err != nil {
		t.Fatal(err)
	}
	_, notes, err := st2.svc.DeleteChannel(admin2, bandID2, ch2.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	pm2 := patchByChannel(t, st2, admin2, bandID2, slID2)
	if hasGroupKey(pm2) || len(pm2) != 0 {
		t.Fatalf("two-part force delete should drop the ambiguous group patch, got %v", pm2)
	}
	if len(notes) != 1 || notes[0].DroppedInput != 5 {
		t.Fatalf("two-part delete should report the dropped patch, got %+v", notes)
	}
}

// Binding 3: a re-keyed patch is stored in a stable order (by input).
func TestRekeyPatches_StableOrder(t *testing.T) {
	st := newStack()
	admin, bandID, g, p1, p2, slID := rekeyFixture(t, st)
	p3, _ := st.svc.CreatePart(admin, bandID, "Keys", app.PartSpec{})
	ch, _ := st.svc.CreateChannel(admin, bandID, "DI")
	// three patched tracks on inputs 9, 2, 5; then group p1 (re-keys, rebuilding the slice).
	if _, err := st.svc.SetPatch(admin, bandID, slID, []app.ChannelPatch{
		{Channel: app.PatchKey(g.ID, "p:"+p1.ID), Input: 9},
		{Channel: app.PatchKey(g.ID, "p:"+p2.ID), Input: 2},
		{Channel: app.PatchKey(g.ID, "p:"+p3.ID), Input: 5},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.SetPartChannel(admin, bandID, p1.ID, ch.ID); err != nil {
		t.Fatal(err)
	}
	d, _ := st.svc.Setlist(admin, bandID, slID)
	last := -1
	for _, c := range d.Setlist.Patch {
		if c.Input < last {
			t.Fatalf("patch not sorted by input: %+v", d.Setlist.Patch)
		}
		last = c.Input
	}
}

func mustSetPartChannel(t *testing.T, st stack, caller app.User, bandID, partID, channelID string) {
	t.Helper()
	if _, err := st.svc.SetPartChannel(caller, bandID, partID, channelID); err != nil {
		t.Fatalf("SetPartChannel: %v", err)
	}
}

func hasGroupKey(pm map[string]int) bool {
	for k := range pm {
		if strings.Contains(k, ":g:") { // ch:<m>:g:<id>
			return true
		}
	}
	return false
}
