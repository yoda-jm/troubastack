package app_test

import (
	"encoding/json"
	"errors"
	"testing"

	"troubastack/core/internal/app"
)

// t150Folder builds a minimal hand-authored v2 band folder (no members but the importer) with an
// optionally-declared band id / shortname and an optionally-declared setlist id, one song, one setlist.
func t150Folder(t *testing.T, bandID, shortname, setlistID string) []byte {
	t.Helper()
	content := []byte("%PDF t150")
	bandFields := map[string]any{"formatVersion": 2, "name": "Durable Band", "members": []any{}}
	if bandID != "" {
		bandFields["id"] = bandID
	}
	if shortname != "" {
		bandFields["shortname"] = shortname
	}
	band, _ := json.Marshal(bandFields)
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{
			"slug": "wonderwall", "title": "Wonderwall",
			"files": []any{map[string]any{"filename": "chart.pdf", "contentType": "application/pdf", "size": len(content)}},
		}},
	})
	sl := map[string]any{"name": "Spring Gig", "items": []any{map[string]any{"song": "wonderwall"}}}
	if setlistID != "" {
		sl["id"] = setlistID
	}
	setlists, _ := json.Marshal(map[string]any{"setlists": []any{sl}})
	return rezip(t, map[string][]byte{
		"band.json":            band,
		"repertoire.json":      rep,
		"setlists.json":        setlists,
		"wonderwall/chart.pdf": content,
	})
}

func firstSong(t *testing.T, st stack, bandID string) app.Song {
	t.Helper()
	songs, err := st.repo.SongsOfBand(bandID)
	if err != nil || len(songs) != 1 {
		t.Fatalf("want exactly 1 song in band, got %d (err %v)", len(songs), err)
	}
	return songs[0]
}

func firstSetlist(t *testing.T, st stack, bandID string) app.Setlist {
	t.Helper()
	sls, err := st.repo.SetlistsOfBand(bandID)
	if err != nil || len(sls) != 1 {
		t.Fatalf("want exactly 1 setlist in band, got %d (err %v)", len(sls), err)
	}
	return sls[0]
}

// TestBandImport_SameFolderTwice_OneBandSameID is the T150 flagship (⟨R1⟩): re-importing the same folder
// updates the SAME band — same id, one setlist (same id), one song (same id) — instead of minting a twin.
// RED on pre-T150 code, which mints a fresh UUID on every import.
func TestBandImport_SameFolderTwice_OneBandSameID(t *testing.T) {
	folder := t150Folder(t, "band-uuid-0001", "durable", "setlist-uuid-0001")
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")

	r1, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	song1 := firstSong(t, st, r1.Band.ID)
	setlist1 := firstSetlist(t, st, r1.Band.ID)

	r2, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}

	// One band total, same id.
	bands, _ := st.repo.BandsForUser(owner.ID)
	if len(bands) != 1 {
		t.Fatalf("re-import created a twin: %d bands, want 1", len(bands))
	}
	if r2.Band.ID != r1.Band.ID {
		t.Fatalf("band id changed on re-import: %q -> %q", r1.Band.ID, r2.Band.ID)
	}
	// Declared id used verbatim.
	if r1.Band.ID != "band-uuid-0001" {
		t.Fatalf("declared band id not used verbatim: %q", r1.Band.ID)
	}
	// Song + setlist stable (id, not just count).
	song2 := firstSong(t, st, r2.Band.ID)
	if song2.ID != song1.ID {
		t.Fatalf("song id changed on re-import: %q -> %q", song1.ID, song2.ID)
	}
	setlist2 := firstSetlist(t, st, r2.Band.ID)
	if setlist2.ID != setlist1.ID {
		t.Fatalf("setlist id changed on re-import: %q -> %q", setlist1.ID, setlist2.ID)
	}
	if setlist1.ID != "setlist-uuid-0001" {
		t.Fatalf("declared setlist id not used verbatim: %q", setlist1.ID)
	}
	// A bake made before the second import still resolves to its band (the id is unchanged).
	if _, _, err := st.svc.GetBand(owner, r1.Band.ID); err != nil {
		t.Fatalf("band no longer resolvable by its original id after re-import: %v", err)
	}
}

