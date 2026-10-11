package chartpdf

import (
	"math"
	"sort"

	"troubastack/core/internal/domain"
)

// T195 — anchoring stage 2: the LINKING RULE and the RIGID PROJECTION (docs/design/anchoring/critique.md §7,
// anchoring-spec.md Part B, P2–P29; decisions in docs/tasks/T195-anchoring-stage2-rigid-follow.md).
//
// A mark on a generated chart is linked to ONE source line by the line's stable id (stage 1,
// LineIdentity). The server decides the link (LinkMark) from where the mark is, at create, move, resize
// and import. At serve and bake the mark is moved as one block with its line (ProjectMark):
//   - translated by the movement of its line's reference point;
//   - scaled only by the type-size ratio (newLH / LH);
//   - never reshaped: point order is preserved, a stamp keeps its aspect, nothing is fitted to a text box;
//   - shifted sideways by the character map when its own line was edited before its left-edge character
//     (P19); a margin mark keeps its x (P4).
//
// Anything uncertain still follows and becomes "check"; a deleted line makes the mark "lost", where it was
// (P20). There is one transform for every shape: what differs per shape is only how its box is measured
// (a text mark's box is its anchor point) and that a text mark's font size is scaled too.
//
// Old-style links (domain.SourceAnchor, text + occurrence) are never interpreted here: such a mark has no
// Link, so ProjectMark leaves it at its stored coordinates. Stage 4 re-links them.

// ChartRender is one generated chart's CURRENT render, with everything linking and projection need: its
// source, its line ids (which must describe the source), the anchor manifest of a render that reproduces
// the stored PDF (P32: the caller only builds one when it does), and that PDF's blob hash.
type ChartRender struct {
	Source  string
	Lines   LineIdentity
	Anchors []Anchor
	Hash    string
}

// The linking thresholds, in line-heights (lh) of the line concerned.
const (
	// linkMaxGapLH is P3/P5's reach: a mark farther than this from every line in its column is fixed. The
	// same 3.0 T172 measured on VLL's unanchored marks (median 1.09, p90 2.65, one outlier at 6.49).
	linkMaxGapLH = 3.0
	// linkTieLH: two candidate lines closer than this to the same distance are a tie, and the line ABOVE
	// wins (P3).
	linkTieLH = 0.25
	// linkRunnerUpLH: a second line within this of the winner's distance is recorded as the runner-up
	// (P7, P22).
	linkRunnerUpLH = 0.5
	// linkSeparateLH: a watched peer line that moves by more than this relative to the reference line
	// makes the mark "check" (P12, P22) — the follow tolerance's vertical half.
	linkSeparateLH = 0.25
)

// Horizontal references (domain.MarkLink.XRef).
const (
	xrefChar   = "char"
	xrefStart  = "start"
	xrefEnd    = "end"
	xrefMargin = "margin"
)

// visLine is one VISUAL line of a source line: the runs drawn for (SrcLine, SrcWrap), in seg order.
type visLine struct {
	id         string // the source line's id
	line, wrap int
	page, col  int
	runs       []Anchor
	x0, x1     float64 // the drawn text's horizontal extent
	top, lh    float64 // the first run's top and height: the line's reference point and type size
	bot        float64
}

func (v visLine) centre() float64 { return v.top + v.lh/2 }

// boundsOf is the [0,1] bounding box of a mark's points (min/max over the path/corners).
func boundsOf(pts []domain.Point) (x0, y0, x1, y1 float64) {
	if len(pts) == 0 {
		return 0, 0, 0, 0
	}
	x0, y0 = pts[0].X, pts[0].Y
	x1, y1 = x0, y0
	for _, p := range pts[1:] {
		x0, x1 = math.Min(x0, p.X), math.Max(x1, p.X)
		y0, y1 = math.Min(y0, p.Y), math.Max(y1, p.Y)
	}
	return
}

// markBox is the box the linking rule measures. Every shape is its points' bounds, except a text mark,
// which is its anchor point alone: the server cannot measure the glyphs, and a label's top-left is where
// the user put it (its "beside" relation is from there).
func markBox(o domain.Object) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = boundsOf(o.Points)
	if o.Type == domain.TypeText && len(o.Points) > 0 {
		return o.Points[0].X, o.Points[0].Y, o.Points[0].X, o.Points[0].Y
	}
	return
}

