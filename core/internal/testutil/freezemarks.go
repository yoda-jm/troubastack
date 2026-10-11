package testutil

import "troubastack/core/internal/domain"

// FreezeShapes names the 8 mark shapes of the anchoring matrix, in its column order.
var FreezeShapes = []string{"freehand", "line", "rect", "ellipse", "text", "highlight", "icon", "jump_pair"}

// FreezeChart is a synthetic generated chart (no real song) for the T193 freeze cells; FreezeTarget is the
// line the marks sit on. FreezeEdits are the two source edits the freeze must survive: a line inserted
// above the target, and a type-size change.
const (
	FreezeChart = "# The Open Road\n\n## Verse 1\nG            D\nPack a little light for the road ahead\n" +
		"Em           C\nLeave the porch light on when the sun goes down\n\n## Chorus\nC        G\nOh the open road\n"
	FreezeTarget = "Leave the porch light on when the sun goes down"
)

// FreezeEdits maps an edit name to the edited chart source.
var FreezeEdits = map[string]string{
	"insert_line_above": "# The Open Road\n\n## Verse 1\nHold the map up to the light\nG            D\n" +
		"Pack a little light for the road ahead\nEm           C\nLeave the porch light on when the sun goes down\n\n" +
		"## Chorus\nC        G\nOh the open road\n",
	"size_change": "# The Open Road\nsize: 16\n\n## Verse 1\nG            D\nPack a little light for the road ahead\n" +
		"Em           C\nLeave the porch light on when the sun goes down\n\n## Chorus\nC        G\nOh the open road\n",
}

// FreezeMarks returns one mark per shape (a jump pair is two icons) drawn on the line box (x0,y0)-(x1,y1)
// of page `page`, on layer layerID, keyed by shape: realistic geometry, so an old-style projection would
// visibly move or reshape every one of them.
func FreezeMarks(layerID string, page int, x0, y0, x1, y1 float64) map[string][]domain.Object {
	h, w := y1-y0, x1-x0
	at := func(fx, fy float64) domain.Point { return domain.Point{X: x0 + fx*w, Y: y0 + fy*h} }
	style := domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.003, FontSize: 0.02}
	mk := func(uuid string, typ domain.ObjectType, text string, pts ...domain.Point) domain.Object {
		return domain.Object{UUID: uuid, LayerID: layerID, Type: typ, Page: page, Version: 1, Text: text,
			Points: pts, Style: style}
	}
	src := mk("jump-src", domain.TypeIcon, "star", at(0.70, -0.3), at(0.74, 1.3))
	src.JumpTo = "jump-dst"
	return map[string][]domain.Object{
		"freehand": {mk("freehand", domain.TypeFreehand, "",
			at(0.30, -0.2), at(0.40, 0.5), at(0.50, 1.2), at(0.45, 0.3), at(0.35, 0.9))},
		"line":      {mk("line", domain.TypeLine, "", at(0.80, 0.6), at(0.10, 0.6))},
		"rect":      {mk("rect", domain.TypeRect, "", at(-0.02, -0.4), at(1.02, 1.4))},
		"ellipse":   {mk("ellipse", domain.TypeEllipse, "", at(0.20, -0.5), at(0.40, 1.5))},
		"text":      {mk("text", domain.TypeText, "softly", at(0.55, 0.5))},
		"highlight": {mk("highlight", domain.TypeHighlight, "", at(0.10, -0.5), at(0.50, 1.5))},
		"icon":      {mk("icon", domain.TypeIcon, "star", at(0.60, -0.3), at(0.64, 1.3))},
		"jump_pair": {src, mk("jump-dst", domain.TypeIcon, "star", at(0.90, -0.3), at(0.94, 1.3))},
	}
}
