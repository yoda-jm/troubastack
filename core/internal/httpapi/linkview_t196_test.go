package httpapi

// T196 (anchoring stage 3) at the production entry points: REST GET /annotations and the realtime echo
// serve link.at / link.text; nothing a client sends is read back as a view; the view is never stored; the
// pre-bake link-check counts Check and Lost marks. Expectations come from the renderer's runs.

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
)

// atOf decodes a raw served object's link.at (nil when absent) and link.text.
func atOf(t *testing.T, raw map[string]any) (*domain.LinkAt, string) {
	t.Helper()
	l, ok := raw["link"].(map[string]any)
	if !ok {
		return nil, ""
	}
	text, _ := l["text"].(string)
	a, ok := l["at"].(map[string]any)
	if !ok {
		return nil, text
	}
	b, _ := json.Marshal(a)
	var at domain.LinkAt
	_ = json.Unmarshal(b, &at)
	return &at, text
}

func checkAt(t *testing.T, label string, at *domain.LinkAt, a chartpdf.Anchor, x float64) {
	t.Helper()
	if at == nil {
		t.Fatalf("%s: no link.at on the wire", label)
	}
	if at.Page != a.Page || math.Abs(at.X-x) > 1e-9 || math.Abs(at.Y-a.Y0) > 1e-9 || math.Abs(at.H-(a.Y1-a.Y0)) > 1e-9 {
		t.Fatalf("%s: link.at %+v, want page %d x %.5f y %.5f h %.5f", label, *at, a.Page, x, a.Y0, a.Y1-a.Y0)
	}
}

// boxOn is a rect around runes [from, to) of a run.
func boxOn(a chartpdf.Anchor, from, to float64) []domain.Point {
	c, h := e2eCW(a), a.Y1-a.Y0
	return []domain.Point{{X: a.X0 + from*c, Y: a.Y0 - 0.15*h}, {X: a.X0 + to*c, Y: a.Y1 + 0.15*h}}
}

