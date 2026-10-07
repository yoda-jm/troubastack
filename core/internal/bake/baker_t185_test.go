package bake

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// multiPageRaster renders N pages for a file, N=2 when the content says PAGES=2, else 1. Each (file, page)
// gets a distinct raster (and so a distinct hash), so the pool arithmetic and the hash match are both
// observable. This is what lets T185's fixture be a 2-page file beside a 1-page file.
type multiPageRaster struct{}

func (multiPageRaster) Rasterize(_ context.Context, pdf []byte) ([][]byte, error) {
	h := sha256.Sum256(pdf)
	n := 1
	if bytes.Contains(pdf, []byte("PAGES=2")) {
		n = 2
	}
	out := make([][]byte, n)
	for i := 0; i < n; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.SetRGBA(0, 0, color.RGBA{R: h[0], G: h[1], B: byte(i), A: 255}) // vary by page so each is unique
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		out[i] = buf.Bytes()
	}
	return out, nil
}

func (e t137Env) bakeMultiPage(t *testing.T) (ConcertBundle, *Baker) {
	t.Helper()
	b := &Baker{svc: e.svc, eng: e.eng, raster: multiPageRaster{}, overlays: fakeOverlays{png: []byte("ov")}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
	cb, _, err := b.Bake(context.Background(), e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	return cb, b
}

// T185 ⟨D1⟩/⟨D2⟩ acceptance §6: VLL's case. A 2-page file and a 1-page file, a selection ordering them
// [2-page, 1-page]; a note on sequence position 2 (his "page 3") carries the 1-page file's hash, and must
// resolve to that file, page 0 — matched on the hash, not the index. The fixture is discriminating: the
// sequence index (2) is NOT the filePage (0), so a resolver that merely echoed the index would fail.
func TestBakeT185_ResolvesNoteToFileAndPage(t *testing.T) {
	e := newT137Env(t)
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, two.ID, one.ID) // Marie's sequence: two.p0, two.p1, one.p0

	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]
	if len(song.Pages) != 3 {
		t.Fatalf("pool = %d pages, want 3 (two has 2, one has 1)", len(song.Pages))
	}
	seq := seqFor(song.MemberPages, e.marie.ID)
	if len(seq) != 3 {
		t.Fatalf("Marie's sequence = %v, want 3 entries", seq)
	}

	// The sidecar exists on disk (⟨D1⟩: persisted server-side, beside bundle.json).
	side := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10), "page-sources.json")
	raw, err := os.ReadFile(side)
	if err != nil {
		t.Fatalf("source sidecar not written: %v", err)
	}
	var sm SourceMap
	if err := json.Unmarshal(raw, &sm); err != nil {
		t.Fatalf("sidecar parse: %v", err)
	}
	if got := len(sm.Songs[song.SongID]); got != 3 {
		t.Fatalf("sidecar has %d source entries for the song, want 3", got)
	}

	// ⟨D1⟩ server-side ONLY: the sidecar is in the published rev dir (above) but must NOT be in the .tstage
	// the tablet downloads.
	tstage := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10)+".tstage")
	zr, zerr := zip.OpenReader(tstage)
	if zerr != nil {
		t.Fatalf("open tstage: %v", zerr)
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.Name == "page-sources.json" {
			t.Error("page-sources.json leaked into the .tstage the tablet downloads (⟨D1⟩: server-side only)")
		}
	}

	// The note sits at sequence position 2 — his "page 3". That pool page's hash is what a note carries.
	const seqPos = 2
	poolIdx := seq[seqPos]
	hash := song.Pages[poolIdx].RasterHash

	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, hash)
	if !ok {
		t.Fatalf("resolve failed; want the 1-page file")
	}
	if fileID != one.ID {
		t.Errorf("resolved fileID = %q, want the 1-page file %q (not the 2-page file %q)", fileID, one.ID, two.ID)
	}
	if filePage != 0 {
		t.Errorf("resolved filePage = %d, want 0", filePage)
	}
	// Discriminating: the sequence index is NOT the file page.
	if filePage == seqPos {
		t.Fatalf("fixture is not discriminating: filePage equals the sequence index %d", seqPos)
	}
}

