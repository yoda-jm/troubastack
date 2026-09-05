package chartpdf

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

// T144 — a GOLDEN LAYOUT guard. The renderer's output is a checked property: any change to a layout
// metric (font size, leading, margin, pagination) moves a fixture's page count or layout hash, so a
// layout change can no longer ship invisibly in a diff. A DELIBERATE layout change updates the golden
// values in the SAME commit — that is what makes it visible in review.
//
// The event this prevents (T144 ⟨V1⟩): a re-seed re-rendered every stored chart with the current, more
// compact renderer (the intended T73/T75/T76 changes), and page counts moved 4→3 / 2→1 with nothing to
// flag it. Measured, the change was NOT a regression in the reported window — it was intended compaction
// finally applied on re-render — so this task adds the guard, not a revert.
//
// The layout hash is a digest of the rendered PDF bytes; chartpdf.Render is deterministic (no embedded
// timestamp), so the digest is stable run-to-run. The fixtures are invented — no band data.

// fx repeats body after header (builds a chart of a chosen length without a giant literal).
func fx(reps int, header, body string) string {
	var b strings.Builder
	b.WriteString(header)
	for i := 0; i < reps; i++ {
		b.WriteString(body)
	}
	return b.String()
}

var goldenPageRe = regexp.MustCompile(`/Type\s*/Page[^s]`)

func goldenLayout(t *testing.T, src string) (pages int, hash string) {
	t.Helper()
	pdf, err := Render(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	sum := sha256.Sum256(pdf)
	return len(goldenPageRe.FindAll(pdf, -1)), hex.EncodeToString(sum[:])
}

func TestGoldenLayout(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		pages int
		hash  string
	}{
		{
			"short",
			"# Short Song\n## Demo\n\n## Verse\nC       G       Am      F\nthe kettle hums a quiet tune\nF       C       G\nbeneath a paper moon\n",
			1, "9c7d4c98b2a030ef243ce094ef0fe849e3a433d6c2dc9f0b32a953cb0d24b7b8",
		},
		{
			"boundary",
			fx(11, "# Boundary Song\n## Demo\n\n", "## Verse\nC       G       Am      F\nthe river folds the evening light\nF       C       G\nand carries it from sight\n\n"),
			2, "70ab5c0fba83f08b445d41f55d73c0a442119f970398dcec881008f66c22b05c",
		},
		{
			"long",
			fx(30, "# Long Song\n## Demo\n\n", "## Section\nC       G       Am      F\na longer wandering verse that keeps on going\nF       C       G       Am\nwith chords above the words still showing\n\n"),
			4, "bab847d4ae81c14e45cea37cdd566c6bdad21eb7f0820a202542b3fbe07a8adf",
		},
		{
			"tab",
			"# Tab Song\n## Demo\n\n## Riff\n{start_of_tab}\ne|-----0-----3-----|\nB|---1-----1-----1-|\nG|-0-----0-----0---|\n{end_of_tab}\n\n## Verse\nC       G\nthe intro rings and fades\n",
			1, "c74e7cdc16619c1638159ddc16eb5d10c88b522fc697d351a8a1dac0288fb90d",
		},
		{
			// lyric-ONLY lines (no chord rows) so the lyric-line leading is exercised — the chord+lyric
			// fixtures above use a different advance, and a leading sabotage on lyric-only lines slips past
			// them otherwise.
			"lyriconly",
			"# A Spoken Verse\n## Demo\n\nthe lantern sways above the quiet lane\nand every shadow learns your name again\nwe count the sparks that drift across the dark\nuntil the morning lifts them one by one\nand carries every ember toward the sun\nthe kettle cools, the window pales to grey\n",
			1, "ff5f7e3dcb9926cd5f0b56fbcf432031a67be860b7b637fb27bfd2a8631c0e6f",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pages, hash := goldenLayout(t, c.src)
			if pages != c.pages {
				t.Errorf("%s: PAGE COUNT changed %d → %d — a layout metric moved. If deliberate, update the golden in this commit.",
					c.name, c.pages, pages)
			}
			if hash != c.hash {
				t.Errorf("%s: LAYOUT HASH changed (pages %d→%d)\n  old %s\n  new %s\nIf this layout change is deliberate, update the golden value in the SAME commit so it is visible in review.",
					c.name, c.pages, pages, c.hash, hash)
			}
		})
	}
}
