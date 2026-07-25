package app_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

type stack struct {
	svc   *app.Service
	repo  app.Repo
	blobs blob.Store
	eng   *engine.Engine
}

func newStack() stack {
	repo := memrepo.New()
	blobs := blob.NewMem()
	svc := app.NewService(repo).WithBlobStore(blobs)
	eng := engine.New(memstore.New().(store.HistoryAware))
	return stack{svc, repo, blobs, eng}
}

// buildSourceBand seeds a band with the full T62 acceptance surface.
func buildSourceBand(t *testing.T, st stack) (admin, member app.User, bandID, songID, file1, file2 string) {
	t.Helper()
	admin, err := st.svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(admin, "The Troubadours")
	if err != nil {
		t.Fatal(err)
	}
	member, err = st.svc.Register("leo", "Leo", "password123", "leo@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.repo.AddMembership(app.Membership{BandID: band.ID, UserID: member.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	song, err := st.svc.CreateSong(admin, band.ID, "Wonderwall", "Oasis")
	if err != nil {
		t.Fatal(err)
	}
	key, tempo, tags, notes := "G", 87, []string{"britpop", "closer"}, "capo 2"
	if _, err := st.svc.UpdateSong(admin, band.ID, song.ID, app.SongPatch{Key: &key, Tempo: &tempo, Tags: &tags, Notes: &notes}); err != nil {
		t.Fatal(err)
	}
	f1, err := st.svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 the score"))
	if err != nil {
		t.Fatal(err)
	}
	f2, err := st.svc.CreateTextChart(admin, band.ID, song.ID, "# Wonderwall\n## Verse\nEm7          G\nlyrics here\n")
	if err != nil {
		t.Fatal(err)
	}
	apply := func(m domain.Mutation) {
		if _, err := st.eng.Apply(song.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	apply(domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L-shared", FileID: f1.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Order: 0, Access: domain.AccessRW,
	}})
	apply(domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: member.ID, Layer: &domain.Layer{
		ID: "L-mine", FileID: f2.ID, Name: "Leo notes", OwnerID: member.ID, Zone: domain.ZonePersonal, Order: 1, Access: domain.AccessRW,
	}})
	apply(domain.Mutation{Kind: domain.KindCreate, UUID: "o1", AuthorID: admin.ID, Object: &domain.Object{
		UUID: "o1", LayerID: "L-shared", Type: domain.TypeRect, Page: 0, Version: 1,
		Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}}, Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004}, OwnerID: admin.ID,
	}})
	apply(domain.Mutation{Kind: domain.KindCreate, UUID: "o2", AuthorID: member.ID, Object: &domain.Object{
		UUID: "o2", LayerID: "L-mine", Type: domain.TypeRect, Page: 0, Version: 1,
		Points: []domain.Point{{X: 0.3, Y: 0.4}, {X: 0.6, Y: 0.5}}, Style: domain.Style{Color: "#2563eb", Opacity: 1, Width: 0.004}, OwnerID: member.ID,
	}})
	if _, err := st.svc.SetMyCues(member, band.ID, song.ID, []app.SongCue{{Icon: "mic"}, {Icon: "guitar-electric", Color: "#e11d48"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.svc.SetMyFileSelection(member, band.ID, song.ID, []string{f2.ID, f1.ID}); err != nil {
		t.Fatal(err)
	}
	sl, err := st.svc.CreateSetlist(admin, band.ID, "Sat @ The Anchor", "2026-07-04", "The Anchor Pub", "60-min set")
	if err != nil {
		t.Fatal(err)
	}
	item, err := st.svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID)
	if err != nil {
		t.Fatal(err)
	}
	over, yes := "A", true
	if _, err := st.svc.UpdateSetlistItem(admin, band.ID, sl.ID, item.ID, app.SetlistItemPatch{KeyOverride: &over, TransposeChords: &yes, OnCall: &yes}); err != nil {
		t.Fatal(err)
	}
	return admin, member, band.ID, song.ID, f1.ID, f2.ID
}

func TestBandExportImport_RoundTrip(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)

	zipBytes, filename, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(zipBytes) == 0 || filename == "" {
		t.Fatal("empty export")
	}

	// Import into a FRESH server. Pre-create "leo" (username match) so marie is created.
	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tgt.svc.Register("leo", "Leo On Target", "password123", "leo@target.com"); err != nil {
		t.Fatal(err)
	}

	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !contains(rep.Matched, "leo") || !contains(rep.Created, "marie") {
		t.Fatalf("member report matched=%v created=%v, want leo matched + marie created", rep.Matched, rep.Created)
	}
	if rep.Songs != 1 || rep.Files != 2 || rep.Setlists != 1 {
		t.Fatalf("counts songs=%d files=%d setlists=%d, want 1/2/1", rep.Songs, rep.Files, rep.Setlists)
	}
	nb, role, err := tgt.svc.GetBand(importer, rep.Band.ID)
	if err != nil || role != app.RoleAdmin || nb.OwnerID != importer.ID {
		t.Fatalf("imported band owner=%q role=%v err=%v", nb.OwnerID, role, err)
	}

	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	if len(songs) != 1 {
		t.Fatalf("want 1 song, got %d", len(songs))
	}
	ns := songs[0]
	if ns.Title != "Wonderwall" || ns.Key != "G" || ns.Tempo != 87 || ns.Notes != "capo 2" || len(ns.Tags) != 2 {
		t.Fatalf("song metadata not preserved: %+v", ns)
	}
	files, _ := tgt.repo.FilesOfSong(ns.ID)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
	var genFileID, newF1 string
	for _, f := range files {
		if f.Filename == "score.pdf" {
			newF1 = f.ID
			data, err := tgt.blobs.Get(f.BlobHash)
			if err != nil || string(data) != "%PDF-1.4 the score" {
				t.Fatalf("pdf blob not byte-identical: %q err=%v", string(data), err)
			}
		}
		if f.Generated {
			genFileID = f.ID
			if src, err := tgt.repo.GetChartSource(f.ID); err != nil || src == "" {
				t.Fatalf("chart source not imported: %v", err)
			}
		}
	}
	if genFileID == "" || newF1 == "" {
		t.Fatal("expected an uploaded + a generated file")
	}

	snap, err := tgt.eng.Head(ns.ID)
	if err != nil || len(snap.Layers) != 2 {
		t.Fatalf("annotations: %d layers, err=%v", len(snap.Layers), err)
	}
	leoTarget, _ := tgt.repo.GetUserByUsername("leo")
	for _, l := range snap.Layers {
		if l.FileID != newF1 && l.FileID != genFileID {
			t.Fatalf("layer FileID %q not rewritten to a new file id", l.FileID)
		}
		if l.ID == "L-shared" && l.OwnerID != domain.SharedOwner {
			t.Fatalf("shared layer owner rewritten to %q, want SharedOwner", l.OwnerID)
		}
		if l.ID == "L-mine" && l.OwnerID != leoTarget.ID {
			t.Fatalf("personal layer owner %q, want leo's target id %q", l.OwnerID, leoTarget.ID)
		}
	}
	live := 0
	for _, o := range snap.Objects {
		if !o.Deleted {
			live++
		}
	}
	if live != 2 {
		t.Fatalf("want 2 objects, got %d", live)
	}

	cues, _ := tgt.repo.GetSongCues(leoTarget.ID, ns.ID)
	if len(cues.Cues) != 2 || cues.Cues[0].Icon != "mic" {
		t.Fatalf("cues not imported under leo: %+v", cues.Cues)
	}
	sel, err := tgt.repo.GetFileSelection(leoTarget.ID, ns.ID)
	if err != nil || len(sel.FileIDs) != 2 {
		t.Fatalf("selection not imported: %+v err=%v", sel, err)
	}

	setlists, _ := tgt.repo.SetlistsOfBand(rep.Band.ID)
	items, _ := tgt.repo.ItemsOfSetlist(setlists[0].ID)
	if len(items) != 1 || items[0].KeyOverride != "A" || !items[0].TransposeChords || !items[0].OnCall {
		t.Fatalf("setlist item overrides not preserved: %+v", items)
	}
}

