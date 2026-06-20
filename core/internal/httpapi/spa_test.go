package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":        {Data: []byte(`<!doctype html><div id="root"></div>`)},
		"assets/app-abc.js": {Data: []byte("console.log(1)")},
	}
	h := spaHandler(assets)
	get := func(p string) (int, string) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
		return rr.Code, rr.Body.String()
	}

	cases := []struct {
		path     string
		wantCode int
		wantRoot bool // body should be the SPA shell (index.html)
	}{
		{"/assets/app-abc.js", 200, false},  // real asset served as-is
		{"/", 200, true},                    // root → index
		{"/bands", 200, true},               // SPA deep link → index fallback
		{"/bands/123/songs/456", 200, true}, // nested SPA route → index fallback
		{"/api/bogus", 404, false},          // unknown API path is a real 404, never the SPA
	}
	for _, c := range cases {
		code, body := get(c.path)
		if code != c.wantCode {
			t.Errorf("%s: code = %d, want %d", c.path, code, c.wantCode)
		}
		if c.wantRoot && !strings.Contains(body, `id="root"`) {
			t.Errorf("%s: expected SPA shell, got %q", c.path, body)
		}
		if c.path == "/assets/app-abc.js" && !strings.Contains(body, "console.log") {
			t.Errorf("asset not served verbatim: %q", body)
		}
	}
}
