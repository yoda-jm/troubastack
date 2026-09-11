package app_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// repos runs a test against BOTH Repo backends. A rehearsal note is a new record type, so the
// two implementations must agree on it exactly as they do on everything else; the file backend
// is the one that can also get the persisted-map wiring wrong.
func repos(t *testing.T) map[string]func(*testing.T) app.Repo {
	t.Helper()
	return map[string]func(*testing.T) app.Repo{
		"mem": func(*testing.T) app.Repo { return memrepo.New() },
		"file": func(t *testing.T) app.Repo {
			r, err := filerepo.New(t.TempDir())
			if err != nil {
				t.Fatalf("filerepo.New: %v", err)
			}
			return r
		},
	}
}

func testPNG(t *testing.T, tag uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.NRGBA{R: tag, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// noteFixture wires a service with an inspectable blob store and returns the owner + a band
// with one song.
func noteFixture(t *testing.T, repo app.Repo) (*app.Service, *blob.Mem, app.User, string, string) {
	t.Helper()
	blobs := blob.NewMem()
	svc := app.NewService(repo).WithBlobStore(blobs)
	u, err := svc.Register("alice", "Alice", "pw123456", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "S", "")
	if err != nil {
		t.Fatal(err)
	}
	return svc, blobs, u, band.ID, song.ID
}

// TestRehearsalNoteBlobLifetime is the whole reference-counting story in one place. Bytes are
// content-addressed and therefore SHARED, so every rule here is about not deleting pixels
// somebody else still points at — and about actually deleting the ones nobody does.
func TestRehearsalNoteBlobLifetime(t *testing.T) {
	for name, mk := range repos(t) {
		t.Run(name, func(t *testing.T) {
			svc, blobs, alice, bandID, songID := noteFixture(t, mk(t))
			meta := app.RehearsalNoteMeta{RasterHash: "h", ConcertID: "c"}

			first, second := testPNG(t, 1), testPNG(t, 2)
			n1, err := svc.PutRehearsalNote(alice, bandID, songID, 0, meta, first, false)
			if err != nil {
				t.Fatal(err)
			}
			// replacing with DIFFERENT bytes drops the old ones: nothing else points at them
			if _, err := svc.PutRehearsalNote(alice, bandID, songID, 0, meta, second, true); err != nil {
				t.Fatal(err)
			}
			if _, err := blobs.Get(n1.BlobHash); err == nil {
				t.Error("the replaced note's bytes were left behind — a blob leak on every overwrite")
			}

			// two notes (different pages) on the SAME bytes share one blob; deleting one must
			// not take the other's pixels
			shared := testPNG(t, 3)
			a, err := svc.PutRehearsalNote(alice, bandID, songID, 1, meta, shared, false)
			if err != nil {
				t.Fatal(err)
			}
			b, err := svc.PutRehearsalNote(alice, bandID, songID, 2, meta, shared, false)
			if err != nil {
				t.Fatal(err)
			}
			if a.BlobHash != b.BlobHash {
				t.Fatal("identical bytes must be one blob — the rest of this test assumes content addressing")
			}
			if err := svc.DeleteRehearsalNote(alice, bandID, songID, 1); err != nil {
				t.Fatal(err)
			}
			if _, err := blobs.Get(b.BlobHash); err != nil {
				t.Error("deleting one of two notes on the same bytes deleted the bytes")
			}
			// and once the last one goes, the bytes go
			if err := svc.DeleteRehearsalNote(alice, bandID, songID, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := blobs.Get(b.BlobHash); err == nil {
				t.Error("the last note referencing these bytes was deleted; the bytes should be gone")
			}
		})
	}
}

// TestRehearsalNoteAndSongFileShareABlob is the cross-feature case, and the reason derefBlob
// could not stay as it was. An image song file and a note can be the SAME bytes — that is what
// content addressing means — and before T170 the file-delete path asked only "does any SongFile
// still point here?". The answer was no, and the note's pixels went with the file.
func TestRehearsalNoteAndSongFileShareABlob(t *testing.T) {
	for name, mk := range repos(t) {
		t.Run(name, func(t *testing.T) {
			svc, blobs, alice, bandID, songID := noteFixture(t, mk(t))
			shared := testPNG(t, 7)

			f, err := svc.UploadSongFile(alice, bandID, songID, "page.png", "image/png", shared)
			if err != nil {
				t.Fatal(err)
			}
			n, err := svc.PutRehearsalNote(alice, bandID, songID, 0, app.RehearsalNoteMeta{}, shared, false)
			if err != nil {
				t.Fatal(err)
			}
			if f.BlobHash != n.BlobHash {
				t.Fatal("the file and the note should be the same blob")
			}

			if err := svc.DeleteSongFile(alice, bandID, songID, f.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := blobs.Get(n.BlobHash); err != nil {
				t.Fatal("deleting the song file took the rehearsal note's pixels with it")
			}
			// the note itself still serves
			if _, data, err := svc.RehearsalNoteBytes(alice, bandID, songID, 0); err != nil || !bytes.Equal(data, shared) {
				t.Fatalf("note unreadable after the file delete: %v", err)
			}
		})
	}
}

// stubBake is the narrow seam, stubbed: the component-level half of the pageChanged tests
// (httpapi exercises the real Baker over a fixture bundle).
type stubBake struct {
	hash                string
	pageExists, isBaked bool
	calls               int
}

func (s *stubBake) PageRasterHash(string, string, int) (string, bool, bool) {
	s.calls++
	return s.hash, s.pageExists, s.isBaked
}

// TestPageChangedIsThreeValued pins the distinction the *bool exists for: "we could not check"
// must never reach the musician as "your reference is current".
func TestPageChangedIsThreeValued(t *testing.T) {
	cases := []struct {
		name string
		bake *stubBake
		want *bool
	}{
		{"same hash", &stubBake{hash: "h", pageExists: true, isBaked: true}, boolPtr(false)},
		{"rehashed", &stubBake{hash: "other", pageExists: true, isBaked: true}, boolPtr(true)},
		{"page gone", &stubBake{hash: "", pageExists: false, isBaked: true}, boolPtr(true)},
		{"never baked", &stubBake{isBaked: false}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, alice, bandID, songID := noteFixture(t, memrepo.New())
			svc.WithBakeLookup(tc.bake)
			if _, err := svc.PutRehearsalNote(alice, bandID, songID, 0, app.RehearsalNoteMeta{RasterHash: "h", ConcertID: "c"}, testPNG(t, 1), false); err != nil {
				t.Fatal(err)
			}
			views, err := svc.ListRehearsalNotes(alice, bandID, songID)
			if err != nil {
				t.Fatal(err)
			}
			if len(views) != 1 {
				t.Fatalf("want 1 note, got %d", len(views))
			}
			got := views[0].PageChanged
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("pageChanged = %v, want unknown", *got)
			case tc.want != nil && got == nil:
				t.Fatalf("pageChanged = unknown, want %v", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("pageChanged = %v, want %v", *got, *tc.want)
			}
		})
	}
}

// TestPageChangedWithoutConcertIsUnknown: a note that never recorded which bake it came from
// cannot be compared to anything. Not asking the lookup at all is the point — asking with an
// empty concert id would match whatever a sloppy lookup returns first.
func TestPageChangedWithoutConcertIsUnknown(t *testing.T) {
	svc, _, alice, bandID, songID := noteFixture(t, memrepo.New())
	stub := &stubBake{hash: "h", pageExists: true, isBaked: true}
	svc.WithBakeLookup(stub)
	if _, err := svc.PutRehearsalNote(alice, bandID, songID, 0, app.RehearsalNoteMeta{}, testPNG(t, 1), false); err != nil {
		t.Fatal(err)
	}
	views, err := svc.ListRehearsalNotes(alice, bandID, songID)
	if err != nil {
		t.Fatal(err)
	}
	if views[0].PageChanged != nil {
		t.Fatalf("pageChanged = %v for a note with no concert id, want unknown", *views[0].PageChanged)
	}
	if stub.calls != 0 {
		t.Fatalf("the lookup was called %d times for a note with nothing to look up", stub.calls)
	}
}

// TestRehearsalNoteConflictIsNotDestructive: a refused overwrite must not have already written
// the new bytes into the blob store on its way to the 409.
func TestRehearsalNoteConflictIsNotDestructive(t *testing.T) {
	svc, blobs, alice, bandID, songID := noteFixture(t, memrepo.New())
	meta := app.RehearsalNoteMeta{RasterHash: "h", ConcertID: "c"}
	if _, err := svc.PutRehearsalNote(alice, bandID, songID, 0, meta, testPNG(t, 1), false); err != nil {
		t.Fatal(err)
	}
	rejected := testPNG(t, 2)
	_, err := svc.PutRehearsalNote(alice, bandID, songID, 0, meta, rejected, false)
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("second send err = %v, want ErrConflict", err)
	}
	if _, err := blobs.Get(blob.HashOf(rejected)); err == nil {
		t.Fatal("the refused note's bytes were stored anyway — a 409 that still writes is not a refusal")
	}
}

func boolPtr(b bool) *bool { return &b }

// TestRehearsalNoteIsNotExported: a note is personal, per-device and not band data. It must not
// travel in a .tband — handing a band export to someone else must not hand them what a member
// scribbled on their own tablet. Checked by walking the archive's DECOMPRESSED entries, not by
// scanning the zip bytes (deflate makes that search unable to find anything, so it would pass
// no matter what the export contained) and not against a field list (the failure mode is a new
// serializer quietly picking the record up).
func TestRehearsalNoteIsNotExported(t *testing.T) {
	svc, _, alice, bandID, songID := noteFixture(t, memrepo.New())
	// A song whose title cannot occur by accident: the positive control below needs a string
	// that is in the export only because band content is in the export.
	control := "Zephyr Control Piece"
	if _, err := svc.CreateSong(alice, bandID, control, ""); err != nil {
		t.Fatal(err)
	}
	marker := testPNG(t, 42)
	if _, err := svc.PutRehearsalNote(alice, bandID, songID, 0, app.RehearsalNoteMeta{RasterHash: "h", ConcertID: "c"}, marker, false); err != nil {
		t.Fatal(err)
	}

	eng := engine.New(memstore.New().(store.HistoryAware))
	zipped, _, err := svc.ExportBand(alice, eng, bandID)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		t.Fatal(err)
	}
	foundControl := false
	for _, f := range zr.File {
		if name := strings.ToLower(f.Name); strings.Contains(name, "rehearsal") || strings.Contains(name, "note") {
			t.Errorf(".tband carries an entry named %q", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, marker) {
			t.Errorf(".tband entry %q carries the note's pixels", f.Name)
		}
		if bytes.Contains(bytes.ToLower(body), []byte("rehearsalnote")) {
			t.Errorf(".tband entry %q mentions a rehearsal note", f.Name)
		}
		if bytes.Contains(body, []byte(control)) {
			foundControl = true
		}
	}
	// POSITIVE CONTROL. An absence proves nothing until the same walk is shown to find what IS
	// there — otherwise "no note found" only means the walk found nothing at all, which is
	// exactly what an earlier version of this test was doing.
	if !foundControl {
		t.Fatalf("the walk never saw %q, a song that IS exported — it cannot testify that the note is absent", control)
	}
}
