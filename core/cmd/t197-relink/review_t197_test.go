package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

// The adversarial review of 94006019 (review 7), kept as permanent tests: an end-to-end apply of an
// old-style mark that then follows its line like a native link, and F1 — an approval only covers the
// proposal VLL saw on the crop.

// dryRunReview runs the dry run and returns the path of its marks.json (what VLL reviews).
func dryRunReview(t *testing.T, dir string) string {
	t.Helper()
	outDir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{"-data", dir, "-out", outDir}, &out); err != nil {
		t.Fatal(err, out.String())
	}
	return filepath.Join(outDir, "marks.json")
}

func svcOf(t *testing.T, dir string) (*app.Service, app.User) {
	t.Helper()
	repo, err := filerepo.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	blobs, _ := blob.NewFile(filepath.Join(dir, "blobs"))
	svc := app.NewService(repo).WithBlobStore(blobs)
	u, _, err := svc.Login("owner", "password123")
	if err != nil {
		t.Fatal(err)
	}
	return svc, u
}

func approveFile(t *testing.T, lines ...string) string {
	p := filepath.Join(t.TempDir(), "approve.txt")
	_ = os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	return p
}

// Old-style mark, end to end: apply → serve (project) → edit the chart → serve: it follows exactly like a
// natively linked mark drawn at the same place. Also: ValidLink, no Anchor, Points unchanged; :fixed stores
// Fixed (not absent) and serve keeps it Fixed; second apply is a no-op byte for byte.
func TestReview7_ApplyEndToEnd(t *testing.T) {
	f := newDataDir(t)
	// pre-T194 data: no stored line ids for the chart
	{
		repo, _ := filerepo.New(f.dir)
		if err := repo.DeleteChartLineIdentity(f.chart); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := byID(t, f.dir, f.song)
	review := dryRunReview(t, f.dir)
	var out bytes.Buffer
	if err := run([]string{"-data", f.dir, "-apply", "-approve", approveFile(t, "m-old", "m-margin:fixed"), "-review", review}, &out); err != nil {
		t.Fatal(err, out.String())
	}
	after, _ := byID(t, f.dir, f.song)
	o := after["m-old"]
	if o.Anchor != nil || o.Link == nil || o.Link.State != domain.LinkLinked || !chartpdf.ValidLink(o.Link) {
		t.Fatalf("m-old after apply: anchor %+v link %+v", o.Anchor, o.Link)
	}
	if !reflect.DeepEqual(o.Points, before["m-old"].Points) || o.Page != before["m-old"].Page || o.Version != before["m-old"].Version+1 {
		t.Errorf("m-old geometry/version: %+v", o)
	}
	m := after["m-margin"]
	if m.Link == nil || m.Link.State != domain.LinkFixed || m.Anchor != nil {
		t.Errorf("m-margin :fixed stored link %+v", m.Link)
	}
	svc, u := svcOf(t, f.dir)
	r, err := svc.ChartRenderForFile(f.chart)
	if err != nil {
		t.Fatal(err)
	}
	if st := chartpdf.ProjectMark(o, r).Link.State; st != domain.LinkLinked {
		t.Errorf("m-old served on the current render as %s (Rederived %+v)", st, r.Lines.Rederived)
	}
	if st := chartpdf.ProjectMark(m, r); st.Link.State != domain.LinkFixed || !reflect.DeepEqual(st.Points, m.Points) {
		t.Errorf("m-margin served %+v", st.Link)
	}
	// A native link at the same place, for comparison.
	native := chartpdf.LinkMark(before["m-old"].Clone(), r)
	native.UUID = "native"
	// Edit the chart: insert lines above the target line.
	cur, _, _ := svc.ChartSource(u, bandOf(t, svc, u), f.song, f.chart)
	src := strings.Replace(testutil.FreezeChart, "Leave the porch", "a new line above\nLeave the porch", 1)
	if _, err := svc.SaveChartSource(u, bandOf(t, svc, u), f.song, f.chart, cur.Revision, src); err != nil {
		t.Fatal(err)
	}
	r2, err := svc.ChartRenderForFile(f.chart)
	if err != nil {
		t.Fatal(err)
	}
	po, pn := chartpdf.ProjectMark(o, r2), chartpdf.ProjectMark(native, r2)
	if po.Link.State != domain.LinkLinked || !reflect.DeepEqual(po.Points, pn.Points) {
		t.Errorf("after an edit: migrated %v (%s), native %v (%s)", po.Points, po.Link.State, pn.Points, pn.Link.State)
	}
	if reflect.DeepEqual(po.Points, o.Points) {
		t.Errorf("the migrated mark did not move with its line")
	}
	if pm := chartpdf.ProjectMark(m, r2); !reflect.DeepEqual(pm.Points, m.Points) || pm.Link.State != domain.LinkFixed {
		t.Errorf("fixed mark moved/changed: %+v", pm.Link)
	}
	// Idempotence.
	d1 := dataDigest(t, f.dir)
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply", "-approve", approveFile(t, "m-old", "m-margin:fixed"), "-review", review}, &out); err != nil {
		t.Fatal(err)
	}
	if d2 := dataDigest(t, f.dir); d1 != d2 {
		t.Errorf("second apply changed the data dir")
	}
	t.Logf("second apply: %s", strings.TrimSpace(out.String()))
}

