// Package filestore persists to a plain append-only file tree — the simplest
// zero-infra local-dev backend (ADR 0002). For versioned history with native
// revert + reachability GC, see the gitstore sibling.
//
// Layout (mirrors the append-only domain, I4):
//
//	<dir>/songs/<songID>.jsonl   append-only JSONL; one record per line (mutation or
//	                             revision or pin). Folding the log materializes state
//	                             (I2 idempotent, I5 tombstones, I7 the log IS history).
//	<dir>/blobs/<sha256>         content-addressed PDF/image assets (once)
//
// Human-inspectable and diffable. Implements the full Collector contract.
// Boundary: imports store + domain + stdlib only.
package filestore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// File is a file-tree-backed Store rooted at dir.
type File struct {
	dir string
	mu  sync.Mutex
}

// New returns a file-backed Store rooted at dir (created on first write).
func New(dir string) store.Store { return &File{dir: dir} }

// Capability assertion: File is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*File)(nil)

// record is one JSONL line. Exactly one of the pointer fields is set, selected by Type.
type record struct {
	Type     string           `json:"type"` // "mut" | "rev" | "pin"
	Mutation *domain.Mutation `json:"mutation,omitempty"`
	Revision *domain.Revision `json:"revision,omitempty"`
	Pin      *domain.Pin      `json:"pin,omitempty"`
}

func (f *File) songPath(songID string) string {
	return filepath.Join(f.dir, "songs", songID+".jsonl")
}

// loaded is the folded in-memory view of a song's JSONL log.
type loaded struct {
	log       []domain.Mutation
	revisions []domain.Revision
	revLogLen []int
	pins      map[string]domain.Pin
}

func (f *File) load(songID string) (*loaded, error) {
	l := &loaded{pins: map[string]domain.Pin{}}
	file, err := os.Open(f.songPath(songID))
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, err
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("filestore: corrupt record in %s: %w", songID, err)
		}
		switch r.Type {
		case "mut":
			l.log = append(l.log, *r.Mutation)
		case "rev":
			l.revisions = append(l.revisions, *r.Revision)
			l.revLogLen = append(l.revLogLen, len(l.log))
		case "pin":
			l.pins[r.Pin.Name] = *r.Pin
		}
	}
	return l, sc.Err()
}

func (f *File) append(songID string, r record) error {
	path := f.songPath(songID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(f.dir, "blobs"), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := file.Write(b); err != nil {
		return err
	}
	return file.Sync()
}

// Head folds the whole log.
func (f *File) Head(songID string) (domain.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return store.Fold(l.log, uint64(len(l.revisions))), nil
}

// Apply appends a mutation record (I2 idempotent by seq — skip a seq already logged).
func (f *File) Apply(songID string, mut domain.Mutation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mut.Seq != 0 {
		l, err := f.load(songID)
		if err != nil {
			return err
		}
		for _, prev := range l.log {
			if prev.Seq == mut.Seq {
				return nil
			}
		}
	}
	m := mut.Clone()
	return f.append(songID, record{Type: "mut", Mutation: &m})
}

// SnapshotAt folds the prefix captured when revision N was appended (I4).
func (f *File) SnapshotAt(songID string, revision uint64) (domain.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if revision == 0 || revision > uint64(len(l.revisions)) {
		return domain.Snapshot{}, store.ErrNotFound
	}
	return store.Fold(l.log[:l.revLogLen[revision-1]], revision), nil
}

// AppendRevision appends a revision record; returns its number (I4).
func (f *File) AppendRevision(songID string, r domain.Revision) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return 0, err
	}
	r.Number = uint64(len(l.revisions)) + 1
	if r.Number > 1 {
		r.Parent = r.Number - 1
	}
	if err := f.append(songID, record{Type: "rev", Revision: &r}); err != nil {
		return 0, err
	}
	return r.Number, nil
}

// Revisions returns the linear history oldest→newest.
func (f *File) Revisions(songID string) ([]domain.Revision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return nil, err
	}
	return l.revisions, nil
}

// MovePin appends a pin record (last write wins on fold/load); never deletes targets.
func (f *File) MovePin(songID, pinName string, revision uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return err
	}
	if revision == 0 || revision > uint64(len(l.revisions)) {
		return store.ErrNotFound
	}
	p := domain.Pin{SongID: songID, Name: pinName, RevisionNumber: revision}
	return f.append(songID, record{Type: "pin", Pin: &p})
}

// Pins lists the pins for a song.
func (f *File) Pins(songID string) ([]domain.Pin, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, err := f.load(songID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Pin, 0, len(l.pins))
	for _, p := range l.pins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Collect is the on-disk reachability sweep (I7) — currently a reference-safe no-op
// (keep-all is the default and always legal). A real prune is DEFERRED (P202): the
// per-song JSONL is a delta chain, so reclaiming space means rewriting it around a
// synthesized baseline snapshot at the oldest kept revision, not just deleting lines —
// an invariant-invasive change with its own design. The no-op keeps every reference
// reconstructable; storetest's ReachabilityI7 subtest locks that guarantee.
func (f *File) Collect(_ store.RootSet, _ store.RetentionPolicy) error {
	return nil
}
