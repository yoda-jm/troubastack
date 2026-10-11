package chartpdf

import (
	"sort"
	"strings"
	"unicode"

	"troubastack/core/internal/domain"
)

// T196 — anchoring stage 3: what Studio needs to SHOW a mark's link (docs/tasks/T196-anchoring-stage3-link-states.md).
//
// Studio draws a dot on the linked text and a thin line from it to the mark. The client has no anchor
// manifest, so the server says where the linked text is on the CURRENT render (LinkView.At) and what the
// line says (LinkView.Text). Both come out of the same projection pass that serves the mark (ServeMark):
// no extra render, and nothing is stored.

// ServeMark is ProjectMark plus the serve-time LinkView of the served mark (o.View): set for a Linked or
// Check mark whose line draws on r, nil otherwise (lost, fixed, old-style, unlinked). The serve paths (REST
// GET, the realtime join snapshot) and the realtime echo use it; the bake does not (P34: nothing about
// links reaches the tablet).
func ServeMark(o domain.Object, r ChartRender) domain.Object {
	p := project(o, r)
	out := p.obj
	out.View = nil
	l := p.rebase // the link re-measured on r, when the mark was projected
	all := p.all
	if l == nil && o.Link != nil && o.Link.Render == r.Hash {
		l = o.Link // already measured on r: its own link describes the current render
	}
	if l == nil || out.Link == nil {
		return out
	}
	if all == nil {
		all = r.visualLines()
	}
	out.View = linkView(out, l, out.Link.State, r, all)
	return out
}

// ServeMarks is ServeMark over a slice, returning a new slice.
func ServeMarks(objs []domain.Object, r ChartRender) []domain.Object {
	out := make([]domain.Object, len(objs))
	for i, o := range objs {
		out[i] = ServeMark(o, r)
	}
	return out
}

// linkView is where a link measured on r points, and its line's text. state is the SERVED state (the
// projection may have turned a linked mark into check or lost). o is the served mark.
func linkView(o domain.Object, l *domain.MarkLink, state domain.LinkState, r ChartRender, all []visLine) *domain.LinkView {
	if state != domain.LinkLinked && state != domain.LinkCheck {
		return nil
	}
	vls := linesOf(all, l.Line)
	if len(vls) == 0 {
		return nil
	}
	var v visLine
	var x float64
	switch l.XRef {
	case xrefChar:
		c := 0
		if l.Char != nil {
			c = *l.Char
		}
		run, ok := runForChar(vls, c)
		if !ok {
			return nil
		}
		v = atWrap(vls, run.SrcWrap)
		x = run.X0 + float64(clampInt(c-run.SrcChar, 0, runChars(run)))*cw(run)
	case xrefStart:
		v = atWrap(vls, l.Wrap)
		x = v.x0
	case xrefEnd:
		v = atWrap(vls, l.Wrap)
		x = v.x1
	default: // margin: no character is under the mark; point at the end of the line nearer to it
		v = atWrap(vls, l.Wrap)
		bx0, _, bx1, _ := markBox(o)
		x = v.x1
		if (bx0+bx1)/2 < (v.x0+v.x1)/2 {
			x = v.x0
		}
	}
	return &domain.LinkView{
		At:   domain.LinkAt{Page: v.page, X: x, Y: v.top, H: v.lh},
		Text: lineText(vls, r.Source),
	}
}

// lineText is the text a source line DRAWS, across all its visual lines and runs, in source order. Runs
// are split by **bold** markers, wraps and chord-row notes; between two runs a single space is kept when
// the source had whitespace there (a wrap drops it), and nothing otherwise (a bold marker).
func lineText(vls []visLine, source string) string {
	var runs []Anchor
	for _, v := range vls {
		runs = append(runs, v.runs...)
	}
	if len(runs) == 0 {
		return ""
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].SrcChar < runs[j].SrcChar })
	var src []rune
	if lines := chartLines(source); runs[0].SrcLine >= 0 && runs[0].SrcLine < len(lines) {
		src = []rune(lines[runs[0].SrcLine])
	}
	var b strings.Builder
	end := -1
	for _, a := range runs {
		if end >= 0 && a.SrcChar > end && !endsInSpace(b.String()) && !strings.HasPrefix(a.Text, " ") {
			for k := end; k < a.SrcChar && k < len(src); k++ {
				if unicode.IsSpace(src[k]) {
					b.WriteByte(' ')
					break
				}
			}
		}
		b.WriteString(a.Text)
		end = a.SrcChar + runChars(a)
	}
	return strings.TrimSpace(b.String())
}

func endsInSpace(s string) bool { return s != "" && s[len(s)-1] == ' ' }
