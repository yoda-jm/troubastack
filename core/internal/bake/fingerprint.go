package bake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// T201 — the input-fingerprint sidecar. It answers, per concert, "would a re-bake change the bundle's
// inputs since the last bake?" — the honest staleness signal Home/A92 needs (today Re-bake shows
// unconditionally). It is written beside page-sources.json (per rev, server-only, never in the .tstage),
// and recomputed + compared at query time.
//
// The fingerprint hashes CONTENT, not revisions or timestamps: a save that writes identical values, and a
// draw-then-undo, leave it unchanged (the regression VLL hit). It hashes only what the overlay worker and
// the baker actually READ — the render inputs — never bookkeeping (object Version/CreatedAt/OwnerID/Scope,
// which an undo or a no-op touches). The SAME gather (gatherSongInputs) feeds both the bake-time write and
// the query-time recompute, so the two can't drift; the bake writes it from the snapshot it GATHERED, in the
// same pass, so an edit landing mid-bake is flagged rather than silently recorded as baked.

const fingerprintFile = "input-fingerprint.json"
const fingerprintVersion = 1

// bakeFingerprint is the sidecar shape. Setlist is the setlist-level hash (name + ordered structure +
// per-item overrides + intermission labels); Order is the song ids in setlist order (intermissions excluded)
// for added/removed/moved detection; Songs maps each song id to its input hash + a title snapshot.
type bakeFingerprint struct {
	V       int                  `json:"v"`
	Setlist string               `json:"setlist"`
	Order   []string             `json:"order"`
	Songs   map[string]songPrint `json:"songs"`
}

type songPrint struct {
	Hash  string `json:"hash"`
	Title string `json:"title"`
}

// ConcertChanges is the per-concert "what changed" the manifest reports to admins.
type ConcertChanges struct {
	Songs   []SongChange `json:"songs"`
	Setlist bool         `json:"setlist"`
}

// SongChange names one song that differs from the last bake. What is one of: "added", "removed", "moved"
// (same content, different position), "content" (same position, different inputs).
type SongChange struct {
	SongID string `json:"songId"`
	Title  string `json:"title"`
	What   string `json:"what"`
}

// --- the canonical, render-only projections that get hashed ---

type fpSong struct {
	Title, Artist, Meter, Key string
	Tempo                     int
	OnCall                    bool
	Notes                     string
	DefaultFileID             string
	Transpose                 bool
	Files                     []fpFile  // union order (the bake order)
	Selections                []fpSel   // sorted by member
	Cues                      []fpCue   // sorted by member
	Layers                    []fpLayer // sorted by id
	Objects                   []string  // sorted canonical object encodings
}

type fpFile struct {
	ID           string
	BlobHash     string
	Generated    bool
	DisplayOrder int
	Crop         *app.Crop `json:",omitempty"` // T198 crop transform inputs
	ChartSrc     string    `json:",omitempty"` // generated chart source text (its render depends on it)
}

type fpSel struct {
	MemberID string
	FileIDs  []string
}

type fpCue struct {
	MemberID string
	Cues     []SongCue // the baked cue set (icon/color) for this member
}

type fpLayer struct {
	ID        string
	Owner     string
	Zone      int // domain.Zone (an int enum)
	Order     int
	Mandatory bool
	RoleTag   string
}

// fpObject is one annotation's render inputs — the fields the overlay worker and the baker read. It
// DELIBERATELY omits UUID, OwnerID, Scope, Version, CreatedAt and Deleted: those are identity/bookkeeping an
// undo or a no-op moves without changing a pixel. LiveObjects() already excludes tombstones.
type fpObject struct {
	Type             string
	Page             int
	Text             string
	JumpTo           string
	Order            int
	LayerID          string
	Points           []domain.Point
	Style            domain.Style
	Anchor           *domain.SourceAnchor
	Link             *domain.MarkLink
	PointsRenderHash string
}

type fpItem struct {
	SongID        string
	Kind          string
	Label         string
	Position      int
	KeyOverride   string
	TempoOverride int
	OnCall        bool
	Transpose     bool
	Notes         string
}

