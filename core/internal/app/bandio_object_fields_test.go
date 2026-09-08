package app_test

import (
	"reflect"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// The v2 band folder writes and reads `domain.Object` FIELD BY FIELD (v2Object, both directions). That
// mirror has now silently dropped a newly added field three times — T86's meter (on the song), T152's band
// identity, and P206's jumpTo — and each was fixed with a guard for THAT field. A per-field guard only
// protects fields someone already thought about, and the field that gets dropped is by definition the one
// nobody thought about (Fable, ⟨GO⟩ f424d238).
//
// So this test enumerates the SOURCE TYPE instead: every field of domain.Object is filled with a
// distinctive non-zero value, round-tripped through a real export → import, and compared back. A field
// added to domain.Object tomorrow is covered the moment it exists, with nobody remembering anything.
//
// A field that legitimately must NOT survive belongs in `notCarried` below, with the reason — a deliberate
// exclusion someone had to justify in writing, which is the opposite of an omission that looks like
// nothing at all.
var notCarried = map[string]string{
	// A tombstone is not exported at all (head-only, no history) — an object that comes back is by
	// definition not deleted.
	"Deleted": "tombstones are dropped by design: manifestAnnots carries the live head",
	// The importing server mints its own accounts (T63 dispositions), so ownership is REMAPPED rather
	// than copied. Asserted separately below: it must still be a non-empty id, never silently blanked.
	"OwnerID": "remapped to the target server's user id (T63), never copied verbatim",
	// The engine stamps the version of the mutation it applied; the source's counter is not a value the
	// target's history can adopt.
	"Version": "re-stamped by the importing engine's own history",
}

// fill sets every field of v (a struct pointer) to a distinctive non-zero value, recursively.
func fill(t *testing.T, v reflect.Value, seed int) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		seed += 7
		switch f.Kind() {
		case reflect.String:
			f.SetString("v" + string(rune('a'+seed%26)) + "-round-trip")
		case reflect.Int, reflect.Int32, reflect.Int64:
			f.SetInt(int64(seed))
		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			f.SetUint(uint64(seed))
		case reflect.Float32, reflect.Float64:
			f.SetFloat(float64(seed) / 100)
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Ptr:
			p := reflect.New(f.Type().Elem())
			if p.Elem().Kind() == reflect.Struct {
				fill(t, p.Elem(), seed)
			} else if p.Elem().Kind() == reflect.Bool {
				p.Elem().SetBool(true)
			}
			f.Set(p)
		case reflect.Struct:
			fill(t, f, seed)
		case reflect.Slice:
			e := reflect.New(f.Type().Elem()).Elem()
			if e.Kind() == reflect.Struct {
				fill(t, e, seed)
			}
			f.Set(reflect.Append(reflect.MakeSlice(f.Type(), 0, 1), e))
		}
	}
}

func TestBandFolder_RoundTripsEveryObjectField(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, src)

	var want domain.Object
	fill(t, reflect.ValueOf(&want).Elem(), 0)
	// The fields the round-trip is ALLOWED to key on rather than invent: the object must land on a layer
	// that exists in the export, and every ENUM must hold a real member — the wire maps enums by string,
	// so a filler's arbitrary int would round-trip to zero and read as a bug in the format.
	want.UUID = "every-field"
	want.LayerID = "L-shared"
	want.Type = domain.TypeIcon
	want.Scope = domain.ScopePart
	want.Deleted = false
	want.Version = 1
	want.OwnerID = admin.ID
	if _, err := src.eng.Apply(songID, domain.Mutation{Kind: domain.KindCreate, UUID: want.UUID, AuthorID: admin.ID, Object: &want}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	var got *domain.Object
	for _, o := range snap.LiveObjects() {
		if o.UUID == want.UUID {
			cp := o
			got = &cp
		}
	}
	if got == nil {
		t.Fatalf("the object did not survive the round-trip at all")
	}

	wv, gv := reflect.ValueOf(want), reflect.ValueOf(*got)
	ty := wv.Type()
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if _, skip := notCarried[name]; skip {
			continue
		}
		if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
			t.Errorf("domain.Object.%s did not survive the band-folder round-trip:\n  wrote %#v\n  read  %#v\n"+
				"Either carry it in v2Object (both directions), or add it to notCarried with the reason.",
				name, wv.Field(i).Interface(), gv.Field(i).Interface())
		}
	}
	// OwnerID is remapped, not copied — but it must still name somebody.
	if got.OwnerID == "" {
		t.Errorf("OwnerID was blanked by the round-trip; it is remapped (T63), not dropped")
	}
}

// The same instrument, pointed at the OTHER hand-maintained mirror in this file (Fable's ⟨state⟩ item 4).
// `v2Layer` is built field by field in both directions exactly like v2Object, so it can lose a field just
// as silently — and a layer's fields are the visibility rules (zone, access, mandatory, roleTag), where a
// silent loss means a private layer coming back shared.
var layerNotCarried = map[string]string{
	// The importing server mints its own accounts (T63 dispositions); ownership is REMAPPED, not copied.
	// Asserted separately below as non-empty, never silently blanked.
	"OwnerID": "remapped to the target server's user id (T63), never copied verbatim",
	// A layer binds to a FILE, and the target mints new file ids; the folder carries the FILENAME instead.
	// Asserted separately: it must still point at a real file of the imported song.
	"FileID": "remapped: the folder names the file, the importer resolves it to the new file's id",
}

