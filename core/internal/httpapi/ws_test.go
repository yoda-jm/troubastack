package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---- wire shapes the realtime protocol speaks (must match internal/sync) ----

type wsMutation struct {
	Kind        string     `json:"kind"`
	UUID        string     `json:"uuid"`
	Object      *annObject `json:"object,omitempty"`
	Layer       *annLayer  `json:"layer,omitempty"`
	BaseVersion uint64     `json:"baseVersion,omitempty"`
	ClientTS    int64      `json:"clientTs,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Seq         uint64     `json:"seq,omitempty"`
	AuthorID    string     `json:"authorId,omitempty"`
}

type wsClientMsg struct {
	Type     string     `json:"type"`
	Mutation wsMutation `json:"mutation"`
}

// wsServerMsg is the union of snapshot|echo|reject (decoded loosely by type).
type wsServerMsg struct {
	Type     string      `json:"type"`
	Layers   []annLayer  `json:"layers"`
	Objects  []annObject `json:"objects"`
	Seq      uint64      `json:"seq"`
	Mutation wsMutation  `json:"mutation"`
	UUID     string      `json:"uuid"`
	Reason   string      `json:"reason"`
}

// sharingClient returns a second client bound to the SAME httptest server (hence the
// same sync hub + apply engine) as c, but with its own cookie jar. Realtime fan-out
// only works when both clients hit one server, so multi-client tests use this rather
// than newClient (which spins up an isolated server/hub per client).
func (c *client) sharingClient() *client {
	return &client{t: c.t, srv: c.srv}
}

// dialWS opens a WebSocket to the song's /ws endpoint carrying the client's session
// cookie. On a rejected upgrade it returns the HTTP status (resp non-nil, ws nil).
func (c *client) dialWS(bandID, songID string) (*websocket.Conn, *http.Response, error) {
	c.t.Helper()
	url := "ws" + strings.TrimPrefix(c.srv.URL, "http") +
		"/api/bands/" + bandID + "/songs/" + songID + "/ws"

	jar, _ := cookiejar.New(nil)
	dialer := websocket.Dialer{Jar: jar, HandshakeTimeout: 5 * time.Second}
	hdr := http.Header{}
	for _, ck := range c.jar {
		hdr.Add("Cookie", ck.Name+"="+ck.Value)
	}
	return dialer.Dial(url, hdr)
}

// readMsg reads one server frame with a deadline and decodes it.
func readMsg(t *testing.T, ws *websocket.Conn) wsServerMsg {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	var m wsServerMsg
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode ws message %q: %v", data, err)
	}
	return m
}

// sendMut writes a mutation envelope.
func sendMut(t *testing.T, ws *websocket.Conn, m wsMutation) {
	t.Helper()
	if err := ws.WriteJSON(wsClientMsg{Type: "mutation", Mutation: m}); err != nil {
		t.Fatalf("write mutation: %v", err)
	}
}

// expectSnapshot reads and asserts the first frame is a snapshot.
func expectSnapshot(t *testing.T, ws *websocket.Conn) wsServerMsg {
	t.Helper()
	m := readMsg(t, ws)
	if m.Type != "snapshot" {
		t.Fatalf("first frame type = %q, want snapshot", m.Type)
	}
	return m
}

// meID returns the caller's own user id (the authoritative authorId).
func (c *client) meID() string {
	c.t.Helper()
	_, body := c.do(http.MethodGet, "/api/me", nil)
	var u struct {
		ID string `json:"id"`
	}
	unmarshalField(c.t, body, "user", &u)
	return u.ID
}

// sampleObject is a minimal freehand object on the shared layer.
func sampleObject(uuid string) *annObject {
	return &annObject{
		UUID:    uuid,
		LayerID: "L-shared",
		Type:    "freehand",
		Page:    0,
		Points:  []annPoint{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}},
		Style:   annStyle{Color: "#112233", Opacity: 1, Width: 0.01},
	}
}

// TestWSCreateBroadcastAndPersist: two clients in the same song; A creates → BOTH
// receive an echo with a server seq and the correct authorId; the engine HEAD (and
// the REST GET) then includes the object.
func TestWSCreateBroadcastAndPersist(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")
	aliceID := alice.meID()

	// Bob joins the same band so he can connect to the song's room. He must share
	// alice's server (one hub + one engine) for the broadcast to reach him.
	inviteMember(t, alice, band, "bob")
	bob := alice.sharingClient()
	bob.registerLogin("bob", "pw-bob")
	acceptOnlyInvite(t, bob)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	wsB, _, err := bob.dialWS(band, song)
	if err != nil {
		t.Fatalf("bob dial: %v", err)
	}
	defer wsB.Close()
	expectSnapshot(t, wsB)

	sendMut(t, wsA, wsMutation{Kind: "create", UUID: "o-1", Object: sampleObject("o-1"), AuthorID: "SPOOFED", ClientTS: 1720000000000})

	for _, pair := range []struct {
		name string
		ws   *websocket.Conn
	}{{"alice", wsA}, {"bob", wsB}} {
		m := readMsg(t, pair.ws)
		if m.Type != "echo" {
			t.Fatalf("%s: type = %q, want echo", pair.name, m.Type)
		}
		if m.Mutation.Seq == 0 {
			t.Fatalf("%s: echo has no server seq", pair.name)
		}
		if m.Mutation.AuthorID != aliceID {
			t.Fatalf("%s: authorId = %q, want %q (spoofed client value must be ignored)", pair.name, m.Mutation.AuthorID, aliceID)
		}
		if m.Mutation.UUID != "o-1" {
			t.Fatalf("%s: echo uuid = %q, want o-1", pair.name, m.Mutation.UUID)
		}
		// createdAt is server-stamped from the author's clientTs (z-order tiebreak, T27).
		if m.Mutation.Object == nil || m.Mutation.Object.CreatedAt != 1720000000000 {
			t.Fatalf("%s: echo createdAt = %v, want 1720000000000", pair.name, m.Mutation.Object)
		}
	}

	// REST GET (same engine) now reflects the create.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 1 || got.Objects[0].UUID != "o-1" {
		t.Fatalf("REST HEAD missing the created object: %+v", got.Objects)
	}
}

// TestWSJoinSnapshot: a client joining after objects exist gets them in the snapshot.
func TestWSJoinSnapshot(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")

	// Seed via import (same engine) so the snapshot has prior layers + objects.
	in := sampleDoc("file-1")
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", in)
	mustStatus(t, resp, http.StatusOK)

	ws, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()
	snap := expectSnapshot(t, ws)
	if len(snap.Layers) != 3 {
		t.Fatalf("snapshot layers = %d, want 3", len(snap.Layers))
	}
	if len(snap.Objects) != 7 {
		t.Fatalf("snapshot objects = %d, want 7", len(snap.Objects))
	}
	if snap.Seq == 0 {
		t.Fatal("snapshot seq should be > 0 after an import")
	}
}

// TestWSDeleteThenMoveRejected: delete an object, then move it → sender gets a reject
// "deleted-remotely"; restore revives it (accepted echo).
func TestWSDeleteThenMoveRejected(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")

	ws, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()
	expectSnapshot(t, ws)

	// create
	sendMut(t, ws, wsMutation{Kind: "create", UUID: "o-1", Object: sampleObject("o-1")})
	if m := readMsg(t, ws); m.Type != "echo" {
		t.Fatalf("create: type = %q, want echo", m.Type)
	}
	// delete
	sendMut(t, ws, wsMutation{Kind: "delete", UUID: "o-1"})
	if m := readMsg(t, ws); m.Type != "echo" {
		t.Fatalf("delete: type = %q, want echo", m.Type)
	}
	// move a deleted object → reject deleted-remotely
	sendMut(t, ws, wsMutation{Kind: "move", UUID: "o-1", Object: sampleObject("o-1")})
	rej := readMsg(t, ws)
	if rej.Type != "reject" {
		t.Fatalf("move-after-delete: type = %q, want reject", rej.Type)
	}
	if rej.Reason != "deleted-remotely" {
		t.Fatalf("reject reason = %q, want deleted-remotely", rej.Reason)
	}
	if rej.UUID != "o-1" {
		t.Fatalf("reject uuid = %q, want o-1", rej.UUID)
	}
	// restore revives
	sendMut(t, ws, wsMutation{Kind: "restore", UUID: "o-1", Object: sampleObject("o-1")})
	if m := readMsg(t, ws); m.Type != "echo" || m.Mutation.Kind != "restore" {
		t.Fatalf("restore: got type=%q kind=%q, want echo/restore", m.Type, m.Mutation.Kind)
	}

	// HEAD now has the live object again.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 1 {
		t.Fatalf("after restore HEAD should have 1 live object, got %d", len(got.Objects))
	}
}

// TestWSPerSongIsolation: a client on song X does not receive song Y's echoes.
func TestWSPerSongIsolation(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, songX := alice.makeBandSong("alice")
	// Second song in the same band.
	var songY struct{ ID string }
	_, body := alice.do(http.MethodPost, "/api/bands/"+band+"/songs", map[string]string{"title": "Y"})
	unmarshalField(t, body, "song", &songY)

	wsX, _, err := alice.dialWS(band, songX)
	if err != nil {
		t.Fatalf("dial X: %v", err)
	}
	defer wsX.Close()
	expectSnapshot(t, wsX)

	wsY, _, err := alice.dialWS(band, songY.ID)
	if err != nil {
		t.Fatalf("dial Y: %v", err)
	}
	defer wsY.Close()
	expectSnapshot(t, wsY)

	// Create on Y. X must NOT see it.
	sendMut(t, wsY, wsMutation{Kind: "create", UUID: "y-1", Object: sampleObject("y-1")})
	if m := readMsg(t, wsY); m.Type != "echo" {
		t.Fatalf("Y create: type = %q, want echo", m.Type)
	}

	// X should time out (no frame). A short read deadline proves isolation.
	_ = wsX.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if _, _, err := wsX.ReadMessage(); err == nil {
		t.Fatal("song X received a frame from song Y (isolation broken)")
	}
}

// TestWSUnauthenticatedRejected: no session cookie → upgrade rejected (401).
func TestWSUnauthenticatedRejected(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")

	anon := alice.sharingClient() // shares the server; no cookie set
	_, resp, err := anon.dialWS(band, song)
	if err == nil {
		t.Fatal("unauthenticated upgrade should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upgrade status = %v, want 401", resp)
	}
}

// TestWSNonMemberRejected: a logged-in non-member → upgrade rejected (403).
func TestWSNonMemberRejected(t *testing.T) {
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")

	mallory := alice.sharingClient()
	mallory.registerLogin("mallory", "pw")
	_, resp, err := mallory.dialWS(band, song)
	if err == nil {
		t.Fatal("non-member upgrade should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member upgrade status = %v, want 403", resp)
	}
}

// ---- LAYER WRITE-ACCESS (hub-only) ----

// twoMemberBand sets up a band with admin "alice" and member "bob" who both share one
// server (one hub + engine). It returns (bandID, songID, alice, bob, bobID).
func twoMemberBand(t *testing.T) (string, string, *client, *client, string) {
	t.Helper()
	repo := backends()[0].make(t)
	alice := newClient(t, repo)
	band, song := alice.makeBandSong("alice")
	inviteMember(t, alice, band, "bob")
	bob := alice.sharingClient()
	bob.registerLogin("bob", "pw-bob")
	acceptOnlyInvite(t, bob)
	return band, song, alice, bob, bob.meID()
}

// importLayers bulk-imports just the given layers (no objects) via the REST seed path,
// which is intentionally permissive about layer ownership.
func (c *client) importLayers(band, song string, layers ...annLayer) {
	c.t.Helper()
	resp, _ := c.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{Layers: layers, Objects: []annObject{}})
	mustStatus(c.t, resp, http.StatusOK)
}

// objOn is a minimal freehand object pinned to a specific layer.
func objOn(uuid, layerID string) *annObject {
	o := sampleObject(uuid)
	o.LayerID = layerID
	return o
}

// TestWSCreateForbiddenOnForeignROLayer: a RO layer owned by Bob; Alice (a member who
// is NOT the owner) creates into it → Alice gets reject "forbidden", HEAD unchanged,
// nothing broadcast (Bob, also connected, sees no echo).
func TestWSCreateForbiddenOnForeignROLayer(t *testing.T) {
	band, song, alice, bob, bobID := twoMemberBand(t)
	// Bob owns a read-only personal layer; the import path provisions it for him.
	alice.importLayers(band, song,
		annLayer{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"})

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	wsB, _, err := bob.dialWS(band, song)
	if err != nil {
		t.Fatalf("bob dial: %v", err)
	}
	defer wsB.Close()
	expectSnapshot(t, wsB)

	sendMut(t, wsA, wsMutation{Kind: "create", UUID: "o-x", Object: objOn("o-x", "L-bob-ro")})
	rej := readMsg(t, wsA)
	if rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("alice create into bob's RO layer: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}
	if rej.UUID != "o-x" {
		t.Fatalf("reject uuid = %q, want o-x", rej.UUID)
	}

	// Nothing broadcast: Bob (owner, connected) must NOT see an echo.
	_ = wsB.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	if _, _, err := wsB.ReadMessage(); err == nil {
		t.Fatal("forbidden create must not broadcast (bob received a frame)")
	}

	// HEAD unchanged: REST GET shows no objects.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 0 {
		t.Fatalf("forbidden create must not change HEAD: %+v", got.Objects)
	}
}

// TestWSOwnerCreateOnOwnROLayer: the OWNER (Bob) creating into his own RO layer is
// accepted and broadcast.
func TestWSOwnerCreateOnOwnROLayer(t *testing.T) {
	band, song, alice, bob, bobID := twoMemberBand(t)
	alice.importLayers(band, song,
		annLayer{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"})

	wsB, _, err := bob.dialWS(band, song)
	if err != nil {
		t.Fatalf("bob dial: %v", err)
	}
	defer wsB.Close()
	expectSnapshot(t, wsB)

	sendMut(t, wsB, wsMutation{Kind: "create", UUID: "o-own", Object: objOn("o-own", "L-bob-ro")})
	m := readMsg(t, wsB)
	if m.Type != "echo" || m.Mutation.UUID != "o-own" {
		t.Fatalf("owner create into own RO layer: type=%q uuid=%q, want echo/o-own", m.Type, m.Mutation.UUID)
	}
}

// TestWSCreateAllowedOnSharedRWLayer: any member may create into a shared RW layer.
func TestWSCreateAllowedOnSharedRWLayer(t *testing.T) {
	band, song, alice, _, _ := twoMemberBand(t)
	alice.importLayers(band, song,
		annLayer{ID: "L-shared", FileID: "f1", Name: "Shared", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"})

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	sendMut(t, wsA, wsMutation{Kind: "create", UUID: "o-rw", Object: objOn("o-rw", "L-shared")})
	m := readMsg(t, wsA)
	if m.Type != "echo" || m.Mutation.UUID != "o-rw" {
		t.Fatalf("create into shared RW layer: type=%q uuid=%q, want echo/o-rw", m.Type, m.Mutation.UUID)
	}
}

// TestWSEditForbiddenOnForeignROLayer: an object already living in Bob's RO layer;
// Alice (non-owner) moves it, then deletes it → both rejected "forbidden".
func TestWSEditForbiddenOnForeignROLayer(t *testing.T) {
	band, song, alice, _, bobID := twoMemberBand(t)
	// Seed both the RO layer AND an object on it via the (permissive) import path, so
	// the target object exists in HEAD before Alice tries to edit it.
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{
			Layers:  []annLayer{{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"}},
			Objects: []annObject{*objOn("o-bob", "L-bob-ro")},
		})
	mustStatus(t, resp, http.StatusOK)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	// Move (non-owner) → forbidden.
	sendMut(t, wsA, wsMutation{Kind: "move", UUID: "o-bob", Object: objOn("o-bob", "L-bob-ro")})
	if rej := readMsg(t, wsA); rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("alice move in bob's RO layer: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}
	// Delete (non-owner) → forbidden.
	sendMut(t, wsA, wsMutation{Kind: "delete", UUID: "o-bob"})
	if rej := readMsg(t, wsA); rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("alice delete in bob's RO layer: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}

	// HEAD unchanged: the object is still live and on its layer.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 1 || got.Objects[0].UUID != "o-bob" {
		t.Fatalf("forbidden edits must not change HEAD: %+v", got.Objects)
	}
}

// TestWSReorderOwnObjectPersists: an object on a shared-RW layer; a member sends a
// `reorder` mutation → the echo carries the new order with a server seq, and the
// engine HEAD (REST GET) reflects it. Exercises the full gated reorder path (T27).
func TestWSReorderOwnObjectPersists(t *testing.T) {
	band, song, alice, _, _ := twoMemberBand(t)
	// Seed a shared-RW layer + an object on it (import is permissive).
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{
			Layers:  []annLayer{{ID: "L-shared", FileID: "f1", Name: "Shared", OwnerID: "_shared_", Zone: "shared", Order: 0, Access: "rw"}},
			Objects: []annObject{*objOn("o-1", "L-shared")},
		})
	mustStatus(t, resp, http.StatusOK)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	// Bring-to-front: reorder carries the full object with the new order (like setStyle).
	ro := objOn("o-1", "L-shared")
	ro.Order = 7
	sendMut(t, wsA, wsMutation{Kind: "reorder", UUID: "o-1", Object: ro})

	echo := readMsg(t, wsA)
	if echo.Type != "echo" {
		t.Fatalf("reorder: type = %q, want echo", echo.Type)
	}
	if echo.Mutation.Seq == 0 {
		t.Fatalf("reorder echo has no server seq")
	}
	if echo.Mutation.Object == nil || echo.Mutation.Object.Order != 7 {
		t.Fatalf("reorder echo must carry the new order: %+v", echo.Mutation.Object)
	}

	// REST HEAD reflects the new order.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 1 || got.Objects[0].Order != 7 {
		t.Fatalf("HEAD must reflect the reordered object: %+v", got.Objects)
	}
}

// TestWSReorderForbiddenOnForeignROLayer: a member reordering an object on a foreign
// RO layer is rejected "forbidden" (same gate as move/resize), HEAD unchanged.
func TestWSReorderForbiddenOnForeignROLayer(t *testing.T) {
	band, song, alice, _, bobID := twoMemberBand(t)
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{
			Layers:  []annLayer{{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"}},
			Objects: []annObject{*objOn("o-bob", "L-bob-ro")},
		})
	mustStatus(t, resp, http.StatusOK)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	ro := objOn("o-bob", "L-bob-ro")
	ro.Order = 9
	sendMut(t, wsA, wsMutation{Kind: "reorder", UUID: "o-bob", Object: ro})
	if rej := readMsg(t, wsA); rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("alice reorder in bob's RO layer: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}

	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Objects) != 1 || got.Objects[0].Order != 0 {
		t.Fatalf("forbidden reorder must not change HEAD order: %+v", got.Objects)
	}
}

// TestImportRequiresAdmin (T08): import is an admin-only bulk/seed tool. A band
// ADMIN may provision layers+objects for an arbitrary owner (Bob) even into a
// read-only layer — that permissive seeding is intentional. But a non-admin
// MEMBER can no longer use import to bypass the live-editing write gate: their
// import is rejected 403, so it cannot write a locked/foreign layer the WS path
// would reject for them.
func TestImportRequiresAdmin(t *testing.T) {
	band, song, alice, bob, bobID := twoMemberBand(t)

	// Admin (Alice) import is permissive: provisions Bob's RO layer + an object.
	doc := annDoc{
		Layers:  []annLayer{{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"}},
		Objects: []annObject{*objOn("o-seed", "L-bob-ro")},
	}
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", doc)
	mustStatus(t, resp, http.StatusOK)

	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Layers) != 1 || got.Layers[0].OwnerID != bobID {
		t.Fatalf("admin import should provision bob's layer: %+v", got.Layers)
	}
	if len(got.Objects) != 1 || got.Objects[0].LayerID != "L-bob-ro" {
		t.Fatalf("admin import should provision the object on bob's RO layer: %+v", got.Objects)
	}

	// Non-admin member (Bob) import is forbidden — closes the escalation gap.
	resp, _ = bob.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{
			Layers:  []annLayer{{ID: "L-bob2", FileID: "f1", Name: "Bob 2", OwnerID: bobID, Zone: "personal", Order: 1, Access: "rw"}},
			Objects: []annObject{*objOn("o-bob", "L-bob2")},
		})
	mustStatus(t, resp, http.StatusForbidden)
}

// ---- #3 CONDUCTOR ZONE (role-governed write access) ----

// setRole has the band admin set a member's role ("conductor"|"admin"|"member").
func setRole(t *testing.T, admin *client, bandID, userID, role string) {
	t.Helper()
	resp, _ := admin.do(http.MethodPatch, "/api/bands/"+bandID+"/members/"+userID,
		map[string]string{"role": role})
	mustStatus(t, resp, http.StatusOK)
}

// conductorBand sets up a band with admin "alice" plus members "carol" (promoted to
// the conductor role) and "bob" (plain member). It imports a conductor-zone layer
// owned by _shared_. Returns (band, song, alice, carol, bob).
func conductorBand(t *testing.T) (band, song string, alice, carol, bob *client) {
	t.Helper()
	repo := backends()[0].make(t)
	alice = newClient(t, repo)
	band, song = alice.makeBandSong("alice")

	inviteMember(t, alice, band, "carol")
	carol = alice.sharingClient()
	carol.registerLogin("carol", "pw-carol")
	acceptOnlyInvite(t, carol)
	setRole(t, alice, band, carol.meID(), "conductor")

	inviteMember(t, alice, band, "bob")
	bob = alice.sharingClient()
	bob.registerLogin("bob", "pw-bob")
	acceptOnlyInvite(t, bob)

	// A conductor-zone layer (owned by _shared_, RO). Write is role-governed.
	alice.importLayers(band, song,
		annLayer{ID: "L-cond", FileID: "f1", Name: "Cues", OwnerID: "_shared_", Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"})
	return band, song, alice, carol, bob
}

// TestWSConductorZoneWritableByConductorRole: a conductor-role author may create into
// the conductor zone (accepted echo).
func TestWSConductorZoneWritableByConductorRole(t *testing.T) {
	band, song, _, carol, _ := conductorBand(t)
	ws, _, err := carol.dialWS(band, song)
	if err != nil {
		t.Fatalf("carol dial: %v", err)
	}
	defer ws.Close()
	expectSnapshot(t, ws)

	sendMut(t, ws, wsMutation{Kind: "create", UUID: "c-1", Object: objOn("c-1", "L-cond")})
	m := readMsg(t, ws)
	if m.Type != "echo" || m.Mutation.UUID != "c-1" {
		t.Fatalf("conductor create into conductor zone: type=%q uuid=%q, want echo/c-1", m.Type, m.Mutation.UUID)
	}
}

// TestWSConductorZoneForbiddenForMemberAndAdmin: a plain member AND a plain admin
// (who is NOT a conductor) are both rejected "forbidden" writing the conductor zone.
func TestWSConductorZoneForbiddenForMemberAndAdmin(t *testing.T) {
	band, song, alice, _, bob := conductorBand(t)

	for _, tc := range []struct {
		name string
		c    *client
		uuid string
	}{
		{"member-bob", bob, "m-1"},
		{"admin-alice", alice, "a-1"},
	} {
		ws, _, err := tc.c.dialWS(band, song)
		if err != nil {
			t.Fatalf("%s dial: %v", tc.name, err)
		}
		expectSnapshot(t, ws)
		sendMut(t, ws, wsMutation{Kind: "create", UUID: tc.uuid, Object: objOn(tc.uuid, "L-cond")})
		rej := readMsg(t, ws)
		if rej.Type != "reject" || rej.Reason != "forbidden" {
			t.Fatalf("%s create into conductor zone: type=%q reason=%q, want reject/forbidden", tc.name, rej.Type, rej.Reason)
		}
		ws.Close()
	}
}

// ---- #4 LAYER ACCESS (lock/unlock) ----

// layerUpdateAccess sends a layerUpdate mutation that flips a layer's access. The
// layer fields must match the existing layer except for the new access value.
func layerUpdateAccess(layer annLayer, access string) wsMutation {
	layer.Access = access
	return wsMutation{Kind: "layerUpdate", Layer: &layer}
}

// TestWSLayerLockBlocksOthersThenUnlock: a shared layer Alice owns is RW; Bob can edit
// an object on it. Alice locks it (access=ro) → Bob's edit is rejected forbidden;
// Alice unlocks it (access=rw) → Bob's edit is accepted again.
func TestWSLayerLockBlocksOthersThenUnlock(t *testing.T) {
	band, song, alice, bob, _ := twoMemberBand(t)
	sharedLayer := annLayer{ID: "L-shared", FileID: "f1", Name: "Shared", OwnerID: alice.meID(), Zone: "shared", Order: 0, Access: "rw"}
	// Seed the shared layer + an object on it (permissive import).
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{Layers: []annLayer{sharedLayer}, Objects: []annObject{*objOn("o-sh", "L-shared")}})
	mustStatus(t, resp, http.StatusOK)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)
	wsB, _, err := bob.dialWS(band, song)
	if err != nil {
		t.Fatalf("bob dial: %v", err)
	}
	defer wsB.Close()
	expectSnapshot(t, wsB)

	// While RW, Bob can move the object.
	sendMut(t, wsB, wsMutation{Kind: "move", UUID: "o-sh", Object: objOn("o-sh", "L-shared")})
	if m := readMsg(t, wsB); m.Type != "echo" {
		t.Fatalf("bob move on RW shared layer: type=%q, want echo", m.Type)
	}
	readMsg(t, wsA) // drain Alice's echo of the move

	// Alice (owner) locks the layer → access ro.
	sendMut(t, wsA, layerUpdateAccess(sharedLayer, "ro"))
	if m := readMsg(t, wsA); m.Type != "echo" || m.Mutation.Kind != "layerUpdate" {
		t.Fatalf("alice lock layer: type=%q kind=%q, want echo/layerUpdate", m.Type, m.Mutation.Kind)
	}
	readMsg(t, wsB) // bob also sees the layerUpdate echo

	// Now Bob's edit is rejected forbidden.
	sendMut(t, wsB, wsMutation{Kind: "move", UUID: "o-sh", Object: objOn("o-sh", "L-shared")})
	if rej := readMsg(t, wsB); rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("bob move on locked layer: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}

	// Alice unlocks → access rw.
	sendMut(t, wsA, layerUpdateAccess(sharedLayer, "rw"))
	if m := readMsg(t, wsA); m.Type != "echo" {
		t.Fatalf("alice unlock layer: type=%q, want echo", m.Type)
	}
	readMsg(t, wsB)

	// Bob can edit again.
	sendMut(t, wsB, wsMutation{Kind: "move", UUID: "o-sh", Object: objOn("o-sh", "L-shared")})
	if m := readMsg(t, wsB); m.Type != "echo" {
		t.Fatalf("bob move after unlock: type=%q, want echo", m.Type)
	}
}

// TestWSLayerAccessForbiddenForNonOwnerNonAdmin: a plain member (not owner, not admin)
// cannot change a shared layer's access.
func TestWSLayerAccessForbiddenForNonOwnerNonAdmin(t *testing.T) {
	band, song, alice, bob, _ := twoMemberBand(t)
	sharedLayer := annLayer{ID: "L-shared", FileID: "f1", Name: "Shared", OwnerID: alice.meID(), Zone: "shared", Order: 0, Access: "rw"}
	alice.importLayers(band, song, sharedLayer)

	wsB, _, err := bob.dialWS(band, song)
	if err != nil {
		t.Fatalf("bob dial: %v", err)
	}
	defer wsB.Close()
	expectSnapshot(t, wsB)

	sendMut(t, wsB, layerUpdateAccess(sharedLayer, "ro"))
	if rej := readMsg(t, wsB); rej.Type != "reject" || rej.Reason != "forbidden" {
		t.Fatalf("bob change layer access: type=%q reason=%q, want reject/forbidden", rej.Type, rej.Reason)
	}
}

// TestWSLayerAccessAllowedForAdmin: a band admin (not the layer owner) MAY change a
// shared layer's access.
func TestWSLayerAccessAllowedForAdmin(t *testing.T) {
	band, song, alice, _, bobID := twoMemberBand(t)
	// Bob owns the shared layer; Alice is the band admin (band creator).
	sharedLayer := annLayer{ID: "L-shared", FileID: "f1", Name: "Shared", OwnerID: bobID, Zone: "shared", Order: 0, Access: "rw"}
	alice.importLayers(band, song, sharedLayer)

	wsA, _, err := alice.dialWS(band, song)
	if err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	defer wsA.Close()
	expectSnapshot(t, wsA)

	sendMut(t, wsA, layerUpdateAccess(sharedLayer, "ro"))
	if m := readMsg(t, wsA); m.Type != "echo" || m.Mutation.Kind != "layerUpdate" {
		t.Fatalf("admin change layer access: type=%q kind=%q, want echo/layerUpdate", m.Type, m.Mutation.Kind)
	}
}

// ---- membership helpers (invite + accept) ----

// inviteMember has the band admin invite username by username (pending invite).
func inviteMember(t *testing.T, admin *client, bandID, username string) {
	t.Helper()
	resp, _ := admin.do(http.MethodPost, "/api/bands/"+bandID+"/invites",
		map[string]string{"identifier": username, "kind": "username"})
	mustStatus(t, resp, http.StatusCreated)
}

// acceptOnlyInvite accepts the caller's single pending invite.
func acceptOnlyInvite(t *testing.T, c *client) {
	t.Helper()
	_, body := c.do(http.MethodGet, "/api/invites", nil)
	var invites []struct{ ID string }
	unmarshalField(t, body, "invites", &invites)
	if len(invites) != 1 {
		t.Fatalf("expected exactly 1 pending invite, got %d", len(invites))
	}
	resp, _ := c.do(http.MethodPost, "/api/invites/"+invites[0].ID+"/accept", nil)
	mustStatus(t, resp, http.StatusOK)
}
