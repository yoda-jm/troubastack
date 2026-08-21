package engine_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
	"troubastack/core/internal/store/gitstore"
	"troubastack/core/internal/store/memstore"
)

// backends runs fn against the engine on each store backend, proving the engine's
// authority logic is backend-agnostic.
func backends(t *testing.T, fn func(t *testing.T, e *engine.Engine)) {
	t.Helper()
	t.Run("mem", func(t *testing.T) {
		fn(t, engine.New(memstore.New().(store.HistoryAware)))
	})
	t.Run("file", func(t *testing.T) {
		fn(t, engine.New(filestore.New(t.TempDir()).(store.HistoryAware)))
	})
	t.Run("git", func(t *testing.T) {
		st, err := gitstore.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		fn(t, engine.New(st.(store.HistoryAware)))
	})
}

const song = "s1"

func obj(uuid, layer, owner string, version uint64) domain.Object {
	return domain.Object{
		UUID: uuid, Type: domain.TypeFreehand, LayerID: layer, OwnerID: owner,
		Version: version, Points: []domain.Point{{X: 0.1, Y: 0.1}},
		Style: domain.Style{Color: "#112233", Width: 0.01, Opacity: 1},
	}
}

func create(o domain.Object) domain.Mutation {
	cp := o
	return domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, Object: &cp, AuthorID: o.OwnerID, Summary: "create " + o.UUID}
}

func mustApply(t *testing.T, e *engine.Engine, m domain.Mutation) domain.Mutation {
	t.Helper()
	acc, err := e.Apply(song, m)
	if err != nil {
		t.Fatalf("apply %v %s: %v", m.Kind, m.UUID, err)
	}
	return acc
}

func liveHead(t *testing.T, e *engine.Engine) []domain.Object {
	t.Helper()
	snap, err := e.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	return snap.LiveObjects()
}

func TestCreateAndHead(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		a := mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		if a.Seq != 1 {
			t.Fatalf("first accepted seq must be 1, got %d", a.Seq)
		}
		b := mustApply(t, e, create(obj("b", "L1", "u1", 1)))
		if b.Seq != 2 {
			t.Fatalf("second seq must be 2, got %d", b.Seq)
		}
		if n := len(liveHead(t, e)); n != 2 {
			t.Fatalf("want 2 live, got %d", n)
		}
	})
}

func TestIdempotentCreate(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		// Re-create same UUID/version: idempotent no-op (no dup, no error).
		if _, err := e.Apply(song, create(obj("a", "L1", "u1", 1))); err != nil {
			t.Fatalf("re-create should be idempotent, got %v", err)
		}
		if n := len(liveHead(t, e)); n != 1 {
			t.Fatalf("idempotent create must not duplicate: got %d", n)
		}
	})
}

func TestLWW(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		// Higher version wins.
		hi := obj("a", "L1", "u1", 5)
		hi.Points = []domain.Point{{X: 0.9, Y: 0.9}}
		mustApply(t, e, domain.Mutation{Kind: domain.KindMove, UUID: "a", Object: &hi, BaseVersion: 1, AuthorID: "u1", Summary: "move"})
		live := liveHead(t, e)
		if live[0].Version != 5 || live[0].Points[0].X != 0.9 {
			t.Fatalf("higher version should win: %+v", live[0])
		}
		// Stale edit (BaseVersion behind current) is rejected.
		stale := obj("a", "L1", "u2", 2)
		_, err := e.Apply(song, domain.Mutation{Kind: domain.KindMove, UUID: "a", Object: &stale, BaseVersion: 1, AuthorID: "u2", Summary: "stale move"})
		if !errors.Is(err, engine.ErrStaleVersion) {
			t.Fatalf("stale edit must be rejected with ErrStaleVersion, got %v", err)
		}
		// HEAD unchanged by the rejected edit.
		if live2 := liveHead(t, e); live2[0].Version != 5 {
			t.Fatalf("rejected edit must not change HEAD: %+v", live2[0])
		}
	})
}

func TestTombstone(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		del := obj("a", "L1", "u1", 2)
		mustApply(t, e, domain.Mutation{Kind: domain.KindDelete, UUID: "a", Object: &del, BaseVersion: 1, AuthorID: "u1", Summary: "delete"})
		if n := len(liveHead(t, e)); n != 0 {
			t.Fatalf("deleted object must not be live: %d", n)
		}
		// A move targeting a tombstoned UUID is rejected (deleted-remotely).
		mv := obj("a", "L1", "u1", 3)
		_, err := e.Apply(song, domain.Mutation{Kind: domain.KindMove, UUID: "a", Object: &mv, BaseVersion: 2, AuthorID: "u1", Summary: "move dead"})
		if !errors.Is(err, engine.ErrDeletedRemotely) {
			t.Fatalf("move on tombstone must be ErrDeletedRemotely, got %v", err)
		}
		// Re-delete is also rejected.
		if _, err := e.Apply(song, domain.Mutation{Kind: domain.KindDelete, UUID: "a", Object: &mv, BaseVersion: 2}); !errors.Is(err, engine.ErrDeletedRemotely) {
			t.Fatalf("re-delete must be ErrDeletedRemotely, got %v", err)
		}
		// Only Restore revives.
		res := obj("a", "L1", "u1", 4)
		mustApply(t, e, domain.Mutation{Kind: domain.KindRestore, UUID: "a", Object: &res, BaseVersion: 2, AuthorID: "u1", Summary: "restore"})
		if n := len(liveHead(t, e)); n != 1 {
			t.Fatalf("restore must revive: %d", n)
		}
	})
}

