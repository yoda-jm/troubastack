// Command mkbundle is a DEV-ONLY generator of baked concert bundles that conform to the container
// spec (docs/design/08-bundle-container.md): a directory of bundle.json (proto3 canonical JSON of
// ConcertBundle) + blobs/ page rasters and transparent overlays, optionally zipped as .tstage.
//
// Why it exists: the real producer is the server-side bake (I8/I11), which is still a stub, and the
// presenter track (A04) must not wait for it. This tool emits real, decodable input plus "torture"
// variants that exercise the presenter's never-crash contract (I12). Regenerate the committed
// fixtures with `make fixtures` (see app/shared/src/commonTest/resources/fixtures/README.md).
//
// RENDERER BOUNDARY (I8): the one stroke renderer is web/ink; Go must NEVER render user strokes.
// The overlays here are SYNTHETIC test patterns (rectangles/lines) — deliberately not real
// annotations. Do not grow this into a bake: real flattening shells out to web/bake (see
// core/internal/bake). This tool is fixtures only.
//
// Output is DETERMINISTIC: same flags ⇒ byte-identical bytes (PNG carries no timestamp, JSON field
// order is fixed, bakedAt comes from -seed, zip entry times are pinned), so committed fixtures stay
// stable in review diffs.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"troubastack/core/internal/bake"
)

func main() {
	out := flag.String("out", "", "output directory for the valid demo bundle")
	torture := flag.String("torture", "", "if set, also write the four torture variants under this dir")
	songs := flag.Int("songs", 2, "number of songs in the demo bundle")
	pages := flag.Int("pages", 3, "pages per song")
	seed := flag.Int64("seed", 1700000000, "epoch seconds used for bakedAt (determinism)")
	doZip := flag.Bool("zip", false, "also emit <out>.tstage (zip with bundle.json at the root)")
	flag.Parse()

	if *out == "" && *torture == "" {
		log.Fatal("mkbundle: pass -out <dir> and/or -torture <dir>")
	}
	if err := run(*out, *torture, *songs, *pages, *seed, *doZip); err != nil {
		log.Fatalf("mkbundle: %v", err)
	}
}

func run(out, torture string, songs, pages int, seed int64, doZip bool) error {
	if out != "" {
		bundle := buildBundle(songs, pages, seed)
		if err := writeBundle(out, bundle); err != nil {
			return err
		}
		if doZip {
			if err := bake.WriteTstage(out+".tstage", out, time.Unix(seed, 0)); err != nil {
				return err
			}
		}
	}
	if torture != "" {
		if err := writeTortureVariants(torture, seed); err != nil {
			return err
		}
	}
	return nil
}

// --- ConcertBundle model: the ONE Go mirror lives in internal/bake (T18) --------------------------
// This dev tool reuses bake's ConcertBundle/BakedSong/PageImages/LayerImage + MarshalCanonical +
// WriteTstage (the real producer's mirror + writers). Only mkbundle's SYNTHETIC content and its
// fixture short-hash stay local. NOTE: mkbundle keeps its own `shortHash` (sha256[:8]) deliberately
// — the committed fixtures use short hashes; bake.Sha256Hex (full) is production's, and adopting it
// would rewrite every fixture hash. Hashing is fixture-specific here, not part of the shared mirror.

// A blob is a ref path plus the PNG bytes to write for it.
type blob struct {
	ref  string
	data []byte
}

const (
	rasterW = 800
	rasterH = 1130
)

// buildBundle assembles the manifest plus the PNG blobs for it, deterministically.
func buildBundle(songs, pages int, seed int64) *builtBundle {
	b := &builtBundle{
		manifest: bake.ConcertBundle{
			ConcertID:  "demo-concert",
			Name:       "Demo Concert",
			ConcertRev: 1,
			BakedAt:    seed,
			BakedBy:    "mkbundle",
		},
	}
	for si := 0; si < songs; si++ {
		song := bake.BakedSong{
			SongID:         fmt.Sprintf("song-%d", si+1),
			SourceRevision: uint64(si + 1),
			SongRev:        1,
		}
		for pi := 0; pi < pages; pi++ {
			rasterRef := fmt.Sprintf("blobs/s%d-p%d-raster.png", si+1, pi+1)
			raster := encodePNG(pageRaster(si, pi))
			b.blobs = append(b.blobs, blob{rasterRef, raster})

			// Two overlays: one mandatory, one targeted by roleTag (per A03).
			mand := encodePNG(overlay(si, pi, 0))
			cond := encodePNG(overlay(si, pi, 1))
			mandRef := fmt.Sprintf("blobs/s%d-p%d-L0.png", si+1, pi+1)
			condRef := fmt.Sprintf("blobs/s%d-p%d-L1.png", si+1, pi+1)
			b.blobs = append(b.blobs, blob{mandRef, mand}, blob{condRef, cond})

			song.Pages = append(song.Pages, bake.PageImages{
				PageRasterRef: rasterRef,
				RasterHash:    shortHash(raster),
				Overlays: []bake.LayerImage{
					{LayerID: "marks", ImageRef: mandRef, ContentHash: shortHash(mand), Order: 0, Mandatory: true},
					{LayerID: "conductor", ImageRef: condRef, ContentHash: shortHash(cond), Order: 1, RoleTag: "conductor"},
				},
			})
		}
		b.manifest.Songs = append(b.manifest.Songs, song)
	}
	return b
}

