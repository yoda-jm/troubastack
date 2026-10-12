// Command t197-relink is anchoring stage 4 (T197, docs/design/anchoring/critique.md §7 "Stage 4"): it
// re-links the marks that predate the stage-2 link model, ONE BY ONE, and only those VLL approved.
//
// Candidates: every live mark on a GENERATED chart that has no new-style Link — an old-style anchored mark
// (Anchor set: frozen at its stored coordinates since T193/T195) or a coordinates-only mark (neither).
// Marks on uploaded PDFs, and marks already carrying a Link, are never candidates.
//
// Two modes:
//
//	t197-relink -data <dataDir> -out <reviewDir>              (the default: -dry)
//	t197-relink -data <dataDir> -apply -approve <idsFile>     (writes)
//
// The DRY RUN writes nothing to the data dir. For each candidate it computes what stage 2's linking rule
// (chartpdf.LinkMark) decides from the mark's CURRENT STORED position on the chart's current render — only
// when that render reproduces the stored PDF (else the mark is listed as skipped, ErrRenderDiverged) — and
// writes <reviewDir>/marks.json (one entry per candidate) and <reviewDir>/<objectId>.png: a crop of the
// stored chart page (pdftoppm -r 110) with the mark in red, the proposed line's run box in blue with a thin
// tie from its reference point, and, for an old-style mark, a dashed orange box where the OLD anchor would
// have projected it (the pre-T195 Project, ported in oldproject.go). marks.json and the crops carry real
// chart text: they belong in a private scratch dir, never in the repo or a gate entry.
//
// APPLY reads the approval file: one object id per line, optionally suffixed ":fixed" to store Fixed
// instead of the proposal; blank lines and lines starting with '#' are ignored. For each approved id that
// is still a candidate, it RECOMPUTES the link on the current render (the dry run is not trusted) and writes
// it through the engine as a normal mutation (a new revision): Points and Page unchanged, the old Anchor
// dropped, PointsRenderHash set to the current render. Nothing else is touched: an unapproved candidate is
// never written, and an approved id that is not a candidate (already linked, deleted, unknown) is listed and
// skipped.
//
// SAFETY: both modes refuse while a server holds the data dir (the app.lock writer lock, the same check as
// t193-freeze-served); file backends only; stdout carries ids and numbers only. Back the data dir up first.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "t197-relink:", err)
		os.Exit(1)
	}
}

// errLive is returned when a server holds the data dir.
var errLive = errors.New("a server holds this data dir (app.lock) — stop it first; this tool never runs behind a live server")

const usage = "usage: t197-relink -data <dataDir> [-dry] -out <reviewDir> | t197-relink -data <dataDir> -apply -approve <idsFile> [-actor id]"

// Old and proposed states, as marks.json and the summary spell them.
const (
	oldStyle   = "old-style"
	oldNone    = "unlinked"
	propSkip   = "skipped"
	forceFixed = "fixed"
)

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("t197-relink", flag.ContinueOnError)
	fs.SetOutput(out)
	dir := fs.String("data", "", "the data dir (TROUBA_DATA_DIR) of a STOPPED server")
	_ = fs.Bool("dry", true, "dry run (the default): write the review files to -out, nothing to the data dir")
	outDir := fs.String("out", "", "dry run: the directory to write marks.json and the crops into")
	apply := fs.Bool("apply", false, "write the links of the approved marks")
	approve := fs.String("approve", "", "apply: the file listing approved object ids, one per line (id or id:fixed)")
	actor := fs.String("actor", "t197-relink", "author id stamped on the writes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" || fs.NArg() != 0 || (*apply && *approve == "") || (!*apply && *outDir == "") {
		return errors.New(usage)
	}
	var approvals map[string]bool
	if *apply {
		var err error
		if approvals, err = readApprovals(*approve); err != nil {
			return err
		}
	}
	if _, err := os.Stat(*dir); err != nil {
		return err
	}
	lock, err := filerepo.LockDir(*dir)
	if errors.Is(err, filerepo.ErrLocked) {
		return errLive
	}
	if err != nil {
		return err
	}
	defer lock.Close()

	w, err := open(*dir)
	if err != nil {
		return err
	}
	if *apply {
		_, err = w.apply(approvals, *actor, out)
		return err
	}
	_, err = w.dryRun(*outDir, out)
	return err
}

