package httpapi

import (
	"testing"
	"time"
)

// TestPdfJobRegistry (T202 §3 + the two binding fixes): dedup reuses only an IN-FLIGHT job; a new rev, a new
// creator, or a terminal job each start a fresh job; visibility is creator-or-band-admin and band-scoped; a
// job expires after the TTL.
func TestPdfJobRegistry(t *testing.T) {
	clock := time.Unix(1700000000, 0)
	r := newPdfJobRegistry(func() time.Time { return clock })

	id1, created := r.startOrGet("band", "concert", 1, "member", "", "creator", "f.pdf")
	if !created {
		t.Fatal("first request must create the job")
	}
	// A repeated request while queued/running reuses it.
	if id2, created2 := r.startOrGet("band", "concert", 1, "member", "", "creator", "f.pdf"); created2 || id2 != id1 {
		t.Fatalf("an in-flight job must be reused (got %s created=%v)", id2, created2)
	}

	// Binding 1: a NEW REV is a new job (so a re-bake never serves the stale PDF).
	if idRev2, created := r.startOrGet("band", "concert", 2, "member", "", "creator", "f.pdf"); !created || idRev2 == id1 {
		t.Fatalf("a new rev must start a new job (got %s created=%v)", idRev2, created)
	}

	// Binding 2: a different CREATOR is a new job (so an admin's job for M is not handed to M, who view denies).
	if idOther, created := r.startOrGet("band", "concert", 1, "member", "", "someone-else", "f.pdf"); !created || idOther == id1 {
		t.Fatalf("a different creator must start a new job (got %s created=%v)", idOther, created)
	}

	// Visibility.
	if _, ok := r.view(id1, "band", "creator", false); !ok {
		t.Fatal("the creator must see their job")
	}
	if _, ok := r.view(id1, "band", "stranger", false); ok {
		t.Fatal("a non-creator non-admin must NOT see the job")
	}
	if _, ok := r.view(id1, "band", "stranger", true); !ok {
		t.Fatal("a band admin must see the job")
	}
	if _, ok := r.view(id1, "otherband", "creator", false); ok {
		t.Fatal("a job must not be visible under another band's path")
	}

	// A terminal job is not reused — a fresh request re-composes.
	r.finish(id1, []byte("%PDF"))
	if idAfterDone, created := r.startOrGet("band", "concert", 1, "member", "", "creator", "f.pdf"); !created || idAfterDone == id1 {
		t.Fatalf("a done job must not be reused (got %s created=%v)", idAfterDone, created)
	}

	// Expiry → gone.
	clock = clock.Add(pdfJobTTL + time.Minute)
	if _, ok := r.view(id1, "band", "creator", false); ok {
		t.Fatal("an expired job must be swept (404)")
	}
}
