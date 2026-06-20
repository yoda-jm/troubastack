package store

import "troubastack/core/internal/domain"

// Fold materializes an ordered mutation log into a Snapshot at the given revision.
//
// The store is a passive sink: by the time a mutation reaches here the engine has
// already done LWW and tombstone enforcement, so Fold simply applies each accepted
// action in seq order. It is the SAME fold used to compute HEAD and any past
// SnapshotAt — pass the prefix of the log up to the desired revision.
//
// Object kinds replace/patch the object keyed by UUID (idempotent, I2). Delete sets
// the tombstone; Restore clears it. Layer kinds maintain the layer set.
func Fold(log []domain.Mutation, revision uint64) domain.Snapshot {
	objects := map[string]domain.Object{}
	var order []string // stable creation order for deterministic output
	layers := map[string]domain.Layer{}
	var layerOrder []string

	upsertObj := func(o domain.Object) {
		if _, ok := objects[o.UUID]; !ok {
			order = append(order, o.UUID)
		}
		objects[o.UUID] = o
	}

	for _, m := range log {
		switch m.Kind {
		case domain.KindCreate:
			if m.Object != nil {
				upsertObj(m.Object.Clone())
			}
		case domain.KindMove, domain.KindResize, domain.KindSetStyle, domain.KindSetText:
			if m.Object != nil {
				cur, ok := objects[m.UUID]
				if !ok {
					// Engine guarantees the target exists; defensively create it.
					upsertObj(m.Object.Clone())
					continue
				}
				next := m.Object.Clone()
				next.Deleted = cur.Deleted
				objects[m.UUID] = next
			}
		case domain.KindDelete:
			if cur, ok := objects[m.UUID]; ok {
				cur.Deleted = true
				if m.Object != nil {
					cur.Version = m.Object.Version
				}
				objects[m.UUID] = cur
			}
		case domain.KindRestore:
			if cur, ok := objects[m.UUID]; ok {
				cur.Deleted = false
				if m.Object != nil {
					next := m.Object.Clone()
					next.Deleted = false
					objects[m.UUID] = next
				} else {
					objects[m.UUID] = cur
				}
			}
		case domain.KindLayerCreate, domain.KindLayerUpdate, domain.KindLayerReorder:
			if m.Layer != nil {
				if _, ok := layers[m.Layer.ID]; !ok {
					layerOrder = append(layerOrder, m.Layer.ID)
				}
				layers[m.Layer.ID] = *m.Layer
			}
		case domain.KindLayerDelete:
			if m.Layer != nil {
				if _, ok := layers[m.Layer.ID]; ok {
					delete(layers, m.Layer.ID)
					for i, id := range layerOrder {
						if id == m.Layer.ID {
							layerOrder = append(layerOrder[:i], layerOrder[i+1:]...)
							break
						}
					}
				}
			}
		}
	}

	snap := domain.Snapshot{Revision: revision}
	for _, id := range order {
		snap.Objects = append(snap.Objects, objects[id])
	}
	for _, id := range layerOrder {
		snap.Layers = append(snap.Layers, layers[id])
	}
	return snap
}