// readApprovals parses the approval file into id → forceFixed.
func readApprovals(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, suffix, has := strings.Cut(line, ":")
		id = strings.TrimSpace(id)
		if id == "" || strings.ContainsAny(id, " \t") || (has && strings.TrimSpace(suffix) != forceFixed) {
			return nil, fmt.Errorf("%s:%d: want <objectId> or <objectId>:fixed, got %q", path, i+1, line)
		}
		m[id] = has
	}
	return m, nil
}

// world is an opened data dir.
type world struct {
	dir   string
	repo  *filerepo.Repo
	svc   *app.Service
	eng   *engine.Engine
	files map[string]app.SongFile // generated files only
	songs []string                // songs with at least one generated file, sorted
}

func open(dir string) (*world, error) {
	repo, err := filerepo.New(dir)
	if err != nil {
		return nil, fmt.Errorf("open app repo: %w", err)
	}
	ha, ok := filestore.New(dir).(store.HistoryAware)
	if !ok {
		return nil, errors.New("annotation store is not history-aware")
	}
	w := &world{dir: dir, repo: repo, svc: app.NewService(repo), eng: engine.New(ha), files: map[string]app.SongFile{}}
	all, err := repo.AllSongFiles()
	if err != nil {
		return nil, err
	}
	songSet := map[string]bool{}
	for _, f := range all {
		if f.Generated {
			w.files[f.ID] = f
			songSet[f.SongID] = true
		}
	}
	for id := range songSet {
		w.songs = append(w.songs, id)
	}
	sort.Strings(w.songs)
	return w, nil
}

// candidate is one mark stage 4 may re-link.
type candidate struct {
	song, file string
	obj        domain.Object
}

func oldStateOf(o domain.Object) string {
	if o.Anchor != nil {
		return oldStyle
	}
	return oldNone
}

// candidates lists every live mark on a generated chart with no new-style Link, in (song, file, page, id)
// order.
func (w *world) candidates() ([]candidate, error) {
	var out []candidate
	for _, songID := range w.songs {
		snap, err := w.eng.Head(songID)
		if err != nil {
			return nil, fmt.Errorf("song %s: %w", songID, err)
		}
		fileByLayer := map[string]string{}
		for _, l := range snap.Layers {
			fileByLayer[l.ID] = l.FileID
		}
		for _, o := range snap.LiveObjects() {
			fileID := fileByLayer[o.LayerID]
			if _, gen := w.files[fileID]; !gen || o.Link != nil {
				continue
			}
			out = append(out, candidate{song: songID, file: fileID, obj: o})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.song != b.song {
			return a.song < b.song
		}
		if a.file != b.file {
			return a.file < b.file
		}
		if a.obj.Page != b.obj.Page {
			return a.obj.Page < b.obj.Page
		}
		return a.obj.UUID < b.obj.UUID
	})
	return out, nil
}

// propose is the one decision both modes make: stage 2's linking rule from the mark's stored position, or
// Fixed when VLL forced it. LinkMark keeps Points and Page, drops the old Anchor and stamps r.Hash.
func propose(o domain.Object, r chartpdf.ChartRender, fixed bool) domain.Object {
	next := chartpdf.LinkMark(o.Clone(), r)
	if fixed {
		next.Link = &domain.MarkLink{State: domain.LinkFixed, Render: r.Hash}
	}
	return next
}

// servedState is the state serve would show for the proposal: the stored state, unless the projection on
// the same render says otherwise (it never should; this keeps the dry run honest if a rule changes).
func servedState(next domain.Object, r chartpdf.ChartRender) domain.LinkState {
	if p := chartpdf.ProjectMark(next, r); p.Link != nil {
		return p.Link.State
	}
	return next.Link.State
}

// ---- dry run ----

// entry is one candidate in marks.json.
type entry struct {
	ObjectID string     `json:"objectId"`
	SongID   string     `json:"songId"`
	FileID   string     `json:"fileId"`
	Type     string     `json:"type"`
	Text     string     `json:"text,omitempty"` // a label's text, an icon's glyph id
	Page     int        `json:"page"`
	Box      box        `json:"box"`
	OldState string     `json:"oldState"`
	Old      *oldAnchor `json:"oldAnchor,omitempty"`

	Proposed   string   `json:"proposed"` // linked / check / fixed / skipped
	LineID     string   `json:"lineId,omitempty"`
	LineNumber int      `json:"lineNumber,omitempty"` // 1-based source line
	LineText   string   `json:"lineText,omitempty"`
	Wrap       int      `json:"wrap,omitempty"`
	XRef       string   `json:"xRef,omitempty"`
	DX         *float64 `json:"dx,omitempty"`
	DY         *float64 `json:"dy,omitempty"`
	Margin     bool     `json:"margin"`
	Peers      []string `json:"peers,omitempty"` // "inside:<lineId>" / "runner-up:<lineId>"
	Reason     string   `json:"reason"`
	Notes      []string `json:"notes,omitempty"`
	Crop       string   `json:"crop,omitempty"`
	PageImage  string   `json:"pageImage,omitempty"` // a fixed mark: the whole page, for context
}

