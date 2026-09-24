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

import "encoding/json"

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

	// Dash (T177) is the line PATTERN: it applies to a line, and to the BORDER of a rect/ellipse
	// wherever Stroke is on. A small NAMED set, not a dash-array — a musician picks a look, and an open
	// numeric array is a migration surface and a place for the editor and the baker to differ (Fable,
	// T177 D2). "" means solid, which is exactly what every object drawn before T177 renders as.
	//
	// A plain string rather than a pointer, and that is not the T172 sentinel trap: the trap is an
	// IN-BAND value ("0 = none") inside a payload whose zero is legitimate. "" is not a member of this
	// set, so it discriminates without overloading anything — the same shape as Blend above.
	Dash string // "" | "solid" | "dashed" | "dotted"

	// Ends (T177; reshaped in T179) decorates a straight LINE's extremities — one shape per end. Absent =
	// an undecorated line, which is every line drawn before T177. Presence stays STRUCTURAL (T172's rule):
	// nil means both ends bare, and a present record names each end's shape.
	Ends *LineEnds
}

// LineEnds is the terminator at each END of a straight line — one shape per end (T179, replacing T177's
// {Head, Side}). "" and "none" both mean a bare end; the drawable set is "arrow" | "circle" | "square".
//
// Why one-shape-per-end and not a "which side?" axis: the natural way to draw a line is to finish toward
// the thing you are pointing at, so the common terminator is on the end you stopped at and the rare case
// is a different one at each end — which the old {Head, Side} could not express at all, and whose "side"
// asked the reader to answer a question the drawing gesture already answered (Fable, T179).
//
// An UNRECOGNISED shape renders NOTHING (T177's rule, kept): a newer client naming a shape this renderer
// lacks must not have it substituted by one we happen to have, because a wrong mark on a chart reads as a
// musical instruction.
//
// The geometry is NOT here — how big each shape is, whether its far edge or its centre sits at the
// endpoint (§4: the far edge, so nothing pokes past the line's end), and what it does on a line shorter
// than itself, are renderer facts pinned once in web/ink so the editor and the bake cannot differ.
type LineEnds struct {
	Start string // "" | "none" | "arrow" | "circle" | "square"
	End   string // same set; the end the line was drawn TOWARD, so the common terminator
}

