package bake

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// filePDFFixture is the seed band (one song "Song" with score.pdf and a personal layer L1 owned by u holding
// one rect) behind a Baker whose fakes paint a WHITE raster and a solid RED overlay per layer.
func filePDFFixture(t *testing.T, pages int) (*Baker, *app.Service, app.User, string, string, string) {
	t.Helper()
	svc, eng, u, bandID, _ := seed(t)
	white := solidPNG(t, 40, 56, color.White)
	red := solidPNG(t, 40, 56, color.RGBA{R: 0xe1, G: 0x1d, B: 0x48, A: 0xff})
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: pages, png: white},
		overlays: fakeOverlays{png: red},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	songs, err := svc.Songs(u, bandID)
	if err != nil || len(songs) != 1 {
		t.Fatalf("songs: %v (len %d)", err, len(songs))
	}
	files, err := svc.SongFiles(u, bandID, songs[0].ID)
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v (len %d)", err, len(files))
	}
	return b, svc, u, bandID, songs[0].ID, files[0].ID
}

// firstPageImage pulls the first embedded JPEG out of a PDF and decodes it — the composed page as printed.
func firstPageImage(t *testing.T, pdf []byte) image.Image {
	t.Helper()
	start := bytes.Index(pdf, []byte{0xFF, 0xD8, 0xFF})
	if start < 0 {
		t.Fatal("no embedded JPEG in the PDF")
	}
	end := bytes.Index(pdf[start:], []byte{0xFF, 0xD9})
	if end < 0 {
		t.Fatal("embedded JPEG has no EOI marker")
	}
	img, err := jpeg.Decode(bytes.NewReader(pdf[start : start+end+2]))
	if err != nil {
		t.Fatalf("decode embedded JPEG: %v", err)
	}
	return img
}

func isRed(img image.Image) bool {
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	return r>>8 > 0x80 && g>>8 < 0x60 && bl>>8 < 0x80
}

func isWhite(img image.Image) bool {
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	return r>>8 > 0xf0 && g>>8 > 0xf0 && bl>>8 > 0xf0
}

func TestFilePDF_PageCountAndNoConcert(t *testing.T) {
	b, _, u, bandID, songID, fileID := filePDFFixture(t, 3)
	pdf, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, func(string) bool { return true })
	if err != nil {
		t.Fatalf("FilePDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatalf("output is not a complete PDF")
	}
	if got := len(pageLeafRE.FindAll(pdf, -1)); got != 3 {
		t.Fatalf("PDF leaf page count = %d, want 3 (== raster count)", got)
	}
	// No concert: nothing under bakesDir (no rev dir, no bundle, no .tstage), no rev claimed.
	entries, err := os.ReadDir(b.bakesDir)
	if err != nil {
		t.Fatalf("read bakesDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("FilePDF wrote %d entries under bakesDir; it must write nothing there", len(entries))
	}
	if concerts := b.ListConcerts(); len(concerts) != 0 {
		t.Fatalf("FilePDF must not publish a concert, got %d", len(concerts))
	}
}

// TestFilePDF_DeterministicSinglePage: pinned dates make a one-page print byte-identical across runs.
// (Only one page: fpdf v0.9.0 emits equal-width images in map order — putimages sorts by width alone — so a
// multi-page print whose pages DIFFER can vary in object order. That is shared with ConcertPDF, unchanged.)
func TestFilePDF_DeterministicSinglePage(t *testing.T) {
	b, _, u, bandID, songID, fileID := filePDFFixture(t, 1)
	all := func(string) bool { return true }
	pdf, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, all)
	if err != nil {
		t.Fatalf("FilePDF: %v", err)
	}
	again, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, all)
	if err != nil {
		t.Fatalf("FilePDF re-run: %v", err)
	}
	if !bytes.Equal(pdf, again) {
		t.Fatalf("FilePDF is not deterministic (%d vs %d bytes)", len(pdf), len(again))
	}
}

// TestFilePDF_VisibleFilter is the discriminating compositing guard: the one layer (L1) paints solid red over a
// white raster. Visible → the printed page is RED; filtered out → it stays WHITE. Both read the JPEG actually
// embedded in the PDF, so the check covers the whole path (filter → compose → encode → PDF).
func TestFilePDF_VisibleFilter(t *testing.T) {
	b, _, u, bandID, songID, fileID := filePDFFixture(t, 1)

	on, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, func(id string) bool { return id == "L1" })
	if err != nil {
		t.Fatalf("FilePDF (L1 on): %v", err)
	}
	if img := firstPageImage(t, on); !isRed(img) {
		t.Fatalf("L1 visible: the printed page should show the red overlay")
	}

	off, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, func(id string) bool { return id != "L1" })
	if err != nil {
		t.Fatalf("FilePDF (L1 off): %v", err)
	}
	if img := firstPageImage(t, off); !isWhite(img) {
		t.Fatalf("L1 filtered out: the printed page must stay white (the visible filter was ignored)")
	}
}

