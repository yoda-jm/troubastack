package sync

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// ---- wire protocol (JSON text frames) ----
//
// The frontend matches this EXACTLY. Client → server carries a single envelope;
// server → client carries one of three envelopes (snapshot | echo | reject).

// clientMsg is the inbound envelope. Only "mutation" is accepted today.
type clientMsg struct {
	Type     string       `json:"type"` // "mutation"
	Mutation mutationJSON `json:"mutation"`
}

// snapshotMsg is sent once on connect: the full materialized HEAD + the current seq.
type snapshotMsg struct {
	Type    string       `json:"type"` // "snapshot"
	Layers  []layerJSON  `json:"layers"`
	Objects []objectJSON `json:"objects"`
	Seq     uint64       `json:"seq"`
}

// echoMsg broadcasts an accepted mutation to the whole room (incl. the sender). The
// mutation carries the server-assigned seq + the authoritative authorId.
type echoMsg struct {
	Type     string       `json:"type"` // "echo"
	Mutation mutationJSON `json:"mutation"`
}

// rejectMsg is sent to the SENDER only when the engine refuses a mutation.
type rejectMsg struct {
	Type   string `json:"type"`   // "reject"
	UUID   string `json:"uuid"`   // the target object uuid
	Reason string `json:"reason"` // "deleted-remotely" | "stale" | "forbidden"
}

