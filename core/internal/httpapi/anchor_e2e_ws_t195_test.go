package httpapi

// T195 end-to-end vectors, written by the independent review of stage 2 and kept as permanent tests (they
// caught F1: a lost, fixed or old-style mark moved through the real client never re-linked). Production
// entry points only: marks are created/moved over a REAL websocket (sync apply + the real chartAnchorer), the chart is
// edited through Service.SaveChartSource, and marks are read back through REST GET /annotations. Every
// object sent back to the server is the SERVED JSON object with only its points changed (what Studio's
// translateObject spread does). Expectations are computed from the renderer's runs, never from chartpdf's
// link code.

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

type e2eWorld struct {
	t     *testing.T
	svc   *app.Service
	eng   *engine.Engine
	admin app.User
	band  string
	song  string
	file  string
	src   string
	srv   *httptest.Server
	ws    *websocket.Conn
	tok   string
}

func newE2E(t *testing.T, src string) *e2eWorld {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, err := svc.Register("e2eowner", "E2E", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	_, tok, err := svc.Login("e2eowner", "password123")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "E2E Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "E2E Song", "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := svc.CreateTextChart(admin, band.ID, song.ID, src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "RL", FileID: f.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAnnotationsAPI(svc, eng).Mount(mux, func(h authedHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h(w, r, admin) }
	})
	mountWS(mux, svc, eng, nil)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	w := &e2eWorld{t: t, svc: svc, eng: eng, admin: admin, band: band.ID, song: song.ID, file: f.ID, src: src, srv: srv, tok: tok}
	w.dial()
	return w
}

func (w *e2eWorld) dial() {
	w.t.Helper()
	if w.ws != nil {
		w.ws.Close()
	}
	url := "ws" + strings.TrimPrefix(w.srv.URL, "http") + "/api/bands/" + w.band + "/songs/" + w.song + "/ws"
	hdr := http.Header{}
	hdr.Add("Cookie", sessionCookie+"="+w.tok)
	ws, _, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		w.t.Fatal(err)
	}
	w.t.Cleanup(func() { ws.Close() })
	w.ws = ws
	w.read("snapshot")
}