func TestFilePDF_UnknownFileAndOtherSongsFile(t *testing.T) {
	b, svc, u, bandID, songID, _ := filePDFFixture(t, 1)
	all := func(string) bool { return true }

	if _, err := b.FilePDF(context.Background(), bandID, songID, "no-such-file", u, all); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown file: err = %v, want ErrNotFound", err)
	}

	other, err := svc.CreateSong(u, bandID, "Other", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	of, err := svc.UploadSongFile(u, bandID, other.ID, "other.pdf", "application/pdf", []byte("%PDF-1.4 other"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	// The other song's file exists and is readable by u — but not as a file of THIS song.
	if _, err := b.FilePDF(context.Background(), bandID, songID, of.ID, u, all); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("another song's file: err = %v, want ErrNotFound", err)
	}
	// Positive control: the same file under its own song prints.
	if _, err := b.FilePDF(context.Background(), bandID, other.ID, of.ID, u, all); err != nil {
		t.Fatalf("own song's file should print: %v", err)
	}
}

func TestFilePDF_NonMemberRejected(t *testing.T) {
	b, svc, _, bandID, songID, fileID := filePDFFixture(t, 1)
	stranger, err := svc.Register("stranger", "Stranger", "password123", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_, err = b.FilePDF(context.Background(), bandID, songID, fileID, stranger, func(string) bool { return true })
	if !errors.Is(err, app.ErrForbidden) && !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("non-member: err = %v, want ErrForbidden/ErrNotFound", err)
	}
}

func TestFilePDF_ZeroPagesIsValidOnePagePDF(t *testing.T) {
	b, _, u, bandID, songID, fileID := filePDFFixture(t, 0)
	pdf, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, func(string) bool { return true })
	if err != nil {
		t.Fatalf("FilePDF: %v", err)
	}
	if got := len(pageLeafRE.FindAll(pdf, -1)); got != 1 {
		t.Fatalf("zero-page file: PDF leaf page count = %d, want 1", got)
	}
}

// TestFileLayerVisible pins the single-file print's layer choice: mandatory is always painted; an explicit
// request is exactly the requested ids (unknown ignored, empty = none); the default is a fresh viewer's view.
func TestFileLayerVisible(t *testing.T) {
	const me, them = "me", "them"
	layers := []domain.Layer{
		{ID: "M", OwnerID: domain.SharedOwner, Mandatory: true},
		{ID: "S", OwnerID: domain.SharedOwner},
		{ID: "R", OwnerID: domain.SharedOwner, RoleTag: "drums"},
		{ID: "P", OwnerID: me, Zone: domain.ZonePersonal},
		{ID: "Q", OwnerID: them, Zone: domain.ZonePersonal},
		{ID: "MQ", OwnerID: them, Mandatory: true},
	}
	cases := []struct {
		name      string
		requested []string
		explicit  bool
		want      map[string]bool
	}{
		{"default view", nil, false, map[string]bool{"M": true, "S": true, "R": false, "P": true, "Q": false, "MQ": true}},
		{"explicit subset", []string{"S", "Q", "nope"}, true, map[string]bool{"M": true, "S": true, "R": false, "P": false, "Q": true, "MQ": true}},
		{"explicit empty", nil, true, map[string]bool{"M": true, "S": false, "R": false, "P": false, "Q": false, "MQ": true}},
		{"explicit role layer", []string{"R"}, true, map[string]bool{"M": true, "S": false, "R": true, "P": false, "Q": false, "MQ": true}},
	}
	for _, c := range cases {
		vis := fileLayerVisible(layers, me, c.requested, c.explicit)
		for id, want := range c.want {
			if got := vis(id); got != want {
				t.Errorf("%s: layer %s visible = %v, want %v", c.name, id, got, want)
			}
		}
		if vis("nope") {
			t.Errorf("%s: an id that is not a layer of the song must never be visible", c.name)
		}
	}
}

// TestComposeLayers_ZOrderStable: overlays paint ascending by order, ties in the caller's order — the shared
// flattening both prints use.
func TestComposeLayers_ZOrderStable(t *testing.T) {
	solid := func(c color.Color) func() (image.Image, error) {
		return func() (image.Image, error) { return image.NewUniform(c), nil }
	}
	raster := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 0xff, A: 0xff}
	blue := color.RGBA{B: 0xff, A: 0xff}
	green := color.RGBA{G: 0xff, A: 0xff}
	got, err := composeLayers(raster, []composeLayer{
		{order: 2, load: solid(blue)},
		{order: 1, load: solid(red)},
		{order: 2, load: solid(green)}, // ties with blue; listed after it → paints after it → on top
	})
	if err != nil {
		t.Fatalf("composeLayers: %v", err)
	}
	if r, g, b, _ := got.At(1, 1).RGBA(); r != 0 || g>>8 != 0xff || b != 0 {
		t.Fatalf("top pixel = (%d,%d,%d), want green (highest order, last among ties)", r>>8, g>>8, b>>8)
	}
}
