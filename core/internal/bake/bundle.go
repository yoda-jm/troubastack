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

// The ConcertBundle mirror structs are GENERATED (bundle_gen.go, cmd/gen-mirrors
// from proto/bundle.proto — T09). Only methods + helpers live here.

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

	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // T107: the .tstage is the whole repertoire — owner-only
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