// oldAnchor is what the old-style link said, and where it would have projected the mark.
type oldAnchor struct {
	RunText    string `json:"runText"`
	Occurrence int    `json:"occurrence"`
	Projected  *box   `json:"projectedBox,omitempty"`
	ProjPage   *int   `json:"projectedPage,omitempty"`
}

// dryReport counts a dry run.
type dryReport struct {
	Entries []entry
	Counts  map[string]map[string]int // old state → proposed → n
	Crops   int
}

// errDiverged marks a file whose render does not reproduce its blob.
var errDiverged = app.ErrRenderDiverged

// dryRender builds a file's ChartRender WITHOUT writing: like Service.ChartRenderForFile, except that a
// chart with no stored line ids (or ids for another source) gets provisional in-memory ids instead of a
// stored backfill. The apply backfills for real; the line TEXT shown is the same either way.
func (w *world) dryRender(fileID string) (chartpdf.ChartRender, string, error) {
	sf, src, err := w.svc.ChartSourceForFile(fileID)
	if err != nil {
		return chartpdf.ChartRender{}, "", err
	}
	anchors, ok := app.ChartAnchorsIfCurrent(src, sf.BlobHash)
	if !ok {
		return chartpdf.ChartRender{}, "", errDiverged
	}
	note := ""
	li, err := w.repo.GetChartLineIdentity(fileID)
	if err != nil && !errors.Is(err, app.ErrNotFound) {
		return chartpdf.ChartRender{}, "", err
	}
	if err != nil || !li.Describes(src) {
		n := 0
		li = chartpdf.RederiveLineIdentity(nil, "", src, func() string { n++; return fmt.Sprintf("provisional-%d", n) },
			chartpdf.RederiveBackfill, chartpdf.CarryOptions{ToRender: sf.BlobHash})
		note = "this chart has no stored line ids for its source yet: the apply backfills them, so the line id is provisional (the line is not)"
	}
	return chartpdf.ChartRender{Source: src, Lines: li, Anchors: anchors, Hash: sf.BlobHash}, note, nil
}

var typeNames = map[domain.ObjectType]string{
	domain.TypeFreehand: "freehand", domain.TypeLine: "line", domain.TypeRect: "rect", domain.TypeEllipse: "ellipse",
	domain.TypeText: "text", domain.TypeHighlight: "highlight", domain.TypeIcon: "icon",
}

func typeName(t domain.ObjectType) string {
	if n, ok := typeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("type-%d", int(t))
}

func pointsBox(pts []domain.Point) box {
	if len(pts) == 0 {
		return box{}
	}
	b := box{pts[0].X, pts[0].Y, pts[0].X, pts[0].Y}
	for _, p := range pts[1:] {
		b = b.union(box{p.X, p.Y, p.X, p.Y})
	}
	return b
}

// drawnBox is the mark's box as drawn: its points' bounds, or, for a text label (one point, its top-left),
// an estimate of the label's extent from its font size.
func drawnBox(o domain.Object) box {
	b := pointsBox(o.Points)
	if o.Type == domain.TypeText && len(o.Points) > 0 && o.Style.FontSize > 0 {
		p := o.Points[0]
		n := float64(len([]rune(o.Text)))
		b = box{p.X, p.Y, p.X + math.Max(1, n)*0.5*o.Style.FontSize, p.Y + o.Style.FontSize}
	}
	return b
}

func lineIndex(li chartpdf.LineIdentity, id string) int {
	for i, x := range li.IDs {
		if x == id {
			return i
		}
	}
	return -1
}

