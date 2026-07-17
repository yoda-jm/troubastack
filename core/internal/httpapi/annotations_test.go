package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// annPoint / annStyle / annLayer / annObject / annDoc mirror the EXACT annotation
// JSON contract the frontend + seeder speak, so the round-trip test pins the wire shape.
type annPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type annStyle struct {
	Color    string  `json:"color"`
	Opacity  float64 `json:"opacity"`
	Width    float64 `json:"width"`
	FontSize float64 `json:"fontSize"`
}

type annLayer struct {
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

type annObject struct {
	UUID      string     `json:"uuid"`
	LayerID   string     `json:"layerId"`
	Type      string     `json:"type"`
	Points    []annPoint `json:"points"`
	Page      int        `json:"page"`
	Text      string     `json:"text"`
	Order     int        `json:"order"`
	CreatedAt int64      `json:"createdAt"`
	Style     annStyle   `json:"style"`
}

type annDoc struct {
	Layers  []annLayer  `json:"layers"`
	Objects []annObject `json:"objects"`
}

// makeBandSong registers+logs in a fresh user, creates a band and a song, returning
// (bandID, songID). The session cookie is left set on c.
func (c *client) makeBandSong(username string) (string, string) {
	c.t.Helper()
	c.registerLogin(username, "pw-"+username)
	var band struct{ ID string }
	_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band " + username})
	unmarshalField(c.t, body, "band", &band)
	var song struct{ ID string }
	_, body = c.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "Song"})
	unmarshalField(c.t, body, "song", &song)
	return band.ID, song.ID
}

// sampleDoc is one layer per zone + one object of every type, used by the import tests.
func sampleDoc(fileID string) annDoc {
	return annDoc{
		Layers: []annLayer{
			{ID: "L-cond", FileID: fileID, Name: "Cond", OwnerID: "_shared_", Zone: "conductor", Order: 0, Access: "ro", Mandatory: true, RoleTag: "conductor"},
			{ID: "L-shared", FileID: fileID, Name: "Shared", OwnerID: "_shared_", Zone: "shared", Order: 1, Access: "rw", Mandatory: false, RoleTag: ""},
			{ID: "L-pers", FileID: fileID, Name: "Mine", OwnerID: "u1", Zone: "personal", Order: 2, Access: "rw", Mandatory: false, RoleTag: "flute"},
		},
		Objects: []annObject{
			{UUID: "o-free", LayerID: "L-shared", Type: "freehand", Page: 0, Points: []annPoint{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}}, Style: annStyle{Color: "#112233", Opacity: 1, Width: 0.01}},
			{UUID: "o-rect", LayerID: "L-shared", Type: "rect", Page: 0, Points: []annPoint{{X: 0.1, Y: 0.1}, {X: 0.4, Y: 0.4}}, Style: annStyle{Color: "#445566", Opacity: 0.5, Width: 0.02}},
			{UUID: "o-ellipse", LayerID: "L-shared", Type: "ellipse", Page: 1, Points: []annPoint{{X: 0.2, Y: 0.2}, {X: 0.5, Y: 0.5}}, Style: annStyle{Color: "#778899", Opacity: 1, Width: 0.02}},
			{UUID: "o-line", LayerID: "L-cond", Type: "line", Page: 1, Points: []annPoint{{X: 0.0, Y: 0.0}, {X: 1.0, Y: 1.0}}, Style: annStyle{Color: "#aabbcc", Opacity: 1, Width: 0.015}},
			{UUID: "o-highlight", LayerID: "L-shared", Type: "highlight", Page: 2, Points: []annPoint{{X: 0.1, Y: 0.3}, {X: 0.9, Y: 0.35}}, Style: annStyle{Color: "#ffff00", Opacity: 0.4, Width: 0.05}},
			{UUID: "o-text", LayerID: "L-pers", Type: "text", Page: 2, Text: "cue", Points: []annPoint{{X: 0.5, Y: 0.5}}, Style: annStyle{Color: "#000000", Opacity: 1, FontSize: 0.03}},
			// T51 icon stamp: type "icon", glyph id in Text, bbox points, tint in style.
			{UUID: "o-icon", LayerID: "L-shared", Type: "icon", Page: 0, Text: "shaker", Points: []annPoint{{X: 0.6, Y: 0.6}, {X: 0.7, Y: 0.7}}, Style: annStyle{Color: "#2563eb", Opacity: 0.9, Width: 0.01}},
		},
	}
}

func TestAnnotationsEmptySong(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := c.makeBandSong("alice")
			resp, body := c.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
			mustStatus(t, resp, http.StatusOK)
			var doc annDoc
			unmarshalField2(t, body, &doc)
			if doc.Layers == nil || doc.Objects == nil {
				t.Fatalf("empty song must return non-nil empty arrays, got %+v", doc)
			}
			if len(doc.Layers) != 0 || len(doc.Objects) != 0 {
				t.Fatalf("empty song must be empty: %+v", doc)
			}
		})
	}
}