// UnmarshalJSON reads the current {Start, End} AND T177's superseded {Head, Side}, so an arrow drawn in
// the hours T177 was live is TRANSLATED rather than silently lost by T179's rename (deliberate read-compat,
// the D17 lesson: old persisted data keeps being read). T177's Head was only ever "arrow" and Side placed
// it; an empty Head meant the default arrow, and an empty Side meant the far end. This is the only reader
// of a stored domain.LineEnds — the wire, .tband and bake DTOs carry their own {start,end} shape.
func (e *LineEnds) UnmarshalJSON(b []byte) error {
	var raw struct {
		Start, End string
		Head, Side string // T177
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	e.Start, e.End = raw.Start, raw.End
	// When neither CURRENT field is set, this is a T177 document — and it is read with T177's DOCUMENTED
	// DEFAULTS, where an empty Head IS an arrow and an empty Side IS the far end. So a bare `{}` means
	// "arrow at the end", NOT "no ends": the PRESENCE of the record drives the reading, never the emptiness
	// of its members (the T172 sentinel lesson — an in-band "" is "set, with a default" here, not "unset").
	// A new-shape writer never emits an all-empty record (it drops the whole field when both ends are none),
	// so this branch only ever sees a T177 or hand-edited document.
	if raw.Start == "" && raw.End == "" {
		shape := raw.Head
		if shape == "" {
			shape = "arrow"
		}
		switch raw.Side {
		case "start":
			e.Start = shape
		case "both":
			e.Start, e.End = shape, shape
		default: // "" or "end" — T177's default was the far end
			e.End = shape
		}
	}
	return nil
}

// Clone deep-copies a Style INCLUDING its pointer members, so a copy cannot be used to mutate stored
// state. Fill/Stroke have been aliased by every `cp := o` since they were added; nothing mutates through
// them today, which is exactly why it went unnoticed, and Ends makes the hole one member wider.
func (s Style) Clone() Style {
	cp := s
	if s.Fill != nil {
		f := *s.Fill
		cp.Fill = &f
	}
	if s.Stroke != nil {
		st := *s.Stroke
		cp.Stroke = &st
	}
	if s.Ends != nil {
		e := *s.Ends
		cp.Ends = &e
	}
	return cp
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

	// Offset (T172) records that the mark sits BESIDE its run rather than on it — an underline under a
	// line, a bracket above a section. Absent means "on the run", which is every anchor written before
	// T172 and every mark whose centre lands inside a run, so an absent Offset reads exactly as it did.
	//
	// Presence is STRUCTURAL, not a sentinel. The obvious alternative — "all-zero means absent", since an
	// on-run mark spans 0..1 of the run's height — is wrong: a flat underline (TypeLine) has y0 == y1, so
	// a zero-height mark at a run's top edge encodes as all zeros legitimately (Fable, T172 rulings). The
	// discriminator and the payload being the same object is what makes them unable to disagree.
	Offset *AnchorOffset

	// Span (T172 R2) extends the anchor to a RANGE of runs — a bracket or a highlight covering several
	// lines relates to runs N..M, not to one. Absent means a single run, which is what every anchor
	// before T172 is. Same structural-presence rule as Offset.
	Span *AnchorSpan
}

// AnchorOffset is the mark's vertical extent expressed in the RUN'S OWN HEIGHTS, measured from the run's
// top edge: 0..1 is exactly over the run, 1.2..1.4 is just under it, -0.4..-0.1 just above.
//
// Run-heights, because the point is to survive a type-size change: a mark 0.3 run-heights under a line is
// 0.3 run-heights under it at 11 pt and at 13 pt, where "4 mm under" would not be.
//
// There is deliberately NO horizontal twin. CharStart/CharEnd already say which CHARACTERS the mark covers
// — a semantic fact that survives a reflow, a font change and a re-render — and a RelX would be the same
// fact in geometry, which survives none of them. Carrying both would mean carrying the weaker answer and
// writing a rule for when it wins (Fable, T172 rulings). A mark with no horizontal overlap at all is beside
// a BLOCK, not a run, and stays un-anchorable.
type AnchorOffset struct {
	RelY0 float64 // mark top, in run-heights from the run's top edge
	RelY1 float64 // mark bottom, same frame
}

// Clone deep-copies a SourceAnchor INCLUDING its nested optionals. `*a` alone would copy the Offset and
// Span POINTERS, so a clone would share them with the stored object and a caller could mutate state
// through a copy — the precise bug Object.Clone's anchor copy exists to prevent, one level down. Adding a
// pointer field to a type that is cloned by value is how that reappears.
func (a *SourceAnchor) Clone() *SourceAnchor {
	if a == nil {
		return nil
	}
	cp := *a
	if a.Offset != nil {
		o := *a.Offset
		cp.Offset = &o
	}
	if a.Span != nil {
		s := *a.Span
		cp.Span = &s
	}
	return &cp
}

// AnchorSpan names the LAST run of a multi-run anchor; the first is the SourceAnchor's own
// RunText/Occurrence. Both are resolved in source order, so a re-layout cannot renumber either end.
type AnchorSpan struct {
	RunText    string
	Occurrence int // 1-based, document-wide (source order), like SourceAnchor.Occurrence
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
	// JumpTo (P206): set only on an OBJECT_TYPE_ICON that is a JUMP SOURCE — the UUID of
	// the destination icon (the matching glyph) this mark jumps to. Empty = an ordinary
	// icon. The pair matches by looking the same (same glyph + colour), which is the whole
	// point; the destination is its own placed object, so nothing here can drift. Use
	// IsJumpSource() to test it. A deleted destination makes the source read as broken.
	JumpTo string
}

// IsJumpSource reports whether this object is a P206 jump mark's SOURCE — an icon that
// carries a destination reference. What makes an icon a jump is that it carries a target.
func (o Object) IsJumpSource() bool { return o.JumpTo != "" }

// Clone returns a deep copy so callers cannot mutate stored state through aliases.
func (o Object) Clone() Object {
	cp := o
	if o.Points != nil {
		cp.Points = make([]Point, len(o.Points))
		copy(cp.Points, o.Points)
	}
	if o.Anchor != nil { // deep-copy so callers cannot mutate stored state through the pointer
		cp.Anchor = o.Anchor.Clone()
	}
	cp.Style = o.Style.Clone() // Style carries pointers too (Fill/Stroke, and T177's Ends)
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
	ID string
	// GroupID is the BAND id, spelled as the proto field it mirrors (`group_id`, frozen by I1). Every
	// surface a person reads says "band"; only the wire and this mirror of it say "group" (glossary D2).
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
