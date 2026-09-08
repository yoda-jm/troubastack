package app_test

import (
	"reflect"
	"testing"

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
