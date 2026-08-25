package sync

import "testing"

// A conn needs only a songID + a send buffer for the hub's room bookkeeping (no websocket).
func newConn(song string) *conn { return &conn{songID: song, send: make(chan []byte, 1)} }

// T109: rooms are created lazily on first join and garbage-collected when their last conn leaves.
func TestRoomLifecycle(t *testing.T) {
	h := &Hub{rooms: map[string]*room{}}

	c1 := newConn("s")
	h.register(c1)
	if h.rooms["s"] == nil {
		t.Fatal("register did not create the room")
	}
	if c1.room != h.rooms["s"] {
		t.Fatal("conn.room not wired to the room")
	}

	c2 := newConn("s")
	h.register(c2)
	if got := len(h.rooms["s"].conns); got != 2 {
		t.Fatalf("room has %d conns, want 2 (same song shares a room)", got)
	}

	h.unregister(c1)
	if h.rooms["s"] == nil {
		t.Fatal("room garbage-collected while a conn remains")
	}
	if got := len(h.rooms["s"].conns); got != 1 {
		t.Fatalf("room has %d conns after one leaves, want 1", got)
	}

	h.unregister(c2)
	if _, ok := h.rooms["s"]; ok {
		t.Fatal("empty room was not garbage-collected on the last leave")
	}
}

// T109: a connection whose send buffer is full when a frame is broadcast is a slow consumer — it is
// dropped from the room and its send channel closed (which ends its write pump).
func TestSlowConsumerDropped(t *testing.T) {
	h := &Hub{rooms: map[string]*room{}}
	c := newConn("s") // send buffer of 1
	h.register(c)
	r := h.rooms["s"]

	r.broadcast([]byte("first"))  // fits in the buffer
	r.broadcast([]byte("second")) // buffer full → c is a slow consumer → dropped

	r.mu.Lock()
	_, still := r.conns[c]
	r.mu.Unlock()
	if still {
		t.Fatal("slow consumer was not dropped from the room")
	}

	if got := <-c.send; string(got) != "first" {
		t.Fatalf("first buffered frame = %q, want %q", got, "first")
	}
	if _, ok := <-c.send; ok {
		t.Fatal("dropped conn's send channel should be closed")
	}
}