// runBox is the union of the runs drawn for source line idx, visual line wrap, on page.
func runBox(anchors []chartpdf.Anchor, idx, wrap, page int) (box, bool) {
	var b box
	found := false
	for _, a := range anchors {
		if a.SrcLine != idx || a.SrcWrap != wrap || a.Page != page || a.Text == "" {
			continue
		}
		ab := box{a.X0, a.Y0, a.X1, a.Y1}
		if !found {
			b, found = ab, true
		} else {
			b = b.union(ab)
		}
	}
	return b, found
}

// medianRunHeight is the typical line height on a page (the crop margin when nothing is linked).
func medianRunHeight(anchors []chartpdf.Anchor, page int) float64 {
	var hs []float64
	for _, a := range anchors {
		if a.Page == page && a.Text != "" && a.Y1 > a.Y0 {
			hs = append(hs, a.Y1-a.Y0)
		}
	}
	if len(hs) == 0 {
		return 0
	}
	sort.Float64s(hs)
	return hs[len(hs)/2]
}

func xrefWords(x string) string {
	switch x {
	case "char":
		return "x follows the character under its left edge"
	case "start":
		return "x is measured from the line's start (the mark starts left of the text)"
	case "end":
		return "x is measured from the line's end (the mark starts right of the text)"
	case "margin":
		return "a margin mark: x stays where it is, it follows the line vertically"
	}
	return "x reference " + x
}

