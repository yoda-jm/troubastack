package gitstore_test

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/gitstore"
	"troubastack/core/internal/store/storetest"
)

func newGit(t *testing.T) store.Collector {
	t.Helper()
	st, err := gitstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return st.(store.Collector)
}

func TestContract(t *testing.T) {
	storetest.Run(t, newGit)
}

// TestCommitsPerAction proves N accepted actions ⇒ N commits and commit messages ==
// summaries (one commit per completed action; design/01, the git backend).
func TestCommitsPerAction(t *testing.T) {
	dir := t.TempDir()
	st, err := gitstore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := st.(store.Collector)

	const song = "s1"
	summaries := []string{"create a", "create b", "move a"}
	o := domain.Object{UUID: "a", Type: domain.TypeFreehand, OwnerID: "u1", Version: 1}
	mustApply(t, c, song, domain.Mutation{Kind: domain.KindCreate, UUID: "a", Object: &o, Seq: 1, AuthorID: "u1", Summary: summaries[0]})
	b := domain.Object{UUID: "b", Type: domain.TypeFreehand, OwnerID: "u1", Version: 1}
	mustApply(t, c, song, domain.Mutation{Kind: domain.KindCreate, UUID: "b", Object: &b, Seq: 2, AuthorID: "u1", Summary: summaries[1]})
	moved := o
	moved.Version = 2
	mustApply(t, c, song, domain.Mutation{Kind: domain.KindMove, UUID: "a", Object: &moved, Seq: 3, AuthorID: "u1", Summary: summaries[2]})

	// Inspect the raw git log.
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	if err := iter.ForEach(func(c *object.Commit) error {
		msgs = append(msgs, c.Message)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("want 3 commits for 3 actions, got %d: %v", len(msgs), msgs)
	}
	// Log is newest→oldest; reverse-compare against summaries.
	want := []string{summaries[2], summaries[1], summaries[0]}
	for i, m := range msgs {
		if m != want[i] {
			t.Fatalf("commit %d message = %q, want %q", i, m, want[i])
		}
	}
}

// TestDurability proves reopening the repo (a fresh store instance) hydrates the same
// Head — durability across process restarts.
func TestDurability(t *testing.T) {
	dir := t.TempDir()
	st1, err := gitstore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	const song = "s1"
	o := domain.Object{UUID: "a", Type: domain.TypeFreehand, OwnerID: "u1", Version: 1, Points: []domain.Point{{X: 0.5, Y: 0.5}}}
	mustApply(t, st1.(store.Store), song, domain.Mutation{Kind: domain.KindCreate, UUID: "a", Object: &o, Seq: 1, Summary: "create a"})
	if _, err := st1.(store.HistoryAware).AppendRevision(song, domain.Revision{Summary: "r1"}); err != nil {
		t.Fatal(err)
	}

	// Reopen as a brand-new instance.
	st2, err := gitstore.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st2.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	live := snap.LiveObjects()
	if len(live) != 1 || live[0].UUID != "a" {
		t.Fatalf("reopened store did not hydrate the same Head: %+v", live)
	}
	revs, err := st2.(store.HistoryAware).Revisions(song)
	if err != nil || len(revs) != 1 {
		t.Fatalf("reopened store lost revisions: %v %d", err, len(revs))
	}
}

func mustApply(t *testing.T, st store.Store, song string, m domain.Mutation) {
	t.Helper()
	if err := st.Apply(song, m); err != nil {
		t.Fatal(err)
	}
}
