package bake

import "troubastack/core/internal/domain"

// The web/bake CLI input contract (its request.json). This mirrors the annotation
// API's wire shape (core/internal/httpapi/annotations.go) and @troubastack/ink's
// InkObject — but is defined HERE because bake's boundary forbids importing httpapi
// (doc.go). AUTHORITY for the object/style/layer field set: proto + that API.

type docPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type docStyle struct {
	Color    string  `json:"color"`
	Opacity  float64 `json:"opacity"`
	Width    float64 `json:"width"`
	FontSize float64 `json:"fontSize"`
	Fill     *bool   `json:"fill,omitempty"`
	Stroke   *bool   `json:"stroke,omitempty"`
	Blend    string  `json:"blend,omitempty"`
}

type docObject struct {
	UUID    string     `json:"uuid"`
	LayerID string     `json:"layerId"`
	Type    string     `json:"type"`
	Points  []docPoint `json:"points"`
	Page    int        `json:"page"`
	Text    string     `json:"text"`
	Style   docStyle   `json:"style"`
}

// docLayer carries only what the renderer + manifest need (z-order + role flags).
type docLayer struct {
	ID        string `json:"id"`
	Order     int    `json:"order"`
	Mandatory bool   `json:"mandatory"`
	RoleTag   string `json:"roleTag"`
}

type annotationsDoc struct {
	Layers  []docLayer  `json:"layers"`
	Objects []docObject `json:"objects"`
}

// pageSize is one source page's pixel size for the CLI (index + w×h).
type pageSize struct {
	Index  int `json:"index"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// cliRequest is the exact JSON the troubabake CLI reads via --in.
type cliRequest struct {
	Doc          annotationsDoc `json:"doc"`
	Pages        []pageSize     `json:"pages"`
	OverlayWidth int            `json:"overlayWidth"`
}

// snapshotToDoc maps a materialized annotation snapshot to the renderer doc, scoped
// to the file being baked. Only LIVE objects are drawn (tombstones excluded) — same
// as the dry layer studio and the annotation API expose.
//
// fileID SCOPING (B11/T40): a multi-file song carries per-file layers (e.g. House of
// the Rising Sun's guitar-tab layers vs its Drums-part layer). The bake rasterizes ONE
// file (the default part), so only that file's layers may composite onto it — otherwise
// another part's ink (drum notes) bleeds onto the baked page. A layer with an empty
// FileID is song-level (legacy/single-file) and composites onto whatever is baked.
func snapshotToDoc(snap domain.Snapshot, fileID string) annotationsDoc {
	doc := annotationsDoc{Layers: []docLayer{}, Objects: []docObject{}}
	keep := map[string]bool{}
	for _, l := range snap.Layers {
		if l.FileID != "" && l.FileID != fileID {
			continue // another file's part — don't bake it onto this page
		}
		keep[l.ID] = true
		doc.Layers = append(doc.Layers, docLayer{
			ID:        l.ID,
			Order:     l.Order,
			Mandatory: l.Mandatory,
			RoleTag:   l.RoleTag,
		})
	}
	for _, o := range snap.LiveObjects() {
		if !keep[o.LayerID] {
			continue // object belongs to a layer scoped to another file
		}
		pts := make([]docPoint, len(o.Points))
		for i, p := range o.Points {
			pts[i] = docPoint{X: p.X, Y: p.Y}
		}
		doc.Objects = append(doc.Objects, docObject{
			UUID:    o.UUID,
			LayerID: o.LayerID,
			Type:    objectTypeString(o.Type),
			Points:  pts,
			Page:    o.Page,
			Text:    o.Text,
			Style: docStyle{
				Color:    o.Style.Color,
				Opacity:  o.Style.Opacity,
				Width:    o.Style.Width,
				FontSize: o.Style.FontSize,
				Fill:     o.Style.Fill,
				Stroke:   o.Style.Stroke,
				Blend:    o.Style.Blend,
			},
		})
	}
	return doc
}

// objectTypeString mirrors httpapi's objectTypeToString (kept in sync by review).
// AUTHORITY: proto/troubastack/v1/object.proto ObjectType.
func objectTypeString(t domain.ObjectType) string {
	switch t {
	case domain.TypeFreehand:
		return "freehand"
	case domain.TypeRect:
		return "rect"
	case domain.TypeEllipse:
		return "ellipse"
	case domain.TypeLine:
		return "line"
	case domain.TypeText:
		return "text"
	case domain.TypeHighlight:
		return "highlight"
	case domain.TypeIcon:
		return "icon"
	default:
		return ""
	}
}
