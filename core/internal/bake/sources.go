package bake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// T185 ⟨D1⟩ — the page-source sidecar. The baker knows every pool page's (fileId, filePage); the bundle
// the tablet downloads deliberately does not carry it (no Stage contract change). We persist it SERVER-SIDE
// next to bundle.json, in bakes/<concert>/<rev>/page-sources.json, so the server can later answer "which
// file and page produced the raster with this hash in this song" — which is how a rehearsal note drawn on a
// member's resolved sequence (where "page 3" is file 2 page 1) is placed back on the right file in Studio.
//
// The map is keyed by songId; each entry carries the fileId, the 0-based page WITHIN that file, and the
// page's rasterHash (the same Sha256Hex(raster) the pool page stores). A note is resolved by matching its
// rasterHash within its song — never by index, which is a position in a per-member sequence.

const sourceMapFile = "page-sources.json"

type PageSource struct {
	FileID     string `json:"fileId"`
	FilePage   int32  `json:"filePage"` // 0-based page within the file
	RasterHash string `json:"rasterHash"`
}

type SourceMap struct {
	Songs map[string][]PageSource `json:"songs"`
}

// writeSourceMap writes the sidecar into dir atomically (temp + rename). At bake time dir is the private
// staging dir; for a T185 backfill it is a LIVE published rev dir, where the rename keeps a concurrent reader
// from ever seeing a torn file.
func writeSourceMap(dir string, m SourceMap) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("bake: marshal source map: %w", err)
	}
	final := filepath.Join(dir, sourceMapFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("bake: write source map: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("bake: publish source map: %w", err)
	}
	return nil
}

// readSourceMap reads the sidecar for a published rev, or (false) if none exists (an older bake baked before
// T185, or a read error).
func (b *Baker) readSourceMap(concertID string, rev uint64) (SourceMap, bool) {
	p := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10), sourceMapFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return SourceMap{}, false
	}
	var m SourceMap
	if err := json.Unmarshal(data, &m); err != nil || m.Songs == nil {
		return SourceMap{}, false
	}
	return m, true
}

// deriveTimeout bounds a single old-rev derive (one song's files). A stuck pdftoppm is killed via the
// context, the note returns unresolved, and the next list retries — "unresolved" is honest, a hung request
// is not (Fable ⟨1⟩). Generous for one song's few files, yet a hard ceiling.
const deriveTimeout = 30 * time.Second

func matchInSlice(srcs []PageSource, rasterHash string) (string, int, bool) {
	for _, ps := range srcs {
		if ps.RasterHash == rasterHash {
			return ps.FileID, int(ps.FilePage), true
		}
	}
	return "", 0, false
}

func matchHash(m SourceMap, songID, rasterHash string) (string, int, bool) {
	return matchInSlice(m.Songs[songID], rasterHash)
}

// ResolvePageSource answers T185 ⟨D2⟩: which file and 0-based page produced the raster with this hash, in
// this song and bake rev. It matches on rasterHash, never on index (the index is a position in a per-member
// sequence), and never guesses — exact or absent.
//
// Three sources, in order (Fable's ruling):
//  1. the note's own rev sidecar (written at bake time for a T185 bake);
//  2. for a rev with no sidecar (baked before T185), DERIVE one by rasterizing the song's files and caching
//     the result into that rev's sidecar — a hash identifies its file by what renders to it, no selection;
//  3. failing both, any OTHER rev of the same concert whose sidecar already has that hash for that song,
//     newest first — the same hash is the same image, so this stays exact and catches revs since purged.
func (b *Baker) ResolvePageSource(concertID string, rev uint64, songID, rasterHash string) (fileID string, filePage int, resolved bool) {
	if rasterHash == "" {
		return "", 0, false
	}
	// 1 + 2: the note's own rev. Use the sidecar's entry for this song if present; otherwise derive JUST this
	// song's files (bounded), never the whole concert (Fable ⟨1⟩).
	srcs, have := []PageSource(nil), false
	if m, ok := b.readSourceMap(concertID, rev); ok {
		srcs, have = m.Songs[songID]
	}
	if !have {
		srcs, have = b.deriveSongSources(concertID, rev, songID)
	}
	if have {
		if fid, fp, found := matchInSlice(srcs, rasterHash); found {
			return fid, fp, true
		}
	}
	// 3: any OTHER rev's existing sidecar, newest first — the same hash is the same image.
	for _, r := range b.revsWithSidecar(concertID) {
		if r == rev {
			continue
		}
		if mm, ok := b.readSourceMap(concertID, r); ok {
			if fid, fp, found := matchHash(mm, songID, rasterHash); found {
				return fid, fp, true
			}
		}
	}
	return "", 0, false
}

