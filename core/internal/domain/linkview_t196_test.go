package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// T196: the serve-time view is never persisted. The store JSON-encodes Objects (filestore) and the v1 band
// manifest is domain.Object as is, so `json:"-"` on View is what keeps it out of both.
func TestT196_ViewIsNotInTheStoredJSON(t *testing.T) {
	o := Object{UUID: "u", Link: &MarkLink{State: LinkLinked, Line: "L"}, View: &LinkView{At: LinkAt{Page: 1, X: 0.5}, Text: "serve-time text"}}
	b, err := json.Marshal(Mutation{Kind: KindMove, Object: &o})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "View") || strings.Contains(string(b), "serve-time text") {
		t.Errorf("stored mutation JSON carries the view: %s", b)
	}
	if !strings.Contains(string(b), `"line":"L"`) {
		t.Errorf("control: the link itself must be stored: %s", b)
	}
}

// Clone deep-copies the view, so a served copy cannot alias another.
func TestT196_CloneCopiesTheView(t *testing.T) {
	o := Object{View: &LinkView{Text: "a"}}
	c := o.Clone()
	c.View.Text = "b"
	if o.View.Text != "a" {
		t.Errorf("Clone aliased the view")
	}
	w := WireLinkOf(Object{Link: &MarkLink{Line: "L"}, View: &LinkView{At: LinkAt{X: 0.3}, Text: "t"}})
	if w == nil || w.At == nil || w.At.X != 0.3 || w.Text != "t" || w.Stored().Line != "L" {
		t.Errorf("WireLinkOf %+v", w)
	}
	if WireLinkOf(Object{View: &LinkView{}}) != nil {
		t.Errorf("no link, no wire link")
	}
	var nilW *WireLink
	if nilW.Stored() != nil {
		t.Errorf("nil-safe Stored")
	}
}