// chartColumnCount is the number of body columns the source renders with (renderChart's rule: `columns: 2`,
// except that a tab chart stays one column).
func chartColumnCount(source string) int {
	lines := chartLines(source)
	_, _, _, _, _, cols, _ := parseHeader(lines)
	if cols > 1 && hasTabBlock(lines) {
		cols = 1
	}
	if cols < 1 {
		cols = 1
	}
	return cols
}

// columnOf is the index of the body column whose box contains x (page fraction), or the nearest one.
func columnOf(x float64, cols int) int {
	if cols <= 1 {
		return 0
	}
	colW := (right - leftMargin - float64(cols-1)*colGutter) / float64(cols)
	best, bestD := 0, math.MaxFloat64
	for c := 0; c < cols; c++ {
		l := (leftMargin + float64(c)*(colW+colGutter)) / pageW
		r := l + colW/pageW
		d := 0.0
		if x < l {
			d = l - x
		} else if x > r {
			d = x - r
		}
		if d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func runChars(a Anchor) int { return len([]rune(a.Text)) }

// cw is a run's average character width: the same measure the anchoring spec defines (X1−X0)/runes.
func cw(a Anchor) float64 {
	if n := runChars(a); n > 0 {
		return (a.X1 - a.X0) / float64(n)
	}
	return 0
}

// visualLines groups r's manifest into visual lines with a line id. Runs drawn from no source line (the
// "Chart" placeholder title) or with no text are not linkable and are left out. nil when the ids do not
// describe the source (nothing can be linked by id then).
func (r ChartRender) visualLines() []visLine {
	if !r.Lines.Describes(r.Source) {
		return nil
	}
	cols := chartColumnCount(r.Source)
	type key struct{ line, wrap int }
	idx := map[key]int{}
	var out []visLine
	for _, a := range r.Anchors {
		if a.SrcLine < 0 || a.SrcLine >= len(r.Lines.IDs) || a.Text == "" {
			continue
		}
		k := key{a.SrcLine, a.SrcWrap}
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, visLine{id: r.Lines.IDs[a.SrcLine], line: a.SrcLine, wrap: a.SrcWrap, page: a.Page})
		}
		out[i].runs = append(out[i].runs, a)
	}
	for i := range out {
		v := &out[i]
		sort.SliceStable(v.runs, func(p, q int) bool { return v.runs[p].SrcSeg < v.runs[q].SrcSeg })
		f := v.runs[0]
		v.top, v.lh, v.bot = f.Y0, f.Y1-f.Y0, f.Y1
		v.x0, v.x1 = f.X0, f.X1
		for _, a := range v.runs[1:] {
			v.x0, v.x1 = math.Min(v.x0, a.X0), math.Max(v.x1, a.X1)
			v.bot = math.Max(v.bot, a.Y1)
		}
		v.col = columnOf(f.X0, cols)
	}
	return out
}

// linesOf returns the visual lines of one source line id, in wrap order.
func linesOf(all []visLine, id string) []visLine {
	var out []visLine
	for _, v := range all {
		if v.id == id {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].wrap < out[j].wrap })
	return out
}

func atWrap(vls []visLine, wrap int) visLine {
	for _, v := range vls {
		if v.wrap == wrap {
			return v
		}
	}
	if wrap >= len(vls) {
		return vls[len(vls)-1]
	}
	return vls[0]
}

// LinkMark is the linking rule (P2–P7), applied by the SERVER at create, move, resize and import: it
// returns o with a new Link decided from where o is on r, its PointsRenderHash set to r.Hash, and its
// old-style Anchor cleared (the server's new decision supersedes it). Any link o arrived with is ignored.
//
// The reference line, in this order:
//  1. the topmost line the mark's box covers ENTIRELY (a box drawn around a line, or a multi-line mark:
//     P2, P6);
//  2. else the topmost line whose vertical centre the box covers (P2);
//  3. else the line whose box contains the mark's vertical centre — a thin mark, an underline, a label's
//     anchor point; in the zone where two boxes overlap, the one whose centre is nearer, the line above on
//     a tie, and the other is the runner-up (P22);
//  4. else the nearest line by vertical edge gap within 3 lh, the line above on a tie within 0.25 lh, a
//     runner-up within 0.5 lh (P3, P4); horizontal overlap with the text is NOT required;
//  5. else fixed (P5).
//
// Only lines in the mark's column on its page are candidates. A mark right of every line's text in its
// column, or left of all of it (a page margin), keeps its x (XRef margin, P4); otherwise x follows the
// character under its left edge, or the line's start or end when the edge is outside this line's text.
func LinkMark(o domain.Object, r ChartRender) domain.Object {
	o.Anchor = nil
	o.PointsRenderHash = r.Hash
	o.Link = decideLink(o, r)
	return o
}

