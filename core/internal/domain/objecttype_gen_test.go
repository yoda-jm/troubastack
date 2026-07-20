package domain

import "testing"

// TestObjectTypeMaps guards the GENERATED ObjectType⇄string maps (T09 Stage 1b —
// the one source that replaced the diverged httpapi/sync copies, the T51 bug).
func TestObjectTypeMaps(t *testing.T) {
	// Every non-zero type round-trips through its wire string.
	for ty := TypeFreehand; ty <= TypeIcon; ty++ {
		s := ObjectTypeToString(ty)
		if s == "" {
			t.Fatalf("ObjectTypeToString(%d) = \"\" — missing a case", ty)
		}
		if got := ObjectTypeFromString(s); got != ty {
			t.Fatalf("round-trip %d → %q → %d", ty, s, got)
		}
	}
	// The zero value and unknown strings are the empty/unspecified sentinels.
	if ObjectTypeToString(TypeUnspecified) != "" {
		t.Fatal("TypeUnspecified must map to \"\"")
	}
	if ObjectTypeFromString("nope") != TypeUnspecified {
		t.Fatal("unknown string must map to TypeUnspecified")
	}
	// A couple of exact wire strings the app + TS depend on.
	if ObjectTypeToString(TypeFreehand) != "freehand" || ObjectTypeToString(TypeIcon) != "icon" {
		t.Fatal("wire strings drifted")
	}
}
