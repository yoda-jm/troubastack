package sync

// The realtime half of the annotation-anchoring matrix (see chartpdf/anchor_matrix_test.go): what the hub
// does to a mark's LINK when it is moved, resized, restyled or re-texted. Each cell asserts the DESIRED
// property (P-ids from the anchoring spec); known-red cells skip unless ANCHOR_RED=1.
//
// The assertions are on the applied mutation only, so they hold whichever way the fix is built (a new
// re-anchor seam, or AnchorMark after clearing): "the server decides the link, never the client" (P26).

import (
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// headEngine records the applied mutation and serves a HEAD holding one stored, linked mark.
type headEngine struct {
	fakeEngine
	head domain.Snapshot
	got  domain.Mutation
}

func (e *headEngine) Apply(_ string, m domain.Mutation) (domain.Mutation, error) {
	e.got = m
	return m, nil
}
func (e *headEngine) Head(string) (domain.Snapshot, error) { return e.head, nil }

var storedLink = domain.SourceAnchor{RunText: "stored line", Occurrence: 1, CharStart: 0, CharEnd: 6}

func runSyncCell(t *testing.T, kind string, client *anchorJSON) domain.Mutation {
	t.Helper()
	stored := domain.Object{UUID: "o1", LayerID: "l1", Type: domain.TypeRect, Version: 3, OwnerID: "u",
		Points: []domain.Point{{X: 0.1, Y: 0.1}, {X: 0.3, Y: 0.15}}, Anchor: storedLink.Clone(),
		PointsRenderHash: "render-now"}
	eng := &headEngine{
		fakeEngine: fakeEngine{
			layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
			layerFound: true, objExists: true,
		},
		head: domain.Snapshot{Objects: []domain.Object{stored}},
	}
	hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}}
	hub.SetAnchorer(&stubAnchorer{})
	r := &room{songID: "s1", conns: map[*conn]struct{}{}}
	c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
	r.conns[c] = struct{}{}

	pts := []pointJSON{{X: 0.1, Y: 0.4}, {X: 0.3, Y: 0.45}} // moved/resized somewhere else
	if kind == "setStyle" || kind == "setText" {
		pts = []pointJSON{{X: 0.1, Y: 0.1}, {X: 0.3, Y: 0.15}}
	}
	c.handleMutation(mutationJSON{Kind: kind, UUID: "o1", Object: &objectJSON{
		UUID: "o1", LayerID: "l1", Type: "rect", Points: pts, Anchor: client, PointsRenderHash: "render-now",
		Style: styleJSON{Color: "#2563eb"}, Text: "x",
	}})
	if eng.got.Object == nil {
		t.Fatal("the mutation was not applied")
	}
	return eng.got
}

// knownRedSync: red on today's code (baseline 2026-10-11). Delete a line when its fix lands.
var knownRedSync = map[string][2]string{
	"move/relinks_server_side":          {"P23 P26", "T191"},
	"move/old_link_not_kept":            {"P23", "T191"},
	"resize/relinks_server_side":        {"P24 P26", "T191"},
	"setStyle/keeps_stored_link":        {"P25 P26", "T191"},
	"setText/keeps_stored_link":         {"P25 P26", "T191"},
	"setStyle/missing_link_not_dropped": {"P25 P26", "T191"},
}

func TestAnchorMatrixSync(t *testing.T) {
	forged := &anchorJSON{RunText: "client line", Occurrence: 1, CharStart: 0, CharEnd: 6}
	same := &anchorJSON{RunText: storedLink.RunText, Occurrence: 1, CharStart: 0, CharEnd: 6}
	cells := []struct {
		name, kind string
		client     *anchorJSON
		check      func(t *testing.T, got *domain.SourceAnchor)
	}{
		// P23/P26: a move re-links from the new position, decided by the server. Today the client's anchor
		// is applied as sent, so a forged (or merely stale) one survives.
		{"move/relinks_server_side", "move", forged, func(t *testing.T, got *domain.SourceAnchor) {
			if got != nil && got.RunText == "client line" {
				t.Errorf("a move applied the client's link %q; the server must decide it (re-link or clear)", got.RunText)
			}
			if got != nil && got.RunText == storedLink.RunText {
				t.Errorf("a move kept the pre-move link %q; it must re-link from where the mark now is", got.RunText)
			}
		}},
		// P23: the realistic case, the client echoing back the anchor it was served. Still wrong to keep.
		{"move/old_link_not_kept", "move", same, func(t *testing.T, got *domain.SourceAnchor) {
			if got != nil && got.RunText == storedLink.RunText {
				t.Errorf("a move kept the pre-move link %q (the measured bug: the mark jumps back at the next edit)", got.RunText)
			}
		}},
		{"resize/relinks_server_side", "resize", same, func(t *testing.T, got *domain.SourceAnchor) {
			if got != nil && got.RunText == storedLink.RunText && got.CharEnd == 6 {
				t.Errorf("a resize kept the pre-resize link unchanged; it must re-link with the new extent")
			}
		}},
		// P25: restyle / text change keep the STORED link (P26: never the client's).
		{"setStyle/keeps_stored_link", "setStyle", forged, func(t *testing.T, got *domain.SourceAnchor) {
			if got == nil || got.RunText != storedLink.RunText {
				t.Errorf("a restyle changed the link to %+v; it must keep the stored %q", got, storedLink.RunText)
			}
		}},
		{"setText/keeps_stored_link", "setText", forged, func(t *testing.T, got *domain.SourceAnchor) {
			if got == nil || got.RunText != storedLink.RunText {
				t.Errorf("a text change changed the link to %+v; it must keep the stored %q", got, storedLink.RunText)
			}
		}},
		{"setStyle/echoed_link_kept", "setStyle", same, func(t *testing.T, got *domain.SourceAnchor) {
			if got == nil || got.RunText != storedLink.RunText {
				t.Errorf("a restyle dropped the link: %+v", got)
			}
		}},
		// P26 also covers a client that drops the field: a restyle must not unlink the mark.
		{"setStyle/missing_link_not_dropped", "setStyle", nil, func(t *testing.T, got *domain.SourceAnchor) {
			if got == nil {
				t.Errorf("a restyle sent without the anchor field unlinked the mark; the server must keep the stored link")
			}
		}},
	}
	for _, c := range cells {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if r, ok := knownRedSync[c.name]; ok {
				testutil.AnchorKnownRed(t, r[0], r[1])
			}
			got := runSyncCell(t, c.kind, c.client)
			c.check(t, got.Object.Anchor)
		})
	}
}
