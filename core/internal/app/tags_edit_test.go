package app_test

import (
	"errors"
	"testing"
	"time"

	"troubastack/core/internal/app"
)

// T182 — band-wide tag rename (which merges) and delete, at the service layer: the merge dedups within a
// song, a rename keeps position and touches nothing else, validation and auth hold, and the returned count
// is the songs actually changed. The all-or-nothing property is pinned at the repo layer
// (filerepo.TestRenameTagIsAllOrNothing), where the flush that can fail actually lives.
func TestRenameAndDeleteTag(t *testing.T) {
	st := newStack()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "The Troubadours")
	if err != nil {
		t.Fatal(err)
	}

	song := map[string]string{} // title -> id
	seed := func(title string, tags ...string) {
		sg, err := st.svc.CreateSong(admin, band.ID, title, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.svc.UpdateSong(admin, band.ID, sg.ID, app.SongPatch{Tags: &tags}); err != nil {
			t.Fatal(err)
		}
		song[title] = sg.ID
	}
	tagsOf := func(title string) []string {
		songs, err := st.svc.Songs(admin, band.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, sg := range songs {
			if sg.ID == song[title] {
				return sg.Tags
			}
		}
		t.Fatalf("song %q not found", title)
		return nil
	}

	// A carries BOTH spellings — the merge fixture. B has `Encore` between two others, to prove position is
	// kept. C carries neither, to prove untouched songs stay untouched.
	seed("A", "ballad", "Encore", "encore")
	seed("B", "opener", "Encore", "closer")
	seed("C", "ballad")

	// Assert the fixture actually has a both-spellings song, or the merge assertion below proves nothing.
	if a := tagsOf("A"); !has(a, "Encore") || !has(a, "encore") {
		t.Fatalf("fixture A must carry BOTH Encore and encore, got %v", a)
	}

	// Rename Encore -> encore across the band.
	n, err := st.svc.RenameTag(admin, band.ID, "Encore", "encore")
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	// Changed: A and B both carried Encore. C did not. So 2.
	if n != 2 {
		t.Errorf("RenameTag changed = %d, want 2", n)
	}

	// Merge dedups within A: exactly one encore, and no Encore left.
	if a := tagsOf("A"); count(a, "encore") != 1 || has(a, "Encore") {
		t.Errorf("A after merge = %v, want exactly one encore and no Encore", a)
	}
	// B keeps position: [opener, Encore, closer] -> [opener, encore, closer].
	if b := tagsOf("B"); !equal(b, []string{"opener", "encore", "closer"}) {
		t.Errorf("B after rename = %v, want [opener encore closer]", b)
	}
	// C, which never carried Encore, is untouched.
	if c := tagsOf("C"); !equal(c, []string{"ballad"}) {
		t.Errorf("C changed, = %v, want [ballad]", c)
	}

	// Rename stores the chosen spelling VERBATIM — a case-only rename is a plain rename (the UI's
	// adopt-existing offer decides which spelling is passed as `to`; the service does not second-guess it).
	if _, err := st.svc.RenameTag(admin, band.ID, "ballad", "Ballad"); err != nil {
		t.Fatalf("case-only rename: %v", err)
	}
	if a := tagsOf("A"); !has(a, "Ballad") || has(a, "ballad") {
		t.Errorf("A after case rename = %v, want Ballad not ballad", a)
	}

	// Validation. The service wraps ErrInvalidInput with a message, so match with errors.Is, not ==.
	if _, err := st.svc.RenameTag(admin, band.ID, "encore", "   "); !errors.Is(err, app.ErrInvalidInput) {
		t.Errorf("empty `to` err = %v, want ErrInvalidInput", err)
	}
	if _, err := st.svc.RenameTag(admin, band.ID, "encore", "a,b"); !errors.Is(err, app.ErrInvalidInput) {
		t.Errorf("comma `to` err = %v, want ErrInvalidInput", err)
	}
	if _, err := st.svc.RenameTag(admin, band.ID, "nosuchtag", "x"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("unknown `from` err = %v, want ErrNotFound", err)
	}

	// Delete removes from every carrier and reports the count.
	nd, err := st.svc.DeleteTag(admin, band.ID, "encore")
	if err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if nd != 2 { // A and B now both carry encore
		t.Errorf("DeleteTag changed = %d, want 2", nd)
	}
	if a := tagsOf("A"); has(a, "encore") {
		t.Errorf("A still has encore after delete: %v", a)
	}
	if _, err := st.svc.DeleteTag(admin, band.ID, "encore"); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("delete of a now-absent tag err = %v, want ErrNotFound", err)
	}

	// Auth: a non-member is forbidden on BOTH endpoints.
	stranger, err := st.svc.Register("stranger", "Stranger", "password123", "s@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.RenameTag(stranger, band.ID, "opener", "intro"); !errors.Is(err, app.ErrForbidden) {
		t.Errorf("non-member RenameTag err = %v, want ErrForbidden", err)
	}
	if _, err := st.svc.DeleteTag(stranger, band.ID, "opener"); !errors.Is(err, app.ErrForbidden) {
		t.Errorf("non-member DeleteTag err = %v, want ErrForbidden", err)
	}

	// A plain member (not admin) may edit the vocabulary — the same right as editing one song's tags.
	member, err := st.svc.Register("leo", "Leo", "password123", "leo@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.repo.AddMembership(app.Membership{BandID: band.ID, UserID: member.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.RenameTag(member, band.ID, "opener", "intro"); err != nil {
		t.Errorf("member RenameTag err = %v, want nil", err)
	}
}

func has(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func count(ss []string, v string) int {
	n := 0
	for _, s := range ss {
		if s == v {
			n++
		}
	}
	return n
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
