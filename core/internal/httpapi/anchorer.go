package httpapi

import (
	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// chartAnchorer implements sync.ChartAnchorer: anchoring stage 2 (T195) on the realtime path. The SERVER
// decides every mark's link to its chart line — at create (AnchorMark), at move/resize/restore
// (RelinkMark), and it keeps the stored link on a restyle (RebaseMark) — and it serves the join snapshot
// projected (ServeSnapshot). It is the ONE place the sync hub reaches back into the service + renderer,
// kept behind sync's narrow ChartAnchorer seam so the hub itself stays free of those dependencies.
//
// It is best-effort and panic-safe: any failure (an uploaded/sourceless file, a render that does not
// reproduce the stored PDF, an unknown layer) falls back to the conservative answer for that call — no
// link at create, the stored link elsewhere. It must NEVER crash the apply goroutine, so a recover()
// backstops every call.
type chartAnchorer struct {
	svc *app.Service
	eng *engine.Engine
}

func newChartAnchorer(svc *app.Service, eng *engine.Engine) *chartAnchorer {
	return &chartAnchorer{svc: svc, eng: eng}
}

// renderFor resolves the current render of the generated chart a layer annotates. nil for a layer on an
// uploaded file, an unknown layer, or a chart whose render diverged from its stored PDF (P32).
func (c *chartAnchorer) renderFor(songID, layerID string) *chartpdf.ChartRender {
	if layerID == "" {
		return nil
	}
	layer, ok := c.eng.Layer(songID, layerID)
	if !ok || layer.FileID == "" {
		return nil
	}
	return chartRenderOf(c.svc, layer.FileID)
}

// chartRenderOf is the shared lookup (realtime and REST): the file's current render, or nil.
func chartRenderOf(svc *app.Service, fileID string) *chartpdf.ChartRender {
	if fileID == "" {
		return nil
	}
	r, err := svc.ChartRenderForFile(fileID)
	if err != nil {
		return nil
	}
	return &r
}

// AnchorMark links a freshly created mark on a generated chart (P2–P5). Unchanged (unlinked) on an uploaded
// file or on any failure.
func (c *chartAnchorer) AnchorMark(songID string, o domain.Object) (out domain.Object) {
	out = o
	defer func() {
		if recover() != nil {
			out = o
		}
	}()
	if r := c.renderFor(songID, o.LayerID); r != nil {
		return chartpdf.LinkMark(o, *r)
	}
	return o
}

// RelinkMark is a move, resize or restore: chartpdf.RelinkMoved against the chart of the STORED object's
// layer (a move need not carry a trustworthy layerId). Any failure stores the client's geometry with the
// stored link, never the client's.
func (c *chartAnchorer) RelinkMark(songID string, moved, stored domain.Object) (out domain.Object) {
	fallback := moved
	fallback.Link, fallback.Anchor, fallback.PointsRenderHash = stored.Link.Clone(), stored.Anchor.Clone(), stored.PointsRenderHash
	out = fallback
	defer func() {
		if recover() != nil {
			out = fallback
		}
	}()
	layer := stored.LayerID
	if layer == "" {
		layer = moved.LayerID
	}
	return chartpdf.RelinkMoved(moved, stored, c.renderFor(songID, layer))
}

// RebaseMark is a setStyle, setText or reorder: chartpdf.RebaseMark (the stored link; the mark does not
// move). Any failure keeps the stored link and geometry.
func (c *chartAnchorer) RebaseMark(songID string, edited, stored domain.Object) (out domain.Object) {
	defer func() {
		if recover() != nil {
			out = chartpdf.RebaseMark(edited, stored, nil)
		}
	}()
	return chartpdf.RebaseMark(edited, stored, c.renderFor(songID, stored.LayerID))
}

// ServeSnapshot is the join snapshot, served like GET /annotations (projectSnapshot).
func (c *chartAnchorer) ServeSnapshot(songID string, snap domain.Snapshot) (out domain.Snapshot) {
	out = snap
	defer func() {
		if recover() != nil {
			out = snap
		}
	}()
	return projectSnapshot(snap, func(fileID string) *chartpdf.ChartRender { return chartRenderOf(c.svc, fileID) })
}

// projectSnapshot projects every linked mark of a snapshot onto the current render of its layer's chart
// (chartpdf.ProjectMarks, T195) — the ONE serve-side projection, used by the REST read and the realtime join
// snapshot; the bake calls the same chartpdf function (P33). Each chart is resolved once. Marks with no
// stage-2 link (an uploaded file, an old-style anchor, never linked) and marks on a chart with no current
// render (P32) are served exactly as stored. Read-only.
func projectSnapshot(snap domain.Snapshot, renderOf func(fileID string) *chartpdf.ChartRender) domain.Snapshot {
	fileByLayer := make(map[string]string, len(snap.Layers))
	for _, l := range snap.Layers {
		fileByLayer[l.ID] = l.FileID
	}
	renders := map[string]*chartpdf.ChartRender{}
	objs := make([]domain.Object, len(snap.Objects))
	for i, o := range snap.Objects {
		objs[i] = o
		if o.Link == nil {
			continue
		}
		fid := fileByLayer[o.LayerID]
		r, seen := renders[fid]
		if !seen {
			r = renderOf(fid)
			renders[fid] = r
		}
		if r != nil {
			objs[i] = chartpdf.ProjectMark(o, *r)
		}
	}
	snap.Objects = objs
	return snap
}