// TestBandImport_TwoEmptyStores_IdenticalIDs is ⟨D2⟩ + ⟨R1⟩: a folder with declared ids seeds two EMPTY
// stores to IDENTICAL band, setlist and song ids — from-scratch stability, which no lookup can provide.
func TestBandImport_TwoEmptyStores_IdenticalIDs(t *testing.T) {
	folder := t150Folder(t, "band-uuid-0002", "durable", "setlist-uuid-0002")

	a := newStack()
	oa, _ := a.svc.Register("owner", "Owner", "password123", "")
	ra, err := a.svc.ImportBand(oa, a.eng, folder, nil)
	if err != nil {
		t.Fatalf("import A: %v", err)
	}
	b := newStack()
	ob, _ := b.svc.Register("owner", "Owner", "password123", "")
	rb, err := b.svc.ImportBand(ob, b.eng, folder, nil)
	if err != nil {
		t.Fatalf("import B: %v", err)
	}

	if ra.Band.ID != rb.Band.ID {
		t.Errorf("band id differs across empty stores: %q vs %q", ra.Band.ID, rb.Band.ID)
	}
	if sa, sb := firstSong(t, a, ra.Band.ID), firstSong(t, b, rb.Band.ID); sa.ID != sb.ID {
		t.Errorf("song id differs across empty stores: %q vs %q", sa.ID, sb.ID)
	}
	if la, lb := firstSetlist(t, a, ra.Band.ID), firstSetlist(t, b, rb.Band.ID); la.ID != lb.ID {
		t.Errorf("setlist id differs across empty stores: %q vs %q", la.ID, lb.ID)
	}
}

// TestBandImport_AdoptByShortname covers a folder that predates declared ids: no id, but a shortname.
// Re-import adopts the caller's existing band by shortname (one band, same id) and the ids are stable even
// though they were minted (not declared) on first import.
func TestBandImport_AdoptByShortname(t *testing.T) {
	folder := t150Folder(t, "", "durable", "") // no declared band/setlist id, shortname only
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")

	r1, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	song1 := firstSong(t, st, r1.Band.ID)

	r2, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	bands, _ := st.repo.BandsForUser(owner.ID)
	if len(bands) != 1 {
		t.Fatalf("shortname re-import created a twin: %d bands, want 1", len(bands))
	}
	if r2.Band.ID != r1.Band.ID {
		t.Fatalf("adopted band id changed: %q -> %q", r1.Band.ID, r2.Band.ID)
	}
	if song2 := firstSong(t, st, r2.Band.ID); song2.ID != song1.ID {
		t.Fatalf("song id changed on shortname re-import: %q -> %q", song1.ID, song2.ID)
	}
}

// TestBandImport_AdoptByName_RefusesWhenAmbiguous: a legacy folder with neither id nor shortname must NOT
// guess which of two same-named shortname-less bands to adopt — it refuses and creates nothing (⟨R1⟩).
func TestBandImport_AdoptByName_RefusesWhenAmbiguous(t *testing.T) {
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	// Two same-named, shortname-less bands owned by the caller (the exact pre-T150 duplicate situation).
	for _, id := range []string{"pre-a", "pre-b"} {
		if err := st.repo.CreateBand(app.Band{ID: id, Name: "Durable Band", OwnerID: owner.ID}); err != nil {
			t.Fatal(err)
		}
		if err := st.repo.AddMembership(app.Membership{BandID: id, UserID: owner.ID, Role: app.RoleAdmin}); err != nil {
			t.Fatal(err)
		}
	}
	folder := t150Folder(t, "", "", "") // no id, no shortname → name-only
	_, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err == nil {
		t.Fatal("ambiguous name adoption must refuse, got nil error")
	}
	if !errors.Is(err, app.ErrConflict) {
		t.Fatalf("want ErrConflict on ambiguous adoption, got %v", err)
	}
	// Nothing created: still exactly the two pre-existing bands.
	bands, _ := st.repo.BandsForUser(owner.ID)
	if len(bands) != 2 {
		t.Fatalf("refusal must create nothing: %d bands, want 2", len(bands))
	}
}

