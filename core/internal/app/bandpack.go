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
	entries, rerr := readCanonicalDir(fsys)
	if rerr != nil {
		return nil, 0, rerr
	}
	return PackEntries(entries)
}

// readCanonicalDir reads a CANONICAL v2 directory into its entry set: the known JSON manifests +
// annotations/<slug>.json, and ONLY the files DECLARED in repertoire.json under <slug>/<filename> (⟨P3⟩ —
// a stray file or directory is never read, so it cannot ride along into the archive). ⟨P2⟩ verifies a
// declared blobHash from disk; ⟨P4⟩ a declared file missing on disk is refused. Shared by PackBandDir and
// the migration passthrough (a canonical folder), so both exclude strays identically.
func readCanonicalDir(fsys fs.FS) (map[string][]byte, error) {
	entries := map[string][]byte{}
	for _, name := range []string{"band.json", "repertoire.json", "setlists.json", "cues.json"} {
		b, rerr := fs.ReadFile(fsys, name)
		if rerr != nil {
			if name == "band.json" || name == "repertoire.json" {
				return nil, fmt.Errorf("%w: read: missing %s", ErrInvalidInput, name)
			}
			continue // setlists.json / cues.json are optional
		}
		entries[name] = b
	}
	if annEntries, derr := fs.ReadDir(fsys, "annotations"); derr == nil {
		for _, e := range annEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, rerr := fs.ReadFile(fsys, "annotations/"+e.Name())
			if rerr != nil {
				return nil, fmt.Errorf("%w: read: annotations/%s: %v", ErrInvalidInput, e.Name(), rerr)
			}
			entries["annotations/"+e.Name()] = b
		}
	}
	var rep v2Repertoire
	if err := json.Unmarshal(entries["repertoire.json"], &rep); err != nil {
		return nil, fmt.Errorf("%w: read: repertoire.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	for _, s := range rep.Songs {
		for _, f := range s.Files {
			entry := s.Slug + "/" + f.Filename
			data, rerr := fs.ReadFile(fsys, entry)
			if rerr != nil {
				return nil, fmt.Errorf("%w: read: declared file %q is not on disk", ErrInvalidInput, entry) // ⟨P4⟩
			}
			if f.BlobHash != "" && blob.HashOf(data) != f.BlobHash { // ⟨P2⟩
				// "on disk" distinguishes THIS check from the downstream self-validation (Fable's stage-B
				// GO finding — the guard must be the one that fires).
				return nil, fmt.Errorf("%w: read: file %q on disk does not match its declared blobHash", ErrInvalidInput, entry)
			}
			entries[entry] = data
		}
	}
	return entries, nil
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
