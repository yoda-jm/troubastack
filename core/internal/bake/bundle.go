package bake

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Go mirror of proto ConcertBundle + friends for writing bundle.json.
//
// AUTHORITY: proto/troubastack/v1/bundle.proto (I1). The JSON encoding is the
// proto3 canonical JSON pinned by docs/design/08-bundle-container.md — field
// names are lowerCamelCase and 64-bit ints (uint64/int64) are JSON STRINGS. The
// Kotlin loader (app/.../bundle/BundleModel.kt) round-trips exactly this shape.
//
// NOTE: core/cmd/mkbundle (the dev-only fixture generator, A03) carries a parallel
// copy of these structs for its synthetic fixtures. This is the REAL producer;
// de-duplicating the two mirrors is a safe follow-up (kept separate here to avoid
// perturbing the committed fixtures mkbundle regenerates).

// LayerImage is one layer's transparent overlay on one page (proto LayerImage).
type LayerImage struct {
	LayerID     string `json:"layerId,omitempty"`
	ImageRef    string `json:"imageRef,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	Order       int32  `json:"order,omitempty"` // int32 → JSON number
	Mandatory   bool   `json:"mandatory,omitempty"`
	RoleTag     string `json:"roleTag,omitempty"`
	Name        string `json:"name,omitempty"` // human name (mirrors Layer.Name), for viewer labels (T53)
}

// PageImages is one page's raster + z-ordered overlays (proto PageImages).
type PageImages struct {
	PageRasterRef string       `json:"pageRasterRef,omitempty"`
	RasterHash    string       `json:"rasterHash,omitempty"`
	Overlays      []LayerImage `json:"overlays,omitempty"`
}

// BakedSong is one song's baked pages + setlist-override metadata (proto BakedSong).
type BakedSong struct {
	SongID         string       `json:"songId,omitempty"`
	SourceRevision uint64       `json:"sourceRevision,string,omitempty"` // uint64 → JSON string
	SongRev        uint64       `json:"songRev,string,omitempty"`
	Pages          []PageImages `json:"pages,omitempty"`
	DisplayNotes   string       `json:"displayNotes,omitempty"` // setlist item Notes (B02)
	Key            string       `json:"key,omitempty"`          // setlist KeyOverride
	Tempo          int32        `json:"tempo,omitempty"`        // setlist TempoOverride
	OnCall         bool         `json:"onCall,omitempty"`       // bench/encore item — jumpable, outside the running order (T23)
	Title          string       `json:"title,omitempty"`        // song Title at bake time (T26); empty → client "Song N" fallback
	Cues           []SongCue    `json:"cues,omitempty"`         // the baked-for member's personal cues (T50); per-member bake only, shared bake has none
}

// SongCue is one personal cue on a song: a stable icon id + an optional tint
// (proto SongCue, T50). Unknown icon ids render as the `note` fallback client-side.
type SongCue struct {
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"` // "#rrggbb" or "" (neutral)
}

// ConcertBundle is the manifest of a baked concert (proto ConcertBundle, I11/I12).
type ConcertBundle struct {
	ConcertID   string      `json:"concertId,omitempty"`
	Name        string      `json:"name,omitempty"`
	ConcertRev  uint64      `json:"concertRev,string,omitempty"` // uint64 → JSON string
	BakedAt     int64       `json:"bakedAt,string,omitempty"`    // int64 → JSON string
	BakedBy     string      `json:"bakedBy,omitempty"`
	FinalLocked bool        `json:"finalLocked,omitempty"`
	Songs       []BakedSong `json:"songs,omitempty"`
}

// MarshalCanonical renders a ConcertBundle as the container spec's canonical JSON
// (indented, trailing newline) — the exact bytes bundle.json holds.
func (b *ConcertBundle) MarshalCanonical() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Sha256Hex is the content hash used for raster/overlay change detection (R10).
func Sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteTstage zips a bundle directory into dst (.tstage): bundle.json at the zip
// ROOT, blobs/ alongside (docs/design/08). Entries are written in a fixed
// (sorted) order with a pinned mod time so a re-bake of identical content yields
// an identical archive.
func WriteTstage(dst, srcDir string, modTime time.Time) (err error) {
	var names []string
	if werr := filepath.WalkDir(srcDir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			rel, rerr := filepath.Rel(srcDir, p)
			if rerr != nil {
				return rerr
			}
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	}); werr != nil {
		return werr
	}
	sort.Strings(names)

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)
	for _, name := range names {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modTime.UTC()}
		w, werr := zw.CreateHeader(hdr)
		if werr != nil {
			return werr
		}
		data, rerr := os.ReadFile(filepath.Join(srcDir, filepath.FromSlash(name)))
		if rerr != nil {
			return rerr
		}
		if _, werr := w.Write(data); werr != nil {
			return werr
		}
	}
	return zw.Close()
}
