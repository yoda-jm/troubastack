package app

import (
	"container/list"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"sync"

	"troubastack/core/internal/chartpdf"
)

// T194 — anchoring stage 1: the app half of LINE IDENTITY. The pure model (ids, the line diff, character
// maps, LocateLine) is chartpdf/lineid_t194.go; this file keeps one chartpdf.LineIdentity per generated
// file, beside its source (Repo.SetChartLineIdentity), and keeps the two in step on every write.
//
// Who writes the ids, and how:
//   - CreateTextChart: fresh ids, no flag.
//   - SaveChartSource: the old ids are carried to the new source by the line diff; a History step records
//     the character maps of the lines it modified, keyed by the PDF blob hashes before and after.
//   - TransposeChartSource: the same, but paired BY INDEX — a transposition rewrites every chord row
//     (G C D → A D E is not similar text) and moves no line.
//   - Band import (v1 manifest or v2 folder): the archive's ids are kept when they describe the imported
//     source exactly; otherwise they are re-derived (RederiveImport, or RederiveImportChanged when the
//     import replaced a source that already had ids — those are carried by the diff, then flagged).
//   - A chart with no ids (it predates T194), or ids that no longer describe its source: re-derived on first
//     use (RederiveBackfill / RederiveMismatch).
//   - cmd/rerender-charts (the renderer changed, the source did not): the ids are unchanged; an empty step
//     records the render change so CharMapSince's chain does not break (chartpdf LineIdentity.WithRerender).
//   - DeleteSongFile: the ids go with the file.
// Nothing here moves a mark: the T193 freeze is on, and stage 2 is the consumer.

// ChartLineIdentity is a generated file's line identity, as stored (an alias, so the repos stay free of a
// chartpdf import).
type ChartLineIdentity = chartpdf.LineIdentity

// ErrRenderDiverged is returned by LocateChartLine when a fresh render of the source does not reproduce
// the stored PDF: the manifest's boxes would describe pixels nobody sees, so no runs are returned (P32).
var ErrRenderDiverged = errors.New("chart render does not reproduce the stored PDF")

// lineLocks serialise every read-modify-write of a file's line identity, so two saves cannot both carry
// from the same old ids, and a reader never sees a source without its ids (T195). One lock PER FILE
// (T195 review F5): a save holds it across the render-free writes and the eager re-placing of the file's
// marks, which must not stall every other chart. An entry is never removed: one small mutex per chart.
var lineLocks = struct {
	sync.Mutex
	m map[string]*sync.Mutex
}{m: map[string]*sync.Mutex{}}

// lockLines takes fileID's line-identity lock and returns its unlock.
func lockLines(fileID string) func() {
	lineLocks.Lock()
	mu, ok := lineLocks.m[fileID]
	if !ok {
		mu = &sync.Mutex{}
		lineLocks.m[fileID] = mu
	}
	lineLocks.Unlock()
	mu.Lock()
	return mu.Unlock
}

// newLineID mints a line id: 64 random bits, base32 (13 characters). Ids only need to be unique within a
// file, for the file's whole life, so a retired id is never re-minted; a random id also cannot collide with
// one a re-derivation retired, which a per-file counter reset by an import could.
func newLineID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("app: crypto/rand failed: " + err.Error())
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}

// lineIdentityLocked returns fileID's ids for source, re-deriving and storing them when there are none or
// they describe another source. render is the file's current PDF blob hash. Caller holds lockLines(fileID).
func (s *Service) lineIdentityLocked(fileID, source, render string) (ChartLineIdentity, error) {
	li, err := s.repo.GetChartLineIdentity(fileID)
	switch {
	case err == nil && li.Describes(source):
		return li, nil
	case err == nil:
		li = chartpdf.RederiveLineIdentity(nil, "", source, newLineID, chartpdf.RederiveMismatch, chartpdf.CarryOptions{ToRender: render})
	case errors.Is(err, ErrNotFound):
		li = chartpdf.RederiveLineIdentity(nil, "", source, newLineID, chartpdf.RederiveBackfill, chartpdf.CarryOptions{ToRender: render})
	default:
		return ChartLineIdentity{}, err
	}
	if err := s.repo.SetChartLineIdentity(fileID, li); err != nil {
		return ChartLineIdentity{}, err
	}
	return li, nil
}