func fixedLink(r ChartRender) *domain.MarkLink {
	return &domain.MarkLink{State: domain.LinkFixed, Render: r.Hash}
}

func decideLink(o domain.Object, r ChartRender) *domain.MarkLink {
	if len(o.Points) == 0 {
		return fixedLink(r)
	}
	mx0, my0, mx1, my1 := markBox(o)
	cols := chartColumnCount(r.Source)
	col := columnOf((mx0+mx1)/2, cols)
	var cand []visLine
	for _, v := range r.visualLines() {
		if v.page == o.Page && v.col == col {
			cand = append(cand, v)
		}
	}
	if len(cand) == 0 {
		return fixedLink(r)
	}
	sort.SliceStable(cand, func(i, j int) bool { return cand[i].top < cand[j].top })

	const eps = 1e-9
	ref, runner, inside := -1, -1, -1
	tall := my1-my0 > eps
	if tall {
		for i, v := range cand { // 1. entirely covered
			if v.top >= my0-eps && v.bot <= my1+eps {
				ref = i
				break
			}
		}
		if ref < 0 { // 2. centre covered
			for i, v := range cand {
				if c := v.centre(); c >= my0-eps && c <= my1+eps {
					ref = i
					break
				}
			}
		}
		if ref >= 0 { // the bottommost covered line below the reference: what P12 watches
			for i := len(cand) - 1; i > ref; i-- {
				if c := cand[i].centre(); c >= my0-eps && c <= my1+eps {
					inside = i
					break
				}
			}
		}
	}
	if ref < 0 { // 3. the mark's centre inside a line's box
		cy := (my0 + my1) / 2
		var in []int
		for i, v := range cand {
			if cy >= v.top-eps && cy <= v.bot+eps {
				in = append(in, i)
			}
		}
		if len(in) > 0 {
			// A tie goes to the line above (P3) — except for a text label, whose point is its TOP-left: the
			// glyphs hang below it, so in the zone where two boxes overlap it belongs to the line below.
			ref = in[0]
			if o.Type == domain.TypeText {
				ref = in[len(in)-1]
			}
			for _, i := range in {
				d, best := math.Abs(cy-cand[i].centre()), math.Abs(cy-cand[ref].centre())
				if d < best-linkTieLH*cand[ref].lh {
					ref = i
				}
			}
			for _, i := range in {
				if i != ref {
					runner = i
					break
				}
			}
		}
	}
	if ref < 0 { // 4. nearest by vertical edge gap
		gap := func(v visLine) float64 {
			switch {
			case my0 > v.bot:
				return my0 - v.bot
			case my1 < v.top:
				return v.top - my1
			}
			return 0
		}
		for i, v := range cand {
			if gap(v) > linkMaxGapLH*v.lh {
				continue
			}
			if ref < 0 {
				ref = i
				continue
			}
			// cand is sorted top-down, so ref is above i: i only wins if it is clearly nearer.
			if gap(v) < gap(cand[ref])-linkTieLH*cand[ref].lh {
				ref = i
			}
		}
		if ref < 0 {
			return fixedLink(r)
		}
		for i, v := range cand {
			if i != ref && gap(v) <= linkMaxGapLH*v.lh && gap(v)-gap(cand[ref]) <= linkRunnerUpLH*cand[ref].lh {
				if runner < 0 || gap(v) < gap(cand[runner]) {
					runner = i
				}
			}
		}
	}
	v := cand[ref]
	l := &domain.MarkLink{State: domain.LinkLinked, Line: v.id, Wrap: v.wrap, LH: v.lh, Render: r.Hash, Cols: cols}

	// The horizontal reference.
	colX0, colX1 := math.MaxFloat64, -math.MaxFloat64
	for _, c := range cand {
		colX0, colX1 = math.Min(colX0, c.x0), math.Max(colX1, c.x1)
	}
	refX := 0.0
	switch {
	case mx0 >= colX1-eps || mx1 <= colX0+eps:
		l.XRef = xrefMargin
	case mx0 < v.x0:
		l.XRef, refX = xrefStart, v.x0
	case mx0 >= v.x1:
		l.XRef, refX = xrefEnd, v.x1
	default:
		c, x := charAt(v, mx0)
		l.XRef, refX, l.Char = xrefChar, x, intPtr(c)
	}
	// The characters under the mark (P19): wherever it overlaps its line's text horizontally. A text mark's
	// extent is its first character (the server cannot measure the label's glyphs).
	if l.XRef != xrefMargin && mx1 >= v.x0 && mx0 < v.x1 {
		from, _ := charAt(v, math.Max(mx0, v.x0))
		to := from + 1
		if o.Type != domain.TypeText {
			to = charEndAt(v, math.Min(mx1, v.x1))
		}
		if to <= from {
			to = from + 1
		}
		l.Span = &domain.CharSpan{From: from, To: to}
	}
	if l.XRef != xrefMargin && v.lh > 0 {
		l.DX = (mx0 - refX) / v.lh
	}
	if v.lh > 0 {
		l.DY = (my0 - v.top) / v.lh
	}
	for _, i := range []int{inside, runner} {
		if i < 0 {
			continue
		}
		why := "inside"
		if i == runner {
			why = "runner-up"
		}
		l.Peers = append(l.Peers, domain.LinkPeer{Line: cand[i].id, Wrap: cand[i].wrap, Rel: (cand[i].top - v.top) / v.lh, Why: why})
	}
	return l
}