// deriveSongSources derives (and caches) the source entries for ONE song of a rev baked before T185, by
// rasterizing only THAT song's files through the same raster path and hashing each page (Fable ⟨1⟩: a note
// pays for its song, never the concert). Returns false — and caches NOTHING — when the rev is not here, the
// render times out, or ANY file fails (Fable ⟨2⟩: "absent" means no file renders to the hash, never "we
// failed to look", so a transient failure must stay retryable). On success it merges the song's entries into
// the rev's sidecar under a per-(concert,rev) lock, so concurrent opens render once and never clobber the
// file.
func (b *Baker) deriveSongSources(concertID string, rev uint64, songID string) ([]PageSource, bool) {
	revDir := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10))
	if _, err := os.Stat(filepath.Join(revDir, "bundle.json")); err != nil {
		return nil, false // not a rev we hold
	}
	lk := b.deriveLock(concertID, rev)
	lk.Lock()
	defer lk.Unlock()
	// Re-check under the lock: a concurrent open may have derived this song while we waited.
	if m, ok := b.readSourceMap(concertID, rev); ok {
		if srcs, present := m.Songs[songID]; present {
			return srcs, true
		}
	}

	files, ferr := b.svc.SongFilesForBake(songID)
	if ferr != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), deriveTimeout)
	defer cancel()
	srcs := make([]PageSource, 0, len(files))
	for _, f := range files {
		_, pdf, berr := b.svc.SongFileBytesForBake(f.ID)
		if berr != nil {
			return nil, false // ⟨2⟩: a failure is not an absence — cache nothing, retry next time
		}
		pages, rerr := b.raster.Rasterize(ctx, pdf)
		if rerr != nil {
			return nil, false
		}
		for i, p := range pages {
			srcs = append(srcs, PageSource{FileID: f.ID, FilePage: int32(i), RasterHash: Sha256Hex(p)})
		}
	}

	// Every file rendered → merge this song into the rev's sidecar (read-modify-write under the held lock).
	m, _ := b.readSourceMap(concertID, rev)
	if m.Songs == nil {
		m.Songs = map[string][]PageSource{}
	}
	m.Songs[songID] = srcs
	_ = writeSourceMap(revDir, m) // best-effort cache; a write failure only costs a re-derive next time
	return srcs, true
}

// deriveLock returns the per-(concert,rev) mutex, creating it on first use.
func (b *Baker) deriveLock(concertID string, rev uint64) *sync.Mutex {
	key := concertID + "/" + strconv.FormatUint(rev, 10)
	b.deriveMu.Lock()
	defer b.deriveMu.Unlock()
	if b.deriveLocks == nil {
		b.deriveLocks = map[string]*sync.Mutex{}
	}
	lk := b.deriveLocks[key]
	if lk == nil {
		lk = &sync.Mutex{}
		b.deriveLocks[key] = lk
	}
	return lk
}

// revsWithSidecar lists the concert's rev numbers that have a page-sources.json, newest first.
func (b *Baker) revsWithSidecar(concertID string) []uint64 {
	entries, err := os.ReadDir(filepath.Join(b.bakesDir, concertID))
	if err != nil {
		return nil
	}
	var revs []uint64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n, perr := strconv.ParseUint(e.Name(), 10, 64)
		if perr != nil {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(b.bakesDir, concertID, e.Name(), sourceMapFile)); statErr == nil {
			revs = append(revs, n)
		}
	}
	sort.Slice(revs, func(i, j int) bool { return revs[i] > revs[j] })
	return revs
}