// setFreshLineIdentity stores fresh ids for a just-created chart.
func (s *Service) setFreshLineIdentity(fileID, source string) error {
	defer lockLines(fileID)()
	return s.repo.SetChartLineIdentity(fileID, chartpdf.NewLineIdentity(source, newLineID))
}

// carryLineIdentity carries fileID's ids from oldSource (rendered as fromRender) to newSource (toRender),
// as a save does. byIndex is the transposition mode.
func (s *Service) carryLineIdentity(fileID, oldSource, newSource, fromRender, toRender string, byIndex bool) error {
	defer lockLines(fileID)()
	return s.carryLineIdentityLocked(fileID, oldSource, newSource, fromRender, toRender, byIndex)
}

// carryLineIdentityLocked is carryLineIdentity for a caller that already holds lockLines(fileID) (saveChartSource,
// which holds it across the source write, T195).
func (s *Service) carryLineIdentityLocked(fileID, oldSource, newSource, fromRender, toRender string, byIndex bool) error {
	prev, err := s.lineIdentityLocked(fileID, oldSource, fromRender)
	if err != nil {
		return err
	}
	next := chartpdf.CarryLineIdentity(prev, oldSource, newSource, newLineID,
		chartpdf.CarryOptions{ByIndex: byIndex, FromRender: fromRender, ToRender: toRender})
	return s.repo.SetChartLineIdentity(fileID, next)
}

// importLineIdentity is the defined behaviour of a band import writing a generated chart's source: keep the
// archive's ids if they describe the source exactly; else carry the file's existing ids (a re-import over
// the same stable file id) by diff and flag the file; else fresh ids, flagged. prevSource/prev are what the
// file held before this import (prev nil when it had no ids).
func importLineIdentity(carried *ChartLineIdentity, source, render string, prev *ChartLineIdentity, prevSource string) ChartLineIdentity {
	if carried != nil && carried.Describes(source) {
		return *carried
	}
	opt := chartpdf.CarryOptions{ToRender: render}
	if prev != nil && prev.Describes(prevSource) {
		if prevSource == source {
			return *prev
		}
		return chartpdf.RederiveLineIdentity(prev, prevSource, source, newLineID, chartpdf.RederiveImportChanged, opt)
	}
	return chartpdf.RederiveLineIdentity(nil, "", source, newLineID, chartpdf.RederiveImport, opt)
}

// exportLineIdentity is what a band export carries for a chart: its ids and source hash, and any pending
// Rederived flag — not the History (render hashes of this server's past saves; an import re-renders, so a
// mark arriving with the current render needs none, and one that does not gets "to check" from
// CharMapSince, never a guess). nil when the file has no ids that describe source.
func (s *Service) exportLineIdentity(fileID, source string) *ChartLineIdentity {
	li, err := s.repo.GetChartLineIdentity(fileID)
	if err != nil || !li.Describes(source) {
		return nil
	}
	li.History = nil
	return &li
}

// ChartLines returns a generated file's line ids, backfilling them (flagged RederiveBackfill) for a
// chart that has none. Auth-free and server-internal, like ChartSourceForFile: stage 2 calls it from the
// already-authorised mark paths.
func (s *Service) ChartLines(fileID string) (ChartLineIdentity, error) {
	// T195: the source is read under the lock too. Read before it, a save landing in between would leave
	// lineIdentityLocked holding the OLD source against the NEW ids, and it would re-derive and store fresh
	// ids over the carried ones.
	defer lockLines(fileID)()
	f, src, err := s.ChartSourceForFile(fileID)
	if err != nil {
		return ChartLineIdentity{}, err
	}
	return s.lineIdentityLocked(fileID, src, f.BlobHash)
}

