// Package storetest is a SHARED CONTRACT TEST SUITE for store backends. The same
// tests run against memstore, filestore and gitstore (via a factory) to prove the
// three backends behave IDENTICALLY — they are the passive sink below the engine, so
// "fold an ordered log into the same materialized state" must hold everywhere.
package storetest

import (
	"errors"
	"fmt"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// Factory builds a fresh, empty Collector. For disk-backed stores it should point at
// a unique temp dir (use t.TempDir in the caller).
type Factory func(t *testing.T) store.Collector

func freehand(uuid, layer, owner string, version uint64) domain.Object {
	return domain.Object{
		UUID: uuid, Type: domain.TypeFreehand, LayerID: layer, OwnerID: owner,
		Version: version, Points: []domain.Point{{X: 0.1, Y: 0.2}, {X: 0.3, Y: 0.4}},
		Style: domain.Style{Color: "#000000", Width: 0.01, Opacity: 1},
	}
}

func create(o domain.Object, seq uint64) domain.Mutation {
	cp := o
	return domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, Object: &cp, Seq: seq, AuthorID: o.OwnerID, Summary: "create " + o.UUID}
}

// Run executes the full contract suite against the factory.
func Run(t *testing.T, newStore Factory) {
	t.Helper()
	t.Run("ApplyAndHead", func(t *testing.T) { testApplyAndHead(t, newStore) })
	t.Run("IdempotentBySeq", func(t *testing.T) { testIdempotent(t, newStore) })
	t.Run("Tombstone", func(t *testing.T) { testTombstone(t, newStore) })
	t.Run("Reorder", func(t *testing.T) { testReorder(t, newStore) })
	t.Run("LinearHistory", func(t *testing.T) { testLinearHistory(t, newStore) })
	t.Run("SnapshotAt", func(t *testing.T) { testSnapshotAt(t, newStore) })
	t.Run("Pins", func(t *testing.T) { testPins(t, newStore) })
	t.Run("EmptySong", func(t *testing.T) { testEmpty(t, newStore) })
	t.Run("Collect", func(t *testing.T) { testCollect(t, newStore) })
	t.Run("ReachabilityI7", func(t *testing.T) { testReachabilityI7(t, newStore) })
}

