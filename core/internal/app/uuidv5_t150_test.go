package app

import (
	"regexp"
	"testing"
)

var uuidV5Shape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestUUIDv5 is the T150 determinism guard: same (namespace, name) → same id (that is what makes a
// from-scratch re-seed stable), different inputs → different id (so distinct songs/setlists don't collide),
// and the output is a well-formed RFC 4122 v5 UUID. Works for a UUID namespace and a non-UUID one alike.
func TestUUIDv5(t *testing.T) {
	const ns = "band-uuid-0001"
	a := uuidV5(ns, "song:wonderwall")
	if a != uuidV5(ns, "song:wonderwall") {
		t.Fatal("uuidV5 is not deterministic for identical inputs")
	}
	if a == uuidV5(ns, "song:champagne") {
		t.Fatal("different names produced the same id")
	}
	if a == uuidV5("band-uuid-0002", "song:wonderwall") {
		t.Fatal("different namespaces produced the same id")
	}
	if !uuidV5Shape.MatchString(a) {
		t.Fatalf("not a well-formed v5 UUID: %q", a)
	}

	// A canonical-UUID namespace takes the RFC path; still deterministic + well-formed.
	b := uuidV5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "setlist:Spring Gig")
	if b != uuidV5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "setlist:Spring Gig") || !uuidV5Shape.MatchString(b) {
		t.Fatalf("UUID-namespace path not deterministic/well-formed: %q", b)
	}
}
