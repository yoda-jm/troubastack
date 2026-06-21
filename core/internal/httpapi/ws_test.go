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

	sendMut(t, wsA, wsMutation{Kind: "create", UUID: "o-1", Object: sampleObject("o-1"), AuthorID: "SPOOFED"})

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
	if len(snap.Objects) != 6 {
		t.Fatalf("snapshot objects = %d, want 6", len(snap.Objects))
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

// TestImportNotBlockedByWriteAccess: the seed/import REST path remains permissive — it
// provisions layers AND objects for an arbitrary owner (Bob) even though the caller
// (Alice) is not that owner and the layer is read-only. Import bypasses the hub gate.
func TestImportNotBlockedByWriteAccess(t *testing.T) {
	band, song, alice, _, bobID := twoMemberBand(t)
	resp, _ := alice.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import",
		annDoc{
			Layers:  []annLayer{{ID: "L-bob-ro", FileID: "f1", Name: "Bob RO", OwnerID: bobID, Zone: "personal", Order: 0, Access: "ro"}},
			Objects: []annObject{*objOn("o-seed", "L-bob-ro")},
		})
	mustStatus(t, resp, http.StatusOK)

	// The object landed on Bob's RO layer despite Alice being a non-owner.
	resp, body := alice.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
	mustStatus(t, resp, http.StatusOK)
	var got annDoc
	unmarshalField2(t, body, &got)
	if len(got.Layers) != 1 || got.Layers[0].OwnerID != bobID {
		t.Fatalf("import should provision bob's layer: %+v", got.Layers)
	}
	if len(got.Objects) != 1 || got.Objects[0].LayerID != "L-bob-ro" {
		t.Fatalf("import should provision the object on bob's RO layer: %+v", got.Objects)
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
