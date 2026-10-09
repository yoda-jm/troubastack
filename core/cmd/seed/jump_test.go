package main

import "testing"

// The demo's jump pair must survive the trip to the import payload: wireToCanon is a hand-written
// field-by-field copy, and it silently dropped JumpTo once — the icons arrived, the jump did not.
func TestOpenRoadJumpPairReachesTheImport(t *testing.T) {
	an := mustAnchors("open-road-leadsheet")
	canon := wireToCanon(buildOpenRoadAnnotations("song-or", "file-or", testUsers(), "u-leo", an))
	byUUID := map[string]canonObject{}
	for _, o := range canon.Objects {
		byUUID[o.UUID] = o
	}
	sources := 0
	for _, o := range canon.Objects {
		if o.JumpTo == "" {
			continue
		}
		sources++
		dst, ok := byUUID[o.JumpTo]
		if !ok {
			t.Fatalf("jump source %s points at %s, which is not in the import", o.UUID, o.JumpTo)
		}
		if o.Type != "icon" || dst.Type != "icon" || o.Text != dst.Text || o.Style.Color != dst.Style.Color {
			t.Errorf("a jump pair is two icons with the same glyph and colour: src %s/%s/%s, dst %s/%s/%s",
				o.Type, o.Text, o.Style.Color, dst.Type, dst.Text, dst.Style.Color)
		}
		if o.Page == dst.Page {
			t.Errorf("the demo pair should cross pages (riff page → Verse 1), both on page %d", o.Page)
		}
	}
	if sources != 1 {
		t.Fatalf("want exactly one jump source in The Open Road's import, got %d", sources)
	}
}
