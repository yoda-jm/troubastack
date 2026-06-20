package engine

import (
	"fmt"

	"troubastack/core/internal/domain"
)

// SongRevert appends inverse mutations so HEAD's content equals revision toRevision —
// across ALL layers (design/01: "revert — song-revert (all layers)"). This is
// git-revert, NOT git-reset (I4): old revisions stay; a new head is APPENDED that
// reproduces the target state. Returns the accepted mutations.
func (e *Engine) SongRevert(songID string, toRevision uint64) ([]domain.Mutation, error) {
	return e.revert(songID, toRevision, nil)
}

// LayerRevert reverts ONLY the objects owned by layerID to their state at toRevision
// (design/01: "layer-revert — inverse mutations scoped to one layerId"). Objects in
// other layers are untouched. Append-only; old revisions remain.
func (e *Engine) LayerRevert(songID string, layerID string, toRevision uint64) ([]domain.Mutation, error) {
	return e.revert(songID, toRevision, &layerID)
}

// revert computes the diff HEAD→target and applies inverse mutations. layerFilter, if
// non-nil, restricts the scope to objects in that layer (in EITHER head or target).
func (e *Engine) revert(songID string, toRevision uint64, layerFilter *string) ([]domain.Mutation, error) {
	target, err := e.st.SnapshotAt(songID, toRevision)
	if err != nil {
		return nil, err
	}
	se, err := e.song(songID)
	if err != nil {
		return nil, err
	}

	se.mu.Lock()
	head := se.snapshot()
	se.mu.Unlock()

	headByUUID := indexByUUID(head.Objects)
	targetByUUID := indexByUUID(target.Objects)

	inScope := func(o domain.Object) bool {
		return layerFilter == nil || o.LayerID == *layerFilter
	}

	var muts []domain.Mutation
	// Bump version above everything currently present so the revert wins LWW.
	nextVersion := maxVersion(head.Objects, target.Objects) + 1

	apply := func(m domain.Mutation) error {
		accepted, err := e.Apply(songID, m)
		if err != nil {
			return err
		}
		muts = append(muts, accepted)
		return nil
	}

	// 1. Objects that should be live in target.
	for _, t := range target.Objects {
		if t.Deleted || !inScope(t) {
			continue
		}
		h, inHead := headByUUID[t.UUID]
		want := t.Clone()
		want.Version = nextVersion
		switch {
		case !inHead:
			// Never existed on this head path — recreate it as-is (same UUID).
			if err := apply(domain.Mutation{
				Kind: domain.KindCreate, UUID: want.UUID, Object: &want,
				AuthorID: t.OwnerID, Summary: revertSummary(toRevision, "recreate", t.UUID),
			}); err != nil {
				return muts, err
			}
		case h.Deleted:
			// Tombstoned on head → restore to the target content.
			if err := apply(domain.Mutation{
				Kind: domain.KindRestore, UUID: want.UUID, Object: &want, BaseVersion: h.Version,
				AuthorID: t.OwnerID, Summary: revertSummary(toRevision, "restore", t.UUID),
			}); err != nil {
				return muts, err
			}
		case !objectEqual(h, want):
			// Differs → move/replace to the target content.
			if err := apply(domain.Mutation{
				Kind: domain.KindMove, UUID: want.UUID, Object: &want, BaseVersion: h.Version,
				AuthorID: t.OwnerID, Summary: revertSummary(toRevision, "restore-state", t.UUID),
			}); err != nil {
				return muts, err
			}
		}
	}

	// 2. Objects live on head but absent/dead in target → delete them.
	for _, h := range head.Objects {
		if h.Deleted || !inScope(h) {
			continue
		}
		t, inTarget := targetByUUID[h.UUID]
		if inTarget && !t.Deleted {
			continue
		}
		del := h.Clone()
		del.Version = nextVersion
		if err := apply(domain.Mutation{
			Kind: domain.KindDelete, UUID: h.UUID, Object: &del, BaseVersion: h.Version,
			AuthorID: h.OwnerID, Summary: revertSummary(toRevision, "delete", h.UUID),
		}); err != nil {
			return muts, err
		}
	}

	return muts, nil
}

// ImportOntoHead re-creates the live objects of the given layers (from fromRevision)
// as NEW objects (new UUIDs) on HEAD via append-only Create mutations (design/01:
// "import-onto-HEAD — re-apply as new additive actions on HEAD (new UUIDs)"). It is
// additive and conflict-free — originals are untouched. With no layerIDs, all layers
// are imported. newUUID assigns the fresh UUIDs (so tests can be deterministic).
func (e *Engine) ImportOntoHead(songID string, fromRevision uint64, newUUID func(old string) string, layerIDs ...string) ([]domain.Mutation, error) {
	src, err := e.st.SnapshotAt(songID, fromRevision)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range layerIDs {
		want[id] = true
	}
	var muts []domain.Mutation
	for _, o := range src.Objects {
		if o.Deleted {
			continue
		}
		if len(want) > 0 && !want[o.LayerID] {
			continue
		}
		cp := o.Clone()
		cp.UUID = newUUID(o.UUID)
		cp.Version = 1
		accepted, err := e.Apply(songID, domain.Mutation{
			Kind: domain.KindCreate, UUID: cp.UUID, Object: &cp,
			AuthorID: o.OwnerID,
			Summary:  fmt.Sprintf("import from r%d: %s", fromRevision, o.UUID),
		})
		if err != nil {
			return muts, err
		}
		muts = append(muts, accepted)
	}
	return muts, nil
}

func indexByUUID(objs []domain.Object) map[string]domain.Object {
	m := make(map[string]domain.Object, len(objs))
	for _, o := range objs {
		m[o.UUID] = o
	}
	return m
}

func maxVersion(sets ...[]domain.Object) uint64 {
	var mx uint64
	for _, set := range sets {
		for _, o := range set {
			if o.Version > mx {
				mx = o.Version
			}
		}
	}
	return mx
}

func objectEqual(a, b domain.Object) bool {
	if a.Type != b.Type || a.Text != b.Text || a.Style != b.Style ||
		a.LayerID != b.LayerID || a.Scope != b.Scope || a.OwnerID != b.OwnerID ||
		len(a.Points) != len(b.Points) {
		return false
	}
	for i := range a.Points {
		if a.Points[i] != b.Points[i] {
			return false
		}
	}
	return true
}

func revertSummary(to uint64, op, uuid string) string {
	return fmt.Sprintf("revert to r%d: %s %s", to, op, uuid)
}
