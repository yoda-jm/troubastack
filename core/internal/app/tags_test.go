package app_test

import (
	"testing"
	"time"

	"troubastack/core/internal/app"
)

// T180 ⟨D4⟩ — the band-wide tag aggregate. The COUNT is the feature (it lets a member tell a convention
// from a typo), so the test is about the count being right per distinct spelling, the order being
// most-used-first and deterministic, a within-song duplicate not inflating a count, and a non-member
// getting nothing.
func TestTagCounts(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "The Troubadours")
	if err != nil {
		t.Fatal(err)
	}

	// Three songs. "encore" is on two, "slow blues" (a SPACE inside one tag) on two, "Encore" is a
	// SECOND spelling on one — it must stay distinct, because that fragmentation is what the count is
	// meant to surface, not hide.
	tag := func(title string, tags ...string) {
		sg, err := st.svc.CreateSong(admin, band.ID, title, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.svc.UpdateSong(admin, band.ID, sg.ID, app.SongPatch{Tags: &tags}); err != nil {
			t.Fatal(err)
		}
	}
	tag("A", "encore", "slow blues")
	tag("B", "encore", "slow blues", "encore") // the within-song dup must not double-count
	tag("C", "Encore")

	got, err := st.svc.TagCounts(admin, band.ID)
	if err != nil {
		t.Fatal(err)
	}

	want := []app.TagCount{
		{Tag: "encore", Count: 2},
		{Tag: "slow blues", Count: 2}, // ties broken by tag, so "encore" < "slow blues"
		{Tag: "Encore", Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tags, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %+v, want %+v (full: %+v)", i, got[i], want[i], got)
		}
	}

	// A non-member is not entitled to the band's vocabulary.
	stranger, err := st.svc.Register("stranger", "Stranger", "password123", "s@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.TagCounts(stranger, band.ID); err != app.ErrForbidden {
		t.Errorf("non-member TagCounts err = %v, want ErrForbidden", err)
	}

	// A member (not just the admin) may read it — the whole band shares the vocabulary.
	member, err := st.svc.Register("leo", "Leo", "password123", "leo@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.repo.AddMembership(app.Membership{BandID: band.ID, UserID: member.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.TagCounts(member, band.ID); err != nil {
		t.Errorf("member TagCounts err = %v, want nil", err)
	}
}