func TestAnnotationsImportAndGet(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := c.makeBandSong("alice")
			in := sampleDoc("file-1")

			resp, body := c.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", in)
			mustStatus(t, resp, http.StatusOK)
			var imported annDoc
			unmarshalField2(t, body, &imported)
			if len(imported.Layers) != 3 {
				t.Fatalf("import should return 3 layers, got %d", len(imported.Layers))
			}
			if len(imported.Objects) != 7 {
				t.Fatalf("import should return 7 objects, got %d", len(imported.Objects))
			}

			// GET returns the same materialized HEAD.
			resp, body = c.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
			mustStatus(t, resp, http.StatusOK)
			var got annDoc
			unmarshalField2(t, body, &got)
			if len(got.Layers) != 3 || len(got.Objects) != 7 {
				t.Fatalf("GET counts: %d layers, %d objects", len(got.Layers), len(got.Objects))
			}

			// Layer fields round-trip (find the conductor layer).
			var cond *annLayer
			for i := range got.Layers {
				if got.Layers[i].ID == "L-cond" {
					cond = &got.Layers[i]
				}
			}
			if cond == nil {
				t.Fatal("L-cond layer missing from HEAD")
			}
			if cond.Zone != "conductor" || cond.Access != "ro" || !cond.Mandatory || cond.RoleTag != "conductor" || cond.FileID != "file-1" {
				t.Fatalf("conductor layer round-trip mismatch: %+v", *cond)
			}

			// A sample object's fields round-trip (the text object exercises every field).
			var txt *annObject
			for i := range got.Objects {
				if got.Objects[i].UUID == "o-text" {
					txt = &got.Objects[i]
				}
			}
			if txt == nil {
				t.Fatal("o-text object missing from HEAD")
			}
			if txt.Type != "text" || txt.LayerID != "L-pers" || txt.Page != 2 || txt.Text != "cue" {
				t.Fatalf("text object round-trip mismatch: %+v", *txt)
			}
			if txt.Style.Color != "#000000" || txt.Style.Opacity != 1 || txt.Style.FontSize != 0.03 {
				t.Fatalf("text style round-trip mismatch: %+v", txt.Style)
			}
			if len(txt.Points) != 1 || txt.Points[0].X != 0.5 || txt.Points[0].Y != 0.5 {
				t.Fatalf("text points round-trip mismatch: %+v", txt.Points)
			}

			// The highlight object round-trips its type + page.
			var hl *annObject
			for i := range got.Objects {
				if got.Objects[i].UUID == "o-highlight" {
					hl = &got.Objects[i]
				}
			}
			if hl == nil || hl.Type != "highlight" || hl.Page != 2 {
				t.Fatalf("highlight object round-trip mismatch: %+v", hl)
			}

			// T51: the icon stamp round-trips — type "icon", glyph id in Text, tint in
			// style. (Pre-T51 the server maps "icon" → Unspecified and drops it.)
			var ic *annObject
			for i := range got.Objects {
				if got.Objects[i].UUID == "o-icon" {
					ic = &got.Objects[i]
				}
			}
			if ic == nil {
				t.Fatal("o-icon object missing from HEAD (server dropped the icon type?)")
			}
			if ic.Type != "icon" || ic.Text != "shaker" || ic.Style.Color != "#2563eb" || ic.Style.Opacity != 0.9 {
				t.Fatalf("icon object round-trip mismatch: %+v (style %+v)", *ic, ic.Style)
			}
			if len(ic.Points) != 2 {
				t.Fatalf("icon should keep its 2 bbox points, got %+v", ic.Points)
			}
		})
	}
}

func TestAnnotationsImportIdempotent(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := c.makeBandSong("alice")
			in := sampleDoc("file-1")

			resp, _ := c.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", in)
			mustStatus(t, resp, http.StatusOK)
			// Re-import the exact same doc: must not duplicate.
			resp, body := c.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", in)
			mustStatus(t, resp, http.StatusOK)
			var got annDoc
			unmarshalField2(t, body, &got)
			if len(got.Layers) != 3 || len(got.Objects) != 7 {
				t.Fatalf("re-import must not duplicate: got %d layers, %d objects", len(got.Layers), len(got.Objects))
			}
		})
	}
}

func TestAnnotationsNonMemberForbidden(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			owner := newClient(t, repo)
			band, song := owner.makeBandSong("alice")

			// A different logged-in user who is not a member of the band.
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")

			resp, _ := outsider.do(http.MethodGet, "/api/bands/"+band+"/songs/"+song+"/annotations", nil)
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = outsider.do(http.MethodPost, "/api/bands/"+band+"/songs/"+song+"/annotations/import", sampleDoc("f"))
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

func TestAnnotationsSongFromAnotherBand(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			c := newClient(t, repo)
			// User belongs to two bands; ask for bandA's path with bandB's song id.
			c.registerLogin("alice", "pw")
			var bandA, bandB struct{ ID string }
			_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "A"})
			unmarshalField(c.t, body, "band", &bandA)
			_, body = c.do(http.MethodPost, "/api/bands", map[string]string{"name": "B"})
			unmarshalField(c.t, body, "band", &bandB)
			var songB struct{ ID string }
			_, body = c.do(http.MethodPost, "/api/bands/"+bandB.ID+"/songs", map[string]string{"title": "S"})
			unmarshalField(c.t, body, "song", &songB)

			// songB does not belong to bandA → 404, even though caller is a member of both.
			resp, _ := c.do(http.MethodGet, "/api/bands/"+bandA.ID+"/songs/"+songB.ID+"/annotations", nil)
			mustStatus(t, resp, http.StatusNotFound)
			resp, _ = c.do(http.MethodPost, "/api/bands/"+bandA.ID+"/songs/"+songB.ID+"/annotations/import", sampleDoc("f"))
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}

// unmarshalField2 decodes the whole annotations response body (which has no wrapper
// key — it is the {layers,objects} doc itself) into dst.
func unmarshalField2(t *testing.T, body map[string]json.RawMessage, dst *annDoc) {
	t.Helper()
	// Reassemble from the two top-level keys present in the response.
	var doc annDoc
	if raw, ok := body["layers"]; ok {
		if err := json.Unmarshal(raw, &doc.Layers); err != nil {
			t.Fatalf("decode layers: %v", err)
		}
	}
	if raw, ok := body["objects"]; ok {
		if err := json.Unmarshal(raw, &doc.Objects); err != nil {
			t.Fatalf("decode objects: %v", err)
		}
	}
	*dst = doc
}
