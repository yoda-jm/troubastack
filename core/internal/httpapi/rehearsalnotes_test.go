package httpapi_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"

	"context"
	"net/http/httptest"

	"troubastack/core/internal/httpapi"
)

// ---- fixtures -------------------------------------------------------------------------

// notePNG builds a small transparent PNG with one opaque pixel — the shape of a real note
// (the first one off the tablet was 1600×2261 and 99.93% transparent). tag varies the bytes
// so two calls give two DIFFERENT content hashes, which is what the blob-sharing cases need.
func notePNG(t *testing.T, tag uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.NRGBA{R: 17, G: 17, B: 17, A: 255})
	img.Set(2, 2, color.NRGBA{R: tag, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

// noteJPEG is the wrong format wearing the right name — §5.1's "a JPEG with a .png name".
func noteJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

// putNote PUTs a multipart note with the §3.2 fields. filename is deliberately a caller's
// claim: the server must decide from the bytes, never from this.
func (c *client) putNote(path, filename string, data []byte, fields map[string]string) (*http.Response, map[string]json.RawMessage) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	hdr := map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="` + filename + `"`},
		"Content-Type":        {"image/png"}, // the LIE, when the bytes are not a PNG
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		c.t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		c.t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		c.t.Fatalf("close mw: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, c.srv.URL+path, &buf)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("put %s: %v", path, err)
	}
	defer resp.Body.Close()
	var decoded map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp, decoded
}

// noteFields is the metadata a tablet sends. capturedAt is absent on purpose: that is the
// real case today (A70 has only a boot-relative counter), so it is the case the tests use.
func noteFields(rasterHash, concertID string) map[string]string {
	return map[string]string{
		"rasterHash": rasterHash,
		"concertId":  concertID,
		"concertRev": "8",
		"takenAs":    "member-7",
		"width":      "1600",
		"height":     "2261",
	}
}

// bandSong registers a user and gives them a band with one song.
func bandSong(t *testing.T, c *client, user string) (string, string) {
	t.Helper()
	c.registerLogin(user, "pw123456")
	_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
	var band app.Band
	unmarshalField(t, body, "band", &band)
	_, body = c.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "S"})
	var song app.Song
	unmarshalField(t, body, "song", &song)
	return band.ID, song.ID
}

func notesBase(bandID, songID string) string {
	return "/api/bands/" + bandID + "/songs/" + songID + "/rehearsal-notes"
}

// ---- the contract ---------------------------------------------------------------------

// TestRehearsalNote_RoundTrip is the spine: a member PUTs, the list shows it, and the bytes
// come back byte-for-byte with a Content-Length derived from the payload (T141 — never from a
// stored Width/Height/size claim).
func TestRehearsalNote_RoundTrip(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			bandID, songID := bandSong(t, c, "alice")
			base := notesBase(bandID, songID)
			data := notePNG(t, 1)

			resp, body := c.putNote(base+"/0", "note.png", data, noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK)
			var n app.RehearsalNote
			unmarshalField(t, body, "note", &n)
			if n.OwnerUserID == "" || n.PageInSong != 0 || n.RasterHash != "hash-a" || n.ConcertRev != 8 {
				t.Fatalf("stored note did not keep what was sent: %+v", n)
			}
			if !n.CapturedAt.IsZero() {
				t.Fatalf("capturedAt was absent from the request; storing %v invents a date the tablet never sent", n.CapturedAt)
			}
			if n.UploadedAt.IsZero() {
				t.Fatal("uploadedAt must always be set — it is the only date the UI can honestly show today")
			}

			resp, listBody := c.do(http.MethodGet, base, nil)
			mustStatus(t, resp, http.StatusOK)
			var listed []app.RehearsalNoteView
			unmarshalField(t, listBody, "notes", &listed)
			if len(listed) != 1 || listed[0].PageInSong != 0 {
				t.Fatalf("list = %+v, want the one note just sent", listed)
			}

			resp, raw := c.getRaw(base + "/0")
			mustStatus(t, resp, http.StatusOK)
			if !bytes.Equal(raw, data) {
				t.Fatalf("served %d bytes, want the %d stored", len(raw), len(data))
			}
			if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(data)) {
				t.Fatalf("Content-Length=%s disagrees with the %d-byte body", cl, len(data))
			}
			if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
				t.Fatalf("Content-Type=%q, want image/png", ct)
			}
		})
	}
}

// TestRehearsalNote_OverwriteIsAsked reproduces VLL's rule: a second send for the same
// song+page must ASK, not silently replace. And when it is allowed, the bytes it replaced must
// not linger — nor may the replacement lose its own bytes to the cleanup.
func TestRehearsalNote_OverwriteIsAsked(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			c := newClient(t, repo)
			bandID, songID := bandSong(t, c, "alice")
			base := notesBase(bandID, songID)

			first := notePNG(t, 1)
			resp, _ := c.putNote(base+"/0", "note.png", first, noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK)

			second := notePNG(t, 2)
			resp, _ = c.putNote(base+"/0", "note.png", second, noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusConflict)

			// the bytes in Studio are still the FIRST ones — a refused overwrite changed nothing
			_, raw := c.getRaw(base + "/0")
			if !bytes.Equal(raw, first) {
				t.Fatal("a 409 must leave the stored note untouched")
			}

			resp, _ = c.putNote(base+"/0?overwrite=1", "note.png", second, noteFields("hash-b", "concert-1"))
			mustStatus(t, resp, http.StatusOK)
			_, raw = c.getRaw(base + "/0")
			if !bytes.Equal(raw, second) {
				t.Fatal("overwrite=1 must replace the bytes")
			}
			// exactly one note for the page: an overwrite must not accumulate
			_, listBody := c.do(http.MethodGet, base, nil)
			var listed []app.RehearsalNoteView
			unmarshalField(t, listBody, "notes", &listed)
			if len(listed) != 1 {
				t.Fatalf("after overwrite there are %d notes for one page, want 1", len(listed))
			}
		})
	}
}

// TestRehearsalNote_ResendIdenticalKeepsItsBytes is the trap inside the overwrite path: the
// ordinary re-send is IDENTICAL bytes, which content-addressing makes the SAME blob. A cleanup
// that dereferences the old hash unconditionally deletes the blob the new record points at,
// and the note reads back as a 404 with everything apparently fine in the database.
func TestRehearsalNote_ResendIdenticalKeepsItsBytes(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			bandID, songID := bandSong(t, c, "alice")
			base := notesBase(bandID, songID)
			data := notePNG(t, 1)

			resp, _ := c.putNote(base+"/0", "note.png", data, noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK)
			resp, _ = c.putNote(base+"/0?overwrite=1", "note.png", data, noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK)

			resp, raw := c.getRaw(base + "/0")
			mustStatus(t, resp, http.StatusOK)
			if !bytes.Equal(raw, data) {
				t.Fatalf("re-sending the same note lost its bytes (%d back, %d sent)", len(raw), len(data))
			}
		})
	}
}

// TestRehearsalNote_IsPrivateToItsSender: two members of the SAME band. Each sees only their
// own, and the other's is 404 — not 403, because "someone has a note on this page" is itself
// information a non-owner does not get.
func TestRehearsalNote_IsPrivateToItsSender(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			alice := newClient(t, repo)
			bandID, songID := bandSong(t, alice, "alice")
			base := notesBase(bandID, songID)

			// bob joins the same band through an invite link, on the same server
			_, body := alice.do(http.MethodPost, "/api/bands/"+bandID+"/invite-links", map[string]any{"role": "member"})
			var token string
			unmarshalField(t, body, "token", &token)
			bob := &client{t: t, srv: alice.srv}
			bob.registerLogin("bob", "pw123456")
			resp, _ := bob.do(http.MethodPost, "/api/invite-links/"+token+"/accept", nil)
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				t.Fatalf("bob could not join the band: %d", resp.StatusCode)
			}

			resp, _ = alice.putNote(base+"/0", "note.png", notePNG(t, 1), noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK)
			resp, _ = bob.putNote(base+"/0", "note.png", notePNG(t, 2), noteFields("hash-a", "concert-1"))
			mustStatus(t, resp, http.StatusOK) // bob's own note for the same page is NOT a conflict

			// each lists exactly one — their own
			for _, who := range []struct {
				name string
				c    *client
				want []byte
			}{{"alice", alice, notePNG(t, 1)}, {"bob", bob, notePNG(t, 2)}} {
				_, listBody := who.c.do(http.MethodGet, base, nil)
				var listed []app.RehearsalNoteView
				unmarshalField(t, listBody, "notes", &listed)
				if len(listed) != 1 {
					t.Fatalf("%s sees %d notes, want only their own", who.name, len(listed))
				}
				_, raw := who.c.getRaw(base + "/0")
				if !bytes.Equal(raw, who.want) {
					t.Fatalf("%s was served someone else's note", who.name)
				}
			}

			// and deleting is per-owner too: bob's DELETE leaves alice's alone
			resp, _ = bob.do(http.MethodDelete, base+"/0", nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, raw := alice.getRaw(base + "/0")
			mustStatus(t, resp, http.StatusOK)
			if !bytes.Equal(raw, notePNG(t, 1)) {
				t.Fatal("bob's delete took alice's note with it")
			}
		})
	}
}

// TestRehearsalNote_NonMemberIsForbidden — the existing band rule, unchanged.
func TestRehearsalNote_NonMemberIsForbidden(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			alice := newClient(t, repo)
			bandID, songID := bandSong(t, alice, "alice")
			base := notesBase(bandID, songID)

			stranger := &client{t: t, srv: alice.srv}
			stranger.registerLogin("mallory", "pw123456")
			resp, _ := stranger.putNote(base+"/0", "note.png", notePNG(t, 1), noteFields("h", "c"))
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = stranger.do(http.MethodGet, base, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

// TestRehearsalNote_RejectsWrongFormatAndOversize: the declared type is a claim, the bytes are
// the fact. A JPEG named .png and declared image/png is 415; over the cap is 413.
func TestRehearsalNote_RejectsWrongFormatAndOversize(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			bandID, songID := bandSong(t, c, "alice")
			base := notesBase(bandID, songID)

			resp, _ := c.putNote(base+"/0", "note.png", noteJPEG(t), noteFields("h", "c"))
			mustStatus(t, resp, http.StatusUnsupportedMediaType)

			// a real PNG header followed by enough bytes to pass the cap: the format is right,
			// the size is not, so this isolates the size rule from the format rule.
			big := append(notePNG(t, 1), bytes.Repeat([]byte{0}, (4<<20)+1)...)
			resp, _ = c.putNote(base+"/0", "note.png", big, noteFields("h", "c"))
			mustStatus(t, resp, http.StatusRequestEntityTooLarge)

			// neither attempt created anything
			_, listBody := c.do(http.MethodGet, base, nil)
			var listed []app.RehearsalNoteView
			unmarshalField(t, listBody, "notes", &listed)
			if len(listed) != 0 {
				t.Fatalf("a rejected upload left %d notes behind", len(listed))
			}
		})
	}
}

// TestRehearsalNote_DeleteAndMissing: "Done, remove" removes it; removing what is not there is
// a 404 and not a cheerful no-op.
func TestRehearsalNote_DeleteAndMissing(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			bandID, songID := bandSong(t, c, "alice")
			base := notesBase(bandID, songID)

			resp, _ := c.do(http.MethodDelete, base+"/3", nil)
			mustStatus(t, resp, http.StatusNotFound)

			resp, _ = c.putNote(base+"/3", "note.png", notePNG(t, 1), noteFields("h", "c"))
			mustStatus(t, resp, http.StatusOK)
			resp, _ = c.do(http.MethodDelete, base+"/3", nil)
			mustStatus(t, resp, http.StatusNoContent)

			resp, _ = c.getRaw(base + "/3")
			mustStatus(t, resp, http.StatusNotFound)
			_, listBody := c.do(http.MethodGet, base, nil)
			var listed []app.RehearsalNoteView
			unmarshalField(t, listBody, "notes", &listed)
			if len(listed) != 0 {
				t.Fatalf("after Done-remove the list still has %d", len(listed))
			}
			// and a second delete is a 404, not a 204 — the row is genuinely gone
			resp, _ = c.do(http.MethodDelete, base+"/3", nil)
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}

// TestRehearsalNote_BadPageSegment — the route matched, the value did not.
func TestRehearsalNote_BadPageSegment(t *testing.T) {
	c := newClient(t, memrepoForTest(t))
	bandID, songID := bandSong(t, c, "alice")
	base := notesBase(bandID, songID)
	for _, seg := range []string{"x", "-1", "1.5"} {
		resp, _ := c.do(http.MethodDelete, base+"/"+seg, nil)
		mustStatus(t, resp, http.StatusBadRequest)
	}
}

func memrepoForTest(t *testing.T) app.Repo { return backends()[0].make(t) }

// ---- pageChanged, against a REAL Baker over a fixture bake dir --------------------------

// newClientWithBakes is newClient plus a real bake.Baker rooted at bakesDir, so the
// app.BakeLookup seam is exercised through the production adapter and the production
// ListConcerts — not a stub that could agree with a wrong field name.
func newClientWithBakes(t *testing.T, repo app.Repo, bakesDir string) *client {
	t.Helper()
	svc := app.NewService(repo)
	eng := engine.New(memstore.New().(store.HistoryAware))
	baker := bake.New(svc, eng, bake.Config{BakesDir: bakesDir})
	h, err := httpapi.Router(context.Background(), svc, eng, baker, false, "")
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &client{t: t, srv: srv}
}

// writeBundle drops a minimal latest-rev bundle.json where ListConcerts will find it.
func writeBundle(t *testing.T, bakesDir, concertID, songID string, pageHashes ...string) {
	t.Helper()
	dir := filepath.Join(bakesDir, concertID, "1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	pages := make([]map[string]any, 0, len(pageHashes))
	for _, h := range pageHashes {
		pages = append(pages, map[string]any{"rasterHash": h})
	}
	b, err := json.Marshal(map[string]any{
		"concertId": concertID, "concertRev": "1",
		"songs": []map[string]any{{"songId": songID, "pages": pages}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRehearsalNote_PageChanged is §3.4's whole truth table, and the reason PageChanged is a
// pointer: "we did not check" is a THIRD answer and must not read as "unchanged".
func TestRehearsalNote_PageChanged(t *testing.T) {
	bakesDir := t.TempDir()
	repo := backends()[0].make(t)
	c := newClientWithBakes(t, repo, bakesDir)
	bandID, songID := bandSong(t, c, "alice")
	base := notesBase(bandID, songID)

	writeBundle(t, bakesDir, "concert-1", songID, "hash-p0", "hash-p1")

	cases := []struct {
		name      string
		page      string
		rasterHsh string
		concert   string
		want      *bool
		why       string
	}{
		{"same hash", "0", "hash-p0", "concert-1", boolp(false), "the page still hashes the same"},
		{"different hash", "1", "stale-hash", "concert-1", boolp(true), "the page was re-rendered under it"},
		{"page index gone", "5", "hash-p0", "concert-1", boolp(true), "the song no longer has that page"},
		{"no bake at all", "7", "hash-p0", "never-baked", nil, "nothing to compare against — unknown, not false"},
	}
	for _, tc := range cases {
		resp, _ := c.putNote(base+"/"+tc.page, "note.png", notePNG(t, 1), noteFields(tc.rasterHsh, tc.concert))
		mustStatus(t, resp, http.StatusOK)
	}
	_, listBody := c.do(http.MethodGet, base, nil)
	var listed []app.RehearsalNoteView
	unmarshalField(t, listBody, "notes", &listed)
	byPage := map[int]app.RehearsalNoteView{}
	for _, n := range listed {
		byPage[n.PageInSong] = n
	}
	for _, tc := range cases {
		p, _ := strconv.Atoi(tc.page)
		got := byPage[p].PageChanged
		if !samePtrBool(got, tc.want) {
			t.Errorf("%s: pageChanged = %s, want %s (%s)", tc.name, showPtrBool(got), showPtrBool(tc.want), tc.why)
		}
	}

	// the JSON must actually carry null for unknown. A client branches on this, and `false`
	// and `null` are the two answers it must never see confused.
	var rawNotes []map[string]json.RawMessage
	unmarshalField(t, listBody, "notes", &rawNotes)
	wantRaw := map[int]string{0: "false", 1: "true", 5: "true", 7: "null"}
	for _, n := range rawNotes {
		var page int
		if err := json.Unmarshal(n["pageInSong"], &page); err != nil {
			t.Fatal(err)
		}
		if got := string(n["pageChanged"]); got != wantRaw[page] {
			t.Errorf("page %d serialised pageChanged as %s, want %s", page, got, wantRaw[page])
		}
	}
}

func boolp(b bool) *bool { return &b }

func samePtrBool(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func showPtrBool(b *bool) string {
	if b == nil {
		return "unknown(null)"
	}
	return strconv.FormatBool(*b)
}

// TestRehearsalNote_UnknownWithoutABaker: a server built without bake must not claim every
// note is current. The nil baker travels all the way to a nil answer.
func TestRehearsalNote_UnknownWithoutABaker(t *testing.T) {
	c := newClient(t, backends()[0].make(t)) // Router(..., baker=nil, ...)
	bandID, songID := bandSong(t, c, "alice")
	base := notesBase(bandID, songID)
	resp, _ := c.putNote(base+"/0", "note.png", notePNG(t, 1), noteFields("hash-a", "concert-1"))
	mustStatus(t, resp, http.StatusOK)

	_, listBody := c.do(http.MethodGet, base, nil)
	var listed []app.RehearsalNoteView
	unmarshalField(t, listBody, "notes", &listed)
	if len(listed) != 1 || listed[0].PageChanged != nil {
		t.Fatalf("pageChanged = %s without a baker; want unknown", showPtrBool(listed[0].PageChanged))
	}
}

// TestRehearsalNote_CapturedAtWhenSent: the field is optional, but when the tablet DOES send a
// real date it must survive. This is the forward case for the mobile slice.
func TestRehearsalNote_CapturedAtWhenSent(t *testing.T) {
	c := newClient(t, backends()[0].make(t))
	bandID, songID := bandSong(t, c, "alice")
	f := noteFields("hash-a", "concert-1")
	f["capturedAt"] = "2026-09-11T20:58:00Z"
	resp, body := c.putNote(notesBase(bandID, songID)+"/0", "note.png", notePNG(t, 1), f)
	mustStatus(t, resp, http.StatusOK)
	var n app.RehearsalNote
	unmarshalField(t, body, "note", &n)
	if n.CapturedAt.IsZero() || n.CapturedAt.UTC().Format("2006-01-02T15:04:05Z") != "2026-09-11T20:58:00Z" {
		t.Fatalf("capturedAt round-trip = %v", n.CapturedAt)
	}

	// and a value that is NOT a date — e.g. A70's boot-relative counter — is dropped, not
	// coerced. Showing 4564568383 as a date is the failure this guards.
	f["capturedAt"] = "4564568383"
	resp, body = c.putNote(notesBase(bandID, songID)+"/1", "note.png", notePNG(t, 2), f)
	mustStatus(t, resp, http.StatusOK)
	unmarshalField(t, body, "note", &n)
	if !n.CapturedAt.IsZero() {
		t.Fatalf("a boot counter was accepted as capturedAt = %v", n.CapturedAt)
	}
}