type builtBundle struct {
	manifest bake.ConcertBundle
	blobs    []blob
}

// writeBundle writes bundle.json + blobs/ into dir (created fresh).
func writeBundle(dir string, b *builtBundle) error {
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o755); err != nil {
		return err
	}
	data, err := b.manifest.MarshalCanonical()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), data, 0o644); err != nil {
		return err
	}
	for _, bl := range b.blobs {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(bl.ref)), bl.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeTortureVariants emits the four never-crash fixtures under parent (docs/design/08 resilience).
func writeTortureVariants(parent string, seed int64) error {
	// (a) missing-blob: a valid bundle with one referenced raster file deleted.
	mb := buildBundle(1, 1, seed)
	mbDir := filepath.Join(parent, "missing-blob")
	if err := writeBundle(mbDir, mb); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(mbDir, filepath.FromSlash(mb.blobs[0].ref))); err != nil {
		return err
	}

	// (b) bad-json: a truncated bundle.json (blobs present, manifest unparseable).
	bj := buildBundle(1, 1, seed)
	bjDir := filepath.Join(parent, "bad-json")
	if err := writeBundle(bjDir, bj); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bjDir, "bundle.json"), []byte(`{"concertId":"demo-concert","songs":[`), 0o644); err != nil {
		return err
	}

	// (c) empty: a valid manifest with zero songs, no blobs.
	empty := &builtBundle{manifest: bake.ConcertBundle{ConcertID: "demo-concert", Name: "Demo Concert", ConcertRev: 1, BakedAt: seed, BakedBy: "mkbundle"}}
	if err := writeBundle(filepath.Join(parent, "empty"), empty); err != nil {
		return err
	}

	// (d) no-manifest: blobs only, no bundle.json.
	nm := buildBundle(1, 1, seed)
	nmDir := filepath.Join(parent, "no-manifest")
	if err := writeBundle(nmDir, nm); err != nil {
		return err
	}
	return os.Remove(filepath.Join(nmDir, "bundle.json"))
}

// (writeTstage removed in T18 — mkbundle now uses bake.WriteTstage, the shared writer.)

// --- synthetic image drawing (deterministic, flat colors so PNGs stay tiny) --------------------

// NRGBA (non-premultiplied) is used throughout: Go's color.RGBA is ALPHA-PREMULTIPLIED, so passing
// straight color components at partial alpha (e.g. {200,40,40,90}) encodes a hue-shifted PNG. NRGBA
// stores the components as-is, so the translucent overlays render in their true colors.

// pageRaster draws a visually distinct A4-ish page: white bg, a per-song colored header band, a
// border, and the page number rendered as a row of blocks. Not typography — just distinct pages.
func pageRaster(songIdx, pageIdx int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, rasterW, rasterH))
	fill(img, img.Bounds(), color.NRGBA{255, 255, 255, 255})
	// Border.
	border(img, 6, color.NRGBA{40, 40, 40, 255})
	// Song header band (distinct hue per song).
	fill(img, image.Rect(20, 20, rasterW-20, 120), songColor(songIdx))
	// Page number as (pageIdx+1) black blocks below the band.
	for i := 0; i <= pageIdx; i++ {
		x := 40 + i*70
		fill(img, image.Rect(x, 160, x+50, 260), color.NRGBA{30, 30, 30, 255})
	}
	return img
}

// overlay draws a mostly-transparent test pattern for one layer (translucent shape).
func overlay(songIdx, pageIdx, layer int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, rasterW, rasterH))
	if layer == 0 {
		// "marks" — translucent red rectangle mid-page.
		fill(img, image.Rect(120, 400, rasterW-120, 520), color.NRGBA{200, 40, 40, 90})
	} else {
		// "conductor" — translucent blue band lower-page.
		fill(img, image.Rect(80, 700, rasterW-80, 760), color.NRGBA{40, 60, 200, 90})
	}
	return img
}

func songColor(i int) color.NRGBA {
	palette := []color.NRGBA{
		{70, 130, 180, 255}, {180, 120, 60, 255}, {90, 160, 90, 255}, {150, 90, 160, 255},
	}
	return palette[i%len(palette)]
}

func fill(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func border(img *image.NRGBA, w int, c color.NRGBA) {
	b := img.Bounds()
	fill(img, image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+w), c)
	fill(img, image.Rect(b.Min.X, b.Max.Y-w, b.Max.X, b.Max.Y), c)
	fill(img, image.Rect(b.Min.X, b.Min.Y, b.Min.X+w, b.Max.Y), c)
	fill(img, image.Rect(b.Max.X-w, b.Min.Y, b.Max.X, b.Max.Y), c)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		panic(err) // encoding a valid in-memory image cannot fail; programmer error if it does
	}
	return buf.Bytes()
}

func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}