// LocateChartLine is the stage-2 entry point: where line lineID of a generated file is now — its source
// line index, its text, the runs it draws (page and box each), and the character map from fromRender (the
// render a mark was drawn on, its PointsRenderHash) to the current one. ok is false when the line no longer
// exists (deleted: the mark is lost, P20). ErrRenderDiverged when the current render does not reproduce
// the stored PDF (P32). Thin wrapper over chartpdf.LocateLine.
func (s *Service) LocateChartLine(fileID, lineID, fromRender string) (chartpdf.LineLocation, bool, error) {
	r, err := s.ChartRenderForFile(fileID)
	if err != nil {
		return chartpdf.LineLocation{}, false, err
	}
	loc, ok := chartpdf.LocateLine(r.Lines, r.Source, r.Anchors, lineID, fromRender, r.Hash)
	return loc, ok, nil
}

// ClearChartLinesRederived drops a file's Rederived flag — stage 2 calls it once it has marked the file's
// marks "to check". A no-op for a file without ids or without the flag.
func (s *Service) ClearChartLinesRederived(fileID string) error {
	defer lockLines(fileID)()
	li, err := s.repo.GetChartLineIdentity(fileID)
	if errors.Is(err, ErrNotFound) || (err == nil && li.Rederived == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.repo.SetChartLineIdentity(fileID, li.ClearRederived())
}

// ChartRenderForFile is stage 2's (T195) one read of a generated chart: its source, its line ids
// (backfilled if it has none) and the anchor manifest of its current render, read together under the file's line lock
// so the three describe the same save. ErrRenderDiverged when a fresh render does not reproduce the stored
// PDF (P32: nothing is linked or projected against it); ErrNotFound for an uploaded or unknown file.
// Auth-free and server-internal, like ChartSourceForFile: its callers are already authorised (the sync
// write gate, the annotation routes, the bake).
func (s *Service) ChartRenderForFile(fileID string) (chartpdf.ChartRender, error) {
	unlock := lockLines(fileID)
	f, src, err := s.ChartSourceForFile(fileID)
	if err != nil {
		unlock()
		return chartpdf.ChartRender{}, err
	}
	li, err := s.lineIdentityLocked(fileID, src, f.BlobHash)
	unlock()
	if err != nil {
		return chartpdf.ChartRender{}, err
	}
	anchors, current := cachedAnchorsIfCurrent(src, f.BlobHash)
	if !current {
		return chartpdf.ChartRender{}, ErrRenderDiverged
	}
	return chartpdf.ChartRender{Source: src, Lines: li, Anchors: anchors, Hash: f.BlobHash}, nil
}

// renderCache memoises ChartAnchorsIfCurrent by (source, blob): every realtime move and restyle on a chart
// needs its manifest, and a render is the expensive part (≈10 ms for a 100-line chart, against ≈80 µs
// cached). Both keys are content hashes, so an entry is never stale; the cache is a bounded LRU (T195
// review F5: a wipe-when-full map let one busy band evict every other band's charts at once).
var renderCache = struct {
	sync.Mutex
	ll *list.List               // front = most recently used; elements hold *renderEntry
	m  map[string]*list.Element // key → element
}{ll: list.New(), m: map[string]*list.Element{}}

type renderEntry struct {
	key     string
	anchors []chartpdf.Anchor
	ok      bool
}

const renderCacheMax = 256

func cachedAnchorsIfCurrent(source, blobHash string) ([]chartpdf.Anchor, bool) {
	key := chartpdf.SourceHash(source) + "/" + blobHash
	renderCache.Lock()
	if el, hit := renderCache.m[key]; hit {
		renderCache.ll.MoveToFront(el)
		e := el.Value.(*renderEntry)
		renderCache.Unlock()
		return e.anchors, e.ok
	}
	renderCache.Unlock()
	anchors, ok := ChartAnchorsIfCurrent(source, blobHash)
	renderCache.Lock()
	defer renderCache.Unlock()
	if el, hit := renderCache.m[key]; hit { // another caller rendered it meanwhile
		renderCache.ll.MoveToFront(el)
		return anchors, ok
	}
	renderCache.m[key] = renderCache.ll.PushFront(&renderEntry{key: key, anchors: anchors, ok: ok})
	for renderCache.ll.Len() > renderCacheMax {
		last := renderCache.ll.Back()
		renderCache.ll.Remove(last)
		delete(renderCache.m, last.Value.(*renderEntry).key)
	}
	return anchors, ok
}