func intPtr(v int) *int { return &v }

// charAt is the source-line rune offset of the character under x on visual line v, and that character's
// left edge. x between two runs picks the next run's first character.
func charAt(v visLine, x float64) (int, float64) {
	for _, a := range v.runs {
		n := runChars(a)
		if n == 0 {
			continue
		}
		if x < a.X0 {
			return a.SrcChar, a.X0
		}
		if x < a.X1 {
			k := int(math.Floor((x - a.X0) / cw(a)))
			if k >= n {
				k = n - 1
			}
			return a.SrcChar + k, a.X0 + float64(k)*cw(a)
		}
	}
	last := v.runs[len(v.runs)-1]
	n := runChars(last)
	return last.SrcChar + n - 1, last.X0 + float64(n-1)*cw(last)
}

// charEndAt is the offset just past the character under x (the right edge of a mark's extent), clamped to
// the visual line's text.
func charEndAt(v visLine, x float64) int {
	end := 0
	for _, a := range v.runs {
		n := runChars(a)
		if n == 0 {
			continue
		}
		if x <= a.X0 {
			if end == 0 {
				return a.SrcChar
			}
			return end
		}
		if x <= a.X1 {
			k := int(math.Ceil((x - a.X0) / cw(a)))
			if k > n {
				k = n
			}
			return a.SrcChar + k
		}
		end = a.SrcChar + n
	}
	return end
}

// projection is ProjectMark's working result: the served object and the link re-measured on the current
// render (what RebaseMark persists).
type projection struct {
	obj    domain.Object
	rebase *domain.MarkLink
}

// ProjectMark moves a LINKED mark onto r, the chart's current render (P8–P20, P22): the one rigid transform
// described at the top of this file, with the mark's state recomputed. It returns o unchanged when o has no
// new-style link (an old-style Anchor stays where it is stored), is fixed, or was measured on r already.
// A mark whose line is gone is returned at its stored coordinates with state lost (P20); its
// PointsRenderHash stays the old render. Pure: o is not mutated.
func ProjectMark(o domain.Object, r ChartRender) domain.Object {
	return project(o, r).obj
}

// ProjectMarks is ProjectMark over a slice (serve and bake), returning a new slice.
func ProjectMarks(objs []domain.Object, r ChartRender) []domain.Object {
	out := make([]domain.Object, len(objs))
	for i, o := range objs {
		out[i] = ProjectMark(o, r)
	}
	return out
}

