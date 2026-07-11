package httpapi

// T37 lyrics-import unit tests — all OFF-NETWORK. The SSRF classifier (isBlockedIP) is
// the security-critical unit and is tabled exhaustively; parsers/classifier run against
// committed fixture HTML; the normalizer has its own table. The live azlyrics fetch is
// verified by hand at the gate (network + Cloudflare are non-deterministic), never in CI.

import (
	"net"
	"os"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true}, // loopback
		{"127.5.6.7", true}, // 127/8
		{"10.0.0.1", true},  // 10/8
		{"10.255.255.255", true},
		{"172.16.0.1", true}, // 172.16/12
		{"172.31.255.1", true},
		{"192.168.1.1", true},           // 192.168/16
		{"169.254.1.1", true},           // link-local
		{"0.0.0.0", true},               // unspecified
		{"::1", true},                   // v6 loopback
		{"fc00::1", true},               // ULA
		{"fe80::1", true},               // v6 link-local
		{"::", true},                    // v6 unspecified
		{"ff02::1", true},               // multicast
		{"224.0.0.1", true},             // v4 multicast
		{"8.8.8.8", false},              // public
		{"1.1.1.1", false},              // public
		{"172.32.0.1", false},           // just outside 172.16/12
		{"172.15.255.1", false},         // just outside 172.16/12
		{"2606:4700:4700::1111", false}, // public v6 (cloudflare)
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := isBlockedIP(ip); got != c.blocked {
			t.Errorf("isBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("isBlockedIP(nil) must be true (fail closed)")
	}
}

func TestValidateFetchURL(t *testing.T) {
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://www.azlyrics.com/lyrics/band/song.html", true},
		{"http://example.com/x", true},
		{"ftp://example.com/x", false},                      // scheme
		{"file:///etc/passwd", false},                       // scheme (classic SSRF/LFI)
		{"gopher://example.com", false},                     // scheme
		{"javascript:alert(1)", false},                      // scheme
		{"http://127.0.0.1/admin", false},                   // literal loopback
		{"http://169.254.169.254/latest/meta-data/", false}, // cloud metadata SSRF
		{"http://10.0.0.5/internal", false},                 // literal private
		{"http://[::1]:8080/", false},                       // literal v6 loopback
		{"https://192.168.0.1/", false},                     // literal private
		{"", false},                                         // empty
		{"not a url", false},                                // no host
	}
	for _, c := range cases {
		_, err := validateFetchURL(c.url)
		if (err == nil) != c.ok {
			t.Errorf("validateFetchURL(%q): err=%v, want ok=%v", c.url, err, c.ok)
		}
	}
}

func TestNormalizeLyrics(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"crlf", "a\r\nb\r\n", "a\nb"},
		{"cr", "a\rb", "a\nb"},
		{"trim outer", "\n\n  hello\n\n", "hello"},
		{"collapse blank runs", "a\n\n\n\n\nb", "a\n\nb"},
		{"keep single blank (section break)", "verse\n\nchorus", "verse\n\nchorus"},
		{"trailing ws", "a   \nb\t\n", "a\nb"},
		{"drop submit corrections", "line one\nSubmit Corrections", "line one"},
		{"drop writers credit", "words\nWriter(s): A. Person, B. Other", "words"},
		{"drop thanks-to", "words\nThanks to Someone for these lyrics", "words"},
		{"keep ambiguous line", "Submit your heart to the song", "Submit your heart to the song"},
		{"keep chords + labels", "## Verse 1\nG    D\nhello", "## Verse 1\nG    D\nhello"},
	}
	for _, c := range cases {
		if got := normalizeLyrics(c.in); got != c.want {
			t.Errorf("%s: normalizeLyrics(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestExtractAzlyrics(t *testing.T) {
	got := normalizeLyrics(extractAzlyrics(readFixture(t, "azlyrics_sample.html")))
	for _, want := range []string{"Well, I woke up this morning", "and the end is always near"} {
		if !strings.Contains(got, want) {
			t.Errorf("azlyrics extract missing %q; got:\n%s", want, got)
		}
	}
	// Must NOT drag in chrome/title/marker or the trailing site cruft.
	for _, bad := range []string{"AZLyrics", "Song Title", "third-party", "Submit Corrections", "ads and chrome"} {
		if strings.Contains(got, bad) {
			t.Errorf("azlyrics extract leaked chrome %q; got:\n%s", bad, got)
		}
	}
}

func TestExtractGeneric(t *testing.T) {
	got := normalizeLyrics(extractGeneric(readFixture(t, "generic_article.html")))
	for _, want := range []string{"First verse, line one", "The chorus goes here"} {
		if !strings.Contains(got, want) {
			t.Errorf("generic extract missing %q; got:\n%s", want, got)
		}
	}
	for _, bad := range []string{"tracking", "document.write", "Home | About", "all rights reserved"} {
		if strings.Contains(got, bad) {
			t.Errorf("generic extract leaked chrome %q; got:\n%s", bad, got)
		}
	}
}

func TestClassifyFetch(t *testing.T) {
	az := readFixture(t, "azlyrics_sample.html")
	generic := readFixture(t, "generic_article.html")
	block := readFixture(t, "cloudflare_block.html")

	if r := classifyFetch("www.azlyrics.com", 200, []byte(az)); r.Status != "ok" || !strings.Contains(r.Text, "woke up this morning") {
		t.Errorf("azlyrics 200 → %+v, want ok with lyrics", r)
	}
	if r := classifyFetch("example.com", 200, []byte(generic)); r.Status != "ok" || !strings.Contains(r.Text, "First verse") {
		t.Errorf("generic 200 → %+v, want ok with text", r)
	}
	if r := classifyFetch("www.azlyrics.com", 403, nil); r.Status != "blocked" {
		t.Errorf("403 → %+v, want blocked", r)
	}
	if r := classifyFetch("www.azlyrics.com", 200, []byte(block)); r.Status != "blocked" {
		t.Errorf("200+challenge → %+v, want blocked", r)
	}
	if r := classifyFetch("example.com", 200, []byte("<html><body></body></html>")); r.Status != "error" {
		t.Errorf("empty page → %+v, want error (no lyrics found)", r)
	}
	if r := classifyFetch("example.com", 500, nil); r.Status != "error" {
		t.Errorf("500 → %+v, want error", r)
	}
}
