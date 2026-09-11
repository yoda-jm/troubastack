package filerepo

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// nulledMaps renders a dataset document whose every map field is an explicit JSON null, named
// by the field's own json tag. Enumerating the struct rather than a list is the whole point:
// the field added tomorrow is in the fixture the moment it is in the type.
func nulledMaps(t *testing.T) []byte {
	t.Helper()
	tp := reflect.TypeOf(dataset{})
	var parts []string
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.Type.Kind() != reflect.Map {
			continue
		}
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if tag == "" || tag == "-" {
			t.Fatalf("dataset.%s is a map with no json tag; this guard cannot name it", f.Name)
		}
		parts = append(parts, `"`+tag+`":null`)
	}
	return []byte("{" + strings.Join(parts, ",") + "}")
}

// TestLoadLeavesNoNilMap replaces a hand-maintained enumeration with one over the SOURCE type.
//
// load() nil-guards each map on the dataset one `if` at a time, so that a file written before a
// field existed still loads. That list mirrors `dataset`, and a mirror maintained by hand rots:
// the field added without its guard loads as nil from every pre-existing app.json and panics on
// the first write — in production, on an upgrade, never in a fresh-dir test. Enumerating the
// struct by reflection means the next field is covered before anyone remembers to cover it.
//
// The fixture is built from the struct too: every map field is written as an explicit JSON
// null. That matters — a field simply ABSENT from the document keeps the value New() seeded via
// emptyDataset(), so an absent-key fixture passes whether or not the guard exists and proves
// nothing. An explicit null is what actually lands nil in the struct, and it is reachable: a
// hand-edited file, a partial write, or any producer that serialises an empty map as null.
func TestLoadLeavesNoNilMap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.json"), nulledMaps(t), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(r.d)
	tp := v.Type()
	checked := 0
	for i := 0; i < tp.NumField(); i++ {
		if tp.Field(i).Type.Kind() != reflect.Map {
			continue
		}
		checked++
		if v.Field(i).IsNil() {
			t.Errorf("dataset.%s is nil after loading a pre-existing app.json — the first write to it panics; add a nil-guard in load()", tp.Field(i).Name)
		}
	}
	if checked < 10 {
		t.Fatalf("only %d map fields found on dataset — this guard is not looking at the struct it thinks it is", checked)
	}
}
