package sync

import (
	"testing"

	"troubastack/core/internal/domain"
)

// P206 uniqueness, server side. Fable's ⟨D⟩ ruling makes this a LEGIBILITY rule: two jumps wearing the
// same glyph and colour corrupt nothing (pairing is by uuid, the bake resolves both), so nothing
// downstream can ever notice. The harm is to a musician meeting two identical segnos on one page mid-song.
// Studio excludes the combination in its palette; this is the refusal for what gets past it — an older
// client, a script, or two authors racing — because a rule only the UI enforces is not enforced.

// jumpEngine serves a HEAD of jump sources plus a layer→file map, which is all jumpDuplicate reads.
type jumpEngine struct {
	objects []domain.Object
	files   map[string]string // layerID → fileID
}

func (e jumpEngine) Apply(string, domain.Mutation) (domain.Mutation, error) {
	return domain.Mutation{}, nil
}
func (e jumpEngine) Head(string) (domain.Snapshot, error) {
	return domain.Snapshot{Objects: e.objects}, nil
}
func (e jumpEngine) Layer(_ string, layerID string) (domain.Layer, bool) {
	f, ok := e.files[layerID]
	return domain.Layer{ID: layerID, FileID: f, OwnerID: "me", Access: domain.AccessRW}, ok
}
func (e jumpEngine) ObjectLayer(songID, uuid string) (domain.Layer, bool, bool) {
	for _, o := range e.objects {
		if o.UUID == uuid {
			l, ok := e.Layer(songID, o.LayerID)
			return l, ok, true
		}
	}
	return domain.Layer{}, false, false
}

func jump(uuid, layerID, glyph, color, target string) domain.Object {
	return domain.Object{
		UUID: uuid, LayerID: layerID, Type: domain.TypeIcon, Text: glyph, JumpTo: target,
		Style: domain.Style{Color: color, Opacity: 1},
	}
}

func TestJumpDuplicate_PerFileGlyphAndColour(t *testing.T) {
	// One existing pair on file A (layer LA) wearing a red segno.
	existing := jump("src-1", "LA", "segno", "#e11d48", "dest-1")
	eng := jumpEngine{
		objects: []domain.Object{existing, jump("dest-1", "LA", "segno", "#e11d48", "")},
		files:   map[string]string{"LA": "fileA", "LB": "fileB"},
	}

	cases := []struct {
		name string
		obj  domain.Object
		want bool
	}{
		{"a second red segno jump on the SAME file → refused", jump("src-2", "LA", "segno", "#e11d48", "dest-2"), true},
		{"the same combination on ANOTHER file is fine — they are never read side by side (⟨D2⟩)",
			jump("src-2", "LB", "segno", "#e11d48", "dest-2"), false},
		{"a different glyph, same colour", jump("src-2", "LA", "coda", "#e11d48", "dest-2"), false},
		{"the same glyph in a different colour — the rule is the COMBINATION", jump("src-2", "LA", "segno", "#2563eb", "dest-2"), false},
		{"colour case does not make it a different look", jump("src-2", "LA", "segno", "#E11D48", "dest-2"), true},
		{"a plain landmark that only LOOKS the same is not a jump", jump("plain", "LA", "segno", "#e11d48", ""), false},
		{"re-saving the SAME jump (a move) is not a duplicate of itself", existing, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &conn{authorID: "me", role: "member", songID: "song", hub: &Hub{eng: eng}}
			if got := c.jumpDuplicate(tc.obj); got != tc.want {
				t.Fatalf("jumpDuplicate = %v, want %v", got, tc.want)
			}
		})
	}
}

// The refusal has to reach the wire as its own reason, or the studio shows "out of date" for something a
// retry can never fix — the author needs to know to pick another glyph or colour.
func TestAuthorizeWrite_RefusesADuplicateJumpOnCreateAndOnEdit(t *testing.T) {
	eng := jumpEngine{
		objects: []domain.Object{jump("src-1", "LA", "segno", "#e11d48", "dest-1")},
		files:   map[string]string{"LA": "fileA"},
	}
	dup := objectJSON{
		UUID: "src-2", LayerID: "LA", Type: "icon", Text: "segno", JumpTo: "dest-2",
		Style: styleJSON{Color: "#e11d48", Opacity: 1},
	}
	c := &conn{authorID: "me", role: "member", songID: "song", hub: &Hub{eng: eng}}

	if reason, ok := c.authorizeWrite(domain.KindCreate, mutationJSON{UUID: "src-2", Object: &dup}); ok || reason != "jump-duplicate" {
		t.Fatalf("create of a duplicate jump = (%q, %v), want (\"jump-duplicate\", false)", reason, ok)
	}
	// An EDIT can create the collision too — recolour an existing jump onto one that already exists.
	if reason, ok := c.authorizeWrite(domain.KindSetStyle, mutationJSON{UUID: "src-1", Object: &dup}); ok || reason != "jump-duplicate" {
		t.Fatalf("edit into a duplicate = (%q, %v), want (\"jump-duplicate\", false)", reason, ok)
	}
	// The ordinary case still passes the gate.
	free := dup
	free.UUID, free.Text = "src-3", "coda"
	if reason, ok := c.authorizeWrite(domain.KindCreate, mutationJSON{UUID: "src-3", Object: &free}); !ok || reason != "" {
		t.Fatalf("a free combination = (%q, %v), want (\"\", true)", reason, ok)
	}
}
