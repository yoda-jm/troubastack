package bake

import (
	"context"
	"testing"
)

// TestConcertPDF_MemberPages (T202 §1, red-first): the concert PDF prints EXACTLY the chosen member's page
// sequence (as the tablet shows), not every page of every song. Marie selected both files → 2 pages; Leo
// made no selection → the default's 1 page. Today's `for pi := range s.Pages` loop prints all pages for
// everyone, so the Leo assertion is red before the fix.
func TestConcertPDF_MemberPages(t *testing.T) {
	e := newT137Env(t)
	a := e.upload(t, "a.pdf", []byte("%PDF-1.4 AAA"))  // default (lowest DisplayOrder), 1 page
	bf := e.upload(t, "b.pdf", []byte("%PDF-1.4 BBB")) // 1 page
	e.selects(t, e.marie, a.ID, bf.ID)                 // Marie reads both → a 2-page sequence
	// Leo makes no selection → resolves to the "" default (file a only, 1 page).

	b := e.bakerFor(t)
	cb, _, err := b.Bake(context.Background(), e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}

	count := func(memberID string) int {
		pdf, err := b.ConcertPDF(cb.ConcertID, "", memberID)
		if err != nil {
			t.Fatalf("ConcertPDF(%s): %v", memberID, err)
		}
		return len(pageLeafRE.FindAll(pdf, -1))
	}
	if n := count(e.marie.ID); n != 2 {
		t.Errorf("Marie selected both files → want 2 pages, got %d", n)
	}
	if n := count(e.leo.ID); n != 1 {
		t.Errorf("Leo (no selection) → want the default's 1 page, got %d (a full-song-pages loop prints all → red)", n)
	}
}