// ctlRaster wraps a rasterizer to COUNT calls and to inject failures — to prove the derive is scoped to one
// song (⟨1⟩) and never caches a failure (⟨2⟩).
type ctlRaster struct {
	inner      Rasterizer
	calls      int
	failFirstN int // fail the first N Rasterize calls, then behave normally
}

func (c *ctlRaster) Rasterize(ctx context.Context, pdf []byte) ([][]byte, error) {
	c.calls++
	if c.failFirstN > 0 {
		c.failFirstN--
		return nil, errors.New("injected raster failure")
	}
	return c.inner.Rasterize(ctx, pdf)
}

// ⟨1⟩ Resolving one song's note derives ONLY that song's files, not the whole concert.
func TestBakeT185_DeriveScopedToSong(t *testing.T) {
	e := newT137Env(t)
	// song 1 (the env's song): two files.
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, two.ID, one.ID)
	// song 2: another song on the same setlist, with its own file.
	song2, err := e.svc.CreateSong(e.admin, e.bandID, "Song2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UploadSongFile(e.admin, e.bandID, song2.ID, "s2.pdf", "application/pdf", []byte("%PDF-1.4 PAGES=2 S2")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddSetlistItem(e.admin, e.bandID, e.sl, song2.ID); err != nil {
		t.Fatal(err)
	}

	cb, b := e.bakeMultiPage(t)
	var song1 BakedSong
	for _, s := range cb.Songs {
		if s.SongID == e.songID {
			song1 = s
		}
	}
	hash := song1.Pages[seqFor(song1.MemberPages, e.marie.ID)[2]].RasterHash // song1's 1-page file

	// Simulate a pre-T185 rev, and swap in the counting raster for the derive.
	side := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10), sourceMapFile)
	if err := os.Remove(side); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}
	cr := &ctlRaster{inner: multiPageRaster{}}
	b.raster = cr

	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, e.songID, hash)
	if !ok || fileID != one.ID || filePage != 0 {
		t.Fatalf("resolve = (%q, %d, %v), want (%q, 0, true)", fileID, filePage, ok, one.ID)
	}
	// Only song1's TWO files were rendered — not song2's. A whole-concert derive would have made 3 calls.
	if cr.calls != 2 {
		t.Errorf("derive made %d Rasterize calls, want 2 (song1's files only, not song2's)", cr.calls)
	}
	// The sidecar holds song1 but NOT song2 (song2 derives only when its own note asks).
	m, _ := b.readSourceMap(cb.ConcertID, cb.ConcertRev)
	if _, present := m.Songs[e.songID]; !present {
		t.Error("song1 should be cached after its note resolved")
	}
	if _, present := m.Songs[song2.ID]; present {
		t.Error("song2 must NOT be derived or cached by resolving song1's note")
	}
}

// ⟨2⟩ A transient render failure is NOT cached as an absence: the next call re-derives and resolves.
func TestBakeT185_FailureNotCached(t *testing.T) {
	e := newT137Env(t)
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, two.ID, one.ID)
	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]
	hash := song.Pages[seqFor(song.MemberPages, e.marie.ID)[2]].RasterHash
	side := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10), sourceMapFile)
	if err := os.Remove(side); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}

	// First attempt: the first file's render fails → unresolved, and NOTHING cached.
	cr := &ctlRaster{inner: multiPageRaster{}, failFirstN: 1}
	b.raster = cr
	if _, _, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, hash); ok {
		t.Fatal("a render failure must not resolve")
	}
	if _, err := os.Stat(side); err == nil {
		t.Error("a failed derive must not write a sidecar (an error is not an absence)")
	}

	// Second attempt (raster now healthy): it re-derives and resolves.
	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, hash)
	if !ok || fileID != one.ID || filePage != 0 {
		t.Fatalf("retry resolve = (%q, %d, %v), want (%q, 0, true)", fileID, filePage, ok, one.ID)
	}
}