func TestT196_GetServesTheView(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "we sail at dawn", 1)
	w.create(domain.Object{UUID: "r", Page: a.Page, Points: boxOn(a, 3.4, 7)}, "rect") // "sail" starts at rune 3
	// An old-style mark (T145 anchor, no link) and a mark on an uploaded file: no view, no link at all.
	h := a.Y1 - a.Y0
	old := domain.Object{UUID: "old", LayerID: "RL", Type: domain.TypeRect, Page: a.Page, Version: 1, OwnerID: w.admin.ID,
		Points: []domain.Point{{X: a.X0, Y: a.Y0 - 0.2*h}, {X: a.X0 + 0.1, Y: a.Y1 + 0.2*h}},
		Anchor: &domain.SourceAnchor{RunText: a.Text, Occurrence: 1, CharEnd: 4}, PointsRenderHash: "an-older-render-hash"}
	if _, err := w.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: old.UUID, AuthorID: w.admin.ID, Object: &old}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.eng.Apply(w.song, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: w.admin.ID, Layer: &domain.Layer{
		ID: "PL", FileID: "not-a-chart-file", Name: "PDF", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	w.mut("create", "pdf", map[string]any{"uuid": "pdf", "layerId": "PL", "type": "rect", "page": 0,
		"points": ptsJSON([]domain.Point{{X: 0.2, Y: 0.2}, {X: 0.3, Y: 0.25}}), "style": map[string]any{"color": "#000000", "opacity": 1, "width": 0.003}})

	raw, _ := w.served()
	at, text := atOf(t, raw["r"])
	checkAt(t, "GET, on-run rect", at, a, a.X0+3*e2eCW(a))
	if text != "we sail at dawn" {
		t.Errorf("link.text %q, want the line's text", text)
	}
	for _, id := range []string{"old", "pdf"} {
		if _, has := raw[id]["link"]; has {
			t.Errorf("%s: carries a link on the wire: %v", id, raw[id]["link"])
		}
	}

	// After an edit, GET serves the view on the NEW render (the same projection that moved the mark).
	w.save(strings.Replace(w.src, "we sail at dawn", "oh we sail at dawn", 1))
	b := e2eRun(t, w.src, "oh we sail at dawn", 1)
	raw, _ = w.served()
	at, text = atOf(t, raw["r"])
	checkAt(t, "GET after an edit before the word", at, b, b.X0+6*e2eCW(b))
	if text != "oh we sail at dawn" {
		t.Errorf("link.text after the edit %q", text)
	}

	// The join snapshot is served the same way (P33).
	url := "ws" + strings.TrimPrefix(w.srv.URL, "http") + "/api/bands/" + w.band + "/songs/" + w.song + "/ws"
	hdr := http.Header{}
	hdr.Add("Cookie", sessionCookie+"="+w.tok)
	ws, _, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	var frame struct {
		Type    string           `json:"type"`
		Objects []map[string]any `json:"objects"`
	}
	if err := ws.ReadJSON(&frame); err != nil || frame.Type != "snapshot" {
		t.Fatalf("join frame %q: %v", frame.Type, err)
	}
	found := false
	for _, ob := range frame.Objects {
		if ob["uuid"] == "r" {
			found = true
			at, _ := atOf(t, ob)
			checkAt(t, "join snapshot", at, b, b.X0+6*e2eCW(b))
		}
	}
	if !found {
		t.Fatalf("the join snapshot has no mark r")
	}
}

// mutEcho sends one mutation and returns the echoed object, raw.
func (w *e2eWorld) mutEcho(kind, uuid string, obj map[string]any) map[string]any {
	w.t.Helper()
	msg := map[string]any{"type": "mutation", "mutation": map[string]any{"kind": kind, "uuid": uuid, "object": obj}}
	if err := w.ws.WriteJSON(msg); err != nil {
		w.t.Fatal(err)
	}
	m := w.read("echo")
	var mut struct {
		Object map[string]any `json:"object"`
	}
	_ = json.Unmarshal(m["mutation"], &mut)
	return mut.Object
}

func TestT196_EchoCarriesTheViewAfterAMove(t *testing.T) {
	w := newE2E(t, e2eChart)
	a := e2eRun(t, w.src, "the tide rolls in across the stones", 1)
	b := e2eRun(t, w.src, "we sail at dawn", 1)
	echo := w.mutEcho("create", "m", map[string]any{"uuid": "m", "layerId": "RL", "type": "rect", "page": a.Page,
		"points": ptsJSON(boxOn(a, 4.2, 9)), "style": map[string]any{"color": "#e11d48", "opacity": 1, "width": 0.003}})
	at, _ := atOf(t, echo)
	checkAt(t, "create echo", at, a, a.X0+4*e2eCW(a))

	// Move the SERVED object (it carries link.at of line A, as Studio's spread does) onto line B: the
	// echo states the new link's view — the server's, never the client's.
	raw, _ := w.served()
	o := raw["m"]
	o["points"] = ptsJSON(boxOn(b, 3.4, 7))
	echo = w.mutEcho("move", "m", o)
	at, text := atOf(t, echo)
	checkAt(t, "move echo", at, b, b.X0+3*e2eCW(b))
	if text != "we sail at dawn" {
		t.Errorf("move echo link.text %q", text)
	}

	// A forged view on a restyle is ignored too: the echo's view is computed from the stored link.
	raw, _ = w.served()
	o = raw["m"]
	o["link"].(map[string]any)["at"] = map[string]any{"page": 9, "x": 0.9, "y": 0.9, "h": 0.5}
	o["link"].(map[string]any)["text"] = "forged"
	o["style"].(map[string]any)["color"] = "#00aa00"
	echo = w.mutEcho("setStyle", "m", o)
	at, text = atOf(t, echo)
	checkAt(t, "restyle echo", at, b, b.X0+3*e2eCW(b))
	if text != "we sail at dawn" {
		t.Errorf("restyle echo link.text %q", text)
	}

	// Never stored: HEAD's copy has no view, and its JSON (what the store writes) has no trace of one.
	head, err := w.eng.Head(w.song)
	if err != nil {
		t.Fatal(err)
	}
	for _, ob := range head.Objects {
		if ob.View != nil {
			t.Errorf("HEAD stored a view on %s: %+v", ob.UUID, ob.View)
		}
		js, _ := json.Marshal(ob)
		if strings.Contains(string(js), "View") || strings.Contains(string(js), "forged") {
			t.Errorf("stored JSON carries the view: %s", js)
		}
	}
}

// The wire DTOs write the view and never read it.
func TestT196_DTOWritesTheViewAndNeverReadsIt(t *testing.T) {
	o := domain.Object{UUID: "u", Type: domain.TypeRect, Link: &domain.MarkLink{State: domain.LinkLinked, Line: "L", XRef: "char", LH: 0.02, Render: "R"},
		View: &domain.LinkView{At: domain.LinkAt{Page: 1, X: 0.25, Y: 0.5, H: 0.02}, Text: "a line"}}
	j := objectToJSON(o)
	if j.Link == nil || j.Link.At == nil || *j.Link.At != o.View.At || j.Link.Text != "a line" || j.Link.Line != "L" {
		t.Fatalf("the DTO did not write the view: %+v", j.Link)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), `"at":{"page":1,"x":0.25,"y":0.5,"h":0.02}`) || !strings.Contains(string(b), `"text":"a line"`) {
		t.Errorf("wire JSON %s lacks link.at / link.text", b)
	}
	back := objectFromJSON(j)
	if back.View != nil {
		t.Errorf("objectFromJSON read a client's view: %+v", back.View)
	}
	if back.Link == nil || back.Link.Line != "L" {
		t.Errorf("the link itself must still be read: %+v", back.Link)
	}
}