func project(o domain.Object, r ChartRender) projection {
	out := o.Clone()
	l := o.Link
	if l == nil || l.State == domain.LinkFixed || l.Render == r.Hash || len(o.Points) == 0 {
		return projection{obj: out}
	}
	lost := func() projection {
		out.Link.State = domain.LinkLost
		return projection{obj: out}
	}
	loc, ok := LocateLine(r.Lines, r.Source, r.Anchors, l.Line, l.Render, r.Hash)
	if !ok {
		return lost()
	}
	all := r.visualLines()
	vls := linesOf(all, l.Line)
	if len(vls) == 0 {
		return lost() // the line is still there but draws nothing now (it became blank or a directive)
	}
	state := domain.LinkLinked
	if l.State == domain.LinkCheck {
		state = domain.LinkCheck // sticky: only a move or resize clears it (P21)
	}
	check := func() { state = domain.LinkCheck }
	if !loc.CharMapKnown {
		// The history cannot say how the line was edited between the mark's render and now: the mark still
		// follows, but a sideways shift is a guess, so it asks to be looked at.
		check()
	}

	base := atWrap(vls, l.Wrap) // the reference visual line for every vertical relation
	vl, refX := base, 0.0
	mapChar := func(c int) int {
		if loc.CharMapKnown {
			n, _ := loc.CharMap.Map(c)
			return n
		}
		return c
	}
	// P19: "check" only if a character under the mark changed, or text was inserted inside its extent —
	// never because of an edit elsewhere on the line, which only shifts it.
	var span *domain.CharSpan
	if sp := l.Span; sp != nil {
		nf := mapChar(sp.From)
		if loc.CharMapKnown {
			for k := sp.From; k < sp.To; k++ {
				if nk, e := loc.CharMap.Map(k); !e || nk-nf != k-sp.From {
					check()
					break
				}
			}
		}
		span = &domain.CharSpan{From: nf, To: mapChar(sp.To-1) + 1}
	}
	var char *int
	switch l.XRef {
	case xrefChar:
		c := 0
		if l.Char != nil {
			c = *l.Char
		}
		nc := mapChar(c)
		run, ok := runForChar(vls, nc)
		if !ok {
			return lost()
		}
		vl = atWrap(vls, run.SrcWrap)
		refX = run.X0 + float64(clampInt(nc-run.SrcChar, 0, runChars(run)))*cw(run)
		char = intPtr(nc)
	case xrefStart:
		refX = vl.x0
	case xrefEnd:
		refX = vl.x1
	case xrefMargin:
		if chartColumnCount(r.Source) != l.Cols && l.Cols != 0 {
			check() // P16: the column layout changed under a margin mark
		}
	}
	if l.LH <= 0 || vl.lh <= 0 {
		return lost()
	}
	k := vl.lh / l.LH
	ox0, oy0, _, _ := markBox(o)
	nx0 := ox0
	if l.XRef != xrefMargin {
		nx0 = refX + l.DX*vl.lh
	}
	ny0 := vl.top + l.DY*vl.lh
	for i, p := range o.Points {
		out.Points[i] = domain.Point{X: nx0 + (p.X-ox0)*k, Y: ny0 + (p.Y-oy0)*k, Pressure: p.Pressure}
	}
	if l.XRef == xrefMargin {
		// Decision 4 (T195 review): a margin stamp scaled up by a larger type size stays on the page — it
		// keeps its size and slides inward rather than being cut off at the paper's edge.
		bx0, _, bx1, _ := boundsOf(out.Points)
		dx := 0.0
		if bx1 > 1 {
			dx = 1 - bx1
		}
		if bx0+dx < 0 {
			dx = -bx0
		}
		for i := range out.Points {
			out.Points[i].X += dx
		}
	}
	if o.Type == domain.TypeText {
		out.Style.FontSize = o.Style.FontSize * k
	}
	out.Page = vl.page
	if !finiteMark(out) {
		// A link that cannot be followed sanely (a forged or corrupt one: lh near zero, offsets huge)
		// must never put NaN or Inf on the wire — one such mark would break the whole song's payload.
		out.Points = append([]domain.Point(nil), o.Points...)
		out.Style.FontSize = o.Style.FontSize
		out.Page = o.Page
		return lost()
	}

	// P12 / P22: the watched peers must keep their place relative to the reference line.
	var peers []domain.LinkPeer
	for _, p := range l.Peers {
		pv := linesOf(all, p.Line)
		if len(pv) == 0 {
			check()
			continue
		}
		pb := atWrap(pv, p.Wrap)
		rel := (pb.top - base.top) / base.lh
		if pb.page != base.page || math.Abs(rel-p.Rel) > linkSeparateLH {
			check()
		}
		peers = append(peers, domain.LinkPeer{Line: p.Line, Wrap: pb.wrap, Rel: rel, Why: p.Why})
	}
	if predatesRederive(r.Lines, l.Render) {
		check() // T194: the file's ids were re-derived since this mark was linked (an import, a mismatch)
	}

	out.PointsRenderHash = r.Hash
	out.Link.State = state
	rb := l.Clone()
	rb.State, rb.Wrap, rb.LH, rb.Render, rb.Peers = state, vl.wrap, vl.lh, r.Hash, peers
	rb.Cols = chartColumnCount(r.Source)
	rb.Span = span
	if l.XRef == xrefChar {
		rb.Char = char
	} else {
		rb.Wrap = base.wrap
	}
	return projection{obj: out, rebase: rb}
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// finiteMark reports whether every coordinate of o (and its font size) is a finite number.
func finiteMark(o domain.Object) bool {
	if !finite(o.Style.FontSize) {
		return false
	}
	for _, p := range o.Points {
		if !finite(p.X) || !finite(p.Y) {
			return false
		}
	}
	return true
}

// The bounds a believable link stays within (ValidLink). A line-height is a fraction of the page height:
// the renderer's smallest type is far above 0.002 and its largest title far below 0.2. An offset of 50
// line-heights is past any linking reach (3 lh) plus any mark's own size.
const (
	linkMinLH     = 0.002
	linkMaxLH     = 0.2
	linkMaxOffset = 50.0
)

// ValidLink reports whether a link that came from OUTSIDE the server (an import, a band folder) is one
// the server could have written: a known state, a line id unless fixed, finite numbers within sane bands,
// sane character offsets. Both import paths drop an invalid link and re-link the mark by the rule (P31)
// instead of keeping it (T195 review F2: a forged lh of 5e-324 made k infinite and broke GET for the song).
func ValidLink(l *domain.MarkLink) bool {
	if l == nil {
		return true
	}
	switch l.State {
	case domain.LinkLinked, domain.LinkCheck, domain.LinkLost:
		if l.Line == "" {
			return false
		}
	case domain.LinkFixed:
		return true
	default:
		return false
	}
	switch l.XRef {
	case xrefChar, xrefStart, xrefEnd, xrefMargin:
	default:
		return false
	}
	for _, v := range []float64{l.DX, l.DY, l.LH} {
		if !finite(v) {
			return false
		}
	}
	if l.LH < linkMinLH || l.LH > linkMaxLH || math.Abs(l.DX) > linkMaxOffset || math.Abs(l.DY) > linkMaxOffset {
		return false
	}
	if l.Wrap < 0 || l.Cols < 0 || (l.Char != nil && *l.Char < 0) || (l.XRef == xrefChar && l.Char == nil) {
		return false
	}
	if sp := l.Span; sp != nil && (sp.From < 0 || sp.To < sp.From) {
		return false
	}
	for _, p := range l.Peers {
		if p.Line == "" || p.Wrap < 0 || !finite(p.Rel) || math.Abs(p.Rel) > linkMaxOffset {
			return false
		}
	}
	return true
}

// RebaseOnRender is the EAGER half of the projection (T195 review F3): what a chart save stores for one
// mark. It returns the mark moved onto r with its link re-measured there (Render = r.Hash, the character
// offsets mapped, the state persisted), and whether that differs from o. A lost mark keeps its stored
// coordinates and link, with its state set to lost. Re-basing on every save keeps a live link's history
// one step deep, so it never outlives the bounded LineIdentity history.
func RebaseOnRender(o domain.Object, r ChartRender) (domain.Object, bool) {
	p := project(o, r)
	if p.rebase == nil {
		if p.obj.Link != nil && o.Link != nil && p.obj.Link.State == domain.LinkLost && o.Link.State != domain.LinkLost {
			out := o.Clone()
			out.Link.State = domain.LinkLost
			return out, true
		}
		return o, false
	}
	out := p.obj
	out.Link = p.rebase
	return out, true
}

// runForChar finds the run of a line (any visual line) that draws rune offset c; c past the end of the
// line resolves to the end of its last run, and c in a gap between runs to the next run.
func runForChar(vls []visLine, c int) (Anchor, bool) {
	var runs []Anchor
	for _, v := range vls {
		runs = append(runs, v.runs...)
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].SrcChar < runs[j].SrcChar })
	for _, a := range runs {
		if c < a.SrcChar {
			return a, true // a gap (a bold marker, a wrap's dropped space): the next run's first character
		}
		if c < a.SrcChar+runChars(a) {
			return a, true
		}
	}
	if len(runs) == 0 {
		return Anchor{}, false
	}
	return runs[len(runs)-1], true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// predatesRederive reports whether a link measured on linkRender predates the file's last line-id
