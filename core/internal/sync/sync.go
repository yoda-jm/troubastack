package sync

import (
	"sync"

	"troubastack/core/internal/domain"
)

// Auth is the membership/session gate the Hub consults before admitting a
// WebSocket. It mirrors EXACTLY the checks the annotation REST routes already use,
// just expressed as a narrow interface so the hub does not import the relational
// app/httpapi layers (boundary, doc.go). httpapi adapts app.Service to it.
//
//   - UserForToken resolves the session-cookie token to the authoritative user id
//     (the authorId of every mutation that connection sends). Empty/invalid → error.
//   - SongForMember enforces that user is a member of the band AND that the song
//     belongs to that band, returning the engine songID (the relational Song.ID).
//     A non-member or a song/band mismatch returns an error → the upgrade is rejected.
type Auth interface {
	UserForToken(token string) (userID string, err error)
	SongForMember(userID, bandID, songID string) (songID2 string, err error)
}

// Engine is the apply authority the Hub drives (internal/engine satisfies it).
// The SAME instance backs the REST `GET …/annotations` read, so HEAD is consistent
// across the realtime and request/response surfaces.
type Engine interface {
	// Apply serializes one mutation per song, resolves LWW, assigns a seq, and
	// folds it into HEAD. It returns the ACCEPTED mutation (with server seq) or a
	// typed error (tombstoned target / stale version).
	Apply(songID string, m domain.Mutation) (domain.Mutation, error)
	// Head returns the materialized HEAD for the snapshot sent on join.
	Head(songID string) (domain.Snapshot, error)
}

// Hub fans realtime annotation traffic across clients and drives ONE apply engine
// per song. Rooms are keyed by the engine songID (the relational Song.ID).
//
// Design:
//   - rooms: one *room per active song, created lazily on first join, dropped when
//     its last connection leaves.
//   - per connection: a read pump (decode inbound client frames → apply) and a
//     write pump (drain a per-connection send channel → socket). The two-pump split
//     is the gorilla idiom: exactly one goroutine writes a socket, one reads it.
//   - per-song serialization: the engine is the single writer (its own per-song
//     mutex). The hub additionally guards each room's connection set with a mutex so
//     register/unregister/broadcast never races the pumps.
//
// The hub holds NO UI logic: it accepts mutations and broadcasts echoes (I6).
type Hub struct {
	eng  Engine
	auth Auth

	mu         sync.Mutex
	rooms      map[string]*room  // keyed by engine songID
	applyLocks map[string]*muRef // per-song apply serialization (read-version → Apply)
}

// muRef is a per-song mutex guarding the hub's read-HEAD-then-Apply window so the
// server-assigned object version is monotonic under concurrent senders.
type muRef struct{ m sync.Mutex }

func (r *muRef) Lock()   { r.m.Lock() }
func (r *muRef) Unlock() { r.m.Unlock() }

// NewHub builds a Hub over the shared apply engine and the membership/session gate.
func NewHub(eng Engine, auth Auth) *Hub {
	return &Hub{eng: eng, auth: auth, rooms: map[string]*room{}, applyLocks: map[string]*muRef{}}
}

// New returns a Hub with nil dependencies (kept for callers that wire later). Prefer
// NewHub. A nil-dependency Hub cannot serve connections.
func New() *Hub { return &Hub{rooms: map[string]*room{}, applyLocks: map[string]*muRef{}} }

// room is one song's live membership: the set of connected clients. Its mutex guards
// the conns set; the engine (not this mutex) serializes the actual applies.
type room struct {
	songID string
	mu     sync.Mutex
	conns  map[*conn]struct{}
}

// register adds a connection to a song's room, creating the room on first join.
func (h *Hub) register(c *conn) {
	h.mu.Lock()
	r := h.rooms[c.songID]
	if r == nil {
		r = &room{songID: c.songID, conns: map[*conn]struct{}{}}
		h.rooms[c.songID] = r
	}
	h.mu.Unlock()

	r.mu.Lock()
	r.conns[c] = struct{}{}
	r.mu.Unlock()
	c.room = r
}

// unregister removes a connection and garbage-collects an empty room.
func (h *Hub) unregister(c *conn) {
	r := c.room
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.conns, c)
	empty := len(r.conns) == 0
	r.mu.Unlock()

	if empty {
		h.mu.Lock()
		// Re-check under the hub lock: a late joiner may have repopulated it.
		if cur := h.rooms[r.songID]; cur == r {
			cur.mu.Lock()
			if len(cur.conns) == 0 {
				delete(h.rooms, r.songID)
			}
			cur.mu.Unlock()
		}
		h.mu.Unlock()
	}
}

// broadcast sends frame to every connection in the room (incl. the sender, I6).
// A connection whose send buffer is full is dropped (slow consumer): its send
// channel is closed, which ends its write pump and tears the connection down.
func (r *room) broadcast(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.conns {
		select {
		case c.send <- frame:
		default:
			close(c.send)
			delete(r.conns, c)
			c.dropped = true
		}
	}
}
