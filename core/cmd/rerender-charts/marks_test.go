package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Fable's condition on this tool: loadMarks is the part that earns the trust. The tool is safe by
// CONSTRUCTION (dry-run default, --guard-port), but the NUMBERS are what a human acts on — VLL approved
// the 13 pt re-render on "15 of 18 marks will not follow", and a silent under-count would have had him
// approving a cost that was not the real one.
//
// So every case here is one where a wrong answer reads as REASSURING: a mark counted as anchored when it
// is not, a layer whose filename does not join, a tombstone inflating the total. An over-count is a
// nuisance; an under-count is a person agreeing to something they were not told about.

func writeDoc(t *testing.T, bandsDir, band, slug, body string) {
	t.Helper()
	dir := filepath.Join(bandsDir, band, "annotations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMarks_CountsAnchoredAndUnanchoredPerFile(t *testing.T) {
	dir := t.TempDir()
	// song A, one file: two anchored marks, two that will NOT follow a re-render (absent + explicit null),
	// and one tombstone that must not be counted at all.
	writeDoc(t, dir, "band-a", "song-a", `{
      "layers": [{"id":"L1","file":"song-a-chart.pdf"}],
      "objects": [
        {"layer":"L1","anchor":{"runText":"x","occurrence":1}},
        {"layer":"L1","anchor":{"runText":"y","occurrence":1}},
        {"layer":"L1"},
        {"layer":"L1","anchor":null},
        {"layer":"L1","deleted":true}
      ]}`)

	total, unanchored, _ := loadMarks(dir)

	const key = "song-a|song-a-chart.pdf"
	if total[key] != 4 {
		t.Errorf("total = %d, want 4 (five objects, one of them a tombstone)", total[key])
	}
	if unanchored[key] != 2 {
		t.Errorf("unanchored = %d, want 2 — an absent anchor AND an explicit null both mean the mark "+
			"cannot follow the text; counting only one of them under-reports the cost", unanchored[key])
	}
}

func TestLoadMarks_JoinsBothFilenameSpellings(t *testing.T) {
	// T79 strips the extension from the stored pool name; older documents kept it on the layer. The caller
	// keys by the POOL's filename, so a document written in the other spelling must still join — a missed
	// join reports a reassuring zero, which is the failure that matters here.
	dir := t.TempDir()
	writeDoc(t, dir, "band-a", "song-b", `{
      "layers": [{"id":"L1","file":"song-b-chart.pdf"}],
      "objects": [{"layer":"L1"}]}`)

	total, unanchored, _ := loadMarks(dir)

	withExt := total["song-b|song-b-chart.pdf"]
	without := total["song-b|song-b-chart"]
	if withExt != 1 || without != 1 {
		t.Fatalf("counts by spelling: with-ext=%d without-ext=%d, want 1 and 1 — the caller may key either way",
			withExt, without)
	}
	if unanchored["song-b|song-b-chart"] != 1 {
		t.Error("the unanchored count must join on both spellings too, not just the total")
	}
}

func TestLoadMarks_DoesNotDoubleCountAnExtensionlessLayer(t *testing.T) {
	// The both-spellings loop must not count a layer twice when the two spellings are identical. This is
	// the over-count direction, and it is the one the `break` in loadMarks exists for.
	dir := t.TempDir()
	writeDoc(t, dir, "band-a", "song-c", `{
      "layers": [{"id":"L1","file":"song-c-chart"}],
      "objects": [{"layer":"L1"},{"layer":"L1"}]}`)

	total, _, _ := loadMarks(dir)
	if got := total["song-c|song-c-chart"]; got != 2 {
		t.Errorf("total = %d, want 2 — an extensionless layer name must not be counted under two keys", got)
	}
}

func TestLoadMarks_AnUnattributableMarkIsReported_NotDropped(t *testing.T) {
	// An object whose layer is not in its own document's layer list cannot be attributed to any chart. It
	// must land in NO per-file row — guessing would inflate one chart's cost and hide it from another —
	// but it must still be COUNTED, because a mark that exists and appears nowhere is the silent
	// under-count that would have VLL approving a cost he was never shown.
	//
	// Found by this test: loadMarks used to file such a mark under the key "<slug>|" (the empty filename).
	// The caller never reads that key, so the mark was invisible rather than miscounted — which is the
	// worse of the two failures and exactly what the comment claimed was not happening.
	dir := t.TempDir()
	writeDoc(t, dir, "band-a", "song-d", `{
      "layers": [{"id":"L1","file":"chart.pdf"}],
      "objects": [{"layer":"L1"},{"layer":"L9-not-in-this-document"}]}`)

	total, _, orphans := loadMarks(dir)

	if orphans != 1 {
		t.Errorf("orphans = %d, want 1 — the unattributable mark must be reported, not dropped", orphans)
	}
	if total["song-d|"] != 0 {
		t.Errorf("an unattributable mark was filed under the empty-filename key (%d) — a key nobody reads, "+
			"so the mark vanishes from the cost entirely", total["song-d|"])
	}
	if got := total["song-d|chart.pdf"]; got != 1 {
		t.Errorf("the joinable mark on the same document = %d, want 1 — an orphan beside it must not "+
			"disturb the counts that DO join", got)
	}
}

func TestLoadMarks_SurvivesJunkAndAnAbsentDir(t *testing.T) {
	// The count is advisory and printed beside a destructive action; it must degrade to "nothing" rather
	// than take the tool down before it has told anyone what it is about to do.
	if total, un, orph := loadMarks(""); len(total) != 0 || len(un) != 0 || orph != 0 {
		t.Error("an empty bandsDir must yield empty maps and no orphans")
	}
	dir := t.TempDir()
	writeDoc(t, dir, "band-a", "broken", `{ this is not json`)
	writeDoc(t, dir, "band-a", "song-e", `{"layers":[{"id":"L1","file":"f.pdf"}],"objects":[{"layer":"L1"}]}`)

	total, _, _ := loadMarks(dir)
	if total["song-e|f.pdf"] != 1 {
		t.Error("a malformed document must be skipped WITHOUT costing the counts of its neighbours")
	}
}