func bandOf(t *testing.T, svc *app.Service, u app.User) string {
	bands, err := svc.BandsForUser(u)
	if err != nil || len(bands) == 0 {
		t.Fatal("no band")
	}
	return bands[0].ID
}

// F1. VLL approves after reviewing the dry run. If the chart changes between the review and the apply, the
// recomputed link names a DIFFERENT line than the one he approved: apply must skip it, say why, and write
// nothing for it. A :fixed override needs only the mark to have been reviewed; a fresh review lets it through.
func TestReview7_ApprovalOfAStaleProposal(t *testing.T) {
	f := newDataDir(t)
	review := dryRunReview(t, f.dir)
	reviewed := readEntries(t, filepath.Dir(review))["m-icon"]
	if reviewed.Proposed != "linked" || !strings.HasPrefix(reviewed.LineText, "Pack a little light") {
		t.Fatalf("fixture: reviewed proposal %s %q", reviewed.Proposed, reviewed.LineText)
	}
	// someone edits the chart after the review: a line is inserted right where the icon sits
	svc, u := svcOf(t, f.dir)
	band := bandOf(t, svc, u)
	cur, src, _ := svc.ChartSource(u, band, f.song, f.chart)
	ns := strings.Replace(src, "Pack a little light", "A brand new opening line\nPack a little light", 1)
	if _, err := svc.SaveChartSource(u, band, f.song, f.chart, cur.Revision, ns); err != nil {
		t.Fatal(err)
	}
	lineOf := func(o domain.Object) string {
		r, err := svc.ChartRenderForFile(f.chart)
		if err != nil {
			t.Fatal(err)
		}
		if o.Link == nil {
			return ""
		}
		for i, id := range r.Lines.IDs {
			if id == o.Link.Line {
				return chartpdf.SourceLines(r.Source)[i]
			}
		}
		return "?"
	}
	before, _ := byID(t, f.dir, f.song)
	var out bytes.Buffer
	err := run([]string{"-data", f.dir, "-apply", "-approve", approveFile(t, "m-icon"), "-review", review}, &out)
	after, _ := byID(t, f.dir, f.song)
	t.Logf("reviewed proposal: line %q; stored after apply: %q (err %v)\n%s", reviewed.LineText, lineOf(after["m-icon"]), err, out.String())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after["m-icon"], before["m-icon"]) {
		t.Errorf("apply wrote m-icon although its proposal changed since the review: link to %q, reviewed %q", lineOf(after["m-icon"]), reviewed.LineText)
	}
	if !strings.Contains(out.String(), "skip  m-icon: "+staleReview) {
		t.Errorf("the stale approval is not listed as %q:\n%s", staleReview, out.String())
	}

	// :fixed needs only the review to contain the mark: it is written even though the proposal moved.
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply", "-approve", approveFile(t, "m-far:fixed"), "-review", review}, &out); err != nil {
		t.Fatal(err)
	}
	if after, _ := byID(t, f.dir, f.song); after["m-far"].Link == nil || after["m-far"].Link.State != domain.LinkFixed {
		t.Errorf("m-far:fixed was not written:\n%s", out.String())
	}

	// Positive control: after a fresh dry run (VLL reviews the new proposal), the same approval writes it.
	review2 := dryRunReview(t, f.dir)
	fresh := readEntries(t, filepath.Dir(review2))["m-icon"]
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply", "-approve", approveFile(t, "m-icon"), "-review", review2}, &out); err != nil {
		t.Fatal(err)
	}
	after, _ = byID(t, f.dir, f.song)
	if got := lineOf(after["m-icon"]); after["m-icon"].Link == nil || got != fresh.LineText {
		t.Errorf("with a fresh review m-icon was not written to the reviewed line: got %q, reviewed %q\n%s", got, fresh.LineText, out.String())
	}
}