// mutationJSON is the wire mutation. kind is the lowerCamel string; object/layer use
// the SAME shapes as the annotations REST API. authorId is server-authoritative on
// echo (the client's value is ignored on input). seq is 0 on input, set on echo.
type mutationJSON struct {
	Kind        string      `json:"kind"`
	UUID        string      `json:"uuid"`
	Object      *objectJSON `json:"object,omitempty"`
	Layer       *layerJSON  `json:"layer,omitempty"`
	BaseVersion uint64      `json:"baseVersion,omitempty"`
	ClientTS    int64       `json:"clientTs,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Seq         uint64      `json:"seq,omitempty"`
	AuthorID    string      `json:"authorId,omitempty"`
}

type pointJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type styleJSON struct {
	Color    string  `json:"color"`
	Opacity  float64 `json:"opacity"`
	Width    float64 `json:"width"`
	FontSize float64 `json:"fontSize"`
	Fill     *bool   `json:"fill,omitempty"`
	Stroke   *bool   `json:"stroke,omitempty"`
	Blend    string  `json:"blend,omitempty"`
}

type layerJSON struct {
	ID        string `json:"id"`
	FileID    string `json:"fileId"`
	Name      string `json:"name"`
	OwnerID   string `json:"ownerId"`
	Zone      string `json:"zone"`
	Order     int    `json:"order"`
	Access    string `json:"access"`
	Mandatory bool   `json:"mandatory"`
	RoleTag   string `json:"roleTag"`
}

type objectJSON struct {
	UUID    string      `json:"uuid"`
	LayerID string      `json:"layerId"`
	Type    string      `json:"type"`
	Points  []pointJSON `json:"points"`
	Page    int         `json:"page"`
	Text    string      `json:"text"`
	Order   int         `json:"order"`
	Style   styleJSON   `json:"style"`
}

// ---- connection ----

const (
	// writeWait bounds a single socket write.
	writeWait = 10 * time.Second
	// pongWait is how long we wait for a pong before assuming the peer is gone.
	pongWait = 60 * time.Second
	// pingPeriod must be < pongWait; we ping on this cadence to keep liveness.
	pingPeriod = (pongWait * 9) / 10
	// sendBuffer is the per-connection outbound queue depth before we drop a slow peer.
	sendBuffer = 64
	// maxMessage caps an inbound frame (a single mutation is small).
	maxMessage = 1 << 20
)

// Band-role strings the hub gates on (mirror app.Role; kept as plain strings so the
// sync package never imports the app/httpapi layers — the role arrives as data).
const (
	roleAdmin     = "admin"
	roleConductor = "conductor"
)

// conn is one client's socket plus its outbound queue. The read pump owns reads; the
// write pump owns writes (gorilla requires at most one concurrent reader and writer).
type conn struct {
	hub      *Hub
	ws       *websocket.Conn
	room     *room
	songID   string
	authorID string
	// role is the author's band role ("admin"|"conductor"|"member"), resolved at
	// upgrade time. It gates the conductor zone (#3) and layer-access changes (#4).
	role    string
	send    chan []byte
	dropped bool
}

// upgrader accepts same-origin upgrades. CheckOrigin is permissive here because the
// SPA is served same-origin from this very binary (I10); tighten if cross-origin
// hosting is ever introduced.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// Serve authenticates and upgrades an HTTP request to a song WebSocket, then runs the
// two pumps until the connection closes. It is the http.Handler the /ws route calls.
//
// Auth mirrors the annotation REST routes: a valid session token (token) AND band
// membership with the song belonging to the band. The resolved user id is the
// authoritative authorId for every mutation this connection sends.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, token, bandID, songID string) {
	if h.auth == nil || h.eng == nil {
		http.Error(w, "sync hub not wired", http.StatusInternalServerError)
		return
	}
	userID, err := h.auth.UserForToken(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	engineSongID, role, err := h.auth.SongForMember(userID, bandID, songID)
	if err != nil {
		// Non-member (or song/band mismatch) → forbidden; mirrors the REST gate.
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error response.
	}

	c := &conn{
		hub:      h,
		ws:       ws,
		songID:   engineSongID,
		authorID: userID,
		role:     role,
		send:     make(chan []byte, sendBuffer),
	}
	h.register(c)

	// Send the join snapshot before the pumps start fanning live echoes, so the
	// client never misses an echo that lands between snapshot and first read.
	if err := c.sendSnapshot(); err != nil {
		h.unregister(c)
		_ = ws.Close()
		return
	}

	go c.writePump()
	c.readPump() // blocks until the socket closes
}

// sendSnapshot reads the engine HEAD and pushes it as the first frame.
func (c *conn) sendSnapshot() error {
	snap, err := c.hub.eng.Head(c.songID)
	if err != nil {
		return err
	}
	msg := snapshotMsg{Type: "snapshot", Layers: []layerJSON{}, Objects: []objectJSON{}, Seq: snap.Revision}
	for _, l := range snap.Layers {
		msg.Layers = append(msg.Layers, layerToJSON(l))
	}
	for _, o := range snap.LiveObjects() {
		msg.Objects = append(msg.Objects, objectToJSON(o))
	}
	frame, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(websocket.TextMessage, frame)
}

// readPump decodes inbound client frames and drives the engine. It is the only reader
// of the socket. On any read error (close, timeout) it tears the connection down.
func (c *conn) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.ws.Close()
		if !c.dropped {
			close(c.send)
		}
	}()

	c.ws.SetReadLimit(maxMessage)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var in clientMsg
		if err := json.Unmarshal(data, &in); err != nil {
			continue // ignore malformed frames rather than tearing the socket down
		}
		if in.Type != "mutation" {
			continue
		}
		c.handleMutation(in.Mutation)
	}
}

// writePump is the only writer of the socket: it drains the send channel and pings on
// a cadence. A closed send channel (slow-consumer drop or normal teardown) ends it.
func (c *conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.ws.Close()
	}()
	for {
		select {
		case frame, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendTo queues a frame to this single connection (used for rejects). A full buffer
// is treated as a slow consumer and the frame is dropped (the read pump will surface
// the eventual socket error).
func (c *conn) sendTo(frame []byte) {
	defer func() { _ = recover() }() // send on a closed channel after a drop is benign
	select {
	case c.send <- frame:
	default:
	}
}

// reject sends a reject envelope to this sender only.
func (c *conn) reject(uuid, reason string) {
	frame, err := json.Marshal(rejectMsg{Type: "reject", UUID: uuid, Reason: reason})
	if err != nil {
		return
	}
	c.sendTo(frame)
}