func (w *e2eWorld) read(want string) map[string]json.RawMessage {
	w.t.Helper()
	_ = w.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := w.ws.ReadMessage()
	if err != nil {
		w.t.Fatal(err)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(data, &m)
	var typ string
	_ = json.Unmarshal(m["type"], &typ)
	if typ != want {
		w.t.Fatalf("ws frame %s, want %s: %s", typ, want, data)
	}
	return m
}

// mut sends one mutation with a raw object and waits for its echo.
func (w *e2eWorld) mut(kind, uuid string, obj map[string]any) {
	w.t.Helper()
	msg := map[string]any{"type": "mutation", "mutation": map[string]any{"kind": kind, "uuid": uuid, "object": obj}}
	if err := w.ws.WriteJSON(msg); err != nil {
		w.t.Fatal(err)
	}
	w.read("echo")
}

func ptsJSON(ps []domain.Point) []any {
	out := []any{}
	for _, p := range ps {
		out = append(out, map[string]any{"x": p.X, "y": p.Y})
	}
	return out
}

func (w *e2eWorld) create(o domain.Object, typ string) {
	w.t.Helper()
	obj := map[string]any{"uuid": o.UUID, "layerId": "RL", "type": typ, "page": o.Page, "points": ptsJSON(o.Points),
		"text": o.Text, "style": map[string]any{"color": "#e11d48", "opacity": 1, "width": 0.003, "fontSize": 0.02},
		// a forged client link and render hash: the server must ignore both (P26)
		"link": map[string]any{"state": "linked", "line": "forged", "render": "x", "lh": 1}, "pointsRenderHash": "x"}
	w.mut("create", o.UUID, obj)
}

// served is GET /annotations as raw objects (what Studio holds and spreads back) and decoded objects.
func (w *e2eWorld) served() (map[string]map[string]any, map[string]domain.Object) {
	w.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, w.srv.URL+"/api/bands/"+w.band+"/songs/"+w.song+"/annotations", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		w.t.Fatalf("GET annotations %d", resp.StatusCode)
	}
	var raw struct {
		Objects []json.RawMessage `json:"objects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		w.t.Fatal(err)
	}
	rm, dm := map[string]map[string]any{}, map[string]domain.Object{}
	for _, b := range raw.Objects {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		var oj objectJSON
		_ = json.Unmarshal(b, &oj)
		rm[oj.UUID] = m
		dm[oj.UUID] = objectFromJSON(oj)
	}
	return rm, dm
}

// moveServed moves the SERVED object (spread, as Studio does) to new points.
func (w *e2eWorld) moveServed(uuid string, pts []domain.Point, kind string) {
	w.t.Helper()
	raw, _ := w.served()
	o, ok := raw[uuid]
	if !ok {
		w.t.Fatalf("%s not served", uuid)
	}
	o["points"] = ptsJSON(pts)
	w.mut(kind, uuid, o)
}

func (w *e2eWorld) save(src string) {
	w.t.Helper()
	cur, _, err := w.svc.ChartSource(w.admin, w.band, w.song, w.file)
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.svc.SaveChartSource(w.admin, w.band, w.song, w.file, cur.Revision, src); err != nil {
		w.t.Fatal(err)
	}
	w.src = src
}

// run is the run whose text is exactly text (occ-th in source order) in a fresh render of src.
func e2eRun(t *testing.T, src, text string, occ int) chartpdf.Anchor {
	t.Helper()
	_, as, err := chartpdf.RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	best := []chartpdf.Anchor{}
	for _, a := range as {
		if a.Text == text {
			best = append(best, a)
		}
	}
	// source order by SrcLine
	for i := range best {
		for j := i + 1; j < len(best); j++ {
			if best[j].SrcLine < best[i].SrcLine {
				best[i], best[j] = best[j], best[i]
			}
		}
	}
	for _, a := range best {
		n++
		if n == occ {
			return a
		}
	}
	t.Fatalf("fixture: %q #%d not drawn", text, occ)
	return chartpdf.Anchor{}
}

func e2eCW(a chartpdf.Anchor) float64 { return (a.X1 - a.X0) / float64(len([]rune(a.Text))) }

func eqPts(t *testing.T, label string, got, want []domain.Point, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d points, want %d", label, len(got), len(want))
	}
	for i := range got {
		if math.Abs(got[i].X-want[i].X) > tol || math.Abs(got[i].Y-want[i].Y) > tol {
			t.Fatalf("%s: point %d at (%.5f,%.5f), want (%.5f,%.5f)", label, i, got[i].X, got[i].Y, want[i].X, want[i].Y)
		}
	}
}

func state(o domain.Object) string {
	if o.Link == nil {
		return "<nil>"
	}
	return string(o.Link.State)
}

const e2eChart = "# The Quay Test Chart\n\n## Verse\nAm        F\nthe tide rolls in across the stones\nC         G\na lantern swings above the quay\n\n## Chorus\nhold on hold on\nwe sail at dawn\nhold on hold on\nthe gulls are calling out\n"

func shift(ps []domain.Point, dx, dy float64) []domain.Point {
	out := make([]domain.Point, len(ps))
	for i, p := range ps {
		out[i] = domain.Point{X: p.X + dx, Y: p.Y + dy}
	}
	return out
}

// 1. Rigid follow: a free-hand stroke circling a word follows its line point for point.
func TestE2E_RigidFollow(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	c, h := e2eCW(a), a.Y1-a.Y0
	var pts []domain.Point
	for i := 0; i < 17; i++ { // an irregular open stroke around "swings"
		th := 2 * math.Pi * float64(i) / 16 * 1.1
		pts = append(pts, domain.Point{X: a.X0 + 13*c + 4.2*c*math.Cos(th), Y: a.Y0 + 0.5*h + 0.9*h*math.Sin(th) + 0.01*float64(i%3)})
	}
	w.create(domain.Object{UUID: "rf", Page: a.Page, Points: pts}, "freehand")
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nDm\nsome new words up here\nand another line\n", 1))
	b := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	_, got := w.served()
	g := got["rf"]
	if state(g) != "linked" {
		t.Errorf("state %s, want linked", state(g))
	}
	eqPts(t, "rigid follow", g.Points, shift(pts, b.X0-a.X0, b.Y0-a.Y0), 1e-6)
}

// 2. Move re-links: a rect around line A moved onto line B follows B, not A, at the next edit.
func TestE2E_MoveRelinks(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "the tide rolls in across the stones", 1)
	bl := e2eRun(t, w.src, "we sail at dawn", 1)
	box := func(r chartpdf.Anchor) []domain.Point {
		h := r.Y1 - r.Y0
		return []domain.Point{{X: r.X0 - 0.01, Y: r.Y0 - 0.3*h}, {X: r.X0 + 0.2, Y: r.Y1 + 0.3*h}}
	}
	w.create(domain.Object{UUID: "mv", Page: a.Page, Points: box(a)}, "rect")
	w.moveServed("mv", box(bl), "move")
	// an edit that moves B but not A: insert a line between them
	w.save(strings.Replace(w.src, "## Chorus\n", "## Chorus\nan inserted chorus line\n", 1))
	nb := e2eRun(t, w.src, "we sail at dawn", 1)
	_, got := w.served()
	g := got["mv"]
	if g.Link == nil || state(g) != "linked" {
		t.Fatalf("state %s", state(g))
	}
	eqPts(t, "moved mark follows its NEW line", g.Points, shift(box(bl), nb.X0-bl.X0, nb.Y0-bl.Y0), 1e-6)
}

// 2b. P21 through the real client path: a LOST mark the user moves (Studio echoes the served object,
// including its served pointsRenderHash) must re-link at its new position.
func TestE2E_LostMarkMovedViaServedObjectRelinks(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "the tide rolls in across the stones", 1)
	h := a.Y1 - a.Y0
	w.create(domain.Object{UUID: "lost", Page: a.Page, Points: []domain.Point{{X: a.X0 + 0.02, Y: a.Y0 - 0.2*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.2*h}}}, "ellipse")
	w.save(strings.Replace(w.src, "the tide rolls in across the stones\n", "", 1))
	_, got := w.served()
	if state(got["lost"]) != "lost" {
		t.Fatalf("fixture: state %s, want lost", state(got["lost"]))
	}
	d := e2eRun(t, w.src, "we sail at dawn", 1)
	dh := d.Y1 - d.Y0
	dst := []domain.Point{{X: d.X0 + 0.02, Y: d.Y0 - 0.2*dh}, {X: d.X0 + 0.1, Y: d.Y1 + 0.2*dh}}
	w.moveServed("lost", dst, "move")
	_, got = w.served()
	if st := state(got["lost"]); st != "linked" {
		t.Errorf("P21: a lost mark moved onto a line (through the real client echo) is %s, want linked", st)
	}
	// and it must follow that line
	w.save(strings.Replace(w.src, "## Chorus\n", "## Chorus\nan inserted chorus line\n", 1))
	nd := e2eRun(t, w.src, "we sail at dawn", 1)
	_, got = w.served()
	eqPts(t, "re-linked lost mark follows", got["lost"].Points, shift(dst, nd.X0-d.X0, nd.Y0-d.Y0), 1e-6)
}

// 2c. P23 for a FIXED mark (placed far from text) later dragged onto a line after a chart edit.
func TestE2E_FixedMarkMovedOntoLineRelinks(t *testing.T) {
	w := newE2E(t, e2eChart)
	w.create(domain.Object{UUID: "fx", Page: 0, Points: []domain.Point{{X: 0.3, Y: 0.8}, {X: 0.4, Y: 0.84}}}, "rect")
	_, got := w.served()
	if state(got["fx"]) != "fixed" {
		t.Fatalf("fixture: %s", state(got["fx"]))
	}
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nan added line\n", 1)) // any chart edit
	d := e2eRun(t, w.src, "we sail at dawn", 1)
	dh := d.Y1 - d.Y0
	dst := []domain.Point{{X: d.X0, Y: d.Y0 - 0.2*dh}, {X: d.X0 + 0.1, Y: d.Y1 + 0.2*dh}}
	w.moveServed("fx", dst, "move")
	_, got = w.served()
	if st := state(got["fx"]); st != "linked" {
		t.Errorf("P23: a fixed mark dragged onto a line after a chart edit is %s, want linked", st)
	}
}

// 2d. Old-style anchored mark (T145) moved by the user: §6 says it gets a new link.
func TestE2E_OldStyleMarkMovedViaServedObjectRelinks(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	h := a.Y1 - a.Y0
	o := domain.Object{UUID: "old", LayerID: "RL", Type: domain.TypeRect, Page: a.Page, Version: 1, OwnerID: w.admin.ID,
		Points: []domain.Point{{X: a.X0, Y: a.Y0 - 0.2*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.2*h}},
		Anchor: &domain.SourceAnchor{RunText: a.Text, Occurrence: 1, CharEnd: 4}, PointsRenderHash: "an-older-render-hash"}
	if _, err := w.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: w.admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
	b := e2eRun(t, w.src, "the gulls are calling out", 1)
	bh := b.Y1 - b.Y0
	w.moveServed("old", []domain.Point{{X: b.X0, Y: b.Y0 - 0.2*bh}, {X: b.X0 + 0.1, Y: b.Y1 + 0.2*bh}}, "move")
	_, got := w.served()
	if g := got["old"]; g.Link == nil || g.Anchor != nil {
		t.Errorf("§6: an old-style mark moved through the real client echo has link %+v anchor %+v; want a new link", g.Link, g.Anchor)
	}
}

// 3. No flip: an arrow drawn right-to-left keeps its direction and every point maps by the one rigid
// transform under a type-size change (k != 1).
func TestE2E_NoFlip(t *testing.T) {
	src := strings.Replace(e2eChart, "# The Quay Test Chart\n", "# The Quay Test Chart\nsize: 11\n", 1)
	w := newE2E(t, src)
	a := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	c, h := e2eCW(a), a.Y1-a.Y0
	arrow := []domain.Point{{X: a.X0 + 25*c, Y: a.Y0 + 0.7*h}, {X: a.X0 + 3*c, Y: a.Y0 + 0.3*h}}
	w.create(domain.Object{UUID: "ar", Page: a.Page, Points: arrow}, "line")
	w.save(strings.Replace(w.src, "size: 11", "size: 15", 1))
	b := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	k := (b.Y1 - b.Y0) / h
	_, got := w.served()
	g := got["ar"]
	if g.Points[0].X <= g.Points[1].X {
		t.Fatalf("the arrow flipped: %v", g.Points)
	}
	// left edge on char 3: TL' = char 3's new x + dx*lh'; TL = (char3 x, y0+0.3h)
	bc := e2eCW(b)
	tl := domain.Point{X: a.X0 + 3*c, Y: a.Y0 + 0.3*h}
	tl2 := domain.Point{X: b.X0 + 3*bc, Y: b.Y0 + 0.3*h*k}
	want := make([]domain.Point, 2)
	for i, p := range arrow {
		want[i] = domain.Point{X: tl2.X + (p.X-tl.X)*k, Y: tl2.Y + (p.Y-tl.Y)*k}
	}
	eqPts(t, "arrow under k", g.Points, want, 2e-3) // char widths are averaged per run; a char's width scales ~k
	if state(g) != "linked" {
		t.Errorf("state %s", state(g))
	}
}

// 4. Typo keeps the link: a stamp on "quay" stays linked and shifts by exactly the typo fix's length change
// earlier on the line; a stamp ON the typo is "check" and still follows.
func TestE2E_TypoKeepsLink(t *testing.T) {
	src := strings.Replace(e2eChart, "a lantern swings above the quay", "a lanten swings above the quay", 1)
	w := newE2E(t, src)
	a := e2eRun(t, w.src, "a lanten swings above the quay", 1)
	c, h := e2eCW(a), a.Y1-a.Y0
	sq := func(x, y, s float64) []domain.Point {
		return []domain.Point{{X: x - s/2, Y: y - s/2}, {X: x + s/2, Y: y + s/2}}
	}
	quay := strings.Index(a.Text, "quay")
	w.create(domain.Object{UUID: "far", Page: a.Page, Points: sq(a.X0+(float64(quay)+1.5)*c, a.Y0+0.5*h, 1.4*h), Text: "star"}, "icon")
	w.create(domain.Object{UUID: "on", Page: a.Page, Points: sq(a.X0+7.0*c, a.Y0+0.5*h, 1.2*h), Text: "star"}, "icon") // straddles the insertion point (between e and n)
	w.save(strings.Replace(w.src, "lanten", "lantern", 1))
	b := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	bc := e2eCW(b)
	_, got := w.served()
	if st := state(got["far"]); st != "linked" {
		t.Errorf("a stamp away from the typo is %s, want linked", st)
	}
	// it sits on the same character, now one position later
	x0 := got["far"].Points[0].X
	wantX0 := b.X0 + float64(quay+1)*bc + (sq(a.X0+(float64(quay)+1.5)*c, 0, 1.4*h)[0].X - (a.X0 + float64(quay)*c))
	if math.Abs(x0-wantX0) > 0.25*bc {
		t.Errorf("stamp x %.5f, want %.5f (on its character)", x0, wantX0)
	}
	if math.Abs(got["far"].Points[0].Y-(sq(0, a.Y0+0.5*h, 1.4*h)[0].Y+b.Y0-a.Y0)) > 1e-6 {
		t.Errorf("stamp y moved")
	}
	if st := state(got["on"]); st != "check" {
		t.Errorf("a stamp over the corrected characters is %s, want check", st)
	}
}

// 5. A duplicate doesn't steal the link: identical lines inserted before AND far above.
func TestE2E_DuplicateDoesNotSteal(t *testing.T) {
	for _, tc := range []struct{ name, edit string }{
		{"right_before", "hold on hold on\nhold on hold on\nwe sail at dawn\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newE2E(t, e2eChart)
			a := e2eRun(t, w.src, "hold on hold on", 2) // the SECOND chorus "hold on"
			h := a.Y1 - a.Y0
			pts := []domain.Point{{X: a.X0 - 0.005, Y: a.Y0 - 0.3*h}, {X: a.X1 + 0.005, Y: a.Y1 + 0.3*h}}
			w.create(domain.Object{UUID: "dup", Page: a.Page, Points: pts}, "rect")
			w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nhold on hold on\n", 1))                                                 // far above
			w.save(strings.Replace(w.src, "we sail at dawn\nhold on hold on\n", "we sail at dawn\nhold on hold on\nhold on hold on\n", 1)) // right before
			b := e2eRun(t, w.src, "hold on hold on", 4)                                                                                    // the original is now the 4th copy
			_, got := w.served()
			if state(got["dup"]) != "linked" {
				t.Errorf("state %s", state(got["dup"]))
			}
			eqPts(t, "duplicate", got["dup"].Points, shift(pts, 0, b.Y0-a.Y0), 1e-6)
		})
	}
}

// 6. Margin follows vertically: icons in the right AND left margins keep x exactly, follow y.
func TestE2E_MarginFollowsVertically(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	h := a.Y1 - a.Y0
	right := []domain.Point{{X: 0.9, Y: a.Y0}, {X: 0.93, Y: a.Y0 + 1.1*h}}
	left := []domain.Point{{X: 0.01, Y: a.Y0 + 0.1*h}, {X: 0.03, Y: a.Y1}}
	w.create(domain.Object{UUID: "mr", Page: a.Page, Points: right, Text: "bell"}, "icon")
	w.create(domain.Object{UUID: "ml", Page: a.Page, Points: left, Text: "bell"}, "icon")
	// lines inserted above AND the line itself lengthened: an end-of-line reference would move x
	s2 := strings.Replace(w.src, "## Verse\n", "## Verse\nG\nthree more words\nand two more\n", 1)
	w.save(strings.Replace(s2, "we sail at dawn", "we sail at first dawn", 1))
	b := e2eRun(t, w.src, "we sail at first dawn", 1)
	_, got := w.served()
	for _, c := range []struct {
		id  string
		pts []domain.Point
	}{{"mr", right}, {"ml", left}} {
		g := got[c.id]
		if g.Link == nil || (state(g) != "linked" && state(g) != "check") {
			t.Errorf("%s: link %+v", c.id, g.Link)
			continue
		}
		eqPts(t, "margin "+c.id, g.Points, shift(c.pts, 0, b.Y0-a.Y0), 1e-9)
	}
}

// Regression: an uploaded-PDF mark is never linked and never moved.
func TestE2E_UploadedPDFUntouched(t *testing.T) {
	w := newE2E(t, e2eChart)
	if _, err := w.eng.Apply(w.song, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: w.admin.ID, Layer: &domain.Layer{
		ID: "PL", FileID: "not-a-chart-file", Name: "PDF", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	pts := []domain.Point{{X: 0.2, Y: 0.2}, {X: 0.3, Y: 0.25}}
	obj := map[string]any{"uuid": "pdf", "layerId": "PL", "type": "rect", "page": 0, "points": ptsJSON(pts),
		"style": map[string]any{"color": "#000000", "opacity": 1, "width": 0.003}}
	w.mut("create", "pdf", obj)
	w.moveServed("pdf", shift(pts, 0.1, 0.1), "move")
	raw, got := w.served()
	if got["pdf"].Link != nil || got["pdf"].Anchor != nil || got["pdf"].PointsRenderHash != "" {
		t.Errorf("an uploaded-PDF mark got %+v", got["pdf"])
	}
	if _, has := raw["pdf"]["link"]; has {
		t.Errorf("uploaded-PDF mark wire carries a link key")
	}
}

// F3 end to end, LAZILY (T195 re-review): an icon nobody touches while its chart is saved 40 times (more
// than the 32 history steps kept as recorded) is never written by a save; a GET part way through compacts
// the history to what the links need; a final edit before its word still moves it onto that word, linked.
func TestE2E_UntouchedMarkSurvivesManySaves(t *testing.T) {
	src := "# A Long Test Chart\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\n\n## Notes\nnote 0\n"
	w := newE2E(t, src)
	a := e2eRun(t, w.src, "beneath a paper moon", 1)
	c, h := e2eCW(a), a.Y1-a.Y0
	moon := float64(strings.Index(a.Text, "moon"))
	icon := []domain.Point{{X: a.X0 + moon*c, Y: a.Y0}, {X: a.X0 + (moon+2)*c, Y: a.Y0 + h}}
	w.create(domain.Object{UUID: "ic", Page: a.Page, Points: icon, Text: "star"}, "icon")
	for i := 1; i <= 40; i++ {
		w.save(strings.Replace(w.src, fmt.Sprintf("note %d\n", i-1), fmt.Sprintf("note %d\n", i), 1))
		if i == 36 {
			w.served() // a reader: the serve compacts the line-id history (one step per referenced render)
			if r, err := w.svc.ChartRenderForFile(w.file); err != nil || len(r.Lines.History) > 33 {
				t.Fatalf("after a serve the history holds %d steps (err %v), want ≤ 33", len(r.Lines.History), err)
			}
		}
	}
	w.save(strings.Replace(w.src, "beneath a paper moon", "beneath a big paper moon", 1))
	b := e2eRun(t, w.src, "beneath a big paper moon", 1)
	_, got := w.served()
	g := got["ic"]
	if state(g) != "linked" {
		t.Errorf("after 41 saves the untouched icon is %s, want linked", state(g))
	}
	wantX := b.X0 + (moon+4)*e2eCW(b) // "moon" moved right by "big "
	if math.Abs(g.Points[0].X-wantX) > 0.25*e2eCW(b) {
		t.Errorf("the icon is at x %.4f, want %.4f (still on \"moon\")", g.Points[0].X, wantX)
	}
}

// F1, the echo half: a restyle's echo carries the stored render hash. A client that moves the mark from
// that echoed object (not from a fresh GET) must still re-link a lost mark.
func TestE2E_LostMarkMovedFromARestyleEchoRelinks(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "the tide rolls in across the stones", 1)
	h := a.Y1 - a.Y0
	w.create(domain.Object{UUID: "le", Page: a.Page, Points: []domain.Point{{X: a.X0 + 0.02, Y: a.Y0 - 0.2*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.2*h}}}, "rect")
	w.save(strings.Replace(w.src, "the tide rolls in across the stones\n", "", 1))
	raw, _ := w.served()
	o := raw["le"]
	o["style"] = map[string]any{"color": "#2563eb", "opacity": 1, "width": 0.003}
	msg := map[string]any{"type": "mutation", "mutation": map[string]any{"kind": "setStyle", "uuid": "le", "object": o}}
	if err := w.ws.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
	echo := w.read("echo")
	var m struct {
		Object map[string]any `json:"object"`
	}
	_ = json.Unmarshal(echo["mutation"], &m)
	d := e2eRun(t, w.src, "we sail at dawn", 1)
	dh := d.Y1 - d.Y0
	m.Object["points"] = ptsJSON([]domain.Point{{X: d.X0 + 0.02, Y: d.Y0 - 0.2*dh}, {X: d.X0 + 0.1, Y: d.Y1 + 0.2*dh}})
	w.mut("move", "le", m.Object)
	if _, got := w.served(); state(got["le"]) != "linked" {
		t.Errorf("a lost mark moved from its restyle echo is %s, want linked", state(got["le"]))
	}
}

// N1 (T195 re-review): a PEER holds the served object from before a chart save (render R1). Someone saves
// (R2), moving the mark's line down three lines; nothing tells the peer. The peer drags the mark 0.05 to the
// right on the page it sees. P27: the server keeps the stored link and shifts its offset by the drag, so the
// next serve shows the mark on its line, 0.05 further right — not snapped back to its R1 position. This was
// red under f2cceb40's eager rebase at save (the stored box was on R2, the drag on R1).
func TestE2E_StalePeerDragLandsRelativeToItsLine(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	h := a.Y1 - a.Y0
	box := []domain.Point{{X: a.X0, Y: a.Y0 - 0.3*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.3*h}}
	w.create(domain.Object{UUID: "pm", Page: a.Page, Points: box}, "rect")
	peerRaw, _ := w.served() // the peer's view, on R1
	peerObj := peerRaw["pm"]
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nDm\nsome new words up here\nand another line\n", 1))
	b := e2eRun(t, w.src, "we sail at dawn", 1)
	peerObj["points"] = ptsJSON(shift(box, 0.05, 0))
	w.mut("move", "pm", peerObj)
	_, got := w.served()
	g := got["pm"]
	if st := state(g); st != "linked" {
		t.Errorf("a stale-peer drag on the link's own render is a known shift: state %s, want linked", st)
	}
	eqPts(t, "stale-peer drag lands relative to its line", g.Points, shift(box, 0.05, b.Y0-a.Y0), 1e-6)
}

// T195 re-review (r4): the peer's view spans TWO saves (R1 → R2 → R3); it drags its R1 object, which is
// the link's own render, so the stored link is shifted by the drag.
func TestE2E_StalePeerAcrossTwoSaves(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	h := a.Y1 - a.Y0
	box := []domain.Point{{X: a.X0, Y: a.Y0 - 0.3*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.3*h}}
	w.create(domain.Object{UUID: "pm", Page: a.Page, Points: box}, "rect")
	peerRaw, _ := w.served()
	peerObj := peerRaw["pm"]
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nDm\nsome new words up here\n", 1))
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nand yet another line\n", 1))
	b := e2eRun(t, w.src, "we sail at dawn", 1)
	peerObj["points"] = ptsJSON(shift(box, 0.05, 0))
	w.mut("move", "pm", peerObj)
	_, got := w.served()
	if st := state(got["pm"]); st != "linked" {
		t.Errorf("two-save stale drag: state %s, want linked", st)
	}
	eqPts(t, "two-save stale drag", got["pm"].Points, shift(box, 0.05, b.Y0-a.Y0), 1e-6)
}

// M2 (T195 re-review r4): a peer opened the song AFTER save 1 — its view is R2, the link was measured on
// R1, so it was served the mark PROJECTED onto R2 — then save 2 lands. Its drag is read in R2's geometry
// (the server keeps recent renders in memory), so the mark lands on its line, where it was dropped, linked.
func TestE2E_StalePeerViewNotTheLinksRender(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	h := a.Y1 - a.Y0
	box := []domain.Point{{X: a.X0, Y: a.Y0 - 0.3*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.3*h}}
	w.create(domain.Object{UUID: "pm", Page: a.Page, Points: box}, "rect")
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nDm\nsome new words up here\n", 1))
	m := e2eRun(t, w.src, "we sail at dawn", 1)
	peerRaw, got := w.served() // view R2: projected points
	peerObj := peerRaw["pm"]
	shown := got["pm"].Points
	w.save(strings.Replace(w.src, "## Verse\n", "## Verse\nand yet another line\n", 1))
	b := e2eRun(t, w.src, "we sail at dawn", 1)
	peerObj["points"] = ptsJSON(shift(shown, 0.05, 0))
	w.mut("move", "pm", peerObj)
	_, got = w.served()
	if st := state(got["pm"]); st != "linked" {
		t.Errorf("stale view R2 (link on R1): state %s, want linked", st)
	}
	eqPts(t, "stale view != link render", got["pm"].Points, shift(shown, 0.05, b.Y0-m.Y0), 1e-6)
}
