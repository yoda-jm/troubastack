package setlistpdf

import (
	"math"
	"testing"
)

// firstRowY renders the header alone and returns the Y (mm) at which the running order would start —
// the exact quantity T163 is about. Render draws the same drawHeader, so this measures the real sheet.
func firstRowY(d Doc) float64 {
	pdf, tr := newSheet()
	drawHeader(pdf, tr, d)
	return pdf.GetY()
}

func fullDoc() Doc {
	return Doc{
		BandName:    "The Band",
		SetlistName: "Saturday Gig",
		Venue:       "The Hall",
		EventDate:   "2026-09-21",
		Main:        []Row{{Number: 1, Title: "Opener", Kind: KindSong}},
	}
}

// TestHeader_FirstSongStartsHigh_T163: the running order must begin well up the page. The pre-T163 header
// (four stacked rows: 20/14/11/11 pt) pushed it to ~50 mm; the new three-row header targets ~20 mm of
// header (first song ~35 mm). Threshold 42 mm is below what today's layout reaches — teeth.
func TestHeader_FirstSongStartsHigh_T163(t *testing.T) {
	const threshold = 42.0
	if y := firstRowY(fullDoc()); y >= threshold {
		t.Fatalf("first song starts at Y=%.1f mm, want < %.1f — the header is still too thick", y, threshold)
	}
}

// TestHeader_VenueAndDateShareOneRow_T163: venue and date must sit on ONE row, not two. Adding both to a
// header that has neither must cost about one row height, not two. Pre-T163 (two stacked 6 mm lines) this
// delta was ~12 mm — teeth.
func TestHeader_VenueAndDateShareOneRow_T163(t *testing.T) {
	none := fullDoc()
	none.Venue, none.EventDate = "", ""
	delta := firstRowY(fullDoc()) - firstRowY(none)
	if delta > 8.0 {
		t.Fatalf("venue+date added %.1f mm of header — that is two rows, not one", delta)
	}
	if delta <= 0 {
		t.Fatalf("venue+date added no height (%.1f mm) — the row is missing entirely", delta)
	}
}

// TestHeader_OneOrBothOnTheSameSingleRow_T163: venue-only, date-only, and both-present all occupy the SAME
// single row (same first-song Y), and both-absent drops the row entirely (first song moves up).
func TestHeader_OneOrBothOnTheSameSingleRow_T163(t *testing.T) {
	both := fullDoc()
	venueOnly := fullDoc()
	venueOnly.EventDate = ""
	dateOnly := fullDoc()
	dateOnly.Venue = ""
	none := fullDoc()
	none.Venue, none.EventDate = "", ""

	yBoth, yVenue, yDate, yNone := firstRowY(both), firstRowY(venueOnly), firstRowY(dateOnly), firstRowY(none)

	// One present or both present ⇒ identical height (they share the one row).
	if math.Abs(yBoth-yVenue) > 0.1 || math.Abs(yBoth-yDate) > 0.1 {
		t.Fatalf("venue-only (%.1f), date-only (%.1f) and both (%.1f) should share one row height", yVenue, yDate, yBoth)
	}
	// Both absent ⇒ the row is not drawn, so the first song moves up.
	if !(yNone < yDate-1.0) {
		t.Fatalf("both-absent Y=%.1f should be clearly above date-present Y=%.1f (the row must drop)", yNone, yDate)
	}
}
