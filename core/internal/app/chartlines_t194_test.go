package app_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// T194 — line identity at the app level: ids are minted at create, carried by every save, kept through a
// transposition, survive a restart, a backup copy of the data dir and a band export → import, and are
// re-derived (flagged) by the writes that bypass SaveChartSource. Synthetic text only.

const t194Chart = "# The Open Road\nThe Wayfarers\n\n## Verse 1\nG C D\nWe drive the road until the evening\nOh the opn road is calling\nUnder the long grey sky\n"

type t194World struct {
	st     stack
	owner  app.User
	bandID string
	songID string
	file   app.SongFile
}

func newT194World(t *testing.T, st stack) t194World {
	t.Helper()
	owner, err := st.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := st.svc.CreateBand(owner, "Demo Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := st.svc.CreateSong(owner, band.ID, "The Open Road", "Demo")
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.svc.CreateTextChart(owner, band.ID, song.ID, t194Chart)
	if err != nil {
		t.Fatal(err)
	}
	return t194World{st, owner, band.ID, song.ID, f}
}

func (w *t194World) save(t *testing.T, src string) {
	t.Helper()
	f, err := w.st.svc.SaveChartSource(w.owner, w.bandID, w.songID, w.file.ID, w.file.Revision, src)
	if err != nil {
		t.Fatal(err)
	}
	w.file = f
}

func (w *t194World) lines(t *testing.T) app.ChartLineIdentity {
	t.Helper()
	li, err := w.st.repo.GetChartLineIdentity(w.file.ID)
	if err != nil {
		t.Fatalf("no stored line identity: %v", err)
	}
	return li
}

func t194Insert(src string, at int, text string) string {
	l := chartpdf.SourceLines(src)
	return strings.Join(append(append(append([]string{}, l[:at]...), text), l[at:]...), "\n")
}

func TestChartLines_CreateMintsFreshIDs(t *testing.T) {
	w := newT194World(t, newStack())
	li := w.lines(t)
	if !li.Describes(t194Chart) || li.Rederived != nil {
		t.Fatalf("create: %+v", li)
	}
	seen := map[string]bool{}
	for _, id := range li.IDs {
		if id == "" || seen[id] {
			t.Fatalf("ids not unique and non-empty: %v", li.IDs)
		}
		seen[id] = true
	}
	// A second chart gets its own ids.
	f2, err := w.st.svc.CreateTextChart(w.owner, w.bandID, w.songID, t194Chart)
	if err != nil {
		t.Fatal(err)
	}
	li2, _ := w.st.repo.GetChartLineIdentity(f2.ID)
	for _, id := range li2.IDs {
		if seen[id] {
			t.Fatal("two charts share a line id")
		}
	}
}

func TestChartLines_SaveCarriesIDsAndRecordsRenders(t *testing.T) {
	w := newT194World(t, newStack())
	before := w.lines(t)
	oldRender := w.file.BlobHash
	w.save(t, t194Insert(t194Chart, 5, "A new line above the road"))
	after := w.lines(t)
	want := append(append(append([]string{}, before.IDs[:5]...), after.IDs[5]), before.IDs[5:]...)
	if !reflect.DeepEqual(after.IDs, want) {
		t.Fatalf("ids after insert-above %v, want %v", after.IDs, want)
	}
	st := after.History[len(after.History)-1]
	if st.FromRender != oldRender || st.ToRender != w.file.BlobHash {
		t.Fatalf("step renders %s→%s, want %s→%s", st.FromRender, st.ToRender, oldRender, w.file.BlobHash)
	}
	// A typo fix keeps the id, with a char map from the render a mark was drawn on.
	typoed := w.file.BlobHash
	src2 := strings.Replace(t194Insert(t194Chart, 5, "A new line above the road"), "opn road", "open road", 1)
	w.save(t, src2)
	li := w.lines(t)
	id := before.IDs[6] // "Oh the opn road is calling"
	loc, ok, err := w.st.svc.LocateChartLine(w.file.ID, id, typoed)
	if err != nil || !ok {
		t.Fatalf("locate: ok=%v err=%v", ok, err)
	}
	if loc.Index != 7 || loc.Text != "Oh the open road is calling" || !loc.CharMapKnown || len(loc.Runs) != 1 {
		t.Fatalf("loc %+v", loc)
	}
	if g, e := loc.CharMap.Map(11); g != 12 || !e { // "road"
		t.Fatalf("'road' maps to %d,%v; want 12", g, e)
	}
	// From the very first render, the composed map still answers.
	if loc0, _, _ := w.st.svc.LocateChartLine(w.file.ID, id, oldRender); !loc0.CharMapKnown {
		t.Fatal("history from the creation render is lost")
	}
	if li.IDs[7] != id {
		t.Fatal("typo fix changed the id")
	}
	// A deleted line is not located (P20).
	w.save(t, strings.Replace(src2, "Oh the open road is calling\n", "", 1))
	if _, ok, err := w.st.svc.LocateChartLine(w.file.ID, id, w.file.BlobHash); ok || err != nil {
		t.Fatalf("deleted line located: ok=%v err=%v", ok, err)
	}
}

func TestChartLines_TranspositionKeepsChordRowIDs(t *testing.T) {
	w := newT194World(t, newStack())
	if _, err := w.st.svc.UpdateSong(w.owner, w.bandID, w.songID, app.SongPatch{Key: strPtr("G")}); err != nil {
		t.Fatal(err)
	}
	before := w.lines(t)
	f, src, err := w.st.svc.TransposeChartSource(w.owner, w.bandID, w.songID, w.file.ID, "A", nil, false, w.file.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "A D E") {
		t.Fatalf("fixture: transposed source %q", src)
	}
	w.file = f
	if after := w.lines(t); !reflect.DeepEqual(after.IDs, before.IDs) {
		t.Fatalf("transposition changed ids: %v → %v", before.IDs, after.IDs)
	}
}

func strPtr(s string) *string { return &s }

// A chart that predates T194 has no ids: they are backfilled (flagged) on first use and stable after.
func TestChartLines_BackfillIsFlaggedAndStable(t *testing.T) {
	w := newT194World(t, newStack())
	if err := w.st.repo.DeleteChartLineIdentity(w.file.ID); err != nil {
		t.Fatal(err)
	}
	li, err := w.st.svc.ChartLines(w.file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if li.Rederived == nil || li.Rederived.Reason != chartpdf.RederiveBackfill || li.Rederived.Render != w.file.BlobHash {
		t.Fatalf("backfill flag %+v", li.Rederived)
	}
	again, _ := w.st.svc.ChartLines(w.file.ID)
	if !reflect.DeepEqual(again.IDs, li.IDs) {
		t.Fatal("backfilled ids changed between reads")
	}
	if err := w.st.svc.ClearChartLinesRederived(w.file.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := w.st.svc.ChartLines(w.file.ID); c.Rederived != nil || !reflect.DeepEqual(c.IDs, li.IDs) {
		t.Fatal("clear did not clear, or changed ids")
	}
	// A first save on a chart without ids backfills the OLD source, then carries: the untouched lines keep
	// the backfilled ids.
	if err := w.st.repo.DeleteChartLineIdentity(w.file.ID); err != nil {
		t.Fatal(err)
	}
	w.save(t, t194Chart+"One more line\n")
	saved := w.lines(t)
	if saved.Rederived == nil || saved.Rederived.Reason != chartpdf.RederiveBackfill {
		t.Fatalf("save after backfill: flag %+v", saved.Rederived)
	}
	// Ids that no longer describe the source (some writer changed it alone) are re-derived, not trusted.
	if err := w.st.repo.SetChartSource(w.file.ID, t194Chart+"Edited behind its back\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.svc.ChartLines(w.file.ID); err != nil {
		t.Fatal(err)
	}
	if m, _ := w.st.repo.GetChartLineIdentity(w.file.ID); m.Rederived == nil || m.Rederived.Reason != chartpdf.RederiveMismatch {
		t.Fatalf("mismatch flag %+v", m.Rederived)
	}
}

func TestChartLines_DeleteFileDropsIDs(t *testing.T) {
	w := newT194World(t, newStack())
	if err := w.st.svc.DeleteSongFile(w.owner, w.bandID, w.songID, w.file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.repo.GetChartLineIdentity(w.file.ID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("ids survived the file: %v", err)
	}
}

func fileStack(t *testing.T, dir string) stack {
	t.Helper()
	repo, err := filerepo.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFile(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	return stack{app.NewService(repo).WithBlobStore(blobs), repo, blobs, engine.New(memstore.New().(store.HistoryAware))}
}

// Save → restart (reload app.json) → the same ids; a backup copy of the data dir opens with them too.
func TestChartLines_SurviveReloadAndBackup(t *testing.T) {
	dir := t.TempDir()
	w := newT194World(t, fileStack(t, dir))
	w.save(t, t194Insert(t194Chart, 5, "A new line above the road"))
	want := w.lines(t)

	re, err := filerepo.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := re.GetChartLineIdentity(w.file.ID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("after reload: %+v (%v), want %+v", got, err, want)
	}

	backup := t.TempDir()
	b, err := os.ReadFile(filepath.Join(dir, "app.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"chartLines"`) {
		t.Fatal("app.json carries no chartLines")
	}
	if err := os.WriteFile(filepath.Join(backup, "app.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := filerepo.New(backup)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := restored.GetChartLineIdentity(w.file.ID); err != nil || !reflect.DeepEqual(got.IDs, want.IDs) {
		t.Fatalf("after backup restore: %+v (%v)", got, err)
	}

	// An app.json from before T194 (no chartLines key) loads, and the chart is backfilled on use.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "chartLines")
	old, _ := json.Marshal(raw)
	pre := t.TempDir()
	if err := os.WriteFile(filepath.Join(pre, "app.json"), old, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pre, "blobs"), 0o700); err != nil {
		t.Fatal(err)
	}
	pst := fileStack(t, pre)
	if _, err := pst.repo.GetChartLineIdentity(w.file.ID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("pre-T194 app.json: %v", err)
	}
}

func exportImport(t *testing.T, from stack, owner app.User, bandID string, mutate func(map[string][]byte), into stack, importer app.User) app.ImportReport {
	t.Helper()
	z, _, err := from.svc.ExportBand(owner, from.eng, bandID)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		files := unzip(t, z)
		mutate(files)
		z = rezip(t, files)
	}
	rep, err := into.svc.ImportBand(importer, into.eng, z, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return rep
}

func importedChart(t *testing.T, st stack, bandID string) (app.SongFile, app.ChartLineIdentity) {
	t.Helper()
	songs, _ := st.repo.SongsOfBand(bandID)
	for _, s := range songs {
		files, _ := st.repo.FilesOfSong(s.ID)
		for _, f := range files {
			if f.Generated {
				li, err := st.repo.GetChartLineIdentity(f.ID)
				if err != nil {
					t.Fatalf("imported chart has no ids: %v", err)
				}
				return f, li
			}
		}
	}
	t.Fatal("no generated chart imported")
	return app.SongFile{}, app.ChartLineIdentity{}
}

// Export → import keeps every id exactly, unflagged; the folder carries them in repertoire.json.
func TestChartLines_SurviveExportImport(t *testing.T) {
	w := newT194World(t, newStack())
	w.save(t, t194Insert(t194Chart, 5, "A new line above the road"))
	want := w.lines(t)

	var sawIDs bool
	tgt := newStack()
	imp, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep := exportImport(t, w.st, w.owner, w.bandID, func(files map[string][]byte) {
		sawIDs = strings.Contains(string(files["repertoire.json"]), want.IDs[0])
	}, tgt, imp)
	if !sawIDs {
		t.Fatal("repertoire.json does not carry the line ids")
	}
	_, got := importedChart(t, tgt, rep.Band.ID)
	if !reflect.DeepEqual(got.IDs, want.IDs) || got.Rederived != nil {
		t.Fatalf("after import: ids %v flag %+v, want %v unflagged", got.IDs, got.Rederived, want.IDs)
	}
}

// A folder whose source was edited by hand (its ids now describe other text) is re-derived and flagged;
// a re-import over the same stable file id with a changed source carries the old ids by diff, flagged.
func TestChartLines_ImportRederivesUntrustedIDs(t *testing.T) {
	w := newT194World(t, newStack())
	editSource := func(files map[string][]byte) {
		var rep map[string]any
		if err := json.Unmarshal(files["repertoire.json"], &rep); err != nil {
			t.Fatal(err)
		}
		for _, s := range rep["songs"].([]any) {
			sm := s.(map[string]any)
			for _, f := range sm["files"].([]any) {
				fm := f.(map[string]any)
				if fm["generated"] == true {
					delete(fm, "blobHash") // a hand edit leaves no valid hash
					name := sm["slug"].(string) + "/" + fm["filename"].(string)
					files[name] = []byte(t194Insert(string(files[name]), 5, "A line typed in the folder"))
				}
			}
		}
		files["repertoire.json"], _ = json.Marshal(rep)
	}
	tgt := newStack()
	imp, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep := exportImport(t, w.st, w.owner, w.bandID, editSource, tgt, imp)
	f, got := importedChart(t, tgt, rep.Band.ID)
	if got.Rederived == nil || got.Rederived.Reason != chartpdf.RederiveImport || got.Rederived.Render != f.BlobHash {
		t.Fatalf("hand-edited folder: flag %+v", got.Rederived)
	}
	src, _ := tgt.repo.GetChartSource(f.ID)
	if !got.Describes(src) {
		t.Fatal("re-derived ids do not describe the imported source")
	}

	// Re-import over the same stable ids, source changed again: carried by diff, flagged import-changed.
	_ = tgt.svc.ClearChartLinesRederived(f.ID)
	before, _ := tgt.repo.GetChartLineIdentity(f.ID)
	rep2 := exportImport(t, tgt, imp, rep.Band.ID, editSource, tgt, imp)
	f2, again := importedChart(t, tgt, rep2.Band.ID)
	if f2.ID != f.ID {
		t.Fatalf("fixture: re-import did not reuse the stable file id (%s vs %s)", f2.ID, f.ID)
	}
	if again.Rederived == nil || again.Rederived.Reason != chartpdf.RederiveImportChanged {
		t.Fatalf("re-import flag %+v", again.Rederived)
	}
	if again.IDs[0] != before.IDs[0] || again.IDs[len(again.IDs)-2] != before.IDs[len(before.IDs)-2] {
		t.Fatalf("re-import did not carry ids: %v → %v", before.IDs, again.IDs)
	}
	// And an unchanged re-import changes nothing.
	_ = tgt.svc.ClearChartLinesRederived(f.ID)
	steady, _ := tgt.repo.GetChartLineIdentity(f.ID)
	exportImport(t, tgt, imp, rep.Band.ID, nil, tgt, imp)
	if _, now := importedChart(t, tgt, rep.Band.ID); !reflect.DeepEqual(now.IDs, steady.IDs) || now.Rederived != nil {
		t.Fatalf("unchanged re-import: %v flag %+v", now.IDs, now.Rederived)
	}
}
