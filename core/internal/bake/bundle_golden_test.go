package bake

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// T09 Stage 0 — byte-compatibility goldens. The bundle format on disk must NOT
// change as the hand-written mirror (bundle.go) becomes a GENERATED artifact
// (Stage 1). These tests pin that: the committed demo concert's bundle.json must
// round-trip BYTE-IDENTICALLY through the Go mirror, so a generated struct that
// renames, reorders, or drops one json key fails HERE, before review. The same
// golden bytes gate the TS + Kotlin mirrors in their own suites (Stages 2/3).

const demoTstage = "../../../docs/demo/demo-concert.tstage"

// readBundleJSON extracts bundle.json from a committed .tstage (a zip).
func readBundleJSON(t *testing.T, tstagePath string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(tstagePath)
	if err != nil {
		t.Fatalf("open %s: %v", tstagePath, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "bundle.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open bundle.json: %v", err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read bundle.json: %v", err)
		}
		return data
	}
	t.Fatalf("no bundle.json in %s", tstagePath)
	return nil
}

// TestBundleGolden_RoundTrip: the committed demo bundle.json survives
// unmarshal → MarshalCanonical byte-for-byte through the Go mirror. This IS the
// Stage-1 acceptance gate — the generated bundle.go must keep this green.
func TestBundleGolden_RoundTrip(t *testing.T) {
	orig := readBundleJSON(t, demoTstage)

	var cb ConcertBundle
	if err := json.Unmarshal(orig, &cb); err != nil {
		t.Fatalf("unmarshal demo bundle.json into the Go mirror: %v", err)
	}
	got, err := cb.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatalf("Go mirror round-trip is NOT byte-identical to the committed bundle.json\n%s",
			firstDiff(orig, got))
	}
}

// TestBundleGolden_CatchesDrift proves the golden is SENSITIVE (red-first): a
// bundle.json carrying a key the mirror doesn't know is silently dropped on
// round-trip, so the byte-compare above would fail — i.e. a renamed/removed field
// in a generated mirror can't slip through green.
func TestBundleGolden_CatchesDrift(t *testing.T) {
	orig := readBundleJSON(t, demoTstage)

	// Rename the top-level "name" key → the mirror won't recognise "name_X", drops
	// it, and re-marshals WITHOUT it: the round-trip must differ from this input.
	mutated := bytes.Replace(orig, []byte(`"name":`), []byte(`"name_X":`), 1)
	if bytes.Equal(mutated, orig) {
		t.Fatal("test setup: expected to mutate a key")
	}
	var cb ConcertBundle
	if err := json.Unmarshal(mutated, &cb); err != nil {
		t.Fatalf("unmarshal mutated bundle: %v", err)
	}
	got, err := cb.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	if bytes.Equal(got, mutated) {
		t.Fatal("golden is INSENSITIVE — a renamed key round-tripped unchanged; it would not catch mirror drift")
	}
}

// firstDiff returns a short human pointer at the first byte that differs.
func firstDiff(a, b []byte) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := i - 40
			if lo < 0 {
				lo = 0
			}
			return "first diff at byte " + itoa(i) + ":\n  orig: …" + string(a[lo:min(i+40, len(a))]) +
				"\n  got:  …" + string(b[lo:min(i+40, len(b))])
		}
	}
	return "lengths differ: orig=" + itoa(len(a)) + " got=" + itoa(len(b))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
