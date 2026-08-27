package bake

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// ---- counting fakes: prove a HIT skips the real renderer -----------------

type countingRasterizer struct {
	calls int32
	out   func(pdf []byte) [][]byte
}

func (c *countingRasterizer) Rasterize(_ context.Context, pdf []byte) ([][]byte, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.out(pdf), nil
}

type cacheCountOverlays struct {
	calls int32
	out   func(s overlaySong) []renderedOverlay
}

func (c *cacheCountOverlays) RenderBatch(_ context.Context, songs []overlaySong) (map[string][]renderedOverlay, error) {
	atomic.AddInt32(&c.calls, 1)
	m := make(map[string][]renderedOverlay, len(songs))
	for _, s := range songs {
		m[s.Key] = c.out(s)
	}
	return m, nil
}

func newCache(t *testing.T) *renderCache {
	t.Helper()
	c, err := newRenderCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// deterministic per-pdf output: N pages whose bytes encode the pdf so a wrong cache entry is detectable.
func rasterOut(pdf []byte) [][]byte {
	return [][]byte{append([]byte("page1:"), pdf...), append([]byte("page2:"), pdf...)}
}

// ---- raster: hit/miss, and byte-identical warm output --------------------

func TestRenderCache_RasterHitMissAndByteIdentical(t *testing.T) {
	inner := &countingRasterizer{out: rasterOut}
	r := cachingRasterizer{inner: inner, cache: newCache(t), dpi: 150, popplerVer: "v1"}
	ctx := context.Background()

	cold, _ := r.Rasterize(ctx, []byte("PDF-A"))
	if inner.calls != 1 {
		t.Fatalf("cold: inner called %d, want 1", inner.calls)
	}
	warm, _ := r.Rasterize(ctx, []byte("PDF-A"))
	if inner.calls != 1 {
		t.Fatalf("warm: inner called %d, want still 1 (cache hit)", inner.calls)
	}
	// Byte-identical warm vs cold — the hard criterion.
	if len(cold) != len(warm) {
		t.Fatalf("page count %d != %d", len(cold), len(warm))
	}
	for i := range cold {
		if !bytes.Equal(cold[i], warm[i]) {
			t.Fatalf("page %d differs cold vs warm", i)
		}
	}
	// A different PDF misses.
	if _, _ = r.Rasterize(ctx, []byte("PDF-B")); inner.calls != 2 {
		t.Fatalf("changed PDF: inner called %d, want 2 (miss)", inner.calls)
	}
}

// ---- stale-key TEETH: dpi and poppler version must be in the raster key --
//
// This is the mutation Fable will re-run: DROP an input from rasterKey and this reddens. As written,
// the same PDF at two DPIs must miss each other (different pixels). If dpi were removed from the key,
// the second call would HIT the first's entry and serve 150-dpi pixels for a 300-dpi request — so the
// `inner.calls == 2` assertion below fails. Same for popplerVer.
func TestRenderCache_RasterKeyIncludesDpiAndPopplerVersion(t *testing.T) {
	cache := newCache(t)
	ctx := context.Background()
	inner := &countingRasterizer{out: rasterOut}

	r150 := cachingRasterizer{inner: inner, cache: cache, dpi: 150, popplerVer: "v1"}
	r300 := cachingRasterizer{inner: inner, cache: cache, dpi: 300, popplerVer: "v1"}
	rV2 := cachingRasterizer{inner: inner, cache: cache, dpi: 150, popplerVer: "v2"}

	r150.Rasterize(ctx, []byte("PDF"))
	r300.Rasterize(ctx, []byte("PDF")) // different dpi → MUST miss
	rV2.Rasterize(ctx, []byte("PDF"))  // different poppler version → MUST miss
	if inner.calls != 3 {
		t.Fatalf("inner called %d, want 3 — dpi and poppler version must each force a distinct key", inner.calls)
	}
	// And the keys are provably distinct.
	k1 := rasterKey([]byte("PDF"), 150, "v1")
	if k1 == rasterKey([]byte("PDF"), 300, "v1") {
		t.Fatal("dpi missing from raster key")
	}
	if k1 == rasterKey([]byte("PDF"), 150, "v2") {
		t.Fatal("poppler version missing from raster key")
	}
}

// ---- overlay: changed annotation misses; the OTHERS do not re-render -----

func songWith(key string, objText string) overlaySong {
	return overlaySong{
		Key: key,
		Doc: annotationsDoc{
			Layers:  []docLayer{{ID: "L1", Order: 0}},
			Objects: []docObject{{UUID: "o1", LayerID: "L1", Type: "text", Page: 0, Text: objText}},
		},
		Pages:        []pageSize{{Index: 0, Width: 800, Height: 600}},
		OverlayWidth: 800,
	}
}

func TestRenderCache_OverlayChangedInputMisses_OthersCached(t *testing.T) {
	var made int32
	inner := &cacheCountOverlays{out: func(s overlaySong) []renderedOverlay {
		atomic.AddInt32(&made, 1)
		return []renderedOverlay{{Page: 0, LayerID: "L1", PNG: []byte("png:" + s.Key + ":" + s.Doc.Objects[0].Text)}}
	}}
	o := cachingOverlayRenderer{inner: inner, cache: newCache(t), inkVer: "ink1"}
	ctx := context.Background()

	// Cold bake of a 2-song setlist: both render.
	o.RenderBatch(ctx, []overlaySong{songWith("A", "hi"), songWith("B", "yo")})
	if made != 2 {
		t.Fatalf("cold: %d overlays made, want 2", made)
	}
	// Warm bake with song A's annotation CHANGED: A misses (re-renders), B hits (not re-rendered).
	made = 0
	inner.calls = 0
	o.RenderBatch(ctx, []overlaySong{songWith("A", "CHANGED"), songWith("B", "yo")})
	if made != 1 {
		t.Fatalf("warm+edit: %d overlays made, want 1 (only the changed song A)", made)
	}
}

// ---- overlay stale-key TEETH: the ink version must be in the overlay key -
func TestRenderCache_OverlayKeyIncludesInkVersion(t *testing.T) {
	s := songWith("A", "hi")
	k1, _ := overlayKey(s, "ink1")
	k2, _ := overlayKey(s, "ink2")
	if k1 == k2 {
		t.Fatal("ink version missing from overlay key — a renderer change would serve stale pixels")
	}
	// And identical content excluding the song KEY collides (content-addressed).
	kSameContent, _ := overlayKey(songWith("DIFFERENT-KEY", "hi"), "ink1")
	if k1 != kSameContent {
		t.Fatal("overlay key is not content-addressed — song identity leaked in")
	}
	// A changed object misses.
	kEdited, _ := overlayKey(songWith("A", "edited"), "ink1")
	if k1 == kEdited {
		t.Fatal("changed annotation did not change the overlay key")
	}
}

// ---- the three controls: force / no-cache / purge ------------------------

func TestRenderCache_ForceWritesButDoesNotRead(t *testing.T) {
	inner := &countingRasterizer{out: rasterOut}
	r := cachingRasterizer{inner: inner, cache: newCache(t), dpi: 150, popplerVer: "v1"}

	r.Rasterize(context.Background(), []byte("PDF")) // warm the entry
	// force: skip the read (re-render) but still write.
	r.Rasterize(withCacheMode(context.Background(), cacheForce), []byte("PDF"))
	if inner.calls != 2 {
		t.Fatalf("force: inner called %d, want 2 (read skipped)", inner.calls)
	}
	// The next default read HITS (force still wrote).
	r.Rasterize(context.Background(), []byte("PDF"))
	if inner.calls != 2 {
		t.Fatalf("after force: inner called %d, want still 2 (force wrote, so this hits)", inner.calls)
	}
}

func TestRenderCache_NoCacheNeitherReadsNorWrites(t *testing.T) {
	cache := newCache(t)
	inner := &countingRasterizer{out: rasterOut}
	r := cachingRasterizer{inner: inner, cache: cache, dpi: 150, popplerVer: "v1"}
	ctx := withCacheMode(context.Background(), cacheOff)

	r.Rasterize(ctx, []byte("PDF"))
	r.Rasterize(ctx, []byte("PDF"))
	if inner.calls != 2 {
		t.Fatalf("no-cache: inner called %d, want 2 (never reads)", inner.calls)
	}
	// And it wrote nothing.
	entries, _ := os.ReadDir(cache.dir)
	if len(entries) != 0 {
		t.Fatalf("no-cache wrote %d cache entries, want 0", len(entries))
	}
}

func TestRenderCache_Purge(t *testing.T) {
	cache := newCache(t)
	inner := &countingRasterizer{out: rasterOut}
	r := cachingRasterizer{inner: inner, cache: cache, dpi: 150, popplerVer: "v1"}

	r.Rasterize(context.Background(), []byte("PDF"))
	if b, ok := cache.get(rasterKey([]byte("PDF"), 150, "v1")); !ok || len(b) == 0 {
		t.Fatal("expected a cached entry before purge")
	}
	if err := cache.purge(); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.get(rasterKey([]byte("PDF"), 150, "v1")); ok {
		t.Fatal("entry survived purge")
	}
}

// ---- file modes: dirs 0o700, files 0o600 (T107 class) --------------------

func TestRenderCache_FileModes(t *testing.T) {
	cache := newCache(t)
	key := rasterKey([]byte("PDF"), 150, "v1")
	if err := cache.put(key, []byte("data")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(cache.path(key))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("cache file mode %o, want 0600", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Join(cache.dir, key[:2]))
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("cache shard dir mode %o, want 0700", di.Mode().Perm())
	}
}

// ---- concurrency: two bakes racing the same key stay correct (go test -race)
func TestRenderCache_ConcurrentSameKey(t *testing.T) {
	inner := &countingRasterizer{out: rasterOut}
	r := cachingRasterizer{inner: inner, cache: newCache(t), dpi: 150, popplerVer: "v1"}
	pdf := []byte("PDF")
	want := rasterOut(pdf)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.Rasterize(context.Background(), pdf)
			if err != nil {
				t.Error(err)
				return
			}
			for p := range want {
				if !bytes.Equal(got[p], want[p]) {
					t.Errorf("torn/incorrect page under concurrency")
				}
			}
		}()
	}
	wg.Wait()
}

// ---- serialization round-trips exactly (byte-identical basis) ------------
func TestRenderCache_SerializationRoundTrip(t *testing.T) {
	pages := [][]byte{[]byte("\x00\x01png"), {}, []byte("\xff\xfe")}
	enc, err := encodeRasters(pages)
	if err != nil {
		t.Fatal(err)
	}
	back, err := decodeRasters(enc)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(pages) != fmt.Sprint(back) {
		t.Fatalf("raster round-trip mismatch")
	}
}