func TestUnknownObjectRejected(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mv := obj("ghost", "L1", "u1", 1)
		if _, err := e.Apply(song, domain.Mutation{Kind: domain.KindMove, UUID: "ghost", Object: &mv, BaseVersion: 0}); !errors.Is(err, engine.ErrUnknownObject) {
			t.Fatalf("move on never-created object must be ErrUnknownObject, got %v", err)
		}
	})
}

func TestLinearHistoryAppends(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		mustApply(t, e, create(obj("b", "L1", "u1", 1)))
		// Each accepted action advanced HEAD's revision (one revision per action).
		snap, _ := e.Head(song)
		if snap.Revision != 2 {
			t.Fatalf("HEAD revision should advance to 2, got %d", snap.Revision)
		}
	})
}

func TestSongRevert(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		r1, err := snapRevision(e) // revision after creating a
		if err != nil {
			t.Fatal(err)
		}
		mustApply(t, e, create(obj("b", "L1", "u1", 1)))
		mustApply(t, e, create(obj("c", "L2", "u2", 1)))
		if n := len(liveHead(t, e)); n != 3 {
			t.Fatalf("precondition: want 3 live, got %d", n)
		}

		// Revert all layers to r1 (only "a" existed). Append-only: a NEW head.
		muts, err := e.SongRevert(song, r1)
		if err != nil {
			t.Fatalf("SongRevert: %v", err)
		}
		if len(muts) == 0 {
			t.Fatal("SongRevert should append mutations")
		}
		live := liveHead(t, e)
		if len(live) != 1 || live[0].UUID != "a" {
			t.Fatalf("after revert to r1 only 'a' should be live: %+v", uuids(live))
		}
		// Old revisions still present (append-only): r1 still reconstructable.
		s1, err := snapAt(e, r1)
		if err != nil {
			t.Fatalf("old revision must remain: %v", err)
		}
		if len(s1.LiveObjects()) != 1 {
			t.Fatalf("r1 snapshot should still have 1 object")
		}
	})
}

func TestLayerRevert(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		rAfterA, _ := snapRevision(e)
		mustApply(t, e, create(obj("b", "L1", "u1", 1))) // L1 gains b
		mustApply(t, e, create(obj("c", "L2", "u2", 1))) // L2 gains c

		// Revert only L1 to the point where it had just 'a'. L2's 'c' is untouched.
		if _, err := e.LayerRevert(song, "L1", rAfterA); err != nil {
			t.Fatalf("LayerRevert: %v", err)
		}
		got := map[string]bool{}
		for _, o := range liveHead(t, e) {
			got[o.UUID] = true
		}
		if !got["a"] || got["b"] || !got["c"] {
			t.Fatalf("layer-revert should keep a, drop b (L1), keep c (L2): %+v", got)
		}
	})
}

func TestImportOntoHead(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		mustApply(t, e, create(obj("b", "L2", "u1", 1)))
		fromRev, _ := snapRevision(e)
		// Delete a so HEAD differs from the source revision.
		del := obj("a", "L1", "u1", 2)
		mustApply(t, e, domain.Mutation{Kind: domain.KindDelete, UUID: "a", Object: &del, BaseVersion: 1})

		// Import only layer L1 from fromRev → re-creates 'a' under a NEW uuid.
		var counter int
		newUUID := func(old string) string {
			counter++
			return fmt.Sprintf("import-%s-%d", old, counter)
		}
		muts, err := e.ImportOntoHead(song, fromRev, newUUID, "L1")
		if err != nil {
			t.Fatalf("ImportOntoHead: %v", err)
		}
		if len(muts) != 1 {
			t.Fatalf("import of L1 should re-create exactly 1 object, got %d", len(muts))
		}
		// New object present with a NEW uuid; original 'a' still tombstoned (unchanged).
		var foundImport bool
		for _, o := range liveHead(t, e) {
			if o.UUID == "a" {
				t.Fatalf("original deleted 'a' must NOT be revived by import")
			}
			if o.UUID == "import-a-1" {
				foundImport = true
				if o.LayerID != "L1" {
					t.Fatalf("imported object should keep layer L1, got %s", o.LayerID)
				}
			}
		}
		if !foundImport {
			t.Fatal("imported object with new UUID not found on HEAD")
		}
		// The source revision is unchanged (append-only).
		src, _ := snapAt(e, fromRev)
		if len(src.LiveObjects()) != 2 {
			t.Fatalf("source revision must be unchanged: %d live", len(src.LiveObjects()))
		}
	})
}