type fpSetlist struct {
	Name  string
	Items []fpItem
}

func hashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "marshal-error" // unreachable for these types; never silently equal to a real hash
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fingerprintSong hashes one setlist item's gathered bake inputs into a content hash.
func fingerprintSong(in songBakeInputs) string {
	unionIDs := make(map[string]bool, len(in.union))
	files := make([]fpFile, 0, len(in.union))
	for _, f := range in.union {
		unionIDs[f.ID] = true
		ff := fpFile{ID: f.ID, BlobHash: f.BlobHash, Generated: f.Generated, DisplayOrder: f.DisplayOrder, Crop: f.Crop}
		if src, ok := in.chartSrc[f.ID]; ok {
			ff.ChartSrc = src
		}
		files = append(files, ff)
	}

	sels := make([]fpSel, 0, len(in.selections))
	for _, s := range in.selections {
		sels = append(sels, fpSel{MemberID: s.MemberID, FileIDs: s.FileIDs})
	}
	sort.Slice(sels, func(i, j int) bool { return sels[i].MemberID < sels[j].MemberID })

	cues := make([]fpCue, 0, len(in.song.MemberCues))
	for _, mc := range in.song.MemberCues {
		cues = append(cues, fpCue{MemberID: mc.MemberID, Cues: mc.Cues})
	}
	sort.Slice(cues, func(i, j int) bool { return cues[i].MemberID < cues[j].MemberID })

	// Layers kept for the union (shared or scoped to a union file), as snapshotToDoc keeps them; objects on
	// those layers. This is exactly the annotation set the bake renders onto these files.
	keptLayer := map[string]bool{}
	layers := make([]fpLayer, 0, len(in.snap.Layers))
	for _, l := range in.snap.Layers {
		if l.FileID != "" && !unionIDs[l.FileID] {
			continue
		}
		keptLayer[l.ID] = true
		layers = append(layers, fpLayer{
			ID: l.ID, Owner: l.OwnerID, Zone: int(l.Zone), Order: l.Order, Mandatory: l.Mandatory, RoleTag: l.RoleTag,
		})
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i].ID < layers[j].ID })

	objs := []string{}
	for _, o := range in.snap.LiveObjects() {
		if !keptLayer[o.LayerID] {
			continue
		}
		objs = append(objs, hashJSON(fpObject{
			Type: domain.ObjectTypeToString(o.Type), Page: o.Page, Text: o.Text, JumpTo: o.JumpTo,
			Order: o.Order, LayerID: o.LayerID, Points: o.Points, Style: o.Style,
			Anchor: o.Anchor, Link: o.Link, PointsRenderHash: o.PointsRenderHash,
		}))
	}
	sort.Strings(objs)

	return hashJSON(fpSong{
		Title: in.song.Title, Artist: in.song.Artist, Meter: in.song.Meter, Key: in.song.Key, Tempo: int(in.song.Tempo),
		OnCall: in.song.OnCall, Notes: in.song.DisplayNotes, DefaultFileID: in.defaultFileID, Transpose: in.transpose,
		Files: files, Selections: sels, Cues: cues, Layers: layers, Objects: objs,
	})
}

// fingerprintSetlist hashes the baked setlist-level inputs: the baked name and the ordered item structure
// (membership, order, intermissions + labels, per-item overrides). Date/venue are excluded — not baked.
func fingerprintSetlist(detail app.SetlistDetail) string {
	items := make([]fpItem, 0, len(detail.Items))
	for i, it := range detail.Items {
		items = append(items, fpItem{
			SongID: it.SongID, Kind: string(it.Kind), Label: it.Label, Position: i,
			KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride,
			OnCall: it.OnCall, Transpose: it.TransposeChords, Notes: it.Notes,
		})
	}
	return hashJSON(fpSetlist{Name: detail.Setlist.Name, Items: items})
}

