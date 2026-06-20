package sync

import (
	"encoding/json"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// handleMutation is the inbound hot path: map the wire mutation to domain, stamp the
// authoritative authorId + server-derived version, apply it through the engine, then
// either broadcast the accepted echo to the room or reject to the sender.
//
// The author id is ALWAYS the authenticated connection's user id — any client-provided
// authorId is ignored (server is authoritative, I6).
func (c *conn) handleMutation(in mutationJSON) {
	kind, ok := kindFromString(in.Kind)
	if !ok {
		return // unknown kind: ignore
	}

	// Serialize the read-version → build → Apply window per song so the
	// server-assigned object version is monotonic even under concurrent senders.
	// (The engine also serializes internally; this just makes our version pick race-free.)
	mu := c.hub.applyMu(c.songID)
	mu.Lock()
	defer mu.Unlock()

	m := domain.Mutation{
		Kind:        kind,
		UUID:        in.UUID,
		BaseVersion: in.BaseVersion,
		AuthorID:    c.authorID, // authoritative; client value ignored
		ClientTS:    in.ClientTS,
		Summary:     in.Summary,
	}
	if in.Layer != nil {
		l := layerFromJSON(*in.Layer)
		m.Layer = &l
	}

	// For object kinds, derive the version server-side from the current HEAD so the
	// engine's LWW (higher Version wins) accepts a legitimate sequential edit without
	// trusting a client-supplied version (the wire Object carries none).
	if !isLayerKind(kind) {
		uuid := in.UUID
		if uuid == "" && in.Object != nil {
			uuid = in.Object.UUID
		}
		curVer, exists := c.currentVersion(uuid)

		switch kind {
		case domain.KindCreate:
			o := domain.Object{}
			if in.Object != nil {
				o = objectFromJSON(*in.Object)
			}
			o.UUID = uuid
			o.OwnerID = c.authorID
			if exists {
				o.Version = curVer + 1 // re-create as an in-place update wins LWW
			} else {
				o.Version = 1
			}
			m.Object = &o
			m.UUID = uuid

		case domain.KindDelete:
			o := domain.Object{UUID: uuid, Version: curVer + 1, Deleted: true}
			m.Object = &o
			m.BaseVersion = curVer

		case domain.KindRestore:
			o := domain.Object{UUID: uuid, Version: curVer + 1}
			if in.Object != nil {
				o = objectFromJSON(*in.Object)
				o.UUID = uuid
				o.Version = curVer + 1
			}
			o.Deleted = false
			m.Object = &o
			m.BaseVersion = curVer

		default: // move, resize, setStyle, setText
			o := domain.Object{}
			if in.Object != nil {
				o = objectFromJSON(*in.Object)
			}
			o.UUID = uuid
			o.Version = curVer + 1
			m.Object = &o
			m.BaseVersion = curVer
			m.UUID = uuid
		}
	}

	accepted, err := c.hub.eng.Apply(c.songID, m)
	if err != nil {
		c.reject(targetUUID(in), reasonFor(err))
		return
	}

	frame, marshalErr := json.Marshal(echoMsg{Type: "echo", Mutation: mutationToJSON(accepted)})
	if marshalErr != nil {
		return
	}
	c.room.broadcast(frame)
}

// currentVersion returns the live version of an object in HEAD (and whether it exists,
// including tombstones). It is read under the per-song apply mutex.
func (c *conn) currentVersion(uuid string) (uint64, bool) {
	if uuid == "" {
		return 0, false
	}
	snap, err := c.hub.eng.Head(c.songID)
	if err != nil {
		return 0, false
	}
	for _, o := range snap.Objects {
		if o.UUID == uuid {
			return o.Version, true
		}
	}
	return 0, false
}

// targetUUID extracts the object uuid a reject should reference.
func targetUUID(in mutationJSON) string {
	if in.UUID != "" {
		return in.UUID
	}
	if in.Object != nil {
		return in.Object.UUID
	}
	return ""
}

// reasonFor maps an engine apply error to the wire reject reason.
func reasonFor(err error) string {
	switch err {
	case engine.ErrDeletedRemotely:
		return "deleted-remotely"
	default:
		// ErrStaleVersion, ErrUnknownObject, ErrInvalidMutation all surface to the
		// client as "stale" — the client rolls back and reconciles to the next echo.
		return "stale"
	}
}

// applyMu returns the per-song apply mutex, created on first use.
func (h *Hub) applyMu(songID string) *muRef {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.applyLocks == nil {
		h.applyLocks = map[string]*muRef{}
	}
	m := h.applyLocks[songID]
	if m == nil {
		m = &muRef{}
		h.applyLocks[songID] = m
	}
	return m
}
