// Package engine is the apply engine — the live authority that sits ABOVE the store
// (design/07, the apply-engine ⟂ store border).
//
// Per song the engine owns the in-memory HEAD (the live object set + layer set),
// SERIALIZES mutations behind a single writer (a per-song mutex), assigns a monotonic
// seq (the total-order spine), resolves LWW by Version (I5), and enforces tombstones
// (Delete is terminal; only Restore revives). Each ACCEPTED mutation is persisted to
// the passive store (Apply + one AppendRevision per action). Concurrency stops here:
// the store never sees a race, never does LWW (design/07).
//
// Invariants served: I2 (idempotent by UUID), I4 (linear append-only history; revert
// is a new appended head, never a reset), I5 (LWW + terminal tombstone), I6 (server
// is authoritative).
//
// Boundary: imports domain + store + stdlib only. No transport, no UI.
package engine

import (
	"sync"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// Engine is the per-song apply authority over a single store backend.
type Engine struct {
	st store.HistoryAware

	mu    sync.Mutex // guards songs map
	songs map[string]*songEngine
}

// New builds an engine over a history-aware store. Every shipped backend (mem, file,
// git) is at least HistoryAware, so a Collector satisfies this too.
func New(st store.HistoryAware) *Engine {
	return &Engine{st: st, songs: map[string]*songEngine{}}
}

// songEngine holds one song's serialized state. The mutex IS the single-writer lock
// (design/02 lock scope): apply→LWW→assign seq→persist→release.
type songEngine struct {
	mu         sync.Mutex
	id         string
	objects    map[string]domain.Object // includes tombstones (Deleted=true)
	order      []string                 // stable creation order
	layers     map[string]domain.Layer
	layerOrder []string
	seq        uint64 // monotonic; last assigned
	hydrated   bool
}

// song returns (creating) the per-song engine and hydrates it from the store once.
func (e *Engine) song(songID string) (*songEngine, error) {
	e.mu.Lock()
	se := e.songs[songID]
	if se == nil {
		se = &songEngine{id: songID, objects: map[string]domain.Object{}, layers: map[string]domain.Layer{}}
		e.songs[songID] = se
	}
	e.mu.Unlock()

	se.mu.Lock()
	if !se.hydrated {
		if err := se.hydrate(e.st); err != nil {
			se.mu.Unlock()
			return nil, err
		}
		se.hydrated = true
	}
	se.mu.Unlock()
	return se, nil
}

// hydrate loads HEAD from the store (engine hydrates HEAD at first access, design/07).
func (se *songEngine) hydrate(st store.HistoryAware) error {
	snap, err := st.Head(se.id)
	if err != nil {
		return err
	}
	for _, o := range snap.Objects {
		if _, ok := se.objects[o.UUID]; !ok {
			se.order = append(se.order, o.UUID)
		}
		se.objects[o.UUID] = o.Clone()
	}
	for _, l := range snap.Layers {
		if _, ok := se.layers[l.ID]; !ok {
			se.layerOrder = append(se.layerOrder, l.ID)
		}
		se.layers[l.ID] = l
	}
	revs, err := st.Revisions(se.id)
	if err != nil {
		return err
	}
	se.seq = uint64(len(revs)) // seq spine resumes after persisted history
	return nil
}

// Apply is the hot path: serialize, resolve LWW (I5), enforce tombstones, assign seq,
// persist (mutation + one revision), update HEAD. Returns the ACCEPTED mutation (with
// its server seq) for echo. Rejects stale/tombstoned with a typed error.
func (e *Engine) Apply(songID string, m domain.Mutation) (domain.Mutation, error) {
	se, err := e.song(songID)
	if err != nil {
		return domain.Mutation{}, err
	}
	se.mu.Lock()
	defer se.mu.Unlock()

	if isLayerKind(m.Kind) {
		return se.applyLayer(e.st, m)
	}
	return se.applyObject(e.st, m)
}

func (se *songEngine) applyObject(st store.HistoryAware, m domain.Mutation) (domain.Mutation, error) {
	if m.UUID == "" {
		if m.Object != nil {
			m.UUID = m.Object.UUID
		}
	}
	if m.UUID == "" {
		return domain.Mutation{}, ErrInvalidMutation
	}
	cur, exists := se.objects[m.UUID]

	switch m.Kind {
	case domain.KindCreate:
		if m.Object == nil {
			return domain.Mutation{}, ErrInvalidMutation
		}
		if exists {
			// Idempotent by UUID (I2): re-create is a no-op unless it wins LWW.
			if !lwwWins(*m.Object, cur, m) {
				return se.reject(cur, m, exists)
			}
			if cur.Deleted {
				// A create cannot resurrect a tombstone; only Restore does (I5).
				return domain.Mutation{}, ErrDeletedRemotely
			}
		}

	case domain.KindRestore:
		if !exists {
			return domain.Mutation{}, ErrUnknownObject
		}
		// Restore revives; restore-vs-redelete is itself LWW on the live/dead flag.

	case domain.KindDelete:
		if !exists {
			return domain.Mutation{}, ErrUnknownObject
		}
		if cur.Deleted {
			return domain.Mutation{}, ErrDeletedRemotely
		}

	default: // Move, Resize, SetStyle, SetText
		if !exists {
			return domain.Mutation{}, ErrUnknownObject
		}
		if cur.Deleted {
			return domain.Mutation{}, ErrDeletedRemotely // mutation for a dead object (I5)
		}
		if m.BaseVersion < cur.Version {
			return domain.Mutation{}, ErrStaleVersion // lost the LWW race (I5)
		}
		if m.Object != nil && !lwwWins(*m.Object, cur, m) {
			return domain.Mutation{}, ErrStaleVersion
		}
	}

	// Accept: assign seq, persist, then update HEAD.
	se.seq++
	m.Seq = se.seq
	if err := se.persist(st, m); err != nil {
		se.seq-- // roll back the spine on durable-write failure
		return domain.Mutation{}, err
	}
	se.foldOne(m)
	return m, nil
}

// reject is used when a re-create loses LWW: idempotent no-op, echo the current state.
func (se *songEngine) reject(cur domain.Object, m domain.Mutation, _ bool) (domain.Mutation, error) {
	// Losing a create-vs-create race by version is not an error; it's an idempotent
	// no-op (the higher-version object already lives). Echo back the current object.
	accepted := m
	o := cur.Clone()
	accepted.Object = &o
	accepted.Seq = 0 // not newly accepted
	return accepted, nil
}

func (se *songEngine) applyLayer(st store.HistoryAware, m domain.Mutation) (domain.Mutation, error) {
	if m.Layer == nil {
		return domain.Mutation{}, ErrInvalidMutation
	}
	se.seq++
	m.Seq = se.seq
	if err := se.persist(st, m); err != nil {
		se.seq--
		return domain.Mutation{}, err
	}
	se.foldOne(m)
	return m, nil
}

// persist drains one accepted action to the store: Apply the mutation then append a
// revision (one commit per completed action, ADR 0003). For the v1 spike this is
// synchronous (R6 permits it); production moves the store write off the hot path.
func (se *songEngine) persist(st store.HistoryAware, m domain.Mutation) error {
	if err := st.Apply(se.id, m); err != nil {
		return err
	}
	_, err := st.AppendRevision(se.id, domain.Revision{
		AuthorID:  m.AuthorID,
		CreatedAt: m.ClientTS,
		Summary:   m.Summary,
	})
	return err
}

// foldOne applies an accepted mutation to the in-memory HEAD.
func (se *songEngine) foldOne(m domain.Mutation) {
	switch {
	case isLayerKind(m.Kind):
		switch m.Kind {
		case domain.KindLayerDelete:
			if _, ok := se.layers[m.Layer.ID]; ok {
				delete(se.layers, m.Layer.ID)
				for i, id := range se.layerOrder {
					if id == m.Layer.ID {
						se.layerOrder = append(se.layerOrder[:i], se.layerOrder[i+1:]...)
						break
					}
				}
				// T83: cascade-by-TOMBSTONE, in this one mutation/revision. Mark every object on the
				// deleted layer as deleted — kept in the map/order as a tombstone, exactly like
				// KindDelete, NOT dropped. Dropping the UUID would let a concurrent peer's edit for it
				// look like a create and resurrect the object into a dead layer; the tombstone instead
				// makes that mutation ErrDeletedRemotely (engine.go rejects mutations on Deleted
				// objects), so I5's delete-is-terminal invariant holds.
				for id, obj := range se.objects {
					if obj.LayerID == m.Layer.ID && !obj.Deleted {
						obj.Deleted = true
						se.objects[id] = obj
					}
				}
			}
		default:
			if _, ok := se.layers[m.Layer.ID]; !ok {
				se.layerOrder = append(se.layerOrder, m.Layer.ID)
			}
			se.layers[m.Layer.ID] = *m.Layer
		}
		return
	}

	switch m.Kind {
	case domain.KindCreate:
		if _, ok := se.objects[m.UUID]; !ok {
			se.order = append(se.order, m.UUID)
		}
		se.objects[m.UUID] = m.Object.Clone()
	case domain.KindDelete:
		cur := se.objects[m.UUID]
		cur.Deleted = true
		if m.Object != nil {
			cur.Version = m.Object.Version
		}
		se.objects[m.UUID] = cur
	case domain.KindRestore:
		cur := se.objects[m.UUID]
		cur.Deleted = false
		if m.Object != nil {
			next := m.Object.Clone()
			next.Deleted = false
			cur = next
		}
		se.objects[m.UUID] = cur
	default: // Move, Resize, SetStyle, SetText
		if m.Object != nil {
			next := m.Object.Clone()
			next.Deleted = false
			se.objects[m.UUID] = next
		}
	}
}

// Head returns the current materialized HEAD (live + tombstoned objects + layers).
func (e *Engine) Head(songID string) (domain.Snapshot, error) {
	se, err := e.song(songID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.snapshot(), nil
}

// Layer returns the layer with id layerID from the song's current HEAD (and whether it
// exists). It reads under the song's single-writer lock so callers see a consistent
// HEAD without bypassing the engine's serialization (design/07). It is the access-check
// helper the realtime hub uses to resolve a mutation's target layer before Apply.
func (e *Engine) Layer(songID, layerID string) (domain.Layer, bool) {
	se, err := e.song(songID)
	if err != nil {
		return domain.Layer{}, false
	}
	se.mu.Lock()
	defer se.mu.Unlock()
	l, ok := se.layers[layerID]
	return l, ok
}

// ObjectLayer resolves the layer of the object identified by uuid in the song's current
// HEAD. It returns:
//   - objExists: whether the object (live or tombstoned) is in HEAD at all;
//   - layer/layerFound: the object's layer and whether that layer is itself in HEAD.
//
// The two flags are distinct because objects and layers are provisioned independently:
// an object can exist while its layer is not (yet) materialized. The access gate uses
// objExists to tell "unknown object" (stale) from "known object, layer absent". Read
// under the song's single-writer lock.
func (e *Engine) ObjectLayer(songID, uuid string) (layer domain.Layer, layerFound, objExists bool) {
	se, err := e.song(songID)
	if err != nil {
		return domain.Layer{}, false, false
	}
	se.mu.Lock()
	defer se.mu.Unlock()
	o, ok := se.objects[uuid]
	if !ok {
		return domain.Layer{}, false, false
	}
	l, lok := se.layers[o.LayerID]
	return l, lok, true
}

// SnapshotAt returns a past revision from the store (read-only view; design/01).
func (e *Engine) SnapshotAt(songID string, revision uint64) (domain.Snapshot, error) {
	return e.st.SnapshotAt(songID, revision)
}

// Revisions returns the song's linear history oldest→newest (audit/browse).
func (e *Engine) Revisions(songID string) ([]domain.Revision, error) {
	return e.st.Revisions(songID)
}

func (se *songEngine) snapshot() domain.Snapshot {
	snap := domain.Snapshot{Revision: se.seq}
	for _, id := range se.order {
		snap.Objects = append(snap.Objects, se.objects[id].Clone())
	}
	for _, id := range se.layerOrder {
		snap.Layers = append(snap.Layers, se.layers[id])
	}
	return snap
}

func isLayerKind(k domain.Kind) bool {
	switch k {
	case domain.KindLayerCreate, domain.KindLayerUpdate, domain.KindLayerReorder, domain.KindLayerDelete:
		return true
	}
	return false
}

// lwwWins reports whether candidate beats current under LWW (I5): higher Version
// wins; tie-break by higher AuthorID, then higher Seq (the total-order spine).
func lwwWins(candidate, current domain.Object, m domain.Mutation) bool {
	if candidate.Version != current.Version {
		return candidate.Version > current.Version
	}
	if m.AuthorID != current.OwnerID {
		return m.AuthorID > current.OwnerID
	}
	return true // same version & author → idempotent in-place replace
}