// re-derivation (T194's Rederived flag): such a mark is "check". Walking the history newest first, meeting
// linkRender first means the mark was linked at or after the re-derivation. The flag is never cleared by
// stage 2 — the state is computed per mark, so the flag must stay to answer for the marks not yet served.
func predatesRederive(li LineIdentity, linkRender string) bool {
	if li.Rederived == nil {
		return false
	}
	if li.Rederived.Render == linkRender {
		return false
	}
	for k := len(li.History) - 1; k >= 0; k-- {
		st := li.History[k]
		if st.ToRender == linkRender {
			return false
		}
		if st.ToRender == li.Rederived.Render || st.FromRender == li.Rederived.Render {
			return true
		}
	}
	return true
}

// RelinkMoved is what a MOVE, RESIZE or RESTORE stores (P23, P24, P27). moved is the client's object (its
// new points, and in PointsRenderHash the render the client was looking at); stored is the server's copy
// (HEAD). r is the chart's current render, nil when there is none (an uploaded file, or a render that does
// not reproduce the stored PDF, P32).
//   - The client saw the current render: re-link from the new position, exactly as at create (LinkMark).
//     A lost or check mark moved onto a line leaves that state (P21).
//   - It saw an older render (a stale tab), or no current render exists: keep the previous link (P27) and
//     shift its offset by the move, so the next serve puts the mark where it was dropped relative to its
//     line; if the client's render is not even the link's own, the shift is a guess and the mark is check.
//
// The client's own link and anchor are never read (P26).
func RelinkMoved(moved, stored domain.Object, r *ChartRender) domain.Object {
	view := moved.PointsRenderHash
	if r != nil && (view == "" || view == r.Hash) {
		return LinkMark(moved, *r)
	}
	// T195 review F1: a mark that does NOT move between renders (no stage-2 link, fixed, or lost) is
	// served at its stored coordinates, so whatever render the client says it saw, the points it sends are
	// the page coordinates the user dragged to: re-link them as current. Studio echoes the served object's
	// pointsRenderHash on a move, which for such a mark used to be its stored (old) one — read as a stale
	// tab, the move never re-linked.
	if r != nil && view == stored.PointsRenderHash && !projectable(stored, *r) {
		return LinkMark(moved, *r)
	}
	out := moved
	out.Anchor = stored.Anchor.Clone()
	out.Link = stored.Link.Clone()
	out.PointsRenderHash = stored.PointsRenderHash
	if l := out.Link; l != nil && l.State != domain.LinkFixed && l.LH > 0 && len(stored.Points) > 0 && len(moved.Points) > 0 {
		sx, sy, _, _ := markBox(stored)
		nx, ny, _, _ := markBox(moved)
		l.DX += (nx - sx) / l.LH
		l.DY += (ny - sy) / l.LH
		if view != l.Render {
			l.State = domain.LinkCheck
		}
	}
	return out
}

