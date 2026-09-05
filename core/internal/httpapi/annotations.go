package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// AnnotationsAPI is the HTTP edge for a song's annotation layers/objects, which live
// in the per-song apply engine (internal/engine over internal/store) — DISTINCT from
// the relational app store. It is view-only for now: read the materialized HEAD, or
// bulk-import layers+objects (the seeder + viewer-test path; no realtime/editing yet).
//
// Membership/ownership policy stays in app.Service (SongForMember); this adapter does
// the wire<->domain mapping and drives the engine. The relational Song.ID is the
// engine's songID.
type AnnotationsAPI struct {
	svc *app.Service
	eng *engine.Engine
}

// NewAnnotationsAPI builds the annotation adapter over the relational Service (for
// member/song authorization) and the per-song apply engine (the annotation authority).
func NewAnnotationsAPI(svc *app.Service, eng *engine.Engine) *AnnotationsAPI {
	return &AnnotationsAPI{svc: svc, eng: eng}
}

// Mount registers the annotation routes. They sit under the song path so the band +
// song scope (and thus membership) is explicit.
func (a *AnnotationsAPI) Mount(mux *http.ServeMux, authed func(authedHandler) http.HandlerFunc) {
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/annotations", authed(a.getAnnotations))
	mux.HandleFunc("POST /api/bands/{bandId}/songs/{songId}/annotations/import", authed(a.importAnnotations))
	mux.HandleFunc("GET /api/bands/{bandId}/songs/{songId}/live-setlists", authed(a.liveSetlists))
}

// ---- wire types (the EXACT contract the frontend + seeder speak) ----

type pointJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type styleJSON struct {
	Color    string  `json:"color"`            // "#RRGGBB"
	Opacity  float64 `json:"opacity"`          // 0..1
	Width    float64 `json:"width"`            // stroke width as fraction of page width
	FontSize float64 `json:"fontSize"`         // fraction of page height (text)
	Fill     *bool   `json:"fill,omitempty"`   // rect/ellipse interior; absent = infer
	Stroke   *bool   `json:"stroke,omitempty"` // rect/ellipse border; absent = infer
	Blend    string  `json:"blend,omitempty"`  // ""|"normal"|"multiply"
}

type layerJSON struct {
	ID        string `json:"id"`
	FileID    string `json:"fileId"`
	Name      string `json:"name"`
	OwnerID   string `json:"ownerId"`
	Zone      string `json:"zone"` // "conductor"|"shared"|"personal"
	Order     int    `json:"order"`
	Access    string `json:"access"` // "rw"|"ro"
	Mandatory bool   `json:"mandatory"`
	RoleTag   string `json:"roleTag"`
}

type objectJSON struct {
	UUID      string      `json:"uuid"`
	LayerID   string      `json:"layerId"`
	Type      string      `json:"type"` // freehand|rect|ellipse|line|text|highlight|icon
	Points    []pointJSON `json:"points"`
	Page      int         `json:"page"`
	Text      string      `json:"text"`
	Order     int         `json:"order"`     // z-order within the layer (T27)
	CreatedAt int64       `json:"createdAt"` // z-order tiebreak after order (T27)
	Style     styleJSON   `json:"style"`
	// T145: the SOURCE-scoped anchor (when the mark sits on a generated chart), and the render hash the
	// Points cache was projected from. Both omitempty — an uploaded PDF / legacy mark carries neither.
	Anchor           *anchorJSON `json:"anchor,omitempty"`
	PointsRenderHash string      `json:"pointsRenderHash,omitempty"`
}

// anchorJSON is the wire shape of domain.SourceAnchor (T145).
type anchorJSON struct {
	RunText    string `json:"runText"`
	Occurrence int    `json:"occurrence"`
	CharStart  int    `json:"charStart"`
	CharEnd    int    `json:"charEnd"`
}

// annotationsJSON is both the GET response and the import request body.
type annotationsJSON struct {
	Layers  []layerJSON  `json:"layers"`
	Objects []objectJSON `json:"objects"`
}

// ---- handlers ----

