// Package domain holds the pure data model and resolution helpers: objects with
// client-generated UUIDs, ordered layers, linear append-only revision history,
// setlist pins, per-object last-write-wins, and terminal tombstones.
//
// Invariants served: I2 (idempotent objects by UUID), I3 (coordinates are
// PDF-relative [0,1]), I4 (linear append-only history; revert = new appended
// head), I5 (LWW + tombstone-wins), I7 (the reference set GC must preserve).
//
// Boundary: pure types + logic. MUST NOT import store, sync, session, bake or
// httpapi (no I/O, no transport, no UI) so it stays trivially testable.
//
// These are hand-written domain types for the v1 spike. The single-source-of-truth
// proto codegen (I1) is a later step; the structs mirror proto/troubastack/v1.
package domain

// ----- enums (typed Go constants mirroring the proto enums) -----

// ObjectType is the kind of annotation object (proto ObjectType).
type ObjectType int

const (
	TypeUnspecified ObjectType = iota
	TypeFreehand               // the only type the native wet layer renders (I9)
	TypeLine
	TypeRect
	TypeEllipse
	TypeText
	TypeHighlight
)

// Scope is who may see an object (proto Scope). Largely subsumed by layer role_tag.
type Scope int

const (
	ScopeUnspecified Scope = iota
	ScopePersonal
	ScopePart
	ScopeAll
)

// Zone is the fixed Z-band a layer lives in (proto LayerZone). The per-viewer
// stack is PDF < Conductor < Shared < Personal (own-personal floats on top).
type Zone int

const (
	ZoneUnspecified Zone = iota
	ZoneConductor
	ZoneShared
	ZonePersonal
)

// Access controls who may add/edit objects in a layer (proto Access).
type Access int

const (
	AccessUnspecified Access = iota
	AccessRW                 // any band member may add/edit objects
	AccessRO                 // only the owner may add/edit; others read
)

// Kind is a mutation kind (proto Mutation.Kind) plus the layer ops that flow
// through the same per-action mutation + commit model (design/01).
type Kind int

const (
	KindUnspecified Kind = iota
	KindCreate
	KindMove
	KindResize
	KindSetStyle
	KindSetText
	KindDelete  // terminal tombstone (I5)
	KindRestore // the ONLY revive (I5)
	KindLayerCreate
	KindLayerUpdate
	KindLayerReorder
	KindLayerDelete
)

// SharedOwner is the synthetic owner id for a band-shared layer.
const SharedOwner = "_shared_"

// Role is a viewer's role, which seeds default layer visibility (design/01).
type Role int

const (
	RoleMember Role = iota
	RoleConductor
	RoleAdmin
)

// ----- value types -----

// Point is a coordinate in PDF-relative [0,1] (I3). Pressure is [0,1], 0=unknown.
type Point struct {
	X, Y, Pressure float64
}

// Style is the visual style of an object. Color is "#RRGGBB". Width is a fraction
// of page width and FontSize a fraction of page height (text only), both [0,1] (I3).
type Style struct {
	Color    string
	Opacity  float64
	Width    float64
	FontSize float64
}

// Object is an annotation identified by a client-generated UUID (I2). Applying the
// same UUID twice is idempotent (no-op or in-place replace, never a duplicate).
type Object struct {
	UUID      string
	Type      ObjectType
	Points    []Point
	Page      int // 0-based page index this object is on
	Text      string
	Style     Style
	OwnerID   string
	Scope     Scope
	LayerID   string
	Version   uint64 // for LWW (I5)
	CreatedAt int64  // unix ms (author-stamped; server is tiebreak authority)
	Deleted   bool   // tombstone flag (I5); terminal until an explicit Restore
}

// Clone returns a deep copy so callers cannot mutate stored state through aliases.
func (o Object) Clone() Object {
	cp := o
	if o.Points != nil {
		cp.Points = make([]Point, len(o.Points))
		copy(cp.Points, o.Points)
	}
	return cp
}

// Layer stacks annotation objects above the PDF (design/01, R2/R7).
type Layer struct {
	ID        string
	FileID    string
	Name      string
	OwnerID   string // member UUID, or SharedOwner for a band-shared layer
	Zone      Zone
	Order     int    // ordering WITHIN the owner's layers in that zone
	Access    Access // who may add/edit objects; Delete is owner-only always
	Mandatory bool   // true = viewers cannot hide it (admin/conductor cues)
	RoleTag   string // optional target role/part for default visibility
}

// Mutation is one completed action (proto Mutation). Object carries the full object
// for Create; Layer carries the layer for the Layer* kinds.
type Mutation struct {
	Kind        Kind
	UUID        string  // target object (I2) — empty for layer ops
	Object      *Object // present for Create (and as needed)
	Layer       *Layer  // present for Layer* kinds
	BaseVersion uint64  // client's known version, for LWW (I5)
	AuthorID    string  // the ACTOR (may differ from object owner)
	Seq         uint64  // server-assigned total order; 0 until accepted
	Checkpoint  bool    // tag a notable milestone in history
	Summary     string  // human-readable; with git this IS the commit message
	ClientTS    int64   // unix ms (client clock)
}

// Clone deep-copies a mutation so the store and engine never alias each other.
func (m Mutation) Clone() Mutation {
	cp := m
	if m.Object != nil {
		o := m.Object.Clone()
		cp.Object = &o
	}
	if m.Layer != nil {
		l := *m.Layer
		cp.Layer = &l
	}
	return cp
}

// Revision is one entry in a song's single linear history (I4). Parent is the
// immediately preceding revision number (0 for the root).
type Revision struct {
	Number     uint64
	Parent     uint64
	AuthorID   string
	CreatedAt  int64
	Summary    string
	IsRevert   bool
	RevertedTo uint64 // if IsRevert: the revision number whose content this equals
}

// Pin is a named reference onto the revision line (setlist entry, etc.). A pinned
// revision is part of the live reference set and is immortal to GC (I7).
type Pin struct {
	SongID         string
	Name           string
	RevisionNumber uint64
}

// Song has one linear, append-only history; Head is the latest revision number.
type Song struct {
	ID      string
	GroupID string
	Title   string
	Head    uint64
}

// Snapshot is a materialized view of a song at some revision: the live object set
// plus the layer set. It is what Head/SnapshotAt return.
type Snapshot struct {
	Revision uint64
	Objects  []Object // includes tombstones (Deleted=true) so revert is reconstructable
	Layers   []Layer
}

// LiveObjects returns only non-tombstoned objects (the "live" set per design/01).
func (s Snapshot) LiveObjects() []Object {
	out := make([]Object, 0, len(s.Objects))
	for _, o := range s.Objects {
		if !o.Deleted {
			out = append(out, o.Clone())
		}
	}
	return out
}
