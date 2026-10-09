package httpapi

import (
	"net/url"
	"reflect"
	"testing"
)

// TestParseLayersParam: absent ≠ present-but-empty. Absent selects the default view; present (even empty)
// is an explicit list, blanks dropped. (Which layers that list paints — mandatory always — is pinned at
// bake.fileLayerVisible, TestFileLayerVisible.)
func TestParseLayersParam(t *testing.T) {
	cases := []struct {
		query    string
		ids      []string
		explicit bool
	}{
		{"", nil, false},
		{"other=1", nil, false},
		{"layers=", nil, true},
		{"layers=a", []string{"a"}, true},
		{"layers=a,b,,%20c%20", []string{"a", "b", "c"}, true},
		{"layers=a&layers=b", []string{"a", "b"}, true},
	}
	for _, c := range cases {
		q, err := url.ParseQuery(c.query)
		if err != nil {
			t.Fatalf("parse %q: %v", c.query, err)
		}
		ids, explicit := parseLayersParam(q)
		if explicit != c.explicit || !reflect.DeepEqual(ids, c.ids) {
			t.Errorf("%q: got (%v, %v), want (%v, %v)", c.query, ids, explicit, c.ids, c.explicit)
		}
	}
}
