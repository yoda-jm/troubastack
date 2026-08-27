package bake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// T120 — a CONTENT-KEYED render cache over the bake's two shell-out stages (poppler page rasters +
// web/bake overlay PNGs). Content-keyed, not dirty-marked: the key IS the content hash of every input
// that can change a pixel, so a changed input simply misses — no mutation path can forget to invalidate
// (a forgotten invalidation would serve a stale annotation on stage, silently, forever).
//
// Granularity is the RENDERER'S UNIT: one entry per PDF (rasterizer input) and one per SONG (the overlay
// batch's per-song request). A miss re-renders that whole unit through the real renderer, so warm output
// is byte-identical to cold BY CONSTRUCTION — never a partial re-render that could diverge.
//
// The cache is invisible to the app + R10: it caches build intermediates only, and does not touch
// RasterHash/ContentHash semantics or the bundle format.

// cacheMode is per-BAKE-CALL (threaded via context, so concurrent bakes each carry their own): "force"
// and "don't cache" are different needs.
type cacheMode int

const (
	cacheDefault cacheMode = iota // read + write — the normal path
	cacheForce                    // skip READS, still write — "I don't trust the cache right now" (next bake is warm)
	cacheOff                      // neither read nor write — "I'm measuring/testing, give me a cold path"
)

type cacheModeKey struct{}

func withCacheMode(ctx context.Context, m cacheMode) context.Context {
	return context.WithValue(ctx, cacheModeKey{}, m)
}

// WithCacheControl sets the render-cache mode on ctx from a bake request's force/no-cache flags — the
// httpapi bake handler calls it, then passes ctx to Bake. no-cache wins over force (measuring a cold
// path beats distrusting the cache). Exported because the two flags cross the httpapi boundary.
func WithCacheControl(ctx context.Context, force, noCache bool) context.Context {
	switch {
	case noCache:
		return withCacheMode(ctx, cacheOff)
	case force:
		return withCacheMode(ctx, cacheForce)
	default:
		return ctx // cacheDefault is the zero-value read
	}
}

func cacheModeFromContext(ctx context.Context) cacheMode {
	if m, ok := ctx.Value(cacheModeKey{}).(cacheMode); ok {
		return m
	}
	return cacheDefault
}

func (m cacheMode) reads() bool  { return m == cacheDefault }
func (m cacheMode) writes() bool { return m == cacheDefault || cacheForce == m }

// renderCache is a flat content-addressed store on disk: <dir>/<key[:2]>/<key>. Entries are immutable
// (a key is a content hash), so concurrent bakes racing on the same key write byte-identical data — the
// write is atomic (temp + rename) so a reader never sees a torn entry.
type renderCache struct {
	dir string
}

func newRenderCache(dir string) (*renderCache, error) {
	if dir == "" {
		return nil, nil // caching disabled (mem-backed cores, and tests that don't opt in)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("render cache dir: %w", err)
	}
	return &renderCache{dir: dir}, nil
}

func (c *renderCache) path(key string) string {
	return filepath.Join(c.dir, key[:2], key)
}

// get returns the cached bytes for key, or (nil,false) on a miss.
func (c *renderCache) get(key string) ([]byte, bool) {
	b, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil, false
	}
	return b, true
}

