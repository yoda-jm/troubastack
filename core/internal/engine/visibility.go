package engine

import (
	"sort"

	"troubastack/core/internal/domain"
)

// VisibleStack computes the per-viewer ordered layer stack (design/01, R7) — a PURE,
// derived presentation concern, NOT shared state. The order is zone-major:
//
//	PDF (implicit, below all) < CONDUCTOR < SHARED < PERSONAL
//
// and within PERSONAL the VIEWER's own layers float ABOVE other members' layers.
// Within an (zone, owner) bucket, layers are ordered by Layer.Order.
//
// Default visibility (the band is the privacy boundary, R2 — all layers are received;
// this only picks the DEFAULT on/off):
//   - shared (owner == _shared_) OR mandatory → ON for everyone (mandatory can't be hidden);
//   - the viewer's OWN non-shared layers → ON by default;
//   - other members' non-shared OPTIONAL layers → OFF by default (toggleable);
//   - role_tag, if set and matching the viewer's role bucket, flips an otherwise-off
//     optional layer ON by default.
//
// VisibleStack returns only the layers that are ON by default for this viewer, in
// stacking order (bottom→top). Use FullStack for every layer + its default flag.
func VisibleStack(snap domain.Snapshot, viewerID string, role domain.Role) []domain.Layer {
	var out []domain.Layer
	for _, le := range FullStack(snap, viewerID, role) {
		if le.VisibleByDefault {
			out = append(out, le.Layer)
		}
	}
	return out
}

// LayerEntry pairs a layer with its computed default visibility for a viewer.
type LayerEntry struct {
	Layer            domain.Layer
	VisibleByDefault bool
	Mandatory        bool // convenience: viewer may NOT hide it when true
}

// FullStack returns EVERY layer in per-viewer stacking order with its default-on flag
// (design/01). The viewer keeps a local visibility set seeded from these defaults.
func FullStack(snap domain.Snapshot, viewerID string, role domain.Role) []LayerEntry {
	entries := make([]LayerEntry, 0, len(snap.Layers))
	for _, l := range snap.Layers {
		entries = append(entries, LayerEntry{
			Layer:            l,
			VisibleByDefault: defaultVisible(l, viewerID, role),
			Mandatory:        l.Mandatory,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return less(entries[i].Layer, entries[j].Layer, viewerID)
	})
	return entries
}

// defaultVisible applies the design/01 default-visibility rules for one layer/viewer.
func defaultVisible(l domain.Layer, viewerID string, role domain.Role) bool {
	if l.Mandatory {
		return true // mandatory always on; viewer cannot hide
	}
	if l.OwnerID == domain.SharedOwner {
		return true // shared → visible to everyone
	}
	if l.OwnerID == viewerID {
		return true // your own non-shared layers are on for you
	}
	// Other members' optional layers: off by default, unless role_tag targets this viewer.
	if l.RoleTag != "" && roleMatches(l.RoleTag, role) {
		return true
	}
	return false
}

// roleMatches maps a layer's role_tag to the viewer's role for default visibility.
// Conductor/admin cue tags ("conductor", "admin") default-on for those roles.
func roleMatches(tag string, role domain.Role) bool {
	switch tag {
	case "conductor":
		return role == domain.RoleConductor || role == domain.RoleAdmin
	case "admin":
		return role == domain.RoleAdmin
	default:
		return false
	}
}

// zoneRank gives the base z-band order: PDF < Conductor < Shared < Personal.
func zoneRank(z domain.Zone) int {
	switch z {
	case domain.ZoneConductor:
		return 1
	case domain.ZoneShared:
		return 2
	case domain.ZonePersonal:
		return 3
	default:
		return 0
	}
}

// less orders layers bottom→top for a given viewer.
func less(a, b domain.Layer, viewerID string) bool {
	ra, rb := zoneRank(a.Zone), zoneRank(b.Zone)
	if ra != rb {
		return ra < rb
	}
	// In PERSONAL, the viewer's own layers float ABOVE other members'.
	if a.Zone == domain.ZonePersonal {
		aMine := a.OwnerID == viewerID
		bMine := b.OwnerID == viewerID
		if aMine != bMine {
			return !aMine // mine sorts later (higher = on top)
		}
	}
	// Same zone & ownership bucket: owner-controlled Order, then stable by ID.
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.ID < b.ID
}