// projectable reports whether serve moves stored onto r: it has a live stage-2 link whose line still draws.
func projectable(stored domain.Object, r ChartRender) bool {
	l := stored.Link
	if l == nil || l.State == domain.LinkFixed || l.State == domain.LinkLost {
		return false
	}
	p := project(stored, r)
	return p.obj.Link == nil || p.obj.Link.State != domain.LinkLost
}

// RebaseMark is what a SETSTYLE, SETTEXT or REORDER stores (P25, P26): edited is the client's object, of
// which only the non-geometric fields (style, text, jump target, order) are taken. The link is the STORED
// one and the mark does not move: when the stored mark is projected on the current render (r), the
// projected position is what the client saw and edited, so that position is stored, with the link
// re-measured against the SAME line on r and its state persisted. Otherwise the stored geometry is kept
// as is.
func RebaseMark(edited, stored domain.Object, r *ChartRender) domain.Object {
	out := edited
	out.Anchor = stored.Anchor.Clone()
	out.Link = stored.Link.Clone()
	out.PointsRenderHash = stored.PointsRenderHash
	out.Points = append([]domain.Point(nil), stored.Points...)
	out.Page = stored.Page
	if r == nil || stored.Link == nil {
		return out
	}
	p := project(stored, *r)
	if p.rebase == nil {
		return out // fixed, already current, or lost: stored geometry is what was served
	}
	out.Points = p.obj.Points
	out.Page = p.obj.Page
	out.PointsRenderHash = r.Hash
	out.Link = p.rebase
	return out
}
