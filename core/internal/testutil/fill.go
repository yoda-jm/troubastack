// Package testutil holds the small test helpers shared across core's packages. It is test support that
// happens to live in a normal package: Go cannot import another package's _test.go, and copying this
// helper a fourth time is exactly the "somebody remembered" failure the guards it feeds exist to catch
// (Fable, ⟨GO⟩ dd6fed2d).
package testutil

import (
	"reflect"
	"testing"
)

// Fill sets EVERY field of the struct behind `ptr` to a distinctive non-zero value, recursively through
// nested structs, pointers and one-element slices. It is the engine of the field-completeness guards: fill
// a domain type, push it through a hand-maintained mirror (a band folder, the realtime wire, the REST DTO)
// and read it back, and any field the mirror forgot comes back zero and is named by the comparison.
//
// Two things it deliberately does NOT do, because they are the caller's knowledge:
//   - ENUMS. A field whose type is an enum must be set by the caller to a REAL member: every wire here maps
//     enums by string, so a filler's arbitrary int round-trips to zero and reads as a mirror bug when it is
//     the fixture that is wrong.
//   - DERIVED or CANONICALISED fields (a slug from a title, a canonical meter). Set them from the source
//     after the fact rather than inventing a value the system will not keep.
func Fill(t testing.TB, ptr any, seed int) {
	t.Helper()
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		t.Fatalf("testutil.Fill wants a pointer to a struct, got %T", ptr)
	}
	fillStruct(t, v.Elem(), seed)
}

func fillStruct(t testing.TB, v reflect.Value, seed int) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() { // unexported: not part of any wire, and not settable anyway
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
			switch p.Elem().Kind() {
			case reflect.Struct:
				fillStruct(t, p.Elem(), seed)
			case reflect.Bool:
				p.Elem().SetBool(true)
			}
			f.Set(p)
		case reflect.Struct:
			fillStruct(t, f, seed)
		case reflect.Slice:
			e := reflect.New(f.Type().Elem()).Elem()
			if e.Kind() == reflect.Struct {
				fillStruct(t, e, seed)
			}
			f.Set(reflect.Append(reflect.MakeSlice(f.Type(), 0, 1), e))
		}
	}
}

// DiffFields reports the fields of two values of the same struct type that differ, skipping `skip` (whose
// values are the REASONS a field legitimately does not survive — a skip list whose entries had to be
// written down is the opposite of an omission that looks like nothing at all).
func DiffFields(want, got any, skip map[string]string) []string {
	wv, gv := reflect.ValueOf(want), reflect.ValueOf(got)
	ty := wv.Type()
	var out []string
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if _, s := skip[name]; s {
			continue
		}
		if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
			out = append(out, name)
		}
	}
	return out
}