func (w *world) dryRun(outDir string, out io.Writer) (dryReport, error) {
	rep := dryReport{Counts: map[string]map[string]int{}}
	cands, err := w.candidates()
	if err != nil {
		return rep, err
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return rep, err
	}
	tmp, err := os.MkdirTemp("", "t197-raster-")
	if err != nil {
		return rep, err
	}
	defer os.RemoveAll(tmp)
	raster := newPageRaster(tmp)

	type rendered struct {
		r    chartpdf.ChartRender
		note string
		err  error
	}
	renders := map[string]rendered{}
	renderOf := func(fileID string) rendered {
		if x, ok := renders[fileID]; ok {
			return x
		}
		r, note, err := w.dryRender(fileID)
		renders[fileID] = rendered{r, note, err}
		return renders[fileID]
	}

	fmt.Fprintf(out, "T197 stage 4 relink — DRY RUN (nothing is written to the data dir): %d candidate(s)\n", len(cands))
	fmt.Fprintf(out, "%-38s %-38s %-4s %-9s %-10s %-8s %s\n", "object", "file", "page", "type", "old", "proposed", "line")
	var cropErrs []string
	for _, c := range cands {
		o := c.obj
		e := entry{ObjectID: o.UUID, SongID: c.song, FileID: c.file, Type: typeName(o.Type), Text: o.Text, Page: o.Page,
			Box: pointsBox(o.Points), OldState: oldStateOf(o)}
		if o.Anchor != nil {
			e.Old = &oldAnchor{RunText: o.Anchor.RunText, Occurrence: o.Anchor.Occurrence}
		}
		ov := overlay{mark: drawnBox(o), minCrop: 600}
		rd := renderOf(c.file)
		switch {
		case errors.Is(rd.err, errDiverged):
			e.Proposed = propSkip
			e.Reason = "skipped: the chart's current render does not reproduce its stored PDF (ErrRenderDiverged), so nothing can be linked against it; re-render the chart, then dry-run again"
		case rd.err != nil:
			return rep, fmt.Errorf("file %s: %w", c.file, rd.err)
		default:
			r := rd.r
			if rd.note != "" {
				e.Notes = append(e.Notes, rd.note)
			}
			next := propose(o, r, false)
			l := next.Link
			e.Proposed = string(servedState(next, r))
			ov.lineH = medianRunHeight(r.Anchors, o.Page)
			if l.State == domain.LinkFixed {
				if len(o.Points) == 0 {
					e.Reason = "stays fixed: the mark has no points"
				} else {
					e.Reason = "stays fixed: no line in its column on this page is within 3 line-heights of it"
				}
			} else {
				idx := lineIndex(r.Lines, l.Line)
				dx, dy := l.DX, l.DY
				e.LineID, e.LineNumber, e.Wrap, e.XRef, e.DX, e.DY = l.Line, idx+1, l.Wrap, l.XRef, &dx, &dy
				e.Margin = l.XRef == "margin"
				if idx >= 0 {
					e.LineText = chartpdf.SourceLines(r.Source)[idx]
				}
				for _, p := range l.Peers {
					e.Peers = append(e.Peers, p.Why+":"+p.Line)
				}
				why := fmt.Sprintf("links to source line %d (visual line %d): its top-left sits %.2f lh %s the line's top; %s",
					idx+1, l.Wrap+1, math.Abs(dy), map[bool]string{true: "below", false: "above"}[dy >= 0], xrefWords(l.XRef))
				if !e.Margin {
					why += fmt.Sprintf(" (dx %.2f lh)", dx)
				}
				for _, p := range l.Peers {
					if p.Why == "runner-up" {
						why += "; another line is almost as close (runner-up): the mark turns check if the two separate"
					} else {
						why += "; it covers several lines and watches the bottom one: an insert inside turns it check"
					}
				}
				e.Reason = why
				ov.lineH = l.LH
				if rb, ok := runBox(r.Anchors, idx, l.Wrap, o.Page); ok {
					ov.run = &rb
					tl := markTopLeft(o)
					ov.refX, ov.refY, ov.tie = tl.X-dx*l.LH, tl.Y-dy*l.LH, true
					if e.Margin { // the tie runs from the line's nearer edge
						ov.refX = rb.X0
						if tl.X >= rb.X1 {
							ov.refX = rb.X1
						}
					}
				}
			}
			if o.Anchor != nil {
				if pg, x0, y0, x1, y1, ok := oldProject(*o.Anchor, r.Anchors); !ok {
					e.Notes = append(e.Notes, "the old anchor's run is gone from the current render: no orange box")
				} else {
					pb := box{x0, y0, x1, y1}
					e.Old.Projected, e.Old.ProjPage = &pb, &pg
					if pg == o.Page {
						ov.old = &pb
						if near(pb, pointsBox(o.Points), 0.002) {
							e.Notes = append(e.Notes, "the old anchor projects exactly where the mark is stored (the orange box is on the red one)")
						}
					} else {
						e.Notes = append(e.Notes, fmt.Sprintf("the old anchor projects onto page %d, not this one: no orange box", pg))
					}
				}
			}
		}
		if e.Proposed == propSkip && o.Anchor != nil {
			e.Notes = append(e.Notes, "no current render, so the old anchor's projection is not drawn")
		}

		// The crop, over the STORED PDF (the file's current blob: what is served).
		sf := w.files[c.file]
		if img, err := raster.page(filepath.Join(w.dir, "blobs", sf.BlobHash), o.Page); err != nil {
			cropErrs = append(cropErrs, o.UUID)
			e.Notes = append(e.Notes, "no crop: "+err.Error())
		} else {
			name := o.UUID + ".png"
			if err := writePNG(filepath.Join(outDir, name), renderCrop(img, ov)); err != nil {
				return rep, err
			}
			e.Crop = name
			rep.Crops++
			if e.Proposed == string(domain.LinkFixed) || e.Proposed == propSkip {
				pname := o.UUID + "-page.png"
				if err := writePNG(filepath.Join(outDir, pname), renderPage(img, ov.mark)); err != nil {
					return rep, err
				}
				e.PageImage = pname
			}
		}

		if rep.Counts[e.OldState] == nil {
			rep.Counts[e.OldState] = map[string]int{}
		}
		rep.Counts[e.OldState][e.Proposed]++
		rep.Entries = append(rep.Entries, e)
		fmt.Fprintf(out, "%-38s %-38s %-4d %-9s %-10s %-8s %s\n", o.UUID, c.file, o.Page, e.Type, e.OldState, e.Proposed, e.LineID)
	}

	b, err := json.MarshalIndent(rep.Entries, "", "  ")
	if err != nil {
		return rep, err
	}
	if rep.Entries == nil {
		b = []byte("[]")
	}
	if err := os.WriteFile(filepath.Join(outDir, "marks.json"), append(b, '\n'), 0o600); err != nil {
		return rep, err
	}
	fmt.Fprintln(out, "summary (old state × proposed):")
	olds := make([]string, 0, len(rep.Counts))
	for k := range rep.Counts {
		olds = append(olds, k)
	}
	sort.Strings(olds)
	for _, k := range olds {
		props := make([]string, 0, len(rep.Counts[k]))
		for p := range rep.Counts[k] {
			props = append(props, p)
		}
		sort.Strings(props)
		for _, p := range props {
			fmt.Fprintf(out, "  %-10s -> %-8s %d\n", k, p, rep.Counts[k][p])
		}
	}
	fmt.Fprintf(out, "wrote %s and %d crop(s)\n", filepath.Join(outDir, "marks.json"), rep.Crops)
	if len(cropErrs) > 0 {
		return rep, fmt.Errorf("%d crop(s) failed: %s", len(cropErrs), strings.Join(cropErrs, ", "))
	}
	return rep, nil
}