// getAnnotations returns the materialized HEAD for a song: all its layers + live
// objects. An empty song yields empty arrays (never an error).
func (a *AnnotationsAPI) getAnnotations(w http.ResponseWriter, r *http.Request, u app.User) {
	song, err := a.svc.SongForMember(u, r.PathValue("bandId"), r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	snap, err := a.eng.Head(song.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	snap = a.reprojectSnapshot(u, r.PathValue("bandId"), song.ID, snap)
	writeJSON(w, http.StatusOK, snapshotToJSON(snap))
}

// reprojectSnapshot re-projects any mark whose cached coordinates predate its chart file's current render
// (T145 forward fix, serve side), so the editor draws the mark on its words after a size/render change.
// Per generated file referenced by a layer: render the source for the anchor manifest and re-project that
// file's marks against the file's BlobHash. Best-effort + read-only: a file with no source (an uploaded
// PDF) or a render error leaves its marks on their stored coordinates.
func (a *AnnotationsAPI) reprojectSnapshot(u app.User, bandID, songID string, snap domain.Snapshot) domain.Snapshot {
	fileByLayer := make(map[string]string, len(snap.Layers))
	for _, l := range snap.Layers {
		fileByLayer[l.ID] = l.FileID
	}
	manifest := a.chartAnchorsFor(u, bandID, songID)
	objs := make([]domain.Object, len(snap.Objects))
	for i, o := range snap.Objects {
		objs[i] = o
		if o.Anchor == nil {
			continue
		}
		if anchors, hash, ok := manifest(fileByLayer[o.LayerID]); ok {
			rp, _ := chartpdf.Reproject([]domain.Object{o}, anchors, hash)
			objs[i] = rp[0]
		}
	}
	snap.Objects = objs
	return snap
}

// chartAnchorsFor returns a per-file resolver of (anchor manifest, current render hash) for a song's
// GENERATED charts, shared by create-time anchoring (import) and re-projection (serve). Each generated
// file's source is rendered once and cached; an uploaded / sourceless file resolves to ok=false. Best-
// effort + read-only (auth via the caller's ChartSource).
func (a *AnnotationsAPI) chartAnchorsFor(u app.User, bandID, songID string) func(fileID string) ([]chartpdf.Anchor, string, bool) {
	type entry struct {
		anchors []chartpdf.Anchor
		hash    string
		ok      bool
	}
	cache := map[string]entry{}
	return func(fileID string) ([]chartpdf.Anchor, string, bool) {
		if fileID == "" {
			return nil, "", false
		}
		if e, seen := cache[fileID]; seen {
			return e.anchors, e.hash, e.ok
		}
		var e entry
		if sf, src, err := a.svc.ChartSource(u, bandID, songID, fileID); err == nil {
			// Anchor/re-project ONLY against a render that reproduces the stored blob (byte-equal); a
			// divergent fresh render (e.g. after a layout change) must not stamp the blob hash onto its own
			// coordinates. See app.ChartAnchorsIfCurrent.
			if anchors, ok := app.ChartAnchorsIfCurrent(src, sf.BlobHash); ok {
				e = entry{anchors: anchors, hash: sf.BlobHash, ok: true}
			}
		}
		cache[fileID] = e
		return e.anchors, e.hash, e.ok
	}
}

// liveSetlists (P201 stage 2b) returns the band's setlists that are in rehearsal live
// mode right now AND contain this song — the signal the editor uses to show its LIVE
// banner. Membership-gated (SongForMember); an empty list = not live. Id + name only.
func (a *AnnotationsAPI) liveSetlists(w http.ResponseWriter, r *http.Request, u app.User) {
	song, err := a.svc.SongForMember(u, r.PathValue("bandId"), r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	sls, err := a.svc.LiveSetlistsForSong(song.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]map[string]string, 0, len(sls))
	for _, sl := range sls {
		out = append(out, map[string]string{"id": sl.ID, "name": sl.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"setlists": out})
}

// importAnnotations bulk-applies layers (LayerCreate) and objects (Create) as
// mutations against the engine, then returns the resulting materialized HEAD. It is
// idempotent: layers are keyed by id and objects by uuid, so re-import does not
// duplicate (the engine's create is a no-op for an already-present uuid/version).
func (a *AnnotationsAPI) importAnnotations(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	song, err := a.svc.SongForMember(u, bandID, r.PathValue("songId"))
	if err != nil {
		writeErr(w, err)
		return
	}
	// Import is a bulk/seed tool: it provisions layers+objects on behalf of ANY
	// owner (see cmd/seed), which necessarily bypasses the per-layer write gate
	// applied to the live editing path (sync/apply.go authorizeWrite). To stop a
	// non-admin member using it to write layers that gate would reject (locked,
	// foreign, or conductor-zone), restrict it to band admins. Regular members
	// edit through the gated WebSocket path, not this endpoint (the studio UI
	// never calls import). Policy: admin-only route (T08 option b).
	if _, role, err := a.svc.GetBand(u, bandID); err != nil {
		writeErr(w, err)
		return
	} else if role != app.RoleAdmin {
		writeErr(w, app.ErrForbidden)
		return
	}
	var in annotationsJSON
	if !decode(w, r, &in) {
		return
	}
	songID := song.ID

	// Layers first so objects land on existing layers.
	for _, lj := range in.Layers {
		l := layerFromJSON(lj)
		if l.ID == "" {
			writeErr(w, app.ErrInvalidInput)
			return
		}
		m := domain.Mutation{
			Kind:     domain.KindLayerCreate,
			Layer:    &l,
			AuthorID: u.ID,
			Summary:  "import layer " + l.ID,
		}
		if _, err := a.eng.Apply(songID, m); err != nil {
			writeErr(w, mapEngineErr(err))
			return
		}
	}

	// T145 forward fix (create side): anchor imported marks to their words on a generated chart, so a later
	// size/render change moves them with the text. Idempotent — a mark that already carries an anchor (a
	// folder that stored one) is left as-is.
	fileByLayer := map[string]string{}
	for _, lj := range in.Layers {
		fileByLayer[lj.ID] = lj.FileID
	}
	manifest := a.chartAnchorsFor(u, bandID, songID)

	for _, oj := range in.Objects {
		o := objectFromJSON(oj)
		if o.UUID == "" {
			writeErr(w, app.ErrInvalidInput)
			return
		}
		if o.Anchor == nil {
			if anchors, hash, ok := manifest(fileByLayer[o.LayerID]); ok {
				o = chartpdf.AnchorObject(o, anchors, hash)
			}
		}
		// Version 1 makes a re-imported object an idempotent same-version no-op
		// rather than a tombstone-resurrection or LWW conflict.
		if o.Version == 0 {
			o.Version = 1
		}
		oc := o
		m := domain.Mutation{
			Kind:     domain.KindCreate,
			UUID:     o.UUID,
			Object:   &oc,
			AuthorID: u.ID,
			Summary:  "import object " + o.UUID,
		}
		if _, err := a.eng.Apply(songID, m); err != nil {
			writeErr(w, mapEngineErr(err))
			return
		}
	}

	snap, err := a.eng.Head(songID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshotToJSON(snap))
}

// mapEngineErr maps engine apply failures to an app sentinel so writeErr emits a
// sensible status. Malformed-mutation maps to 400; anything else is a 500.
func mapEngineErr(err error) error {
	switch err {
	case engine.ErrInvalidMutation:
		return app.ErrInvalidInput
	default:
		return err
	}
}

// ---- mapping: domain <-> wire ----

func snapshotToJSON(snap domain.Snapshot) annotationsJSON {
	out := annotationsJSON{Layers: []layerJSON{}, Objects: []objectJSON{}}
	for _, l := range snap.Layers {
		out.Layers = append(out.Layers, layerToJSON(l))
	}
	for _, o := range snap.LiveObjects() {
		out.Objects = append(out.Objects, objectToJSON(o))
	}
	return out
}

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

// ---- enum string maps ----
// These delegate to domain (domain/enumwire.go) so the HTTP edge and the .tband v2
// export share one encoding and cannot drift (T134). Thin wrappers keep the local
// call sites unchanged.

func zoneToString(z domain.Zone) string     { return domain.ZoneToString(z) }
func zoneFromString(s string) domain.Zone   { return domain.ZoneFromString(s) }
func accessToString(a domain.Access) string { return domain.AccessToString(a) }
func accessFromString(s string) domain.Access {
	return domain.AccessFromString(s)
}
