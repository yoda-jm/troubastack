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

// The fourth and last hand-maintained mirror pair in this file: v2Setlist / v2SetlistItem. A setlist is
// the thing the band plays FROM, and an item's fields are the per-performance decisions (the key it is in,
// whether the chart is transposed, whether it is on the bench, whether it is a break at all). A silent
// loss here is a gig played in the wrong key.
var setlistNotCarried = map[string]string{
	"ID":        "the folder declares its own id (T150) and the importer honours it; asserted separately",
	"BandID":    "the import creates a new band; asserted separately as the imported band's id",
	"CreatedAt": "stamped by the importing server — a folder is a description, not a history",
	"LiveUntil": "rehearsal live mode is a self-expiring RUNTIME flag (P201); a folder must never carry a deadline",
	"LiveBy":    "the other half of live mode, and json:\"-\" — never leaves the server it was set on",
}

var itemNotCarried = map[string]string{
	"ID":        "the importing server mints item ids",
	"SetlistID": "…and setlist ids; asserted separately as the imported setlist's",
	"SongID":    "the folder references a song by SLUG, not by id; asserted separately as the imported song's",
	"Position":  "carried by ARRAY ORDER, not as a field (T140); asserted separately as 0..n-1 in order",
}

func TestBandFolder_RoundTripsEverySetlistField(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, _, _ := buildSourceBand(t, src)

	// buildSourceBand leaves one setlist with one item; fill the rest of the surface: every override on
	// the song item, plus an intermission, whose Kind/Label are the T153 fields most likely to be missed.
	sls, err := src.svc.Setlists(admin, bandID)
	if err != nil || len(sls) != 1 {
		t.Fatalf("setlists: %v (%d)", err, len(sls))
	}
	sl := sls[0]
	detail, err := src.svc.Setlist(admin, bandID, sl.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	tempo, notes := 137, "second half, capo 2"
	if _, err := src.svc.UpdateSetlistItem(admin, bandID, sl.ID, detail.Items[0].ID,
		app.SetlistItemPatch{TempoOverride: &tempo, Notes: &notes}); err != nil {
		t.Fatalf("patch item: %v", err)
	}
	if _, err := src.svc.AddSetlistIntermission(admin, bandID, sl.ID, "Fifteen minutes"); err != nil {
		t.Fatalf("intermission: %v", err)
	}
	want, err := src.svc.Setlist(admin, bandID, sl.ID)
	if err != nil {
		t.Fatalf("re-read: %v", err)
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
	gotLists, err := tgt.svc.Setlists(importer, rep.Band.ID)
	if err != nil || len(gotLists) != 1 {
		t.Fatalf("imported setlists: %v (%d)", err, len(gotLists))
	}
	got, err := tgt.svc.Setlist(importer, rep.Band.ID, gotLists[0].ID)
	if err != nil {
		t.Fatalf("imported detail: %v", err)
	}

	compare := func(label string, w, g reflect.Value, skip map[string]string) {
		ty := w.Type()
		for i := 0; i < ty.NumField(); i++ {
			name := ty.Field(i).Name
			if _, s := skip[name]; s {
				continue
			}
			if !reflect.DeepEqual(w.Field(i).Interface(), g.Field(i).Interface()) {
				t.Errorf("%s.%s did not survive the band-folder round-trip:\n  wrote %#v\n  read  %#v\n"+
					"Either carry it in the v2 shape (both directions), or add it to the skip map with the reason.",
					label, name, w.Field(i).Interface(), g.Field(i).Interface())
			}
		}
	}
	compare("Setlist", reflect.ValueOf(want.Setlist), reflect.ValueOf(got.Setlist), setlistNotCarried)

	if len(got.Items) != len(want.Items) {
		t.Fatalf("imported %d items, want %d", len(got.Items), len(want.Items))
	}
	// The DETAIL view is ordered for reading (a bench item sorts after the main order, T23), which is not
	// the same as the running order's positions. Compare by POSITION, and compare the embedded
	// app.SetlistItem — the view's own fields (song title, and friends) are derived, not carried.
	byPosition := func(items []app.SetlistItemView) map[int]app.SetlistItem {
		m := map[int]app.SetlistItem{}
		for _, it := range items {
			m[it.Position] = it.SetlistItem
		}
		return m
	}
	wantByPos, gotByPos := byPosition(want.Items), byPosition(got.Items)
	for pos, w := range wantByPos {
		g, ok := gotByPos[pos]
		if !ok {
			t.Fatalf("position %d is missing from the imported setlist", pos)
			continue
		}
		compare("SetlistItem", reflect.ValueOf(w), reflect.ValueOf(g), itemNotCarried)
	}

	// The REMAPPED fields, asserted rather than skipped blind.
	if got.Setlist.BandID != rep.Band.ID {
		t.Errorf("setlist belongs to %q, want the imported band %q", got.Setlist.BandID, rep.Band.ID)
	}
	if got.Setlist.CreatedAt.IsZero() {
		t.Errorf("setlist CreatedAt is zero — re-stamped on import, not dropped")
	}
	if !got.Setlist.LiveUntil.IsZero() {
		t.Errorf("an imported setlist must NOT arrive in live mode (%v)", got.Setlist.LiveUntil)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	// Positions are carried by the folder's ARRAY ORDER (T140), so they must come back as a dense 0..n-1
	// with each entry where the source had it — a gap or a duplicate here is a scrambled running order.
	for pos := range wantByPos {
		if _, ok := gotByPos[pos]; !ok {
			t.Errorf("position %d did not survive; imported positions are %v", pos, gotByPos)
		}
	}
	if len(gotByPos) != len(got.Items) {
		t.Errorf("imported items share positions: %d entries, %d distinct positions", len(got.Items), len(gotByPos))
	}
	for i, it := range got.Items {
		if it.SetlistID != got.Setlist.ID {
			t.Errorf("item %d belongs to setlist %q, want %q", i, it.SetlistID, got.Setlist.ID)
		}
		if it.IsIntermission() {
			if it.SongID != "" {
				t.Errorf("an intermission must carry no song id, got %q", it.SongID)
			}
			continue
		}
		if it.SongID != songs[0].ID {
			t.Errorf("item %d points at song %q, want the imported song %q", i, it.SongID, songs[0].ID)
		}
	}
	_ = songID
}

// SongCue is the last of the mirrored shapes: two fields, so a guard looks like overkill — except that
// `color` is the one that is optional, and a cue whose colour is dropped comes back as a DIFFERENT cue to
// the eye. Same instrument, so the next field added to a cue is covered too.
func TestBandFolder_RoundTripsEveryCueField(t *testing.T) {
	src := newStack()
	admin, member, bandID, songID, _, _ := buildSourceBand(t, src)
	want := []app.SongCue{{Icon: "mic", Color: "#e11d48"}, {Icon: "shaker"}} // one tinted, one plain
	if _, err := src.svc.SetMyCues(member, bandID, songID, want); err != nil {
		t.Fatalf("set cues: %v", err)
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
	// The member's cues are PRIVATE to them, so read them back as the imported member, not as the importer.
	leo, err := tgt.repo.GetUserByUsername("leo")
	if err != nil {
		t.Fatalf("imported member: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	got, err := tgt.svc.MyCues(leo, rep.Band.ID, songs[0].ID)
	if err != nil {
		t.Fatalf("read cues: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("imported %d cues, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		wv, gv := reflect.ValueOf(want[i]), reflect.ValueOf(got[i])
		ty := wv.Type()
		for f := 0; f < ty.NumField(); f++ {
			if !reflect.DeepEqual(wv.Field(f).Interface(), gv.Field(f).Interface()) {
				t.Errorf("SongCue[%d].%s did not survive: wrote %#v, read %#v — carry it in manifestCue (both directions)",
					i, ty.Field(f).Name, wv.Field(f).Interface(), gv.Field(f).Interface())
			}
		}
	}
}
