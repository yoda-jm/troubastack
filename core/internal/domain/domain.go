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

// ObjectType is the kind of annotation object. AUTHORITY: proto/troubastack/v1/
// object.proto ObjectType — keep this set in sync with it (and the TS union in
// web/studio/src/api.ts): freehand, line, rect, ellipse, text, highlight.
type ObjectType int

const (
	TypeUnspecified ObjectType = iota
	TypeFreehand               // the only type the native wet layer renders (I9)
	TypeLine
	TypeRect
	TypeEllipse
	TypeText
	TypeHighlight
	TypeIcon // T51: a tinted glyph stamp; the glyph id rides in Object.Text
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
	// KindReorder changes an object's z-order WITHIN its layer (T27). It is an
	// OBJECT kind (flows through applyObject / authorizeWrite like Move/Resize),
	// APPENDED here rather than grouped with the object kinds above so the existing
	// iota values (persisted as ints in the file/git logs) never shift.
	KindReorder
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
//
// Fill/Stroke/Blend extend rect/ellipse into the unified shape model (replacing the
// separate "highlight" type): Fill paints the interior with Color@Opacity, Stroke
// draws the border with Color+Width, and Blend "multiply" composites like a marker.
// They are pointers so an ABSENT value (nil) is distinguishable from an explicit
// false — letting the renderer infer legacy defaults for objects seeded before the
// flags existed (legacy highlight → fill+multiply; legacy rect/ellipse → stroke).
type Style struct {
	Color    string
	Opacity  float64
	Width    float64
	FontSize float64
	Fill     *bool  // paint interior (rect/ellipse); nil = infer from type
	Stroke   *bool  // draw border (rect/ellipse); nil = infer from type
	Blend    string // "" | "normal" | "multiply"
}

// SourceAnchor pins an annotation to the SOURCE text it was drawn on, not to one render's coordinates
// (T145). RunText is the drawn run's text; Occurrence is the 1-based Nth run with that text in the SOURCE
// (document-wide, never per page — a page index is a render property, so per-page counting re-breaks on
// reflow); CharStart/CharEnd are the rune span within the run the mark covers. It is projected to render
// coordinates (Points) at draw/bake time and survives a reflow, so a mark stays on its words. The
// projection lives in chartpdf (which imports this package); domain stays a pure model.
type SourceAnchor struct {
	RunText    string
	Occurrence int // 1-based, document-wide (source order)
	CharStart  int // rune index within the run
	CharEnd    int
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
	// Order is the z-order WITHIN this object's layer (T27). Rendered ascending;
	// ties fall back to insertion/creation order. Default 0 keeps legacy objects
	// in their original order. Set via KindReorder (bring-to-front / send-to-back).
	Order int
	// T145: Anchor is the SOURCE-scoped position of this mark. When set, Points/Page are a PROJECTED
	// CACHE of it for ONE render, and PointsRenderHash names that render (the generated chart's content
	// hash) — so a consumer can see the cache is stale after a re-render and re-project from Anchor
	// instead of reading orphaned coordinates. nil Anchor / empty hash = Points are authoritative (an
	// uploaded PDF has no source; or a mark that predates T145 / could not be anchored).
	Anchor           *SourceAnchor
	PointsRenderHash string
}

// Clone returns a deep copy so callers cannot mutate stored state through aliases.
func (o Object) Clone() Object {
	cp := o
	if o.Points != nil {
		cp.Points = make([]Point, len(o.Points))
		copy(cp.Points, o.Points)
	}
	if o.Anchor != nil { // deep-copy so callers cannot mutate stored state through the pointer
		a := *o.Anchor
		cp.Anchor = &a
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