// TestBandImport_ShortnameChangeKeepsID: the id is declared, not derived from the shortname, so changing
// the shortname on re-import does not change the id (⟨R1⟩).
func TestBandImport_ShortnameChangeKeepsID(t *testing.T) {
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	r1, err := st.svc.ImportBand(owner, st.eng, t150Folder(t, "band-uuid-0003", "durable", ""), nil)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	r2, err := st.svc.ImportBand(owner, st.eng, t150Folder(t, "band-uuid-0003", "renamed", ""), nil)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if r2.Band.ID != r1.Band.ID {
		t.Fatalf("shortname change moved the id: %q -> %q", r1.Band.ID, r2.Band.ID)
	}
	if r2.Band.Shortname != "renamed" {
		t.Fatalf("shortname not updated on re-import: %q", r2.Band.Shortname)
	}
	bands, _ := st.repo.BandsForUser(owner.ID)
	if len(bands) != 1 {
		t.Fatalf("shortname change created a twin: %d bands", len(bands))
	}
}

// TestBandImport_ReimportWithAnnotations_Idempotent: re-importing an adopted band that carries annotations
// must not error (the layer/object mutations are re-applied to the same per-song engine) and must not
// duplicate the marks — the risky path adoption introduces over a fresh import.
func TestBandImport_ReimportWithAnnotations_Idempotent(t *testing.T) {
	content := []byte("%PDF t150 annots")
	band, _ := json.Marshal(map[string]any{"formatVersion": 2, "id": "band-uuid-0005", "name": "Durable Band", "members": []any{}})
	rep, _ := json.Marshal(map[string]any{"songs": []any{map[string]any{
		"slug": "wonderwall", "title": "Wonderwall",
		"files": []any{map[string]any{"filename": "chart.pdf", "contentType": "application/pdf", "size": len(content)}},
	}}})
	ann, _ := json.Marshal(map[string]any{
		"layers": []any{map[string]any{"id": "L1", "file": "chart.pdf", "name": "Cues", "owner": "_shared_", "zone": "shared", "access": "rw"}},
		"objects": []any{map[string]any{
			"uuid": "O1", "layer": "L1", "type": "rect", "page": 0,
			"points": []any{map[string]any{"x": 0.1, "y": 0.1}, map[string]any{"x": 0.5, "y": 0.4}},
			"style":  map[string]any{"color": "#e11d48", "opacity": 1, "width": 0.004},
		}},
	})
	folder := rezip(t, map[string][]byte{
		"band.json": band, "repertoire.json": rep,
		"annotations/wonderwall.json": ann, "wonderwall/chart.pdf": content,
	})

	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	if _, err := st.svc.ImportBand(owner, st.eng, folder, nil); err != nil {
		t.Fatalf("first import: %v", err)
	}
	r2, err := st.svc.ImportBand(owner, st.eng, folder, nil)
	if err != nil {
		t.Fatalf("second import (adopt, re-apply annotations) errored: %v", err)
	}
	song := firstSong(t, st, r2.Band.ID)
	snap, err := st.eng.Head(song.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Layers) != 1 || snap.Layers[0].ID != "L1" {
		t.Fatalf("re-import duplicated/lost layers: %d (want 1 'L1')", len(snap.Layers))
	}
	live := 0
	for _, o := range snap.Objects {
		if !o.Deleted {
			live++
		}
	}
	if live != 1 {
		t.Fatalf("re-import left %d live objects, want 1 (no duplication)", live)
	}
}

