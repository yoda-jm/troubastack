package bake

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// bakerFor builds a Baker with a persistent temp bakesDir so a test can bake AND read the sidecar back.
func (e t137Env) bakerFor(t *testing.T) *Baker {
	t.Helper()
	return &Baker{svc: e.svc, eng: e.eng, raster: contentRaster{}, overlays: fakeOverlays{png: []byte("ov")}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
}

func sharedLayer(id string) *domain.Layer {
	return &domain.Layer{ID: id, OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW}
}

func mark(uuid, layerID string, x float64) *domain.Object {
	return &domain.Object{
		UUID: uuid, Type: domain.TypeFreehand, LayerID: layerID,
		Points: []domain.Point{{X: x, Y: 0.5}}, Style: domain.Style{Color: "#000000", Width: 0.01},
	}
}

// TestFingerprintSong_IgnoresBookkeeping (refinement A): the per-song fingerprint hashes ONLY render inputs.
// Two heads differing only in an object's Version / CreatedAt / OwnerID (what an undo or a no-op touches)
// hash the SAME; a moved point hashes differently. (If fpObject included Version, the first assert reds.)
func TestFingerprintSong_IgnoresBookkeeping(t *testing.T) {
	obj := func(version uint64, createdAt int64, owner string, x float64) domain.Object {
		o := *mark("o1", "L1", x)
		o.Version, o.CreatedAt, o.OwnerID = version, createdAt, owner
		return o
	}
	inputs := func(o domain.Object) songBakeInputs {
		return songBakeInputs{
			song:  BakedSong{SongID: "s", Title: "T"},
			union: []app.SongFile{{ID: "f", BlobHash: "h"}},
			snap: domain.Snapshot{
				Layers:  []domain.Layer{*sharedLayer("L1")},
				Objects: []domain.Object{o},
			},
		}
	}
	base := inputs(obj(1, 100, "u1", 0.2))
	bumped := inputs(obj(2, 999, "u2", 0.2)) // only bookkeeping moved
	if fingerprintSong(base) != fingerprintSong(bumped) {
		t.Error("bookkeeping (version/createdAt/owner) leaked into the fingerprint — it must hash only render inputs")
	}
	moved := inputs(obj(1, 100, "u1", 0.9)) // a render field changed
	if fingerprintSong(base) == fingerprintSong(moved) {
		t.Error("a moved point must change the fingerprint")
	}
}

// TestFingerprint_PositiveControlAndNoChange (spec positive control): the fingerprint the bake WROTE equals
// the one recomputed from the same inputs, and a fresh bake with no edits reads not-edited.
func TestFingerprint_PositiveControlAndNoChange(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 A"))
	b := e.bakerFor(t)
	ctx := context.Background()
	cb, _, err := b.Bake(ctx, e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	stored, ok := b.readFingerprint(cb.ConcertID, cb.ConcertRev)
	if !ok {
		t.Fatal("bake wrote no input-fingerprint sidecar")
	}
	detail, err := e.svc.Setlist(e.admin, e.bandID, e.sl)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := b.computeFingerprint(ctx, e.bandID, e.admin, detail)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, cur) {
		t.Fatalf("recomputed fingerprint != the one the bake wrote:\n stored=%+v\n cur=%+v", stored, cur)
	}
	edited, _, err := b.EditedSinceBake(ctx, e.bandID, e.admin, detail, cb.ConcertID, cb.ConcertRev)
	if err != nil {
		t.Fatal(err)
	}
	if edited == nil || *edited {
		t.Fatalf("a fresh bake must read not-edited, got %v", edited)
	}
}

// TestFingerprint_AnnotationChangeIsContent: a mark drawn after the bake flips the song to "content" (not a
// setlist change).
func TestFingerprint_AnnotationChangeIsContent(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 A"))
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: e.admin.ID, Layer: sharedLayer("L1")}); err != nil {
		t.Fatal(err)
	}
	b := e.bakerFor(t)
	ctx := context.Background()
	cb, _, err := b.Bake(ctx, e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindCreate, UUID: "o1", AuthorID: e.admin.ID, Object: mark("o1", "L1", 0.3)}); err != nil {
		t.Fatal(err)
	}
	detail, _ := e.svc.Setlist(e.admin, e.bandID, e.sl)
	edited, ch, err := b.EditedSinceBake(ctx, e.bandID, e.admin, detail, cb.ConcertID, cb.ConcertRev)
	if err != nil {
		t.Fatal(err)
	}
	if edited == nil || !*edited {
		t.Fatal("an added annotation must read as edited")
	}
	if ch.Setlist {
		t.Error("an annotation change is not a setlist-level change")
	}
	if len(ch.Songs) != 1 || ch.Songs[0].SongID != e.songID || ch.Songs[0].What != "content" {
		t.Fatalf("want one content change for the song, got %+v", ch.Songs)
	}
}

