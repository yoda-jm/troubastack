package sync

import (
	"testing"

	"troubastack/core/internal/domain"
)

// recEngine records the mutation Apply receives (and echoes it back) so a test can inspect what the hub
// handed the engine — the apply-path transformation, after version stamping and T145 anchoring.
type recEngine struct {
	fakeEngine
	got domain.Mutation
}

func (e *recEngine) Apply(_ string, m domain.Mutation) (domain.Mutation, error) {
	e.got = m
	return m, nil
}

// stubAnchorer stands in for the httpapi chart adapter: it records that it was called and stamps a known
// anchor + render hash so the test can prove the create path routed the object through it.
type stubAnchorer struct {
	called     bool
	gotSongID  string
	gotLayerID string

	reanchored   bool // T191: ReanchorMoved was consulted
	reanchorUUID string
}

func (s *stubAnchorer) AnchorMark(songID string, o domain.Object) domain.Object {
	s.called = true
	s.gotSongID = songID
	s.gotLayerID = o.LayerID
	o.Anchor = &domain.SourceAnchor{RunText: "chorus line", Occurrence: 0, CharStart: 0, CharEnd: 11}
	o.PointsRenderHash = "render-hash-xyz"
	return o
}

func (s *stubAnchorer) ReanchorMoved(songID string, o domain.Object) domain.Object {
	s.reanchored = true
	s.reanchorUUID = o.UUID
	o.Anchor = &domain.SourceAnchor{RunText: "moved line", Occurrence: 1, CharStart: 0, CharEnd: 10}
	o.PointsRenderHash = "render-after-move"
	return o
}

// TestCreateRoutesThroughAnchorer proves the T145 create-time anchoring hook: a create mutation is handed
// to the registered ChartAnchorer BEFORE Apply, and the anchor it stamps rides along into the applied
// mutation. Removing the hook (or leaving the anchorer nil) leaves Anchor nil — the pre-T145 behavior.
func TestCreateRoutesThroughAnchorer(t *testing.T) {
	eng := &recEngine{fakeEngine: fakeEngine{
		layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
		layerFound: true,
	}}
	hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}}
	anc := &stubAnchorer{}
	hub.SetAnchorer(anc)

	r := &room{songID: "s1", conns: map[*conn]struct{}{}}
	c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
	r.conns[c] = struct{}{}

	c.handleMutation(mutationJSON{
		Kind:   "create",
		Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand"},
	})

	if !anc.called {
		t.Fatal("anchorer was not consulted on create")
	}
	if anc.gotSongID != "s1" || anc.gotLayerID != "l1" {
		t.Fatalf("anchorer got (song=%q, layer=%q), want (s1, l1)", anc.gotSongID, anc.gotLayerID)
	}
	if eng.got.Object == nil {
		t.Fatal("engine received no object")
	}
	if eng.got.Object.Anchor == nil || eng.got.Object.Anchor.RunText != "chorus line" {
		t.Fatalf("applied object anchor = %+v, want the stub's RunText 'chorus line'", eng.got.Object.Anchor)
	}
	if eng.got.Object.PointsRenderHash != "render-hash-xyz" {
		t.Fatalf("applied object render hash = %q, want render-hash-xyz", eng.got.Object.PointsRenderHash)
	}
}

// TestMoveAndResizeRouteThroughReanchorer is the T191 forward fix's counterpart to the create test: a move
// or a resize is handed to ReanchorMoved BEFORE Apply, and the anchor it recomputes rides into the applied
// mutation (so a manual placement is not discarded at the next text edit).
func TestMoveAndResizeRouteThroughReanchorer(t *testing.T) {
	for _, kind := range []string{"move", "resize"} {
		eng := &recEngine{fakeEngine: fakeEngine{
			layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
			layerFound: true,
			objExists:  true, // the move target exists in HEAD, so the write authorizes
		}}
		hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}}
		anc := &stubAnchorer{}
		hub.SetAnchorer(anc)

		r := &room{songID: "s1", conns: map[*conn]struct{}{}}
		c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
		r.conns[c] = struct{}{}

		c.handleMutation(mutationJSON{
			Kind:   kind,
			UUID:   "o1",
			Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand"},
		})

		if !anc.reanchored {
			t.Fatalf("%s: ReanchorMoved was not consulted", kind)
		}
		if anc.reanchorUUID != "o1" {
			t.Fatalf("%s: ReanchorMoved got uuid %q, want o1", kind, anc.reanchorUUID)
		}
		if eng.got.Object == nil || eng.got.Object.Anchor == nil || eng.got.Object.Anchor.RunText != "moved line" {
			t.Fatalf("%s: applied object did not carry the re-anchored anchor: %+v", kind, eng.got.Object)
		}
		if eng.got.Object.PointsRenderHash != "render-after-move" {
			t.Fatalf("%s: applied render hash = %q, want render-after-move", kind, eng.got.Object.PointsRenderHash)
		}
	}
}

// TestSetStyleAndSetTextKeepAnchor is the negative guard: a restyle or a text edit does not move the mark,
// so it must NOT re-anchor — only move/resize route through ReanchorMoved.
func TestSetStyleAndSetTextKeepAnchor(t *testing.T) {
	for _, kind := range []string{"setStyle", "setText"} {
		eng := &recEngine{fakeEngine: fakeEngine{
			layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
			layerFound: true,
			objExists:  true,
		}}
		hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}}
		anc := &stubAnchorer{}
		hub.SetAnchorer(anc)

		r := &room{songID: "s1", conns: map[*conn]struct{}{}}
		c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
		r.conns[c] = struct{}{}

		c.handleMutation(mutationJSON{
			Kind:   kind,
			UUID:   "o1",
			Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand"},
		})

		if anc.reanchored {
			t.Fatalf("%s must not re-anchor (it does not move the mark)", kind)
		}
	}
}

// TestNilAnchorerLeavesCreateUnanchored is the negative guard: a hub without an anchorer creates a mark
// with no anchor (exactly the pre-T145 path), and the create still applies.
func TestNilAnchorerLeavesCreateUnanchored(t *testing.T) {
	eng := &recEngine{fakeEngine: fakeEngine{
		layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
		layerFound: true,
	}}
	hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}} // no SetAnchorer

	r := &room{songID: "s1", conns: map[*conn]struct{}{}}
	c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
	r.conns[c] = struct{}{}

	c.handleMutation(mutationJSON{
		Kind:   "create",
		Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand"},
	})

	if eng.got.Object == nil {
		t.Fatal("engine received no object")
	}
	if eng.got.Object.Anchor != nil {
		t.Fatalf("nil anchorer must leave Anchor nil, got %+v", eng.got.Object.Anchor)
	}
}