// snapRevision returns the current HEAD revision number after the last action.
func snapRevision(e *engine.Engine) (uint64, error) {
	snap, err := e.Head(song)
	return snap.Revision, err
}

func snapAt(e *engine.Engine, rev uint64) (domain.Snapshot, error) {
	return e.SnapshotAt(song, rev)
}

func uuids(objs []domain.Object) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.UUID
	}
	return out
}

// --- T83: delete a layer → cascade-by-tombstone, one revision, no live orphan, replay==HEAD ---

func layerCreate(id, owner string) domain.Mutation {
	return domain.Mutation{Kind: domain.KindLayerCreate, Layer: &domain.Layer{ID: id, Zone: domain.ZonePersonal, OwnerID: owner, Access: domain.AccessRW}, AuthorID: owner, Summary: "layer " + id}
}
func layerDelete(id, owner string) domain.Mutation {
	return domain.Mutation{Kind: domain.KindLayerDelete, Layer: &domain.Layer{ID: id, Zone: domain.ZonePersonal, OwnerID: owner}, AuthorID: owner, Summary: "delete layer " + id}
}
func headRev(t *testing.T, e *engine.Engine) uint64 {
	t.Helper()
	snap, err := e.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	return snap.Revision
}

// assertReplayEqualsHead: the engine's live HEAD equals the store replayed to the same revision
// (objects incl tombstones, in order, + layers). Guards the "cascade in one fold only" divergence.
func assertReplayEqualsHead(t *testing.T, e *engine.Engine) {
	t.Helper()
	live, err := e.Head(song)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := e.SnapshotAt(song, live.Revision)
	if err != nil {
		t.Fatal(err)
	}
	sig := func(s domain.Snapshot) string {
		var b strings.Builder
		for _, o := range s.Objects {
			fmt.Fprintf(&b, "O:%s/%v/%s;", o.UUID, o.Deleted, o.LayerID)
		}
		for _, l := range s.Layers {
			fmt.Fprintf(&b, "L:%s;", l.ID)
		}
		return b.String()
	}
	if sig(live) != sig(replay) {
		t.Fatalf("replay != live HEAD:\n live  =%s\n replay=%s", sig(live), sig(replay))
	}
}

func TestLayerDeleteCascadeTombstone(t *testing.T) {
	backends(t, func(t *testing.T, e *engine.Engine) {
		mustApply(t, e, layerCreate("L1", "u1"))
		mustApply(t, e, layerCreate("L2", "u2"))
		mustApply(t, e, create(obj("a", "L1", "u1", 1)))
		mustApply(t, e, create(obj("b", "L1", "u1", 1)))
		mustApply(t, e, create(obj("c", "L2", "u2", 1)))
		revBefore := headRev(t, e)

		mustApply(t, e, layerDelete("L1", "u1"))

		// One revision per layer-delete (not 1 + N object deletes).
		if d := headRev(t, e) - revBefore; d != 1 {
			t.Fatalf("layer-delete took %d revisions, want 1", d)
		}
		// No LIVE object references the dead layer; only c (on L2) survives. (Red-first on pre-T83:
		// a,b stay live on L1.)
		for _, o := range liveHead(t, e) {
			if o.LayerID == "L1" {
				t.Fatalf("live object %s still references deleted layer L1", o.UUID)
			}
		}
		if n := len(liveHead(t, e)); n != 1 {
			t.Fatalf("live objects = %d, want 1 (only c)", n)
		}
		// The layer itself is gone from HEAD.
		snap, _ := e.Head(song)
		for _, l := range snap.Layers {
			if l.ID == "L1" {
				t.Fatalf("layer L1 still present in HEAD after delete")
			}
		}
		// The cascade wrote TOMBSTONES: a late mutation for a cascaded object is rejected
		// deleted-remotely, not resurrected as a create (I5 hole closed).
		mv := obj("a", "L1", "u1", 2)
		if _, err := e.Apply(song, domain.Mutation{Kind: domain.KindMove, UUID: "a", Object: &mv, BaseVersion: 1, AuthorID: "u1"}); !errors.Is(err, engine.ErrDeletedRemotely) {
			t.Fatalf("move on cascaded object: err=%v, want ErrDeletedRemotely", err)
		}
		// Live HEAD == replay (both folds cascaded identically).
		assertReplayEqualsHead(t, e)
	})
}