func TestBandExport_AdminOnly(t *testing.T) {
	src := newStack()
	_, member, bandID, _, _, _ := buildSourceBand(t, src)
	if _, _, err := src.svc.ExportBand(member, src.eng, bandID); err != app.ErrForbidden {
		t.Fatalf("non-admin export err=%v, want ErrForbidden", err)
	}
}

func TestBandImport_AllOrNothing(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)
	good, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func([]byte) []byte{
		"formatVersion": func(z []byte) []byte { return retamper(t, z, func(m map[string]any) { m["formatVersion"] = 2 }) },
		"missing-blob":  func(z []byte) []byte { return dropBlob(t, z) },
		"bad-hash":      func(z []byte) []byte { return corruptBlob(t, z) },
	}
	for name, mangle := range cases {
		tgt := newStack()
		importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
		if _, err := tgt.svc.ImportBand(importer, tgt.eng, mangle(good)); err == nil {
			t.Fatalf("%s: import should have failed", name)
		}
		if bands, _ := tgt.repo.BandsForUser(importer.ID); len(bands) != 0 {
			t.Fatalf("%s: %d bands created, want 0 (all-or-nothing)", name, len(bands))
		}
	}
}

// TestBandImport_EmailCollision_Rejected: a would-create member whose email already
// belongs to a DIFFERENT account on the target server is rejected up front (400), with
// nothing written — the collision would otherwise fail at CreateUser mid-import, leaving
// a half-created band (the all-or-nothing hole Fable flagged on the T62 gate review).
func TestBandImport_EmailCollision_Rejected(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)
	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatal(err)
	}

	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "owner@x.com")
	if err != nil {
		t.Fatal(err)
	}
	// A DIFFERENT account (not "marie") already owns marie's email on this server.
	if _, err := tgt.svc.Register("mallory", "Mallory", "password123", "marie@x.com"); err != nil {
		t.Fatal(err)
	}

	if _, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("import err=%v, want ErrInvalidInput (email collision)", err)
	}
	// All-or-nothing: no band, and marie was never created.
	if bands, _ := tgt.repo.BandsForUser(importer.ID); len(bands) != 0 {
		t.Fatalf("%d bands created, want 0 (all-or-nothing)", len(bands))
	}
	if _, err := tgt.repo.GetUserByUsername("marie"); err == nil {
		t.Fatal("marie account was created despite the email collision")
	}
}

// --- helpers ---

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func unzip(t *testing.T, z []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(z), int64(len(z)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = data
	}
	return out
}

func rezip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	zw.Close()
	return buf.Bytes()
}

func retamper(t *testing.T, z []byte, mut func(map[string]any)) []byte {
	files := unzip(t, z)
	var m map[string]any
	if err := json.Unmarshal(files["band.json"], &m); err != nil {
		t.Fatal(err)
	}
	mut(m)
	files["band.json"], _ = json.Marshal(m)
	return rezip(t, files)
}

func dropBlob(t *testing.T, z []byte) []byte {
	files := unzip(t, z)
	for name := range files {
		if len(name) > 6 && name[:6] == "blobs/" {
			delete(files, name)
			break
		}
	}
	return rezip(t, files)
}

func corruptBlob(t *testing.T, z []byte) []byte {
	files := unzip(t, z)
	for name := range files {
		if len(name) > 6 && name[:6] == "blobs/" {
			files[name] = append([]byte("corrupted "), files[name]...)
			break
		}
	}
	return rezip(t, files)
}
