package testutil

import (
	"os"
	"testing"
)

// AnchorKnownRed marks a test of the annotation-anchoring matrix (docs: anchoring spec, properties P1…)
// whose DESIRED property is known to fail on today's code. It skips the test unless ANCHOR_RED=1, so plain
// `go test ./...` stays green while
//
//	ANCHOR_RED=1 go test ./internal/chartpdf/ ./internal/sync/ ./internal/app/ -run AnchorMatrix -v
//
// shows the whole red/green picture. A fix flips a cell by deleting its AnchorKnownRed call (or its entry
// in the matrix's knownRed table): the test then runs in CI and must pass.
//
// prop names the spec property the test asserts; task names the task expected to fix it.
func AnchorKnownRed(t *testing.T, prop, task string) {
	t.Helper()
	if os.Getenv("ANCHOR_RED") != "1" {
		t.Skipf("known red: %s, fixed by %s (ANCHOR_RED=1 runs it)", prop, task)
	}
}