// near reports whether two boxes agree within tol on every edge.
func near(a, b box, tol float64) bool {
	return math.Abs(a.X0-b.X0) <= tol && math.Abs(a.Y0-b.Y0) <= tol && math.Abs(a.X1-b.X1) <= tol && math.Abs(a.Y1-b.Y1) <= tol
}

// markTopLeft is the point the link's (dx, dy) is measured to: the linking rule's box top-left (a text
// label's anchor point, else the points' bounds).
func markTopLeft(o domain.Object) domain.Point {
	b := pointsBox(o.Points)
	if o.Type == domain.TypeText && len(o.Points) > 0 {
		return o.Points[0]
	}
	return domain.Point{X: b.X0, Y: b.Y0}
}

// ---- apply ----

// applyReport counts an apply.
type applyReport struct {
	Written     []string
	Skipped     map[string]string // approved id → why it was not written
	Unapproved  int               // candidates left alone because nobody approved them
	ByProposed  map[string]int
	Candidates  int
	ApprovedIDs int
}

func (w *world) apply(approvals map[string]bool, actor string, out io.Writer) (applyReport, error) {
	rep := applyReport{Skipped: map[string]string{}, ByProposed: map[string]int{}, ApprovedIDs: len(approvals)}
	cands, err := w.candidates()
	if err != nil {
		return rep, err
	}
	rep.Candidates = len(cands)
	fmt.Fprintf(out, "T197 stage 4 relink — APPLY: %d approved id(s), %d candidate(s)\n", len(approvals), len(cands))
	seen := map[string]bool{}
	for _, c := range cands {
		o := c.obj
		fixed, approved := approvals[o.UUID]
		if !approved { // THE approval filter: nothing VLL did not approve is ever written
			rep.Unapproved++
			continue
		}
		seen[o.UUID] = true
		r, err := w.svc.ChartRenderForFile(c.file)
		if errors.Is(err, app.ErrRenderDiverged) {
			rep.Skipped[o.UUID] = "the chart's render does not reproduce its stored PDF (ErrRenderDiverged)"
			fmt.Fprintf(out, "skip  %s: render diverged\n", o.UUID)
			continue
		}
		if err != nil {
			return rep, fmt.Errorf("file %s: %w", c.file, err)
		}
		next := propose(o, r, fixed)
		if next.Page != o.Page || !reflect.DeepEqual(next.Points, o.Points) || next.Anchor != nil || next.PointsRenderHash != r.Hash {
			return rep, fmt.Errorf("%s: the proposal changed the mark's geometry or kept its old anchor (refusing)", o.UUID)
		}
		if !chartpdf.ValidLink(next.Link) {
			return rep, fmt.Errorf("%s: the computed link is not valid (refusing): %+v", o.UUID, *next.Link)
		}
		next.Version = o.Version + 1
		m := domain.Mutation{
			Kind:        domain.KindMove,
			UUID:        o.UUID,
			Object:      &next,
			BaseVersion: o.Version,
			AuthorID:    actor,
			Summary:     "T197 stage 4: re-link " + o.UUID + " (approved by VLL)",
			ClientTS:    time.Now().UnixMilli(),
		}
		if _, err := w.eng.Apply(c.song, m); err != nil {
			return rep, fmt.Errorf("write %s: %w", o.UUID, err)
		}
		st := string(next.Link.State)
		rep.Written = append(rep.Written, o.UUID)
		rep.ByProposed[st]++
		fmt.Fprintf(out, "wrote %s: %s -> %s %s\n", o.UUID, oldStateOf(o), st, next.Link.Line)
	}
	var missing []string
	for id := range approvals {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		rep.Skipped[id] = "not a candidate (already linked, deleted, on an uploaded PDF, or unknown)"
		fmt.Fprintf(out, "skip  %s: not a candidate\n", id)
	}
	fmt.Fprintf(out, "summary: %d written (%d linked, %d check, %d fixed), %d approved id(s) skipped, %d unapproved candidate(s) untouched\n",
		len(rep.Written), rep.ByProposed["linked"], rep.ByProposed["check"], rep.ByProposed["fixed"], len(rep.Skipped), rep.Unapproved)
	return rep, nil
}