// TestFingerprint_DrawThenUndoStaysFalse: draw then delete (a net no-op) advances the engine revision but
// leaves the LIVE content identical — the fingerprint hashes content, so the concert reads NOT edited.
func TestFingerprint_DrawThenUndoStaysFalse(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 A"))
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: e.admin.ID, Layer: sharedLayer("L1")}); err != nil {
		t.Fatal(err)
	}
	b := e.bakerFor(t)
	ctx := context.Background()
	cb, _, err := b.Bake(ctx, e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindCreate, UUID: "o1", AuthorID: e.admin.ID, Object: mark("o1", "L1", 0.3)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindDelete, UUID: "o1", AuthorID: e.admin.ID}); err != nil {
		t.Fatal(err)
	}
	detail, _ := e.svc.Setlist(e.admin, e.bandID, e.sl)
	edited, _, err := b.EditedSinceBake(ctx, e.bandID, e.admin, detail, cb.ConcertID, cb.ConcertRev)
	if err != nil {
		t.Fatal(err)
	}
	if edited == nil || *edited {
		t.Fatalf("draw-then-undo must read NOT edited (content-based, not revision), got %v", edited)
	}
}

// TestFingerprint_EditBetweenGatherAndWrite (refinement B): an edit landing after the bake gathered its
// inputs but before it wrote the sidecar must leave the concert reading edited — the stored fingerprint
// reflects the gathered snapshot, not a later re-read.
func TestFingerprint_EditBetweenGatherAndWrite(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 A"))
	if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: e.admin.ID, Layer: sharedLayer("L1")}); err != nil {
		t.Fatal(err)
	}
	b := e.bakerFor(t)
	ctx := context.Background()
	injected := false
	b.afterGather = func() {
		if injected {
			return
		}
		injected = true
		if _, err := e.eng.Apply(e.songID, domain.Mutation{Kind: domain.KindCreate, UUID: "late", AuthorID: e.admin.ID, Object: mark("late", "L1", 0.6)}); err != nil {
			t.Fatal(err)
		}
	}
	cb, _, err := b.Bake(ctx, e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	b.afterGather = nil
	if !injected {
		t.Fatal("afterGather seam never fired")
	}
	detail, _ := e.svc.Setlist(e.admin, e.bandID, e.sl)
	edited, _, err := b.EditedSinceBake(ctx, e.bandID, e.admin, detail, cb.ConcertID, cb.ConcertRev)
	if err != nil {
		t.Fatal(err)
	}
	if edited == nil || !*edited {
		t.Fatal("an edit injected between gather and write must leave the concert reading edited")
	}
}

// TestFingerprint_NoSidecarIsNull: a bake with no fingerprint sidecar (a pre-T201 bake) reads as UNKNOWN —
// EditedSinceBake returns a nil *bool, never "edited".
func TestFingerprint_NoSidecarIsNull(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 A"))
	b := e.bakerFor(t)
	ctx := context.Background()
	cb, _, err := b.Bake(ctx, e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	// Simulate a pre-T201 bake: remove the sidecar the bake wrote.
	p := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(cb.ConcertRev, 10), fingerprintFile)
	if err := os.Remove(p); err != nil {
		t.Fatalf("remove sidecar: %v", err)
	}
	detail, _ := e.svc.Setlist(e.admin, e.bandID, e.sl)
	edited, _, err := b.EditedSinceBake(ctx, e.bandID, e.admin, detail, cb.ConcertID, cb.ConcertRev)
	if err != nil {
		t.Fatal(err)
	}
	if edited != nil {
		t.Fatalf("a bake with no fingerprint sidecar must read as null (nil), got %v", *edited)
	}
}

// TestDiffFingerprints_EmptyChangesIsArray: with nothing changed, ConcertChanges.Songs must be a non-nil
// empty slice so it serializes as [] (never null) — A92 never null-checks it.
func TestDiffFingerprints_EmptyChangesIsArray(t *testing.T) {
	fp := bakeFingerprint{V: 1, Setlist: "s", Order: []string{"x"}, Songs: map[string]songPrint{"x": {Hash: "h", Title: "T"}}}
	edited, ch := diffFingerprints(fp, fp)
	if edited {
		t.Fatal("identical fingerprints must read not-edited")
	}
	if ch.Songs == nil {
		t.Fatal("Songs must be a non-nil empty slice")
	}
	b, err := json.Marshal(ch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"songs":[]`) {
		t.Fatalf("empty changes must serialize songs as [], got %s", b)
	}
}
