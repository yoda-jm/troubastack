package httpapi

import (
	"sync"

	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// chartAnchorer implements sync.ChartAnchorer: at realtime create time it anchors a mark to the words it
// lands on (T145 forward fix), so a later chart size/render change re-projects the mark onto its line
// instead of orphaning it. It is the ONE place the sync hub reaches back into the service + renderer,
// kept behind sync's narrow ChartAnchorer seam so the hub itself stays free of those dependencies.
//
// It is best-effort and panic-safe: any failure (an uploaded/sourceless file, a render error, an unknown
// layer) leaves the mark exactly as drawn — no anchor — which is the pre-T145 behavior. It must NEVER
// crash the apply goroutine, so a recover() backstops the whole call.
type chartAnchorer struct {
	svc *app.Service
	eng *engine.Engine

	mu    sync.Mutex
	cache map[string]anchorEntry // fileID -> last render, re-rendered when the file's BlobHash changes
}

// anchorEntry caches one generated file's anchoring outcome, tagged with the BlobHash it came from so a
// source edit (new BlobHash) invalidates it. ok records a resolved-but-divergent render (a fresh render
// that does not reproduce the stored blob) so those are not re-rendered on every stroke either.
type anchorEntry struct {
	hash    string
	anchors []chartpdf.Anchor
	ok      bool
}

func newChartAnchorer(svc *app.Service, eng *engine.Engine) *chartAnchorer {
	return &chartAnchorer{svc: svc, eng: eng, cache: map[string]anchorEntry{}}
}

// AnchorMark attaches a source anchor to a freshly-created mark on a generated chart, returning the object
// unchanged on any failure. It only anchors marks that don't already carry an anchor, and only on
// generated (source-backed) files.
func (c *chartAnchorer) AnchorMark(songID string, o domain.Object) (out domain.Object) {
	out = o
	defer func() {
		if recover() != nil {
			out = o // best-effort: a bug degrades to no-anchor, never a crashed apply
		}
	}()
	if o.Anchor != nil || o.LayerID == "" {
		return o
	}
	layer, ok := c.eng.Layer(songID, o.LayerID)
	if !ok || layer.FileID == "" {
		return o
	}
	anchors, hash, ok := c.anchorsFor(layer.FileID)
	if !ok {
		return o
	}
	return chartpdf.AnchorObject(o, anchors, hash)
}

// anchorsFor resolves the current (anchor manifest, render hash) for a generated file, rendering once per
// (fileID, BlobHash) and caching the outcome across creates. ok=false for an uploaded/unknown/unrenderable
// file (no source to anchor against) OR a fresh render that does not reproduce the stored blob byte-for-
// byte (app.ChartAnchorsIfCurrent) — in that case the mark, drawn on the STORED blob's pixels, must not be
// anchored against a divergent geometry. Safe for concurrent creates on different songs (the cache is
// mutex-guarded); the render runs outside the lock so a slow render never serializes other files.
func (c *chartAnchorer) anchorsFor(fileID string) ([]chartpdf.Anchor, string, bool) {
	sf, src, err := c.svc.ChartSourceForFile(fileID)
	if err != nil {
		return nil, "", false
	}
	c.mu.Lock()
	if e, seen := c.cache[fileID]; seen && e.hash == sf.BlobHash {
		c.mu.Unlock()
		return e.anchors, e.hash, e.ok
	}
	c.mu.Unlock()

	anchors, ok := app.ChartAnchorsIfCurrent(src, sf.BlobHash)
	c.mu.Lock()
	c.cache[fileID] = anchorEntry{hash: sf.BlobHash, anchors: anchors, ok: ok}
	c.mu.Unlock()
	return anchors, sf.BlobHash, ok
}
