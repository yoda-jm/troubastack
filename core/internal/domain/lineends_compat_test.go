package domain

import (
	"encoding/json"
	"testing"
)

// T179 read-compat: a stored LineEnds in T177's {Head, Side} shape must translate to per-end shapes rather
// than vanish. There is real such data — arrows drawn in the hours T177 was deployed — and dropping it
// silently is the exact class of loss this session has been auditing for.
func TestLineEnds_ReadsT177HeadSide(t *testing.T) {
	cases := []struct {
		json               string
		wantStart, wantEnd string
	}{
		{`{"Head":"arrow","Side":"end"}`, "", "arrow"},
		{`{"Head":"arrow","Side":"start"}`, "arrow", ""},
		{`{"Head":"arrow","Side":"both"}`, "arrow", "arrow"},
		{`{"Head":"arrow"}`, "", "arrow"},         // T177 default side = end
		{`{"Head":"","Side":"end"}`, "", "arrow"}, // T177 empty head = default arrow
		// The case Fable caught: a bare {} is a LEGAL T177 doc meaning "arrow at the far end" (T177
		// documented empty Head = arrow, empty Side = end). PRESENCE of the record drives the reading,
		// never the emptiness of its members.
		{`{}`, "", "arrow"},
		{`{"Side":"start"}`, "arrow", ""}, // empty head, explicit side
		{`{"Side":"both"}`, "arrow", "arrow"},
		// the current shape must still read as itself
		{`{"Start":"circle","End":"square"}`, "circle", "square"},
		{`{"End":"arrow"}`, "", "arrow"},
	}
	for _, c := range cases {
		var e LineEnds
		if err := json.Unmarshal([]byte(c.json), &e); err != nil {
			t.Fatalf("%s: %v", c.json, err)
		}
		if e.Start != c.wantStart || e.End != c.wantEnd {
			t.Errorf("%s -> {Start:%q End:%q}, want {%q %q}", c.json, e.Start, e.End, c.wantStart, c.wantEnd)
		}
	}
}

// A round trip through the store's own (un)marshalling of a whole Object must preserve the new shape — the
// forward path, so the compat reader does not accidentally break current data.
func TestLineEnds_CurrentShapeRoundTrips(t *testing.T) {
	o := Object{UUID: "o1", Type: TypeLine, Style: Style{Ends: &LineEnds{Start: "arrow", End: "circle"}}}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var got Object
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Style.Ends == nil || got.Style.Ends.Start != "arrow" || got.Style.Ends.End != "circle" {
		t.Fatalf("round trip lost the ends: %+v", got.Style.Ends)
	}
}
