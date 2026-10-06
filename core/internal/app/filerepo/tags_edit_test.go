package filerepo

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
)

// T182 ⟨D5⟩ — a band-wide tag rename is ALL OR NOTHING. The hazard the spec names: filerepo mutates
// r.d.Songs before flush(), so a failed flush could leave memory changed while disk is not; and a per-song
// loop over UpdateSong would flush between songs, so a failure part-way would persist an earlier song with
// no history to repair from.
//
// We inject a flush failure that fires only once the NEW spelling is present on TWO songs. A single-flush
// batch reaches that state in one flush, which then fails, so the batch rolls back and nothing changes —
// neither in memory nor, after a reload, on disk. A per-song loop would flush after the FIRST song (one
// song carrying the new spelling, below the trip) and succeed, then fail on the second — leaving one song
// changed on disk. That is exactly what this test forbids, so sabotaging editBandTags into such a loop
// turns it red. (A failure on the very first write could not tell the two apart — hence the trip at two.)
func TestRenameTagIsAllOrNothing(t *testing.T) {
	const band = "band1"
	seed := func(dir string) *Repo {
		r, err := New(dir)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// Three songs carry "zz"; renaming to "ZZNEW" would change all three.
		for _, id := range []string{"s1", "s2", "s3"} {
			if err := r.CreateSong(app.Song{ID: id, BandID: band, Tags: []string{"keep", "zz"}}); err != nil {
				t.Fatalf("CreateSong: %v", err)
			}
		}
		return r
	}
	carriers := func(r *Repo, tag string) int {
		songs, _ := r.SongsOfBand(band)
		n := 0
		for _, s := range songs {
			for _, tg := range s.Tags {
				if tg == tag {
					n++
					break
				}
			}
		}
		return n
	}

	dir := t.TempDir()
	r := seed(dir)
	// Fail the write the moment the dataset would carry the new spelling on two or more songs. The hook runs
	// INSIDE flush(), under the lock editBandTags already holds, so it reads r.d.Songs directly — calling a
	// locking accessor like SongsOfBand here would deadlock on the non-reentrant mutex.
	injected := errors.New("injected flush failure")
	r.flushHook = func() error {
		if carriersInData(r, "ZZNEW") >= 2 {
			return injected
		}
		return nil
	}

	n, err := r.RenameTag(band, "zz", "ZZNEW")
	if !errors.Is(err, injected) {
		t.Fatalf("RenameTag err = %v, want the injected failure", err)
	}
	if n != 0 {
		t.Errorf("changed count on failure = %d, want 0", n)
	}

	// In memory: nothing changed — no song carries the new spelling, every song still carries the old.
	if got := carriers(r, "ZZNEW"); got != 0 {
		t.Errorf("after failed write, %d songs carry ZZNEW in memory, want 0", got)
	}
	if got := carriers(r, "zz"); got != 3 {
		t.Errorf("after failed write, %d songs still carry zz in memory, want 3", got)
	}

	// On disk: reload the repo from the same directory — a per-song loop would have persisted the first
	// song here.
	r2 := reload(t, dir)
	if got := carriers(r2, "ZZNEW"); got != 0 {
		t.Errorf("after reload, %d songs carry ZZNEW on disk, want 0 (a per-song loop would leave 1)", got)
	}
	if got := carriers(r2, "zz"); got != 3 {
		t.Errorf("after reload, %d songs still carry zz on disk, want 3", got)
	}

	// And with no failure injected, the rename succeeds and persists across a reload.
	dir2 := t.TempDir()
	r3 := seed(dir2)
	if n, err := r3.RenameTag(band, "zz", "ZZNEW"); err != nil || n != 3 {
		t.Fatalf("clean rename n=%d err=%v, want 3, nil", n, err)
	}
	r4 := reload(t, dir2)
	if got := carriers(r4, "ZZNEW"); got != 3 {
		t.Errorf("after clean rename + reload, %d carry ZZNEW, want 3", got)
	}
}

func reload(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := New(dir)
	if err != nil {
		t.Fatalf("reload New: %v", err)
	}
	return r
}

// carriersInData counts the band's songs carrying tag by reading r.d.Songs directly — no lock. It is only
// safe to call from inside flush() (the lock is already held by editBandTags on this goroutine), which is
// exactly where the test's flushHook needs it.
func carriersInData(r *Repo, tag string) int {
	n := 0
	for _, s := range r.d.Songs {
		if s.BandID != "band1" {
			continue
		}
		for _, tg := range s.Tags {
			if tg == tag {
				n++
				break
			}
		}
	}
	return n
}
