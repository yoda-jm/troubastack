package sync

import (
	"encoding/json"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// T196: the realtime wire WRITES a mark's serve-time view (link.at / link.text) and never READS one: a
// client cannot tell the server where its linked text is.
func TestT196_WireWritesTheViewAndNeverReadsIt(t *testing.T) {
	o := domain.Object{UUID: "u", Type: domain.TypeRect, Link: &domain.MarkLink{State: domain.LinkCheck, Line: "L", XRef: "start", LH: 0.02, Render: "R"},
		View: &domain.LinkView{At: domain.LinkAt{Page: 2, X: 0.125, Y: 0.5, H: 0.02}, Text: "a line"}}
	j := objectToJSON(o)
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), `"at":{"page":2,"x":0.125,"y":0.5,"h":0.02}`) || !strings.Contains(string(b), `"text":"a line"`) || !strings.Contains(string(b), `"state":"check"`) {
		t.Fatalf("wire JSON %s lacks link.state / link.at / link.text", b)
	}
	var in objectJSON
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	back := objectFromJSON(in)
	if back.View != nil {
		t.Errorf("objectFromJSON read a client's view: %+v", back.View)
	}
	if back.Link == nil || back.Link.State != domain.LinkCheck || back.Link.Line != "L" {
		t.Errorf("the link itself must survive: %+v", back.Link)
	}
	// no view → no at/text keys at all
	o.View = nil
	b, _ = json.Marshal(objectToJSON(o))
	if strings.Contains(string(b), `"at"`) || strings.Contains(string(b), `"text":"a`) {
		t.Errorf("a mark without a view carries one: %s", b)
	}
}