func TestBandFolder_RoundTripsEveryLayerField(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, file1, _ := buildSourceBand(t, src)

	var want domain.Layer
	fill(t, reflect.ValueOf(&want).Elem(), 3)
	want.ID = "every-layer-field"
	want.FileID = file1
	want.OwnerID = admin.ID
	want.Zone = domain.ZoneShared // enums must hold REAL members: the wire maps them by string
	want.Access = domain.AccessRO // …and RO is the interesting one — a lost access is a leak
	if _, err := src.eng.Apply(songID, domain.Mutation{Kind: domain.KindLayerCreate, Layer: &want, AuthorID: admin.ID}); err != nil {
		t.Fatalf("seed layer: %v", err)
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	var got *domain.Layer
	for i, l := range snap.Layers {
		if l.ID == want.ID {
			got = &snap.Layers[i]
		}
	}
	if got == nil {
		t.Fatalf("the layer did not survive the round-trip at all")
	}

	wv, gv := reflect.ValueOf(want), reflect.ValueOf(*got)
	ty := wv.Type()
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if _, skip := layerNotCarried[name]; skip {
			continue
		}
		if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
			t.Errorf("domain.Layer.%s did not survive the band-folder round-trip:\n  wrote %#v\n  read  %#v\n"+
				"Either carry it in v2Layer (both directions), or add it to layerNotCarried with the reason.",
				name, wv.Field(i).Interface(), gv.Field(i).Interface())
		}
	}
	if got.OwnerID == "" {
		t.Errorf("OwnerID was blanked; it is remapped (T63), not dropped — a shared-by-accident layer")
	}
	files, err := tgt.svc.SongFiles(importer, rep.Band.ID, songs[0].ID)
	if err != nil {
		t.Fatalf("song files: %v", err)
	}
	found := false
	for _, f := range files {
		if f.ID == got.FileID {
			found = true
		}
	}
	if !found {
		t.Errorf("FileID %q resolves to no file of the imported song — the layer lost its part", got.FileID)
	}
}

// And the third hand-maintained mirror: v2Song. T86's dropped `meter` was THIS one — the loss that started
// the pattern — so it gets the same treatment rather than the one-field guard it was given at the time.
var songNotCarried = map[string]string{
	"ID":        "the importing server mints its own song ids; the folder keys songs by slug",
	"BandID":    "the import creates a new band; asserted separately as the imported band's id",
	"CreatedAt": "stamped by the importing server — a folder is a description, not a history",
}

func TestBandFolder_RoundTripsEverySongField(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, src)

	var want app.Song
	fill(t, reflect.ValueOf(&want).Elem(), 11)
	// Some fields cannot be an arbitrary filler value and still mean anything: the title is the folder's
	// identity, the slug is DERIVED from it server-side (so it is read back from the source, not invented),
	// the meter is canonicalised (T86), and a tag must be a real word — an empty one is not stored.
	want.Title = "Every Field"
	want.Tags = []string{"britpop"}
	patch := app.SongPatch{
		Artist: &want.Artist, Key: &want.Key, Tempo: &want.Tempo, Meter: &want.Meter,
		Tags: &want.Tags, Notes: &want.Notes,
	}
	want.Meter = "6/8" // meter is canonicalised (T86), so a filler string would not round-trip as itself
	if _, err := src.svc.UpdateSong(admin, bandID, songID, app.SongPatch{Title: &want.Title}); err != nil {
		t.Fatalf("title: %v", err)
	}
	if _, err := src.svc.UpdateSong(admin, bandID, songID, patch); err != nil {
		t.Fatalf("patch: %v", err)
	}
	srcSongs, _ := src.repo.SongsOfBand(bandID)
	for _, sg := range srcSongs {
		if sg.ID == songID {
			want.Slug = sg.Slug // derived from the title by the service; the folder is named by it
		}
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, err := tgt.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	if len(songs) != 1 {
		t.Fatalf("want the one imported song, got %d", len(songs))
	}
	got := songs[0]

	wv, gv := reflect.ValueOf(want), reflect.ValueOf(got)
	ty := wv.Type()
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if _, skip := songNotCarried[name]; skip {
			continue
		}
		if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
			t.Errorf("Song.%s did not survive the band-folder round-trip:\n  wrote %#v\n  read  %#v\n"+
				"Either carry it in v2Song (both directions), or add it to songNotCarried with the reason.",
				name, wv.Field(i).Interface(), gv.Field(i).Interface())
		}
	}
	if got.BandID != rep.Band.ID {
		t.Errorf("the imported song belongs to band %q, want the imported band %q", got.BandID, rep.Band.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt is zero — it is re-stamped on import, not dropped")
	}
}
