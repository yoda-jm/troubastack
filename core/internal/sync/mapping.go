package sync

import "troubastack/core/internal/domain"

// This file is the wire<->domain mapping for the realtime protocol. The Object/Layer
// shapes and enum strings are IDENTICAL to the annotations REST API
// (internal/httpapi/annotations.go) so a frontend speaks one vocabulary on both.

// ---- mutation kind <-> string ----

// kindFromString maps the lowerCamel wire kind to the engine kind.
func kindFromString(s string) (domain.Kind, bool) {
	switch s {
	case "create":
		return domain.KindCreate, true
	case "move":
		return domain.KindMove, true
	case "resize":
		return domain.KindResize, true
	case "setStyle":
		return domain.KindSetStyle, true
	case "setText":
		return domain.KindSetText, true
	case "delete":
		return domain.KindDelete, true
	case "restore":
		return domain.KindRestore, true
	case "reorder":
		return domain.KindReorder, true
	case "layerCreate":
		return domain.KindLayerCreate, true
	case "layerUpdate":
		return domain.KindLayerUpdate, true
	case "layerDelete":
		return domain.KindLayerDelete, true
	default:
		return domain.KindUnspecified, false
	}
}

// kindToString is the inverse, for echoes.
func kindToString(k domain.Kind) string {
	switch k {
	case domain.KindCreate:
		return "create"
	case domain.KindMove:
		return "move"
	case domain.KindResize:
		return "resize"
	case domain.KindSetStyle:
		return "setStyle"
	case domain.KindSetText:
		return "setText"
	case domain.KindDelete:
		return "delete"
	case domain.KindRestore:
		return "restore"
	case domain.KindReorder:
		return "reorder"
	case domain.KindLayerCreate:
		return "layerCreate"
	case domain.KindLayerUpdate:
		return "layerUpdate"
	case domain.KindLayerReorder:
		return "layerUpdate"
	case domain.KindLayerDelete:
		return "layerDelete" // T83: broadcast a delete AS a delete, not an update (which would re-add it on peers)
	default:
		return ""
	}
}

// isLayerKind reports whether a kind targets a layer (no object uuid/version logic).
func isLayerKind(k domain.Kind) bool {
	switch k {
	case domain.KindLayerCreate, domain.KindLayerUpdate, domain.KindLayerReorder, domain.KindLayerDelete:
		return true
	}
	return false
}

// ---- mutation <-> wire ----

// mutationToJSON renders an accepted mutation for an echo frame (carries seq + authorId).
func mutationToJSON(m domain.Mutation) mutationJSON {
	out := mutationJSON{
		Kind:        kindToString(m.Kind),
		UUID:        m.UUID,
		BaseVersion: m.BaseVersion,
		ClientTS:    m.ClientTS,
		Summary:     m.Summary,
		Seq:         m.Seq,
		AuthorID:    m.AuthorID,
	}
	if m.Object != nil {
		oj := objectToJSON(*m.Object)
		out.Object = &oj
	}
	if m.Layer != nil {
		lj := layerToJSON(*m.Layer)
		out.Layer = &lj
	}
	return out
}

// ---- object / layer <-> wire ----

func layerToJSON(l domain.Layer) layerJSON {
	return layerJSON{
		ID:        l.ID,
		FileID:    l.FileID,
		Name:      l.Name,
		OwnerID:   l.OwnerID,
		Zone:      zoneToString(l.Zone),
		Order:     l.Order,
		Access:    accessToString(l.Access),
		Mandatory: l.Mandatory,
		RoleTag:   l.RoleTag,
	}
}

func layerFromJSON(j layerJSON) domain.Layer {
	return domain.Layer{
		ID:        j.ID,
		FileID:    j.FileID,
		Name:      j.Name,
		OwnerID:   j.OwnerID,
		Zone:      zoneFromString(j.Zone),
		Order:     j.Order,
		Access:    accessFromString(j.Access),
		Mandatory: j.Mandatory,
		RoleTag:   j.RoleTag,
	}
}

func objectToJSON(o domain.Object) objectJSON {
	pts := make([]pointJSON, len(o.Points))
	for i, p := range o.Points {
		pts[i] = pointJSON{X: p.X, Y: p.Y}
	}
	return objectJSON{
		UUID:      o.UUID,
		LayerID:   o.LayerID,
		Type:      domain.ObjectTypeToString(o.Type),
		Points:    pts,
		Page:      o.Page,
		Text:      o.Text,
		Order:     o.Order,
		CreatedAt: o.CreatedAt,
		Style: styleJSON{
			Color:    o.Style.Color,
			Opacity:  o.Style.Opacity,
			Width:    o.Style.Width,
			FontSize: o.Style.FontSize,
			Fill:     o.Style.Fill,
			Stroke:   o.Style.Stroke,
			Blend:    o.Style.Blend,
		},
		Anchor:           anchorToJSON(o.Anchor),
		PointsRenderHash: o.PointsRenderHash,
	}
}

func anchorToJSON(a *domain.SourceAnchor) *anchorJSON {
	if a == nil {
		return nil
	}
	return &anchorJSON{RunText: a.RunText, Occurrence: a.Occurrence, CharStart: a.CharStart, CharEnd: a.CharEnd}
}

func anchorFromJSON(a *anchorJSON) *domain.SourceAnchor {
	if a == nil {
		return nil
	}
	return &domain.SourceAnchor{RunText: a.RunText, Occurrence: a.Occurrence, CharStart: a.CharStart, CharEnd: a.CharEnd}
}

func objectFromJSON(j objectJSON) domain.Object {
	pts := make([]domain.Point, len(j.Points))
	for i, p := range j.Points {
		pts[i] = domain.Point{X: p.X, Y: p.Y}
	}
	return domain.Object{
		UUID:      j.UUID,
		LayerID:   j.LayerID,
		Type:      domain.ObjectTypeFromString(j.Type),
		Points:    pts,
		Page:      j.Page,
		Text:      j.Text,
		Order:     j.Order,
		CreatedAt: j.CreatedAt,
		Style: domain.Style{
			Color:    j.Style.Color,
			Opacity:  j.Style.Opacity,
			Width:    j.Style.Width,
			FontSize: j.Style.FontSize,
			Fill:     j.Style.Fill,
			Stroke:   j.Style.Stroke,
			Blend:    j.Style.Blend,
		},
		Anchor:           anchorFromJSON(j.Anchor),
		PointsRenderHash: j.PointsRenderHash,
	}
}

// ---- enum string maps (mirror annotations.go) ----

func zoneToString(z domain.Zone) string {
	switch z {
	case domain.ZoneConductor:
		return "conductor"
	case domain.ZoneShared:
		return "shared"
	case domain.ZonePersonal:
		return "personal"
	default:
		return ""
	}
}

func zoneFromString(s string) domain.Zone {
	switch s {
	case "conductor":
		return domain.ZoneConductor
	case "shared":
		return domain.ZoneShared
	case "personal":
		return domain.ZonePersonal
	default:
		return domain.ZoneUnspecified
	}
}

func accessToString(a domain.Access) string {
	switch a {
	case domain.AccessRO:
		return "ro"
	default:
		return "rw"
	}
}

func accessFromString(s string) domain.Access {
	switch s {
	case "ro":
		return domain.AccessRO
	default:
		return domain.AccessRW
	}
}
