package sync

// The realtime half of the annotation-anchoring matrix (see chartpdf/anchor_matrix_test.go): what the hub
// does to a mark's LINK when it is created, moved, resized, restyled, re-texted or reordered. Each cell
// asserts the DESIRED property (P-ids from the anchoring spec); known-red cells skip unless ANCHOR_RED=1.
//
// T195 (stage 2): the hub drives a REAL anchorer here — chartAnchorerFor, the same chartpdf calls the
// httpapi adapter makes (LinkMark, RelinkMoved, RebaseMark) over one synthetic chart — so a cell observes
// the real apply path AND the real rule: "the server decides the link, never the client" (P26).

import (
	"testing"

	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// headEngine records the applied mutation and serves a HEAD holding one stored mark.
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

// A synthetic chart (no real song): three lyric lines with room below.
const syncChart = "# A Test Chart\n\n## Verse\nthe first line of words\nthe second line of words\nthe third line of words\n"

// chartTestAnchorer is the httpapi adapter's logic over a fixed render: every layer is on this chart.
type chartTestAnchorer struct{ r chartpdf.ChartRender }

func (a chartTestAnchorer) AnchorMark(_ string, o domain.Object) domain.Object {
	return chartpdf.LinkMark(o, a.r)
}
func (a chartTestAnchorer) RelinkMark(_ string, moved, stored domain.Object) domain.Object {
	return chartpdf.RelinkMoved(moved, stored, &a.r)
}
func (a chartTestAnchorer) RebaseMark(_ string, edited, stored domain.Object) domain.Object {
	return chartpdf.RebaseMark(edited, stored, &a.r)
}
func (a chartTestAnchorer) ServeSnapshot(_ string, snap domain.Snapshot) domain.Snapshot {
	snap.Objects = chartpdf.ProjectMarks(snap.Objects, a.r)
	return snap
}

type syncWorld struct {
	r      chartpdf.ChartRender
	lines  map[string]chartpdf.Anchor // run by text
	ids    map[string]string          // line id by text
	stored domain.Object
}

func newSyncWorld(t *testing.T) *syncWorld {
	t.Helper()
	_, anchors, err := chartpdf.RenderWithAnchors(syncChart)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	li := chartpdf.NewLineIdentity(syncChart, func() string { n++; return "line-" + string(rune('a'+n)) })
	w := &syncWorld{r: chartpdf.ChartRender{Source: syncChart, Lines: li, Anchors: anchors, Hash: "render-now"},
		lines: map[string]chartpdf.Anchor{}, ids: map[string]string{}}
	for _, a := range anchors {
		w.lines[a.Text] = a
		w.ids[a.Text] = li.IDs[a.SrcLine]
	}
	// The stored mark: a rect drawn around the first line, linked by the server at create.
	w.stored = chartpdf.LinkMark(domain.Object{UUID: "o1", LayerID: "l1", Type: domain.TypeRect, Version: 3, OwnerID: "u",
		Page: 0, Points: w.box("the first line of words", 0)}, w.r)
	if w.stored.Link == nil || w.stored.Link.Line != w.ids["the first line of words"] {
		t.Fatalf("fixture: the stored mark is not linked to the first line: %+v", w.stored.Link)
	}
	return w
}

// box is a rect drawn with a margin around a line, shifted down by dy line-heights.
func (w *syncWorld) box(text string, dy float64) []domain.Point {
	a := w.lines[text]
	h := a.Y1 - a.Y0
	return []domain.Point{{X: a.X0 + 0.01, Y: a.Y0 - 0.2*h + dy*h}, {X: a.X0 + 0.15, Y: a.Y1 + 0.2*h + dy*h}}
}

func toPts(ps []domain.Point) []pointJSON {
	out := make([]pointJSON, len(ps))
	for i, p := range ps {
		out[i] = pointJSON{X: p.X, Y: p.Y}
	}
	return out
}

// run applies one client mutation through the real handler and returns what the engine was asked to store.
func (w *syncWorld) run(t *testing.T, kind string, obj objectJSON, anchorer ChartAnchorer) domain.Mutation {
	t.Helper()
	eng := &headEngine{
		fakeEngine: fakeEngine{
			layer:      domain.Layer{ID: "l1", OwnerID: "u", Zone: domain.ZoneShared, Access: domain.AccessRW},
			layerFound: true, objExists: true,
		},
		head: domain.Snapshot{Objects: []domain.Object{w.stored}},
	}
	hub := &Hub{eng: eng, applyLocks: map[string]*muRef{}}
	if anchorer != nil {
		hub.SetAnchorer(anchorer)
	}
	r := &room{songID: "s1", conns: map[*conn]struct{}{}}
	c := &conn{hub: hub, room: r, songID: "s1", authorID: "u", role: "member", send: make(chan []byte, 4)}
	r.conns[c] = struct{}{}
	obj.UUID, obj.LayerID = "o1", "l1"
	if obj.Type == "" {
		obj.Type = "rect"
	}
	c.handleMutation(mutationJSON{Kind: kind, UUID: "o1", Object: &obj})
	if eng.got.Object == nil {
		t.Fatal("the mutation was not applied")
	}
	return eng.got
}

// knownRedSync: cells known to fail; empty since T195 (all 6 that were red on main are green).
var knownRedSync = map[string][2]string{}

func TestAnchorMatrixSync(t *testing.T) {
	forged := &domain.MarkLink{State: domain.LinkLinked, Line: "client-line", Render: "render-now", LH: 1}
	cells := []struct {
		name  string
		props string
		run   func(t *testing.T, w *syncWorld)
	}{
		// P23/P26: a move re-links from the new position, decided by the server; a forged link is ignored.
		{"move/relinks_server_side", "P23 P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), Link: forged,
				PointsRenderHash: "render-now"}, chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.Line != w.ids["the third line of words"] {
				t.Errorf("a move stored link %+v; the server must re-link it to the line it was dropped on", got.Link)
			}
		}},
		// P23: the realistic case, the client echoing back the link it was served. Still wrong to keep.
		{"move/old_link_not_kept", "P23", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), Link: w.stored.Link.Clone(),
				PointsRenderHash: "render-now"}, chartTestAnchorer{w.r}).Object
			if got.Link != nil && got.Link.Line == w.stored.Link.Line {
				t.Errorf("a move kept the pre-move link (the measured bug: the mark jumps back at the next edit)")
			}
		}},
		// P23: moved into empty space, the mark becomes fixed, never keeps a link it is no longer near.
		{"move/into_empty_space_is_fixed", "P23 P5", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "move", objectJSON{Points: toPts([]domain.Point{{X: 0.3, Y: 0.8}, {X: 0.4, Y: 0.85}}),
				PointsRenderHash: "render-now"}, chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.State != domain.LinkFixed {
				t.Errorf("a move into empty space stored link %+v, want fixed", got.Link)
			}
		}},
		// P27: a move made on a render the server does not have keeps the stored line, flagged check.
		{"move/stale_view_keeps_link", "P27", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), Link: forged,
				PointsRenderHash: "render-from-an-old-tab"}, chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.Line != w.stored.Link.Line || got.Link.State != domain.LinkCheck {
				t.Errorf("a stale-view move stored link %+v; want the stored line, state check", got.Link)
			}
		}},
		// P24: a resize re-links with the NEW extent: the mark now also covers the second line.
		{"resize/relinks_server_side", "P24 P26", func(t *testing.T, w *syncWorld) {
			a := w.lines["the second line of words"]
			pts := w.box("the first line of words", 0)
			pts[1].Y = a.Y1 + 0.2*(a.Y1-a.Y0)
			got := w.run(t, "resize", objectJSON{Points: toPts(pts), Link: w.stored.Link.Clone(), PointsRenderHash: "render-now"},
				chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.Line != w.ids["the first line of words"] {
				t.Fatalf("a resize stored link %+v, want the first (top) line", got.Link)
			}
			inside := false
			for _, p := range got.Link.Peers {
				inside = inside || (p.Line == w.ids["the second line of words"] && p.Why == "inside")
			}
			if !inside {
				t.Errorf("a resize kept the pre-resize link %+v; it must re-measure with the new extent", got.Link)
			}
		}},
		// P25/P26: restyle, text change and reorder keep the STORED link and do not move the mark.
		{"setStyle/keeps_stored_link", "P25 P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "setStyle", objectJSON{Points: toPts(w.box("the third line of words", 0)), Link: forged,
				Style: styleJSON{Color: "#2563eb"}}, chartTestAnchorer{w.r}).Object
			keptLink(t, w, got)
			if got.Style.Color != "#2563eb" {
				t.Errorf("the restyle itself was lost: %+v", got.Style)
			}
		}},
		{"setText/keeps_stored_link", "P25 P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "setText", objectJSON{Points: toPts(w.box("the first line of words", 0)), Link: forged, Text: "x"},
				chartTestAnchorer{w.r}).Object
			keptLink(t, w, got)
		}},
		{"reorder/keeps_stored_link", "P25 P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "reorder", objectJSON{Points: toPts(w.box("the first line of words", 0)), Link: forged, Order: 4},
				chartTestAnchorer{w.r}).Object
			keptLink(t, w, got)
		}},
		{"setStyle/echoed_link_kept", "P25", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "setStyle", objectJSON{Points: toPts(w.box("the first line of words", 0)), Link: w.stored.Link.Clone()},
				chartTestAnchorer{w.r}).Object
			keptLink(t, w, got)
		}},
		// P26 also covers a client that drops the field: a restyle must not unlink the mark.
		{"setStyle/missing_link_not_dropped", "P25 P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "setStyle", objectJSON{Points: toPts(w.box("the first line of words", 0))}, chartTestAnchorer{w.r}).Object
			keptLink(t, w, got)
		}},
		// P26 at create: the client's link is dropped, the server links where the mark is drawn.
		{"create/client_link_ignored", "P2 P26", func(t *testing.T, w *syncWorld) {
			w.stored = domain.Object{}
			got := w.run(t, "create", objectJSON{Points: toPts(w.box("the second line of words", 0)), Link: forged},
				chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.Line != w.ids["the second line of words"] {
				t.Errorf("a create stored link %+v; want the server's, to the second line", got.Link)
			}
		}},
		// Without an anchorer the hub still never applies a client link (P26).
		{"move/no_anchorer_no_client_link", "P26", func(t *testing.T, w *syncWorld) {
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), Link: forged}, nil).Object
			if got.Link != nil {
				t.Errorf("with no anchorer a move stored link %+v; it must store none", got.Link)
			}
		}},
		// F1: the same for a FIXED mark echoed with its stored hash, after the chart moved on.
		{"fixed/move_with_echoed_hash_relinks", "P23", func(t *testing.T, w *syncWorld) {
			w.stored = chartpdf.LinkMark(domain.Object{UUID: "o1", LayerID: "l1", Type: domain.TypeRect, Version: 3, OwnerID: "u",
				Points: []domain.Point{{X: 0.3, Y: 0.8}, {X: 0.4, Y: 0.85}}}, w.r)
			w.stored.PointsRenderHash, w.stored.Link.Render = "an-older-render", "an-older-render"
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), PointsRenderHash: "an-older-render"},
				chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.State != domain.LinkLinked || got.Link.Line != w.ids["the third line of words"] {
				t.Errorf("a fixed mark moved onto a line with its echoed hash stored link %+v; want linked to it", got.Link)
			}
		}},
		// Old-style anchors (T145) are kept by a restyle, and superseded by the server's decision on a move.
		{"oldstyle/setStyle_keeps_old_anchor", "P25", func(t *testing.T, w *syncWorld) {
			w.stored.Link = nil
			w.stored.Anchor = &domain.SourceAnchor{RunText: "the first line of words", Occurrence: 1, CharEnd: 5}
			got := w.run(t, "setStyle", objectJSON{Points: toPts(w.box("the first line of words", 0))}, chartTestAnchorer{w.r}).Object
			if got.Anchor == nil || got.Anchor.RunText != "the first line of words" || got.Link != nil {
				t.Errorf("a restyle of an old-style mark stored anchor %+v link %+v; want the old anchor untouched", got.Anchor, got.Link)
			}
		}},
		// F1 (T195 review): what the real client sends back is the object it holds, with the hash it holds —
		// here the old-style mark's own, older render hash (a restyle echo carries the stored one). An
		// old-style mark does not move between renders, so the move is re-linked anyway.
		{"oldstyle/move_relinks", "P23", func(t *testing.T, w *syncWorld) {
			w.stored.Link = nil
			w.stored.Anchor = &domain.SourceAnchor{RunText: "the first line of words", Occurrence: 1, CharEnd: 5}
			w.stored.PointsRenderHash = "an-older-render"
			got := w.run(t, "move", objectJSON{Points: toPts(w.box("the third line of words", 0)), PointsRenderHash: "an-older-render"},
				chartTestAnchorer{w.r}).Object
			if got.Link == nil || got.Link.Line != w.ids["the third line of words"] || got.Anchor != nil {
				t.Errorf("a move of an old-style mark stored link %+v anchor %+v; want a new link, no old anchor", got.Link, got.Anchor)
			}
		}},
	}
	for _, c := range cells {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if r, ok := knownRedSync[c.name]; ok {
				testutil.AnchorKnownRed(t, r[0], r[1])
			}
			c.run(t, newSyncWorld(t))
		})
	}
}

// keptLink asserts a non-moving edit stored the STORED link and the stored geometry.
func keptLink(t *testing.T, w *syncWorld, got *domain.Object) {
	t.Helper()
	if got.Link == nil || got.Link.Line != w.stored.Link.Line {
		t.Errorf("the edit changed the link to %+v; it must keep the stored line %s", got.Link, w.stored.Link.Line)
	}
	if len(got.Points) != len(w.stored.Points) || got.Points[0] != w.stored.Points[0] || got.Points[1] != w.stored.Points[1] {
		t.Errorf("the edit moved the mark: %v, stored %v", got.Points, w.stored.Points)
	}
}
