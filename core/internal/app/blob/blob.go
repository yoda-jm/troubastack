// Package blob is a content-addressed binary store for the relational domain's
// song files. Blobs are keyed by the lowercase hex sha256 of their bytes, so the
// same content is stored once regardless of how many SongFile records point at it.
//
// Two backends mirror the app.Repo split:
//   - Mem: in-memory map (tests, throwaway dev runs).
//   - File: one file per blob under <dir>/, named by its hash.
//
// Boundary: stdlib only. This is NOT the annotation store (internal/store) — it
// holds opaque uploaded files (PDFs, images), not append-only annotation history.
package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrNotFound is returned when a hash is not present in the store.
var ErrNotFound = errors.New("blob: not found")

// Store is the content-addressed blob contract. Put returns the sha256 hash of the
// stored bytes; Get returns the bytes for a hash. Implementations are safe for
// concurrent use.
type Store interface {
	Put(data []byte) (hash string, err error)
	Get(hash string) ([]byte, error)
}

// HashOf returns the lowercase hex sha256 of data (the address used as the key).
func HashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ---- in-memory ----

// Mem is an in-memory Store. State is lost on restart.
type Mem struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMem returns an empty in-memory blob store.
func NewMem() *Mem { return &Mem{data: map[string][]byte{}} }

var _ Store = (*Mem)(nil)

// Put stores data and returns its hash. Storing identical bytes again is a no-op.
func (m *Mem) Put(data []byte) (string, error) {
	h := HashOf(data)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[h]; !ok {
		cp := make([]byte, len(data))
		copy(cp, data)
		m.data[h] = cp
	}
	return h, nil
}

// Get returns a copy of the bytes for hash, or ErrNotFound.
func (m *Mem) Get(hash string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.data[hash]
	if !ok {
		return nil, ErrNotFound
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}

// ---- file-backed ----

// File is a directory-backed Store: each blob is one file named by its hash. The
// content-addressed name makes writes idempotent — re-storing the same bytes just
// overwrites a byte-identical file.
type File struct {
	dir string
	mu  sync.Mutex // serializes the write+rename so concurrent Puts don't race the temp file
}

// NewFile opens (creating if needed) a file-backed blob store under dir.
func NewFile(dir string) (*File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("blob: mkdir: %w", err)
	}
	return &File{dir: dir}, nil
}

var _ Store = (*File)(nil)

func (f *File) path(hash string) string { return filepath.Join(f.dir, hash) }

// Put writes data to <dir>/<hash> atomically and returns the hash.
func (f *File) Put(data []byte) (string, error) {
	h := HashOf(data)
	dst := f.path(h)
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := os.Stat(dst); err == nil {
		return h, nil // already stored
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("blob: write tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("blob: rename: %w", err)
	}
	return h, nil
}

// Get reads the bytes for hash, or ErrNotFound.
func (f *File) Get(hash string) ([]byte, error) {
	b, err := os.ReadFile(f.path(hash))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("blob: read: %w", err)
	}
	return b, nil
}
