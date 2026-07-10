package bake

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeRev creates a fake baked rev under bakesDir/concert/<rev>/ (bundle.json +
// a dummy blob) plus its sibling <rev>.tstage, marking FinalLocked as given.
func writeRev(t *testing.T, bakesDir, concert string, rev uint64, locked bool) {
	t.Helper()
	revDir := filepath.Join(bakesDir, concert, strconv.FormatUint(rev, 10))
	if err := os.MkdirAll(filepath.Join(revDir, "blobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(ConcertBundle{ConcertID: concert, ConcertRev: rev, FinalLocked: locked})
	if err := os.WriteFile(filepath.Join(revDir, "bundle.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(revDir, "blobs", "p0.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bakesDir, concert, strconv.FormatUint(rev, 10)+".tstage"), []byte("TSTAGE"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func revExists(bakesDir, concert string, rev uint64) bool {
	_, err := os.Stat(filepath.Join(bakesDir, concert, strconv.FormatUint(rev, 10)))
	return err == nil
}

func tstageExists(bakesDir, concert string, rev uint64) bool {
	_, err := os.Stat(filepath.Join(bakesDir, concert, strconv.FormatUint(rev, 10)+".tstage"))
	return err == nil
}

func TestPruneOutputs_keepAllByDefault(t *testing.T) {
	dir := t.TempDir()
	for r := uint64(1); r <= 3; r++ {
		writeRev(t, dir, "setA", r, false)
	}
	stats, err := PruneOutputs(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RevsDeleted != 0 {
		t.Fatalf("keepN=0 must prune nothing, deleted %d", stats.RevsDeleted)
	}
	for r := uint64(1); r <= 3; r++ {
		if !revExists(dir, "setA", r) {
			t.Fatalf("rev %d wrongly deleted under keep-all", r)
		}
	}
}

func TestPruneOutputs_keepsNewestN(t *testing.T) {
	dir := t.TempDir()
	for r := uint64(1); r <= 4; r++ {
		writeRev(t, dir, "setA", r, false)
	}
	stats, err := PruneOutputs(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RevsDeleted != 2 || stats.ConcertsScanned != 1 {
		t.Fatalf("want 2 deleted / 1 scanned, got %+v", stats)
	}
	if stats.BytesFreed <= 0 {
		t.Fatalf("BytesFreed should be > 0, got %d", stats.BytesFreed)
	}
	for _, r := range []uint64{3, 4} { // newest two survive, with their .tstage
		if !revExists(dir, "setA", r) || !tstageExists(dir, "setA", r) {
			t.Fatalf("rev %d (or its .tstage) should be kept", r)
		}
	}
	for _, r := range []uint64{1, 2} { // oldest two (dir + .tstage) gone
		if revExists(dir, "setA", r) || tstageExists(dir, "setA", r) {
			t.Fatalf("rev %d (or its .tstage) should be pruned", r)
		}
	}
}

func TestPruneOutputs_neverPrunesFinalLocked(t *testing.T) {
	dir := t.TempDir()
	writeRev(t, dir, "setA", 1, true) // locked — survives, does NOT count toward keepN
	writeRev(t, dir, "setA", 2, false)
	writeRev(t, dir, "setA", 3, false)
	writeRev(t, dir, "setA", 4, false)
	stats, err := PruneOutputs(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Keep newest two non-locked (3,4) + locked rev1 → only rev2 pruned.
	if stats.RevsDeleted != 1 {
		t.Fatalf("want 1 deleted (only rev2), got %d", stats.RevsDeleted)
	}
	if !revExists(dir, "setA", 1) {
		t.Fatal("final-locked rev1 must never be pruned")
	}
	if revExists(dir, "setA", 2) {
		t.Fatal("rev2 should be pruned (oldest non-locked beyond keepN)")
	}
	for _, r := range []uint64{3, 4} {
		if !revExists(dir, "setA", r) {
			t.Fatalf("rev %d should be kept", r)
		}
	}
}

func TestPruneOutputs_ignoresStagingAndMissing(t *testing.T) {
	// Missing bakesDir → no error, zero stats.
	if _, err := PruneOutputs(filepath.Join(t.TempDir(), "nope"), 2); err != nil {
		t.Fatalf("missing bakesDir should not error: %v", err)
	}
	// A `<rev>.tmp` staging dir is not a published rev: never counted or deleted.
	dir := t.TempDir()
	writeRev(t, dir, "setA", 1, false)
	writeRev(t, dir, "setA", 2, false)
	if err := os.MkdirAll(filepath.Join(dir, "setA", "3.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	stats, err := PruneOutputs(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RevsDeleted != 1 { // published revs {1,2}, keepN=1 → prune rev1 only
		t.Fatalf("want 1 deleted, got %d", stats.RevsDeleted)
	}
	if _, err := os.Stat(filepath.Join(dir, "setA", "3.tmp")); err != nil {
		t.Fatal("staging <rev>.tmp dir must be left untouched")
	}
	if !revExists(dir, "setA", 2) {
		t.Fatal("newest published rev2 must be kept")
	}
	if revExists(dir, "setA", 1) {
		t.Fatal("rev1 should be pruned")
	}
}