// The pre-bake list: per song and generated file, the Check and Lost marks, as served.
func TestT196_LinkCheckCountsCheckAndLost(t *testing.T) {
	w := newE2E(t, e2eChart)
	stones := e2eRun(t, w.src, "the tide rolls in across the stones", 1)
	lantern := e2eRun(t, w.src, "a lantern swings above the quay", 1)
	dawn := e2eRun(t, w.src, "we sail at dawn", 1)
	w.create(domain.Object{UUID: "linked", Page: dawn.Page, Points: boxOn(dawn, 3.2, 7)}, "rect")
	w.create(domain.Object{UUID: "check", Page: lantern.Page, Points: boxOn(lantern, 2.2, 9)}, "rect") // over "lantern"
	w.create(domain.Object{UUID: "lost1", Page: stones.Page, Points: boxOn(stones, 4.2, 9)}, "rect")
	w.create(domain.Object{UUID: "lost2", Page: stones.Page, Points: boxOn(stones, 20, 25)}, "rect")
	w.create(domain.Object{UUID: "fixed", Page: 0, Points: []domain.Point{{X: 0.4, Y: 0.93}, {X: 0.5, Y: 0.97}}}, "rect")
	old := domain.Object{UUID: "old", LayerID: "RL", Type: domain.TypeRect, Page: dawn.Page, Version: 1, OwnerID: w.admin.ID,
		Points: boxOn(dawn, 0, 2), Anchor: &domain.SourceAnchor{RunText: dawn.Text, Occurrence: 1}, PointsRenderHash: "older"}
	if _, err := w.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: old.UUID, AuthorID: w.admin.ID, Object: &old}); err != nil {
		t.Fatal(err)
	}
	// a deleted lost mark is not counted
	w.create(domain.Object{UUID: "gone", Page: stones.Page, Points: boxOn(stones, 10, 14)}, "rect")
	w.mut("delete", "gone", map[string]any{"uuid": "gone"})

	src := strings.Replace(w.src, "the tide rolls in across the stones\n", "", 1)
	src = strings.Replace(src, "a lantern swings", "a lanturn swings", 1)
	w.save(src)
	_, got := w.served()
	for id, want := range map[string]string{"linked": "linked", "check": "check", "lost1": "lost", "lost2": "lost", "fixed": "fixed"} {
		if st := state(got[id]); st != want {
			t.Fatalf("fixture: %s is %s, want %s", id, st, want)
		}
	}

	sl, err := w.svc.CreateSetlist(w.admin, w.band, "Test Set", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the song twice: listed once
		if _, err := w.svc.AddSetlistItem(w.admin, w.band, sl.ID, w.song); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.svc.AddSetlistIntermission(w.admin, w.band, sl.ID, "Break"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(w.srv.URL + "/api/bands/" + w.band + "/setlists/" + sl.ID + "/link-check")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("link-check: %d", resp.StatusCode)
	}
	var lc linkCheckJSON
	if err := json.NewDecoder(resp.Body).Decode(&lc); err != nil {
		t.Fatal(err)
	}
	if lc.Check != 1 || lc.Lost != 2 || len(lc.Songs) != 1 || len(lc.Songs[0].Files) != 1 {
		t.Fatalf("link-check %+v, want 1 check and 2 lost, one song, one file", lc)
	}
	s := lc.Songs[0]
	if s.SongID != w.song || s.Title != "E2E Song" || s.Files[0].FileID != w.file || s.Files[0].Check != 1 || s.Files[0].Lost != 2 || s.Files[0].Filename == "" {
		t.Errorf("link-check row %+v", s)
	}
}

// The pure count: an uploaded file is never listed, nor a file with nothing to look at.
func TestT196_CountLinkStatesSkipsUploadsAndCleanFiles(t *testing.T) {
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "g", FileID: "gen"}, {ID: "u", FileID: "up"}, {ID: "c", FileID: "clean"}},
		Objects: []domain.Object{
			{UUID: "1", LayerID: "g", Link: &domain.MarkLink{State: domain.LinkLost}},
			{UUID: "2", LayerID: "u", Link: &domain.MarkLink{State: domain.LinkLost}}, // cannot happen; still not listed
			{UUID: "3", LayerID: "c", Link: &domain.MarkLink{State: domain.LinkLinked}},
			{UUID: "4", LayerID: "g", Link: &domain.MarkLink{State: domain.LinkCheck}, Deleted: true},
		},
	}
	files := []app.SongFile{{ID: "gen", Filename: "Chart", Generated: true}, {ID: "up", Filename: "Scan"}, {ID: "clean", Filename: "Other", Generated: true}}
	row, ok := countLinkStates("s", "T", files, snap)
	if !ok || len(row.Files) != 1 || row.Files[0].FileID != "gen" || row.Files[0].Lost != 1 || row.Files[0].Check != 0 {
		t.Fatalf("row %+v ok %v, want only the generated file with 1 lost", row, ok)
	}
	if _, ok := countLinkStates("s", "T", files, domain.Snapshot{Layers: snap.Layers}); ok {
		t.Errorf("a song with no marks must not be listed")
	}
}
