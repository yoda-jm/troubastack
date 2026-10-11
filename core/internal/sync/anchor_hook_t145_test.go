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
// stage-2 link + render hash so the test can prove the create path routed the object through it.
type stubAnchorer struct {
	called     bool
	gotSongID  string
	gotLayerID string
	gotLink    *domain.MarkLink // the link the object carried when the hub handed it over
}

func (s *stubAnchorer) AnchorMark(songID string, o domain.Object) domain.Object {
	s.called = true
	s.gotSongID = songID
	s.gotLayerID = o.LayerID
	s.gotLink = o.Link
	o.Link = &domain.MarkLink{State: domain.LinkLinked, Line: "chorus-line-id", Render: "render-hash-xyz"}
	o.PointsRenderHash = "render-hash-xyz"
	return o
}

func (s *stubAnchorer) RelinkMark(_ string, moved, _ domain.Object) domain.Object { return moved }
func (s *stubAnchorer) RebaseMark(_ string, edited, _ domain.Object) domain.Object {
	return edited
}
func (s *stubAnchorer) ServeSnapshot(_ string, snap domain.Snapshot) domain.Snapshot { return snap }
func (s *stubAnchorer) ServeEcho(_ string, o domain.Object) domain.Object            { return o }

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

	// The client sends a link of its own: the hub must drop it BEFORE the anchorer sees the object (P26).
	c.handleMutation(mutationJSON{
		Kind: "create",
		Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand",
			Link: &domain.WireLink{MarkLink: domain.MarkLink{State: domain.LinkLinked, Line: "forged"}}, PointsRenderHash: "forged-render"},
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
	if anc.gotLink != nil {
		t.Fatalf("the anchorer was handed the client's link %+v; the hub must drop it first (P26)", anc.gotLink)
	}
	if l := eng.got.Object.Link; l == nil || l.Line != "chorus-line-id" {
		t.Fatalf("applied object link = %+v, want the stub's line 'chorus-line-id'", l)
	}
	if eng.got.Object.PointsRenderHash != "render-hash-xyz" {
		t.Fatalf("applied object render hash = %q, want render-hash-xyz", eng.got.Object.PointsRenderHash)
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
		Kind: "create",
		Object: &objectJSON{UUID: "o1", LayerID: "l1", Type: "freehand",
			Link: &domain.WireLink{MarkLink: domain.MarkLink{State: domain.LinkLinked, Line: "forged"}}},
	})

	if eng.got.Object == nil {
		t.Fatal("engine received no object")
	}
	if eng.got.Object.Anchor != nil || eng.got.Object.Link != nil {
		t.Fatalf("nil anchorer must leave the mark unlinked, got anchor %+v link %+v", eng.got.Object.Anchor, eng.got.Object.Link)
	}
}
