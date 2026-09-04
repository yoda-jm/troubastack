package httpapi_test

import (
	"net/http"
	"strconv"
	"testing"

	"troubastack/core/internal/app"
)

// TestDownloadFile_ContentLengthMatchesBody is the T141 defence: downloadFile must set Content-Length from
// the bytes it is holding, never from the stored Size field. A Size that disagrees with the blob (a
// generated chart whose Size was its shorter source length) otherwise makes Go cap the write at the wrong
// length, and the viewer sees a truncated body / "failed to fetch". Reproduce with a deliberately-wrong
// Size; the served body must still be the whole blob and match its Content-Length.
func TestDownloadFile_ContentLengthMatchesBody(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			member := newClient(t, repo)
			member.registerLogin("alice", "pw")
			_, body := member.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
			var band app.Band
			unmarshalField(t, body, "band", &band)
			_, body = member.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "S"})
			var song app.Song
			unmarshalField(t, body, "song", &song)

			base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"
			resp, body := member.upload(base, "chart.pdf", "application/pdf", smallPDF)
			mustStatus(t, resp, http.StatusCreated)
			var f app.SongFile
			unmarshalField(t, body, "file", &f)

			// Corrupt Size to a value BELOW the blob length (mimics a generated chart whose Size was the
			// shorter source). The blob itself is untouched — only the stored claim is wrong.
			rec, err := repo.GetSongFile(f.ID)
			if err != nil {
				t.Fatal(err)
			}
			rec.Size = int64(len(smallPDF) - 10)
			if err := repo.UpdateSongFile(rec); err != nil {
				t.Fatal(err)
			}

			resp, raw := member.getRaw("/api/files/" + f.ID)
			mustStatus(t, resp, http.StatusOK)
			// the whole blob must come back — this is what the truncation broke
			if len(raw) != len(smallPDF) {
				t.Fatalf("served %d bytes, want the whole blob %d — Content-Length truncated the response", len(raw), len(smallPDF))
			}
			// and the declared length must match what was actually sent
			if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(raw)) {
				t.Fatalf("Content-Length=%s disagrees with the %d-byte body", cl, len(raw))
			}
		})
	}
}