// put writes key→data atomically: a temp file in the SAME shard dir (so rename is atomic on one
// filesystem), 0o600, then rename over the final path. Two bakes racing the same key both write the
// identical content, so whichever rename wins, the entry is correct.
func (c *renderCache) put(key string, data []byte) error {
	shard := filepath.Join(c.dir, key[:2])
	if err := os.MkdirAll(shard, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(shard, "."+key+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(c.dir, key[:2], key))
}

// purge empties the cache dir (keeps the dir itself, 0o700). The bound-growth answer for a LAN box: an
// explicit purge (make target / troubacore subcommand), not a size cap — a setlist's rasters are large
// and a wrong eviction is a cold bake, not a bug.
func (c *renderCache) purge() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(c.dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// PurgeCache empties this baker's render cache (T120's purge control). No-op if caching is disabled.
func (b *Baker) PurgeCache() error {
	if b.cache == nil {
		return nil
	}
	return b.cache.purge()
}

// PurgeCacheDir empties a render cache at dir — the entry point for the purge subcommand / make target,
// which has a dir but no Baker. A no-op for an empty dir (caching disabled).
func PurgeCacheDir(dir string) error {
	c, err := newRenderCache(dir)
	if err != nil || c == nil {
		return err
	}
	return c.purge()
}

// ---- keys ---------------------------------------------------------------

// rasterKey covers EVERY input that can change a page raster: the PDF bytes, the DPI, and the poppler
// version (a poppler upgrade can change pixels for the same PDF+dpi).
func rasterKey(pdf []byte, dpi int, popplerVer string) string {
	h := sha256.New()
	h.Write(pdf)
	fmt.Fprintf(h, "\x00dpi=%d\x00poppler=%s", dpi, popplerVer)
	return "r" + hex.EncodeToString(h.Sum(nil))
}

// overlayKey covers EVERY input that can change a song's overlays: the annotation doc (stroke objects +
// their order + layer id/order/mandatory/roleTag), the page dims, the overlay width, and the ink
// version. The song KEY (identity) is deliberately excluded — two songs with identical content share the
// entry (content-addressed). The doc is marshalled with its object order intact because z-order is a
// pixel input; reordering for the hash would decouple the key from what's actually drawn.
func overlayKey(s overlaySong, inkVer string) (string, error) {
	h := sha256.New()
	enc := gob.NewEncoder(h)
	// A struct WITHOUT Key, so the hash is content only. gob over a fixed-field struct with no maps is
	// deterministic.
	if err := enc.Encode(struct {
		Doc          annotationsDoc
		Pages        []pageSize
		OverlayWidth int
	}{s.Doc, s.Pages, s.OverlayWidth}); err != nil {
		return "", err
	}
	fmt.Fprintf(h, "\x00ink=%s", inkVer)
	return "o" + hex.EncodeToString(h.Sum(nil)), nil
}

// ---- serialization (exact round-trip, for byte-identical warm output) ----

func encodeRasters(pages [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(pages); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeRasters(b []byte) ([][]byte, error) {
	var pages [][]byte
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func encodeOverlays(ovs []renderedOverlay) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ovs); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeOverlays(b []byte) ([]renderedOverlay, error) {
	var ovs []renderedOverlay
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&ovs); err != nil {
		return nil, err
	}
	return ovs, nil
}

// ---- caching decorators over the two render interfaces ------------------

// cachingRasterizer caches poppler page rasters keyed on the PDF bytes + dpi + poppler version.
type cachingRasterizer struct {
	inner      Rasterizer
	cache      *renderCache
	dpi        int
	popplerVer string
}

func (r cachingRasterizer) Rasterize(ctx context.Context, pdf []byte) ([][]byte, error) {
	mode := cacheModeFromContext(ctx)
	key := rasterKey(pdf, r.dpi, r.popplerVer)
	if mode.reads() {
		if b, ok := r.cache.get(key); ok {
			if pages, err := decodeRasters(b); err == nil {
				return pages, nil
			}
			// A corrupt entry (should not happen — immutable, atomic) falls through to a fresh render.
		}
	}
	pages, err := r.inner.Rasterize(ctx, pdf)
	if err != nil {
		return nil, err
	}
	if mode.writes() {
		if enc, err := encodeRasters(pages); err == nil {
			_ = r.cache.put(key, enc) // a cache-write failure must never fail the bake
		}
	}
	return pages, nil
}

// cachingOverlayRenderer caches the overlay batch PER SONG. On a warm batch it renders only the
// cache-MISS songs through the inner renderer (one Node spawn for those, T98 preserved) and merges the
// hits — so an unchanged song is never re-rendered, while a changed one re-renders WHOLE (byte-identical).
type cachingOverlayRenderer struct {
	inner  OverlayRenderer
	cache  *renderCache
	inkVer string
}

func (r cachingOverlayRenderer) RenderBatch(ctx context.Context, songs []overlaySong) (map[string][]renderedOverlay, error) {
	mode := cacheModeFromContext(ctx)
	result := make(map[string][]renderedOverlay, len(songs))
	keyBySong := make(map[string]string, len(songs))
	var miss []overlaySong

	for _, s := range songs {
		key, err := overlayKey(s, r.inkVer)
		if err != nil {
			return nil, err
		}
		keyBySong[s.Key] = key
		if mode.reads() {
			if b, ok := r.cache.get(key); ok {
				if ovs, derr := decodeOverlays(b); derr == nil {
					result[s.Key] = ovs
					continue
				}
			}
		}
		miss = append(miss, s)
	}

	if len(miss) > 0 {
		rendered, err := r.inner.RenderBatch(ctx, miss)
		if err != nil {
			return nil, err
		}
		for _, s := range miss {
			ovs := rendered[s.Key]
			result[s.Key] = ovs
			if mode.writes() {
				if enc, eerr := encodeOverlays(ovs); eerr == nil {
					_ = r.cache.put(keyBySong[s.Key], enc)
				}
			}
		}
	}
	return result, nil
}

// ---- version probes (the "poppler version" / "ink version" in the keys) --

// popplerVersion runs `pdftoppm -v` (version goes to stderr) and returns its trimmed output. On any
// failure it returns "unknown" — the cache still works, just keyed on a constant, and a truly missing
// binary fails the bake itself with a clear error elsewhere.
func popplerVersion(bin string) string {
	cmd := exec.Command(bin, "-v")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // -v exits non-zero on some builds; the text is what we want
	if s := firstLine(out.String()); s != "" {
		return s
	}
	return "unknown"
}

// inkVersion hashes the DEPLOYED overlay renderer — web/bake/dist/cli.js (TROUBA_BAKE_CLI), which bundles
// @troubastack/ink. Its bytes are the ground truth for "can the overlay pixels change": if ink changes
// and web/bake is rebuilt, cli.js changes and every overlay key moves. Chosen over ink's package.json
// version because the built artifact can't drift from what actually renders. On an unreadable path it
// returns "unknown".
func inkVersion(cliPath string) string {
	b, err := os.ReadFile(cliPath)
	if err != nil {
		return "unknown"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
