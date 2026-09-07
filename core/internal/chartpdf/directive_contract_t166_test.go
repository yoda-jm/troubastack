package chartpdf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// T166 — the directive vocabulary is pinned to ONE file, and both sides answer to it.
//
// The editor's "Chart format" help documented none of the seven directives the engine accepts, which is why
// none of them were used: across a real 178-file library the ONLY directive family present was the one the
// UI happens to mention by name. A list of directives written by hand in a second language is the shape that
// rots silently, so the list now lives in docs/contracts/chart-directives.json and this test asserts the
// ENGINE matches it — a Studio-side test asserts the help does.
//
// Both directions matter. Adding a directive to the engine without adding it here reddens (the source scan
// below), and listing one here that the engine does not accept reddens too.

type directiveContract struct {
	Markers []struct {
		Canonical string `json:"canonical"`
		Alias     string `json:"alias"`
	} `json:"markers"`
	Header []struct {
		Key string `json:"key"`
	} `json:"header"`
}

func loadDirectiveContract(t *testing.T) directiveContract {
	t.Helper()
	var path string
	for _, p := range []string{
		"../../../docs/contracts/chart-directives.json",
		"../../docs/contracts/chart-directives.json",
		"docs/contracts/chart-directives.json",
	} {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Fatal("chart-directives.json not found — the contract is the point of this test")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var c directiveContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return c
}

// Every marker in the contract — canonical AND alias — is recognised by the engine.
func TestDirectiveContract_EngineAcceptsEveryMarker_T166(t *testing.T) {
	c := loadDirectiveContract(t)
	if len(c.Markers) == 0 {
		t.Fatal("contract lists no markers")
	}
	for _, m := range c.Markers {
		for _, name := range []string{m.Canonical, m.Alias} {
			line := "{" + name + "}"
			if !isNewPageMarker(line) && !isFootnoteMarker(line) && !isTabStart(line) && !isTabEnd(line) {
				t.Errorf("the engine does not recognise %q, but the contract lists it", line)
			}
		}
	}
	// A marker the contract does NOT list must not be recognised either — otherwise the contract is a
	// subset rather than the vocabulary.
	if isNewPageMarker("{bogus}") || isFootnoteMarker("{bogus}") || isTabStart("{bogus}") || isTabEnd("{bogus}") {
		t.Error("{bogus} is recognised — the vocabulary is wider than the contract says")
	}
}

// Every header key in the contract is consumed by parseHeader, and one that is not listed stays body text.
func TestDirectiveContract_EngineConsumesEveryHeaderKey_T166(t *testing.T) {
	c := loadDirectiveContract(t)
	if len(c.Header) == 0 {
		t.Fatal("contract lists no header directives")
	}
	value := map[string]string{"size": "13", "fit": "page", "columns": "2"}
	for _, h := range c.Header {
		v, ok := value[h.Key]
		if !ok {
			t.Fatalf("contract lists header key %q but this test has no sample value for it — add one", h.Key)
		}
		src := []string{"# T", h.Key + ": " + v, "", "## V", "x"}
		_, _, _, _, _, _, skip := parseHeader(src)
		if !skip[1] {
			t.Errorf("parseHeader did not consume %q — the contract says it is a directive", src[1])
		}
	}
	// An UNLISTED key: value line is not a directive. It is lifted as the artist/subtitle (that is the
	// dialect: the line under the title is the artist, "Foo: Bar" and all), so the meaningful assertion
	// is not "was the line consumed" but "did it change the chart" — it must not.
	sub, _, bodyPt, sizeSet, autoFit, cols, _ := parseHeader([]string{"# T", "capo: 3", "", "## V", "x"})
	if sizeSet || autoFit || cols > 1 || bodyPt != defaultBodyPt {
		t.Errorf("\"capo: 3\" changed the chart (size=%v set=%v fit=%v cols=%d) — an unlisted key must not be a directive", bodyPt, sizeSet, autoFit, cols)
	}
	if sub != "capo: 3" {
		t.Errorf("subtitle = %q, want the unlisted line lifted as the artist line", sub)
	}
}

// A SOURCE scan: no directive regex may exist in the engine that the contract does not describe. This is
// what makes the contract a vocabulary rather than a wish — add `{chorus}` or `capo:` to the engine and
// this reddens until the contract (and therefore the editor's help) learns about it.
func TestDirectiveContract_NoUndeclaredDirectiveRegex_T166(t *testing.T) {
	c := loadDirectiveContract(t)
	declared := map[string]bool{}
	for _, m := range c.Markers {
		declared[m.Canonical], declared[m.Alias] = true, true
	}
	for _, h := range c.Header {
		declared[h.Key] = true
	}
	reBrace := regexp.MustCompile(`MustCompile\(` + "`" + `\(\?i\)\^\\\{\(([a-z_|]+)\)`)
	reKey := regexp.MustCompile(`MustCompile\(` + "`" + `\(\?i\)\^([a-z_]+)\\s\*:`)
	for _, f := range []string{"chart.go", "chart_tab.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range reBrace.FindAllStringSubmatch(string(src), -1) {
			for _, name := range strings.Split(m[1], "|") {
				if !declared[name] {
					t.Errorf("%s declares brace directive {%s}, which docs/contracts/chart-directives.json does not list — add it there and to the editor's Chart format help", f, name)
				}
			}
		}
		for _, m := range reKey.FindAllStringSubmatch(string(src), -1) {
			if !declared[m[1]] {
				t.Errorf("%s declares header directive %q, which the contract does not list", f, m[1])
			}
		}
	}
}
