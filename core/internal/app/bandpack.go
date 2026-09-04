package app

// The packer (T134 phase 2, stage B): a canonical v2 band DIRECTORY <-> a `.tband` zip. A .tband IS the
// directory zipped (amendment 4), so packing is not a translation — it reads the directory's own JSON +
// the file bytes under `<slug>/<filename>` and zips them. Two guards make it more than a blind zip:
//
//   - ⟨P3⟩ the repertoire is the index: only files DECLARED in repertoire.json are packed, so a stray
//     directory (a __pycache__, a .bak) never rides along as a song.
//   - ⟨P2⟩ hashes are verified from disk, not trusted: a declared blobHash that disagrees with the bytes
//     refuses the pack, naming the file — a stale entry would otherwise pack the wrong bytes under a
//     right-looking name. ⟨P4⟩ a declared file missing on disk refuses too (a skipped song is
//     indistinguishable from a song that had nothing).
//
// The packed zip is self-validated through the SAME parseV2 the importer uses, so a directory that would
// not import fails at pack time, and ⟨P5⟩ unzip(pack(dir)) reproduces the directory's canonical content
// byte-for-byte (JSON is copied, never re-marshalled). Legacy folder-vocab is NOT handled here — that is
// a one-shot migration (stage C), by Fable's ruling; the on-disk directory is canonical v2.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"troubastack/core/internal/app/blob"
)

// PackBandDir reads a canonical v2 band directory from fsys and returns the `.tband` zip bytes plus the
// packed size. Required: band.json + repertoire.json. Optional: setlists.json, cues.json, annotations/.
func PackBandDir(fsys fs.FS) (zipBytes []byte, size int, err error) {
	entries := map[string][]byte{}

	// Required + optional top-level manifests, copied verbatim (⟨P5⟩ — no re-marshal, so a round-trip
	// does not diff on whitespace).
	for _, name := range []string{"band.json", "repertoire.json", "setlists.json", "cues.json"} {
		b, rerr := fs.ReadFile(fsys, name)
		if rerr != nil {
			if name == "band.json" || name == "repertoire.json" {
				return nil, 0, fmt.Errorf("%w: pack: missing %s", ErrInvalidInput, name)
			}
			continue // setlists.json / cues.json are optional
		}
		entries[name] = b
	}

	// annotations/<slug>.json (optional directory).
	if annEntries, derr := fs.ReadDir(fsys, "annotations"); derr == nil {
		for _, e := range annEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, rerr := fs.ReadFile(fsys, "annotations/"+e.Name())
			if rerr != nil {
				return nil, 0, fmt.Errorf("%w: pack: annotations/%s: %v", ErrInvalidInput, e.Name(), rerr)
			}
			entries["annotations/"+e.Name()] = b
		}
	}

	// ⟨P3⟩ the repertoire is the index: pack ONLY the files it declares, under `<slug>/<filename>`. A
	// stray directory with no repertoire entry is never included.
	var rep v2Repertoire
	if err := json.Unmarshal(entries["repertoire.json"], &rep); err != nil {
		return nil, 0, fmt.Errorf("%w: pack: repertoire.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	for _, s := range rep.Songs {
		for _, f := range s.Files {
			entry := s.Slug + "/" + f.Filename
			data, rerr := fs.ReadFile(fsys, entry)
			if rerr != nil {
				return nil, 0, fmt.Errorf("%w: pack: declared file %q is not on disk", ErrInvalidInput, entry) // ⟨P4⟩
			}
			if f.BlobHash != "" && blob.HashOf(data) != f.BlobHash { // ⟨P2⟩
				// Distinctive phrasing ("on disk") so a test can assert THIS layer refused, not the
				// downstream self-validation (which would still refuse a hash mismatch, leaving this
				// check unguarded — Fable's stage-B GO finding).
				return nil, 0, fmt.Errorf("%w: pack: file %q on disk does not match its declared blobHash", ErrInvalidInput, entry)
			}
			entries[entry] = data
		}
	}

	return PackEntries(entries)
}

// PackEntries zips a canonical v2 entry set (name→bytes) into a `.tband`, reporting the packed size. It
// is the shared tail of PackBandDir and the migration path (cmd/seed): ⟨P6⟩ refuses past MaxImportBytes
// locally, and self-validates through the importer's own reader so an entry set that would not import
// fails HERE rather than on someone else's upload (covering ⟨P7⟩ safety + integrity + declared-file
// presence).
func PackEntries(entries map[string][]byte) (zipBytes []byte, size int, err error) {
	zipBytes, err = writeV2Zip(entries)
	if err != nil {
		return nil, 0, err
	}
	if len(zipBytes) > MaxImportBytes {
		return nil, len(zipBytes), fmt.Errorf("%w: packed band is %d bytes, over the %d limit", ErrInvalidInput, len(zipBytes), MaxImportBytes)
	}
	if _, _, perr := parseV2(entries); perr != nil {
		return nil, len(zipBytes), fmt.Errorf("pack: the folder does not import: %w", perr)
	}
	return zipBytes, len(zipBytes), nil
}

// UnpackBandZip reads a `.tband` back into its directory entries (name -> bytes), the inverse of
// PackBandDir's zip step — so ⟨P5⟩ unzip(pack(dir)) can be compared to the source directory. Bounded by
// the same entry-count + decompression caps as an import (T63), since a .tband may be untrusted.
func UnpackBandZip(zipBytes []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("%w: not a valid zip archive", ErrInvalidInput)
	}
	if len(zr.File) > maxImportEntries {
		return nil, fmt.Errorf("%w: archive has too many entries (%d > %d)", ErrInvalidInput, len(zr.File), maxImportEntries)
	}
	remaining := maxDecompressedBytes
	out := map[string][]byte{}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if f.UncompressedSize64 > uint64(remaining) {
			return nil, fmt.Errorf("%w: decompressed archive exceeds %d bytes", ErrInvalidInput, maxDecompressedBytes)
		}
		rc, oerr := f.Open()
		if oerr != nil {
			return nil, oerr
		}
		data, rerr := io.ReadAll(io.LimitReader(rc, remaining+1))
		rc.Close()
		if rerr != nil {
			return nil, rerr
		}
		if int64(len(data)) > remaining {
			return nil, fmt.Errorf("%w: decompressed archive exceeds %d bytes", ErrInvalidInput, maxDecompressedBytes)
		}
		remaining -= int64(len(data))
		out[f.Name] = data
	}
	return out, nil
}
