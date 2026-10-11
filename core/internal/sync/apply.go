package sync

import (
	"encoding/json"
	"strings"

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

	// LAYER WRITE-ACCESS gate (hub-only; the import REST path stays permissive because
	// it drives the engine directly, never this code). Reject BEFORE Apply so a
	// forbidden mutation is neither folded into HEAD nor broadcast. The engine stays
	// mechanical — authority lives here, at the transport edge.
	if reason, ok := c.authorizeWrite(kind, in); !ok {
		c.reject(targetUUID(in), reason)
		return
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
			// Stamp the creation time (z-order tiebreak, T27) once, from the author's
			// clock; first write wins so a re-create can't rewrite it.
			if o.CreatedAt == 0 {
				o.CreatedAt = m.ClientTS
			}
			if exists {
				o.Version = curVer + 1 // re-create as an in-place update wins LWW
			} else {
				o.Version = 1
			}
			// T195 (anchoring stage 2): the SERVER links the new mark to its chart line (P2–P5); whatever
			// link, anchor or render hash the client sent is dropped first (P26). Best-effort + nil-safe: an
			// unset anchorer or an uploaded file leaves the mark on its coordinates, unlinked.
			o.Link, o.Anchor, o.PointsRenderHash = nil, nil, ""
			if c.hub.anchorer != nil {
				o = c.hub.anchorer.AnchorMark(c.songID, o)
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
				// T195: a restore re-links like a move (the client's object is where it was last seen).
				o = c.relink(o, uuid)
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
			// T195: the server decides the link (P26). A move or resize re-links from where the mark now is
			// (P23, P24 — reusing T191's seam, re-pointed at the stage-2 link); a restyle, a text change or a
			// reorder keeps the stored link and does not move the mark (P25).
			if kind == domain.KindMove || kind == domain.KindResize {
				o = c.relink(o, uuid)
			} else {
				o = c.rebase(o, uuid)
			}
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
	// P201: notify the autobaker that this song committed (a live setlist containing
	// it debounce-bakes). Non-blocking, best-effort; nil when no autobaker is wired.
	if hook := c.hub.onCommit; hook != nil {
		hook(c.songID)
	}

	// T196: the echo carries the mark's serve-time link view (where its linked text is now), so every
	// client's dot follows the move without a reload. A copy: the accepted mutation is HEAD's.
	if c.hub.anchorer != nil && accepted.Object != nil && !accepted.Object.Deleted && accepted.Object.Link != nil {
		o := c.hub.anchorer.ServeEcho(c.songID, accepted.Object.Clone())
		accepted.Object = &o
	}
	frame, marshalErr := json.Marshal(echoMsg{Type: "echo", Mutation: mutationToJSON(accepted)})
	if marshalErr != nil {
		return
	}
	c.room.broadcast(frame)
}

// authorizeWrite enforces LAYER WRITE-ACCESS for the live editing path. It returns
// (reason, false) to REJECT, or ("", true) to allow. It is called under the per-song
// apply mutex, so the HEAD it reads is the same HEAD Apply will fold into.
//
// Rule: the authenticated author may write the TARGET LAYER iff (canWriteLayer):
//   - the layer is in the CONDUCTOR zone → only a band-role conductor may write it
//     (#3); everyone else (members AND plain admins who are not conductors) is RO; or
//   - the layer is theirs (layer.OwnerID == authorId); or
//   - it is shared read-write (layer.Access == RW).
//
// A read-only non-conductor-zone layer is otherwise writable only by its owner.
//
// Target-layer resolution:
//   - create:                  the mutation's object.layerId (the layer it lands on).
//   - move/resize/setStyle/
//     setText/delete/restore:   the layer of the EXISTING object (by uuid) in HEAD.
//   - layerCreate:              a PERSONAL-zone layer must be owned by the author; a
//     CONDUCTOR-zone layer may only be created by a conductor-role author (#3); other
//     zones (shared) pass.
//   - layerUpdate:              authorized to the layer OWNER or a band ADMIN (#4) —
//     this is the lock/unlock (access) toggle. The conductor zone is role-governed,
//     so a conductor-zone layerUpdate additionally requires the conductor role.
//   - reorder/delete:           not object-affecting and not gated here (left as-is).
//
// A target layer that does not exist rejects "stale" (matching today's unknown-target
// handling), EXCEPT create, whose layer may legitimately be brand-new and unknown to
// HEAD — an unknown create target is allowed (the layer is provisioned out of band).
func (c *conn) authorizeWrite(kind domain.Kind, in mutationJSON) (string, bool) {
	switch kind {
	case domain.KindLayerCreate:
		if in.Layer == nil {
			return "", true
		}
		zone := zoneFromString(in.Layer.Zone)
		// Personal-zone layers must be owned by their creator.
		if zone == domain.ZonePersonal && in.Layer.OwnerID != c.authorID {
			return "forbidden", false
		}
		// Conductor-zone layers may only be created by a conductor-role author (#3).
		if zone == domain.ZoneConductor && c.role != roleConductor {
			return "forbidden", false
		}
		return "", true

	case domain.KindLayerUpdate:
		// Lock/unlock (access) toggle (#4): authorized to the layer OWNER or a band
		// ADMIN. Resolve the EXISTING layer in HEAD by id; an unknown layer is stale.
		layerID := ""
		if in.Layer != nil {
			layerID = in.Layer.ID
		}
		layer, ok := c.hub.eng.Layer(c.songID, layerID)
		if !ok {
			return "stale", false
		}
		// A conductor-zone layer's access is role-governed: only a conductor may change
		// it (a plain admin who is not a conductor cannot touch the conductor zone).
		if layer.Zone == domain.ZoneConductor && c.role != roleConductor {
			return "forbidden", false
		}
		if layer.OwnerID != c.authorID && c.role != roleAdmin {
			return "forbidden", false
		}
		return "", true

	case domain.KindLayerReorder:
		// Not object-affecting (just order); not gated by this write-access rule.
		return "", true

	case domain.KindLayerDelete:
		// T83: deleting a layer cascade-tombstones its objects — destructive — so gate it exactly like
		// layerUpdate (mirror edit rights, don't invent a matrix): the layer OWNER or a band ADMIN, and
		// a conductor-zone layer requires the conductor ROLE. An RO/foreign layer is refused HERE,
		// server-side — a hidden drawer button is not enforcement.
		layerID := ""
		if in.Layer != nil {
			layerID = in.Layer.ID
		}
		layer, ok := c.hub.eng.Layer(c.songID, layerID)
		if !ok {
			return "stale", false
		}
		if layer.Zone == domain.ZoneConductor && c.role != roleConductor {
			return "forbidden", false
		}
		if layer.OwnerID != c.authorID && c.role != roleAdmin {
			return "forbidden", false
		}
		return "", true

	case domain.KindCreate:
		layerID := ""
		if in.Object != nil {
			layerID = in.Object.LayerID
		}
		layer, ok := c.hub.eng.Layer(c.songID, layerID)
		if !ok {
			// The target layer is not (yet) in HEAD; a create may land on a freshly
			// provisioned layer, so we do not block it on absence (the engine still
			// validates the mutation itself).
			return "", true
		}
		if !c.canWriteLayer(layer) {
			return "forbidden", false
		}
		if in.Object != nil && c.jumpDuplicate(objectFromJSON(*in.Object)) {
			return "jump-duplicate", false
		}
		return "", true

	default: // move, resize, setStyle, setText, delete, restore
		uuid := in.UUID
		if uuid == "" && in.Object != nil {
			uuid = in.Object.UUID
		}
		layer, layerFound, objExists := c.hub.eng.ObjectLayer(c.songID, uuid)
		if !objExists {
			// Unknown object → stale, mirroring today's handling of an edit against a
			// UUID the engine has never seen (the engine would also reject it).
			return "stale", false
		}
		if !layerFound {
			// The object exists but its layer is not materialized in HEAD (objects and
			// layers are provisioned independently). There is no layer to gate against,
			// so defer to the engine's own validity checks.
			return "", true
		}
		if !c.canWriteLayer(layer) {
			return "forbidden", false
		}
		// An EDIT can create the collision too: recolour or re-glyph a jump onto one that already exists.
		if in.Object != nil && c.jumpDuplicate(objectFromJSON(*in.Object)) {
			return "jump-duplicate", false
		}
		return "", true
	}
}

// jumpDuplicate reports whether `obj` would be a SECOND jump wearing the same (glyph, colour) as an
// existing one on the same FILE — the server-side backstop for the P206 uniqueness rule.
//
// Fable's ⟨D⟩ ruling: this is a LEGIBILITY rule, not an integrity one. Pairing is by uuid, so a collision
// corrupts nothing — the bake resolves both pairs correctly and every downstream test passes. The harm is
// to a musician meeting two identical segnos on one page mid-song, unable to tell the destination from the
// source. Studio excludes the combination in its palette; this refuses the one that gets past it (an older
// client, a script, a race between two authors), because a rule only the UI enforces is not enforced.
//
// Scoped per FILE, matching the ⟨D2⟩ rule that a jump lives within one file: two identical segnos on
// different parts are never read side by side. Keyed on the SOURCE — two pairs cannot look alike without
// their sources colliding — and colours compare case-insensitively.
func (c *conn) jumpDuplicate(obj domain.Object) bool {
	if !obj.IsJumpSource() {
		return false
	}
	fileOf := func(layerID string) (string, bool) {
		l, ok := c.hub.eng.Layer(c.songID, layerID)
		if !ok {
			return "", false
		}
		return l.FileID, true
	}
	file, ok := fileOf(obj.LayerID)
	if !ok {
		return false // no layer to scope against yet; the engine's own checks still apply
	}
	snap, err := c.hub.eng.Head(c.songID)
	if err != nil {
		return false
	}
	for _, o := range snap.LiveObjects() {
		if o.UUID == obj.UUID || !o.IsJumpSource() {
			continue
		}
		if !strings.EqualFold(o.Text, obj.Text) || !strings.EqualFold(o.Style.Color, obj.Style.Color) {
			continue
		}
		if f, ok := fileOf(o.LayerID); ok && f == file {
			return true
		}
	}
	return false
}

// canWriteLayer reports whether this connection's authenticated author may write
// objects into the given layer:
//   - conductor zone (#3): ONLY a band-role conductor, regardless of ownership/access
//     (members and plain admins see it read-only);
//   - any other zone: they own it, or it is shared read-write.
func (c *conn) canWriteLayer(layer domain.Layer) bool {
	if layer.Zone == domain.ZoneConductor {
		return c.role == roleConductor
	}
	return layer.OwnerID == c.authorID || layer.Access == domain.AccessRW
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

// storedObject is HEAD's copy of uuid (live or tombstoned), the server's side of every link decision.
func (c *conn) storedObject(uuid string) (domain.Object, bool) {
	snap, err := c.hub.eng.Head(c.songID)
	if err != nil {
		return domain.Object{}, false
	}
	for _, o := range snap.Objects {
		if o.UUID == uuid {
			return o, true
		}
	}
	return domain.Object{}, false
}

// relink is the move/resize/restore half of T195: with an anchorer, it decides; without one, the mark is
// stored unlinked (a moved mark cannot keep a link measured somewhere else).
func (c *conn) relink(o domain.Object, uuid string) domain.Object {
	stored, _ := c.storedObject(uuid)
	if c.hub.anchorer == nil {
		o.Link, o.Anchor, o.PointsRenderHash = nil, nil, ""
		return o
	}
	return c.hub.anchorer.RelinkMark(c.songID, o, stored)
}

// rebase is the setStyle/setText/reorder half: the stored link, never the client's (P26), and the stored
// geometry (P25). Without an anchorer the stored fields are copied as they are.
func (c *conn) rebase(o domain.Object, uuid string) domain.Object {
	stored, ok := c.storedObject(uuid)
	if !ok {
		o.Link, o.Anchor, o.PointsRenderHash = nil, nil, ""
		return o
	}
	if c.hub.anchorer == nil {
		o.Link, o.Anchor, o.PointsRenderHash = stored.Link.Clone(), stored.Anchor.Clone(), stored.PointsRenderHash
		if stored.Link != nil {
			o.Points, o.Page = append([]domain.Point(nil), stored.Points...), stored.Page
		}
		return o
	}
	return c.hub.anchorer.RebaseMark(c.songID, o, stored)
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