// diffFingerprints compares a stored fingerprint to the current one. editedSinceBake is true iff anything a
// re-bake consumes differs.
func diffFingerprints(stored, cur bakeFingerprint) (bool, ConcertChanges) {
	// Songs starts as a non-nil empty slice so it serializes as [] (never null) on the wire — A92 never has to
	// null-check it, same as hiddenMarks.
	ch := ConcertChanges{Songs: []SongChange{}, Setlist: stored.Setlist != cur.Setlist}
	pos := func(order []string) map[string]int {
		m := make(map[string]int, len(order))
		for i, id := range order {
			m[id] = i
		}
		return m
	}
	storedPos, curPos := pos(stored.Order), pos(cur.Order)

	for _, id := range cur.Order {
		sp, inStored := stored.Songs[id]
		cp := cur.Songs[id]
		switch {
		case !inStored:
			ch.Songs = append(ch.Songs, SongChange{SongID: id, Title: cp.Title, What: "added"})
		case sp.Hash != cp.Hash:
			ch.Songs = append(ch.Songs, SongChange{SongID: id, Title: cp.Title, What: "content"})
		case storedPos[id] != curPos[id]:
			ch.Songs = append(ch.Songs, SongChange{SongID: id, Title: cp.Title, What: "moved"})
		}
	}
	for _, id := range stored.Order {
		if _, ok := cur.Songs[id]; !ok {
			ch.Songs = append(ch.Songs, SongChange{SongID: id, Title: stored.Songs[id].Title, What: "removed"})
		}
	}
	return ch.Setlist || len(ch.Songs) > 0, ch
}

// --- sidecar I/O (mirrors sources.go: per-rev, atomic temp+rename, server-only) ---

func writeFingerprint(dir string, bf bakeFingerprint) error {
	b, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return fmt.Errorf("bake: marshal input fingerprint: %w", err)
	}
	final := filepath.Join(dir, fingerprintFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("bake: write input fingerprint: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("bake: publish input fingerprint: %w", err)
	}
	return nil
}

// computeFingerprint recomputes the CURRENT input fingerprint for a setlist — the same gather + hash the
// bake uses, with no rendering. Caller must be authorized for the admin-only reads (selections/cues).
func (b *Baker) computeFingerprint(ctx context.Context, bandID string, actor app.User, detail app.SetlistDetail) (bakeFingerprint, error) {
	bf := bakeFingerprint{V: fingerprintVersion, Setlist: fingerprintSetlist(detail), Songs: map[string]songPrint{}}
	for _, it := range detail.Items {
		if it.IsIntermission() {
			continue // intermissions ride the setlist hash, not the per-song map
		}
		in, err := b.gatherSongInputs(ctx, bandID, actor, it)
		if err != nil {
			return bakeFingerprint{}, err
		}
		bf.Order = append(bf.Order, it.SongID)
		bf.Songs[it.SongID] = songPrint{Hash: fingerprintSong(in), Title: in.song.Title}
	}
	return bf, nil
}

// EditedSinceBake answers T201 for one concert: is what would bake now different from the bake at rev? It
// returns a nil *bool when that rev has no fingerprint sidecar (a pre-T201 bake) — the caller reports null.
// It reads admin-only inputs (selections/cues), so ONLY an admin-gated caller may invoke it.
func (b *Baker) EditedSinceBake(ctx context.Context, bandID string, actor app.User, detail app.SetlistDetail, concertID string, rev uint64) (*bool, ConcertChanges, error) {
	stored, ok := b.readFingerprint(concertID, rev)
	if !ok {
		return nil, ConcertChanges{}, nil // unknown: pre-T201 bake
	}
	cur, err := b.computeFingerprint(ctx, bandID, actor, detail)
	if err != nil {
		return nil, ConcertChanges{}, err
	}
	edited, changes := diffFingerprints(stored, cur)
	return &edited, changes, nil
}

// readFingerprint reads the sidecar for a published rev, or (false) if none exists (a pre-T201 bake) — read
// as UNKNOWN (null), never as edited.
func (b *Baker) readFingerprint(concertID string, rev uint64) (bakeFingerprint, bool) {
	p := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10), fingerprintFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return bakeFingerprint{}, false
	}
	var bf bakeFingerprint
	if err := json.Unmarshal(data, &bf); err != nil || bf.Songs == nil {
		return bakeFingerprint{}, false
	}
	return bf, true
}