// testReachabilityI7 is the I7 proof (P202): after Collect at the reachability-prune
// tier, EVERYTHING reachable from a root — head, every pin, and any
// RootSet.KeepRevisions entry — must still reconstruct. It asserts SAFETY only (not
// that anything was reclaimed), so a reference-safe no-op Collect passes on every
// backend; the point is to LOCK the I7 contract so any future real prune (deferred:
// baseline-snapshot compaction) must keep every root reconstructable.
func testReachabilityI7(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	// Four revisions, each a distinct accepted mutation with its own rev marker.
	revs := make([]uint64, 0, 4)
	for i := 1; i <= 4; i++ {
		mustApply(t, st, song, create(freehand(fmt.Sprintf("o%d", i), "L1", "u1", 1), uint64(i)))
		r, err := st.AppendRevision(song, domain.Revision{Summary: fmt.Sprintf("r%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		revs = append(revs, r)
	}
	// A pin holds an EARLY revision (r1); RootSet keeps a MIDDLE one (r2); head is r4.
	if err := st.MovePin(song, "live", revs[0]); err != nil {
		t.Fatal(err)
	}
	roots := store.RootSet{KeepRevisions: map[string][]uint64{song: {revs[1]}}}
	if err := st.Collect(roots, store.RetentionPolicy{Tier: store.TierReachabilityPrune}); err != nil {
		t.Fatal(err)
	}
	// I7: pinned (r1), kept (r2), and head (r4) all still reconstruct.
	for _, r := range []uint64{revs[0], revs[1], revs[3]} {
		if _, err := st.SnapshotAt(song, r); err != nil {
			t.Fatalf("Collect broke a reachable revision r%d: %v", r, err)
		}
	}
	// Head is unchanged (still folds to r4).
	h, err := st.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	if h.Revision != revs[3] {
		t.Fatalf("head revision changed after Collect: got %d want %d", h.Revision, revs[3])
	}
}

func testApplyAndHead(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	if err := st.Apply(song, create(freehand("a", "L1", "u1", 1), 1)); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(song, create(freehand("b", "L1", "u1", 1), 2)); err != nil {
		t.Fatal(err)
	}
	snap, err := st.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	live := snap.LiveObjects()
	if len(live) != 2 {
		t.Fatalf("want 2 live objects, got %d", len(live))
	}
	if live[0].UUID != "a" || live[1].UUID != "b" {
		t.Fatalf("creation order not preserved: %v %v", live[0].UUID, live[1].UUID)
	}
}

func testIdempotent(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	m := create(freehand("a", "L1", "u1", 1), 7)
	for i := 0; i < 3; i++ {
		if err := st.Apply(song, m); err != nil {
			t.Fatal(err)
		}
	}
	snap, _ := st.Head(song)
	if n := len(snap.LiveObjects()); n != 1 {
		t.Fatalf("re-applying same seq must not duplicate: got %d", n)
	}
}

func testTombstone(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	o := freehand("a", "L1", "u1", 1)
	mustApply(t, st, song, create(o, 1))
	// Delete writes a tombstone.
	del := o
	del.Version = 2
	mustApply(t, st, song, domain.Mutation{Kind: domain.KindDelete, UUID: "a", Object: &del, Seq: 2})
	snap, _ := st.Head(song)
	if n := len(snap.LiveObjects()); n != 0 {
		t.Fatalf("deleted object must not be live: got %d", n)
	}
	if len(snap.Objects) != 1 || !snap.Objects[0].Deleted {
		t.Fatalf("tombstone must be retained in full object set")
	}
	// Restore revives.
	res := o
	res.Version = 3
	mustApply(t, st, song, domain.Mutation{Kind: domain.KindRestore, UUID: "a", Object: &res, Seq: 3})
	snap, _ = st.Head(song)
	if n := len(snap.LiveObjects()); n != 1 {
		t.Fatalf("restore must revive: got %d", n)
	}
}

// testReorder proves an object's z-order round-trips through Apply→Head and that a
// KindReorder mutation folds in-place (bumps version, replaces Order) — identically
// across all backends (T27). Covers both the new Object.Order field persistence and
// the fold.go KindReorder case.
func testReorder(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	// Create with an explicit non-zero order so we prove the field persists at all.
	o := freehand("a", "L1", "u1", 1)
	o.Order = 3
	o.CreatedAt = 1720000000000
	mustApply(t, st, song, create(o, 1))
	snap, _ := st.Head(song)
	if got := snap.LiveObjects(); len(got) != 1 || got[0].Order != 3 || got[0].CreatedAt != 1720000000000 {
		t.Fatalf("Order + CreatedAt must persist through create: got %+v", got)
	}
	// Reorder: bring-to-front bumps Order (and version) in place, same uuid.
	ro := o
	ro.Order = 42
	ro.Version = 2
	mustApply(t, st, song, domain.Mutation{Kind: domain.KindReorder, UUID: "a", Object: &ro, Seq: 2, BaseVersion: 1})
	snap, _ = st.Head(song)
	live := snap.LiveObjects()
	if len(live) != 1 {
		t.Fatalf("reorder must not add/drop objects: got %d", len(live))
	}
	if live[0].Order != 42 {
		t.Fatalf("reorder must update Order in place: got %d", live[0].Order)
	}
	if live[0].Version != 2 {
		t.Fatalf("reorder must fold as a version bump: got %d", live[0].Version)
	}
}

func testLinearHistory(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	for i := uint64(1); i <= 3; i++ {
		mustApply(t, st, song, create(freehand(string(rune('a'+i-1)), "L1", "u1", 1), i))
		n, err := st.AppendRevision(song, domain.Revision{AuthorID: "u1", Summary: "r"})
		if err != nil {
			t.Fatal(err)
		}
		if n != i {
			t.Fatalf("revision number want %d got %d", i, n)
		}
	}
	revs, err := st.Revisions(song)
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 3 {
		t.Fatalf("want 3 revisions, got %d", len(revs))
	}
	// Parent chain is linear.
	for i, r := range revs {
		if r.Number != uint64(i+1) {
			t.Fatalf("revision %d has number %d", i, r.Number)
		}
		if i == 0 && r.Parent != 0 {
			t.Fatalf("root revision parent must be 0, got %d", r.Parent)
		}
		if i > 0 && r.Parent != r.Number-1 {
			t.Fatalf("revision %d parent must be %d, got %d", r.Number, r.Number-1, r.Parent)
		}
	}
}

func testSnapshotAt(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	mustApply(t, st, song, create(freehand("a", "L1", "u1", 1), 1))
	rev1, _ := st.AppendRevision(song, domain.Revision{Summary: "after a"})
	mustApply(t, st, song, create(freehand("b", "L1", "u1", 1), 2))
	rev2, _ := st.AppendRevision(song, domain.Revision{Summary: "after b"})

	s1, err := st.SnapshotAt(song, rev1)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s1.LiveObjects()); n != 1 {
		t.Fatalf("snapshot at r%d should have 1 object, got %d", rev1, n)
	}
	s2, err := st.SnapshotAt(song, rev2)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(s2.LiveObjects()); n != 2 {
		t.Fatalf("snapshot at r%d should have 2 objects, got %d", rev2, n)
	}
	if _, err := st.SnapshotAt(song, 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown revision must be ErrNotFound, got %v", err)
	}
}

func testPins(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	mustApply(t, st, song, create(freehand("a", "L1", "u1", 1), 1))
	r1, _ := st.AppendRevision(song, domain.Revision{Summary: "r1"})
	mustApply(t, st, song, create(freehand("b", "L1", "u1", 1), 2))
	r2, _ := st.AppendRevision(song, domain.Revision{Summary: "r2"})

	if err := st.MovePin(song, "setlist", r1); err != nil {
		t.Fatal(err)
	}
	pins, err := st.Pins(song)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 || pins[0].Name != "setlist" || pins[0].RevisionNumber != r1 {
		t.Fatalf("unexpected pins: %+v", pins)
	}
	// Move the pin forward; the old target (r1) must still be reconstructable (I7).
	if err := st.MovePin(song, "setlist", r2); err != nil {
		t.Fatal(err)
	}
	pins, _ = st.Pins(song)
	if len(pins) != 1 || pins[0].RevisionNumber != r2 {
		t.Fatalf("pin should have moved to r%d: %+v", r2, pins)
	}
	if _, err := st.SnapshotAt(song, r1); err != nil {
		t.Fatalf("old pin target must remain reconstructable: %v", err)
	}
	if err := st.MovePin(song, "setlist", 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("pinning unknown revision must be ErrNotFound, got %v", err)
	}
}

func testEmpty(t *testing.T, newStore Factory) {
	st := newStore(t)
	snap, err := st.Head("never-seen")
	if err != nil {
		t.Fatalf("Head of unknown song should be empty, not error: %v", err)
	}
	if len(snap.Objects) != 0 || len(snap.Layers) != 0 {
		t.Fatalf("empty song must yield empty snapshot")
	}
	revs, err := st.Revisions("never-seen")
	if err != nil || len(revs) != 0 {
		t.Fatalf("empty song revisions: %v %d", err, len(revs))
	}
}

func testCollect(t *testing.T, newStore Factory) {
	st := newStore(t)
	const song = "s1"
	mustApply(t, st, song, create(freehand("a", "L1", "u1", 1), 1))
	r1, _ := st.AppendRevision(song, domain.Revision{Summary: "r1"})
	if err := st.MovePin(song, "p", r1); err != nil {
		t.Fatal(err)
	}
	// Collect must never break a reference (I7): the pinned revision survives.
	if err := st.Collect(store.RootSet{}, store.RetentionPolicy{Tier: store.TierKeepAll}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SnapshotAt(song, r1); err != nil {
		t.Fatalf("pinned revision dropped by Collect: %v", err)
	}
}

func mustApply(t *testing.T, st store.Store, song string, m domain.Mutation) {
	t.Helper()
	if err := st.Apply(song, m); err != nil {
		t.Fatal(err)
	}
}
