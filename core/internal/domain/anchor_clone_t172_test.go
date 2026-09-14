package domain

import "testing"

// TestCloneDeepCopiesNestedAnchorOptionals — Object.Clone's anchor copy exists so a caller cannot reach
// stored state through a copy. T172 added two POINTER fields inside SourceAnchor, and `cp := *o.Anchor`
// copies pointers: without a deeper copy the clone shares them and the guarantee silently stops holding
// for the new fields while continuing to hold for the old ones.
func TestCloneDeepCopiesNestedAnchorOptionals(t *testing.T) {
	o := Object{
		UUID: "u1",
		Anchor: &SourceAnchor{
			RunText: "a line", Occurrence: 1, CharStart: 0, CharEnd: 6,
			Offset: &AnchorOffset{RelY0: 1.1, RelY1: 1.3},
			Span:   &AnchorSpan{RunText: "last line", Occurrence: 2},
		},
	}
	cp := o.Clone()

	if cp.Anchor == o.Anchor {
		t.Fatal("the anchor itself is shared")
	}
	if cp.Anchor.Offset == o.Anchor.Offset {
		t.Error("Offset is shared between the original and its clone")
	}
	if cp.Anchor.Span == o.Anchor.Span {
		t.Error("Span is shared between the original and its clone")
	}

	// the behavioural half: mutating the clone must not reach the original
	cp.Anchor.Offset.RelY0 = 99
	cp.Anchor.Span.Occurrence = 99
	if o.Anchor.Offset.RelY0 != 1.1 {
		t.Errorf("mutating the clone's Offset changed the original (RelY0=%v)", o.Anchor.Offset.RelY0)
	}
	if o.Anchor.Span.Occurrence != 2 {
		t.Errorf("mutating the clone's Span changed the original (Occurrence=%d)", o.Anchor.Span.Occurrence)
	}

	// and a nil anchor / nil members still clone cleanly
	if (&Object{UUID: "u2"}).Clone().Anchor != nil {
		t.Error("a nil anchor cloned to non-nil")
	}
	bare := Object{Anchor: &SourceAnchor{RunText: "x", Occurrence: 1}}.Clone()
	if bare.Anchor.Offset != nil || bare.Anchor.Span != nil {
		t.Error("absent optionals were materialised by the clone — absence must survive a copy")
	}
}
