package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
)

// mountRehearsalNotes registers T170's four routes. They sit beside the song-file routes and
// share their shape; the differences are deliberate and all in the same direction — a note is
// one owner's private reference image, so every path is keyed on the caller and a stranger's
// note is 404 rather than 403.
func (a *WebAPI) mountRehearsalNotes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/bands/{bandId}/songs/{songId}/rehearsal-notes/{page}", a.auth(a.putRehearsalNote))
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/rehearsal-notes", a.auth(a.listRehearsalNotes))
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/rehearsal-notes/{page}", a.auth(a.getRehearsalNote))
	mux.HandleFunc("DELETE /api/bands/{bandId}/songs/{songId}/rehearsal-notes/{page}", a.auth(a.deleteRehearsalNote))
}

// notePage parses the {page} path segment. A non-numeric or negative page is a bad request and
// not a 404: the route matched, the value did not.
func notePage(r *http.Request) (int, error) {
	p, err := strconv.Atoi(r.PathValue("page"))
	if err != nil || p < 0 {
		return 0, fmt.Errorf("%w: page must be a non-negative integer, got %q", app.ErrInvalidInput, r.PathValue("page"))
	}
	return p, nil
}

func (a *WebAPI) putRehearsalNote(w http.ResponseWriter, r *http.Request, u app.User) {
	page, err := notePage(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Read past the cap so "too large" is a 413 the tablet can explain, not a truncated body
	// that would sniff as something else. Same three guards as uploadFile.
	r.Body = http.MaxBytesReader(w, r.Body, maxRehearsalNoteUpload+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, fmt.Errorf("%w: expected a multipart body with a \"file\" part", app.ErrInvalidInput))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, fmt.Errorf("%w: expected a multipart body with a \"file\" part", app.ErrInvalidInput))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRehearsalNoteUpload+1))
	if err != nil {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	meta := app.RehearsalNoteMeta{
		RasterHash: r.FormValue("rasterHash"),
		ConcertID:  r.FormValue("concertId"),
		ConcertRev: parseUint(r.FormValue("concertRev")),
		TakenAs:    r.FormValue("takenAs"),
		Width:      parseInt(r.FormValue("width")),
		Height:     parseInt(r.FormValue("height")),
	}
	// capturedAt is OPTIONAL and RFC 3339 when present. The tablet has no wall clock for a note
	// today (A70 stamps a boot-relative SystemClock.elapsedRealtime), so an absent or
	// unparseable value stores the zero time and the UI falls back to the upload date. We do not
	// guess: a number that is not a date must never be shown as one.
	if v := r.FormValue("capturedAt"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			meta.CapturedAt = t
		}
	}
	overwrite := r.URL.Query().Get("overwrite") == "1"
	n, err := a.svc.PutRehearsalNote(u, r.PathValue("bandId"), r.PathValue("songId"), page, meta, data, overwrite)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": n})
}

func (a *WebAPI) listRehearsalNotes(w http.ResponseWriter, r *http.Request, u app.User) {
	notes, err := a.svc.ListRehearsalNotes(u, r.PathValue("bandId"), r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if notes == nil {
		notes = []app.RehearsalNoteView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

func (a *WebAPI) getRehearsalNote(w http.ResponseWriter, r *http.Request, u app.User) {
	page, err := notePage(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	n, data, err := a.svc.RehearsalNoteBytes(u, r.PathValue("bandId"), r.PathValue("songId"), page)
	if err != nil {
		writeErr(w, err)
		return
	}
	// The blob hash is a perfect strong ETag, and a note's bytes for a given hash never change
	// (a re-send is a new blob). The URL is not revision-scoped, so revalidate rather than
	// cache hard: "Done, remove" then a fresh send must not serve the removed pixels.
	etag := `"` + n.BlobHash + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	if ifNoneMatch(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	// T141: from the bytes in hand, never from a stored Width/Height/size claim.
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *WebAPI) deleteRehearsalNote(w http.ResponseWriter, r *http.Request, u app.User) {
	page, err := notePage(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := a.svc.DeleteRehearsalNote(u, r.PathValue("bandId"), r.PathValue("songId"), page); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parseUint(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

// bakePageLookup implements app.BakeLookup over the Baker. It is the T170 twin of
// chartAnchorer: the ONE place the service reaches into bake, kept behind a narrow interface
// so app imports nothing from bake.
//
// It reads the LATEST bundle of each concert, which is what "has this page changed?" means —
// the note names the rev it was drawn on, but the question is about the chart in front of the
// musician now.
type bakePageLookup struct{ baker *bake.Baker }

func (l bakePageLookup) PageRasterHash(concertID, songID string, pageInSong int) (string, bool, bool) {
	if l.baker == nil {
		return "", false, false
	}
	for _, cb := range l.baker.ListConcerts() {
		if cb.ConcertID != concertID {
			continue
		}
		for _, s := range cb.Songs {
			if s.SongID != songID {
				continue
			}
			if pageInSong < 0 || pageInSong >= len(s.Pages) {
				return "", false, true // the bake is there; that page is not
			}
			return s.Pages[pageInSong].RasterHash, true, true
		}
		return "", false, true // baked, but this song is not in the concert any more
	}
	return "", false, false // no bake for this concert — we cannot say
}