// A rev baked BEFORE T185 has no sidecar; resolving a note on it DERIVES one by re-rendering the song's
// files, matches by hash, and caches it (Fable's ruling, option 2). This is VLL's rev-12 case, in miniature.
func TestBakeT185_BackfillOldRevByRerender(t *testing.T) {
	e := newT137Env(t)
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, two.ID, one.ID)
	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]
	seq := seqFor(song.MemberPages, e.marie.ID)
	hash := song.Pages[seq[2]].RasterHash // the 1-page file's page

	// Simulate a pre-T185 bake: remove the sidecar this bake wrote.
	side := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10), sourceMapFile)
	if err := os.Remove(side); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}

	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, hash)
	if !ok || fileID != one.ID || filePage != 0 {
		t.Fatalf("backfill resolve = (%q, %d, %v), want (%q, 0, true)", fileID, filePage, ok, one.ID)
	}
	// The derived map was cached back into the rev's sidecar (compute once).
	if _, err := os.Stat(side); err != nil {
		t.Errorf("derive did not cache the sidecar: %v", err)
	}
}

// Option 3 (generalised): a note naming a rev whose bundle is gone still resolves if ANOTHER rev's sidecar
// has that hash for the song — the same hash is the same image.
func TestBakeT185_CrossRevFallback(t *testing.T) {
	e := newT137Env(t)
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, two.ID, one.ID)
	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]
	hash := song.Pages[seqFor(song.MemberPages, e.marie.ID)[2]].RasterHash

	// A note claims a rev that was never baked here (no bundle to derive from), but the hash lives in the
	// baked rev's sidecar.
	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev+5, song.SongID, hash)
	if !ok || fileID != one.ID || filePage != 0 {
		t.Fatalf("cross-rev resolve = (%q, %d, %v), want (%q, 0, true)", fileID, filePage, ok, one.ID)
	}
}

// A REVERSED selection reaches the same page through a different index (acceptance §6, second bullet).
func TestBakeT185_ReversedSelectionSamePage(t *testing.T) {
	e := newT137Env(t)
	two := e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.leo, one.ID, two.ID) // Leo's sequence: one.p0, two.p0, two.p1

	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]
	seq := seqFor(song.MemberPages, e.leo.ID)
	// Position 0 is the 1-page file for Leo (index 0, not 2 as for Marie).
	hash := song.Pages[seq[0]].RasterHash
	fileID, filePage, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, hash)
	if !ok || fileID != one.ID || filePage != 0 {
		t.Fatalf("reversed: resolve = (%q, %d, %v), want (%q, 0, true)", fileID, filePage, ok, one.ID)
	}
}

// Unresolvable: a hash not in the song, and an unknown rev, both yield (false) — never a guess (§6 third bullet).
func TestBakeT185_Unresolvable(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "two.pdf", []byte("%PDF-1.4 PAGES=2 TWO"))
	one := e.upload(t, "one.pdf", []byte("%PDF-1.4 ONE"))
	e.selects(t, e.marie, one.ID) // make it a 2-file pool so the song bakes with sources
	cb, b := e.bakeMultiPage(t)
	song := cb.Songs[0]

	if _, _, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, "deadbeef-not-a-real-hash"); ok {
		t.Error("a hash not in the song must not resolve")
	}
	// A hash that exists in NO rev of the concert stays unresolved even across the option-3 fallback. (A
	// REAL hash on an unknown rev *does* resolve via option 3 — the same image in another rev — which is
	// covered by TestBakeT185_CrossRevFallback.)
	if _, _, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev+999, song.SongID, "nowhere-hash"); ok {
		t.Error("a hash that exists nowhere must not resolve")
	}
	if _, _, ok := b.ResolvePageSource(cb.ConcertID, cb.ConcertRev, song.SongID, ""); ok {
		t.Error("an empty hash must not resolve")
	}
}