// t150FolderNItems builds a folder whose one setlist has N songs (slugs s0..sN-1), with a declared band +
// setlist id so a re-import adopts. Used to prove a re-import can SHRINK a setlist.
func t150FolderNItems(t *testing.T, bandID, setlistID string, n int) []byte {
	t.Helper()
	content := []byte("%PDF t150 n")
	band, _ := json.Marshal(map[string]any{"formatVersion": 2, "id": bandID, "name": "Durable Band", "members": []any{}})
	songs := make([]any, n)
	items := make([]any, n)
	files := map[string][]byte{}
	for i := 0; i < n; i++ {
		slug := "s" + string(rune('0'+i))
		songs[i] = map[string]any{"slug": slug, "title": "Song " + slug,
			"files": []any{map[string]any{"filename": "c.pdf", "contentType": "application/pdf", "size": len(content)}}}
		items[i] = map[string]any{"song": slug}
		files[slug+"/c.pdf"] = content
	}
	rep, _ := json.Marshal(map[string]any{"songs": songs})
	sls, _ := json.Marshal(map[string]any{"setlists": []any{map[string]any{"id": setlistID, "name": "Gig", "items": items}}})
	files["band.json"] = band
	files["repertoire.json"] = rep
	files["setlists.json"] = sls
	return rezip(t, files)
}

// TestBandImport_ReimportShrinksSetlist is the T150 GO defect (Fable): a re-import must RECONCILE setlist
// items so a shortened folder removes the dropped songs — else phantom songs survive and play at the gig.
// RED before the reconcile: item ids are upserted but never removed, so the setlist keeps 3.
func TestBandImport_ReimportShrinksSetlist(t *testing.T) {
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	if _, err := st.svc.ImportBand(owner, st.eng, t150FolderNItems(t, "band-shrink", "sl-shrink", 3), nil); err != nil {
		t.Fatalf("first import (3 items): %v", err)
	}
	r2, err := st.svc.ImportBand(owner, st.eng, t150FolderNItems(t, "band-shrink", "sl-shrink", 2), nil)
	if err != nil {
		t.Fatalf("second import (2 items): %v", err)
	}
	sl := firstSetlist(t, st, r2.Band.ID)
	items, err := st.repo.ItemsOfSetlist(sl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("re-import with 2 items left %d items — a shortened folder must remove the dropped songs (phantom songs would play)", len(items))
	}
}

// TestBandImport_RemovedSongSurvives is the SYMMETRIC documented property (Fable): a song dropped from the
// folder is NOT deleted on re-import (songs carry annotations + bake history — the folder cannot remove a
// song). Asserted deliberately so the property has a test.
func TestBandImport_RemovedSongSurvives(t *testing.T) {
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	if _, err := st.svc.ImportBand(owner, st.eng, t150FolderNItems(t, "band-keep", "sl-keep", 3), nil); err != nil {
		t.Fatalf("first import (3 songs): %v", err)
	}
	r2, err := st.svc.ImportBand(owner, st.eng, t150FolderNItems(t, "band-keep", "sl-keep", 2), nil)
	if err != nil {
		t.Fatalf("second import (2 songs): %v", err)
	}
	songs, err := st.repo.SongsOfBand(r2.Band.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 3 {
		t.Fatalf("removed song was deleted (%d songs) — the folder must NOT remove a song (annotations/bake history live on it)", len(songs))
	}
}

// TestBandExport_EmitsDeclaredIDs: export writes the band's current id + each setlist id into the folder
// (the write-back that lets a subsequent from-scratch seed be stable).
func TestBandExport_EmitsDeclaredIDs(t *testing.T) {
	st := newStack()
	owner, _ := st.svc.Register("owner", "Owner", "password123", "")
	r1, err := st.svc.ImportBand(owner, st.eng, t150Folder(t, "band-uuid-0004", "durable", "setlist-uuid-0004"), nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	zipBytes, _, err := st.svc.ExportBand(owner, st.eng, r1.Band.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	files := unzip(t, zipBytes)
	var bandJSON struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(files["band.json"], &bandJSON); err != nil {
		t.Fatalf("band.json: %v", err)
	}
	if bandJSON.ID != "band-uuid-0004" {
		t.Errorf("export did not emit the band id: %q", bandJSON.ID)
	}
	var slFile struct {
		Setlists []struct {
			ID string `json:"id"`
		} `json:"setlists"`
	}
	if err := json.Unmarshal(files["setlists.json"], &slFile); err != nil {
		t.Fatalf("setlists.json: %v", err)
	}
	if len(slFile.Setlists) != 1 || slFile.Setlists[0].ID != "setlist-uuid-0004" {
		t.Errorf("export did not emit the setlist id: %+v", slFile.Setlists)
	}
}
