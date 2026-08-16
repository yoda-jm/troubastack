package httpapi

// T37 — "New chart from lyrics": a best-effort, server-side URL fetch that funnels
// azlyrics (and any other page) into a text chart, with a paste fallback in the UI.
//
// VLL made azlyrics a must-have and owns the ToS/copyright call (a private, self-hosted
// tool). The BOUNDARY that does not move: this is an HONEST fetch only — a plain GET with
// a truthful User-Agent, NO anti-bot / Cloudflare-challenge evasion (that is detection-
// evasion tooling, out of scope on principle). azlyrics is Cloudflare-gated, so an honest
// GET will OFTEN come back blocked; that is a normal outcome ({status:"blocked"}), never a
// 500, and the UI's paste fallback is what makes the feature reliable.
//
// A URL-fetch endpoint is an SSRF vector, so the fetch is guarded at DIAL time: every
// TCP connection (including redirect hops) resolves the host and refuses any private /
// loopback / link-local / unspecified address. isBlockedIP is the security core and is
// exhaustively unit-tested.
//
// Two request shapes:
//   - {url}            fetch + scrape that page (azlyrics dedicated extractor, else generic).
//   - {artist, title}  "search by song": query lyrics.ovh (a free JSON lyrics API) and return
//                      its lyrics. Same guarded GET; JSON parse instead of HTML.
//
// THIRD-PARTY DATA FLOW (be explicit): the {artist, title} path sends the song's artist and
// title to lyrics.ovh (or whatever TROUBA_LYRICS_OVH_BASE points at). A self-hosted deployment
// can repoint that env var at a mirror, or set it to "off" to DISABLE the search entirely (the
// {url}/paste path still works). See lyricsOvhBase.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"troubastack/core/internal/app"
)

const (
	lyricsFetchTimeout = 5 * time.Second
	lyricsMaxBytes     = 1 << 20 // 1 MB — lyrics are tiny; cap the read.
	lyricsMaxRedirects = 2
	lyricsUserAgent    = "troubacore/1.0 (+https://github.com/yoda-jm/troubastack; lyrics import)"
	// lyrics.ovh is a free JSON lyrics API (GET /v1/{artist}/{title} → {"lyrics": "..."}). It
	// has no bot wall and good coverage, so it powers the "search by artist/title" path — no URL
	// to paste. Same honest-GET + SSRF dial guard as the URL path; only the parse differs (JSON).
	lyricsOvhDefaultBase = "https://api.lyrics.ovh/v1/"
)

// lyricsOvhBase returns the base URL for the artist/title lyrics search, honoring
// TROUBA_LYRICS_OVH_BASE (like the baker reads TROUBA_PDFTOPPM/… directly): unset → the public
// lyrics.ovh default; a URL → point at a self-hosted mirror; "off" (any case) → "" to DISABLE
// the search entirely (the artist/title path then returns a clean "disabled" error). Note this
// path sends the song's artist+title to that third-party service — a self-hosted deployment can
// repoint or disable it here.
func lyricsOvhBase() string {
	v := strings.TrimSpace(os.Getenv("TROUBA_LYRICS_OVH_BASE"))
	switch {
	case v == "":
		return lyricsOvhDefaultBase
	case strings.EqualFold(v, "off"):
		return ""
	default:
		if !strings.HasSuffix(v, "/") {
			v += "/"
		}
		return v
	}
}

// withinBasePath reports whether the built URL u, after path.Clean, still sits under base's path
// prefix — i.e. a dot-segment field ("..") didn't walk out of the API prefix (e.g. /v1/../admin).
func withinBasePath(base, u string) bool {
	bu, err := url.Parse(base)
	if err != nil {
		return false
	}
	pu, err := url.Parse(u)
	if err != nil {
		return false
	}
	prefix := strings.TrimSuffix(bu.EscapedPath(), "/") + "/"
	return strings.HasPrefix(path.Clean(pu.EscapedPath())+"/", prefix)
}

var (
	errBlockedHost = errors.New("host resolves to a disallowed (private/loopback) address")
	errBadScheme   = errors.New("only http and https URLs are allowed")
)

// isBlockedIP reports whether an address must never be fetched from a URL-import
// endpoint: loopback, RFC1918/ULA private, link-local, unspecified, or multicast.
// This is the SSRF classifier — keep it strict; when in doubt, block.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || // 127/8, ::1
		ip.IsPrivate() || // 10/8, 172.16/12, 192.168/16, fc00::/7
		ip.IsLinkLocalUnicast() || // 169.254/16, fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() // 0.0.0.0, ::
}

// validateFetchURL enforces the scheme allow-list and, for literal-IP hosts, the SSRF
// classifier up front. Hostname hosts are additionally checked at dial time (below),
// which also defends against DNS rebinding and redirect targets.
func validateFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, app.ErrInvalidInput
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errBadScheme
	}
	if u.Host == "" {
		return nil, app.ErrInvalidInput
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && isBlockedIP(ip) {
		return nil, errBlockedHost
	}
	return u, nil
}

// safeDialContext resolves the target host and refuses to connect to any blocked
// address BEFORE dialing — so a hostname that resolves (or rebinds, or a redirect
// points) to a private/loopback IP is rejected at the connection, not just up front.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return nil, errBlockedHost
		}
	}
	d := net.Dialer{Timeout: lyricsFetchTimeout}
	// Dial the validated IP directly (TLS SNI/Host still come from the URL).
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func lyricsHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   lyricsFetchTimeout,
		Transport: &http.Transport{DialContext: safeDialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= lyricsMaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			// Re-validate the redirect target's scheme/literal-IP; the dial guard
			// re-checks the resolved IP for every hop regardless.
			if _, err := validateFetchURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// lyricsResult is the endpoint's response shape. status is one of ok|blocked|error.
type lyricsResult struct {
	Status string `json:"status"`
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// looksBlocked detects a bot wall (Cloudflare/JS challenge) in an otherwise-200 body,
// or a hard 403/429/503. A block is a NORMAL outcome, not an error.
func looksBlocked(status int, body []byte) bool {
	if status == http.StatusForbidden || status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		return true
	}
	low := strings.ToLower(string(body))
	for _, sig := range []string{"just a moment", "cf-browser-verification", "cf-challenge", "attention required", "enable javascript and cookies", "/cdn-cgi/challenge-platform"} {
		if strings.Contains(low, sig) {
			return true
		}
	}
	return false
}

// classifyFetch maps a completed HTTP response to a lyricsResult. Pure + testable: the
// fetch does the (guarded) network I/O and hands the outcome here.
func classifyFetch(host string, status int, body []byte) lyricsResult {
	if looksBlocked(status, body) {
		return lyricsResult{Status: "blocked", Reason: "the site blocked the request (bot protection)"}
	}
	if status < 200 || status >= 300 {
		return lyricsResult{Status: "error", Reason: fmt.Sprintf("upstream returned HTTP %d", status)}
	}
	var text string
	if isAzlyricsHost(host) {
		text = extractAzlyrics(string(body))
	} else {
		text = extractGeneric(string(body))
	}
	text = normalizeLyrics(text)
	if strings.TrimSpace(text) == "" {
		return lyricsResult{Status: "error", Reason: "no lyrics found on the page"}
	}
	return lyricsResult{Status: "ok", Text: text}
}

// guardedGet performs the honest, SSRF-guarded GET shared by the URL and lyrics.ovh paths and
// returns the validated URL, HTTP status, and (capped) body. Transport/validation errors are
// returned so callers can map them to {status:"error"} — never a 500.
func guardedGet(ctx context.Context, rawURL string) (*url.URL, int, []byte, error) {
	u, err := validateFetchURL(rawURL)
	if err != nil {
		return nil, 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", lyricsUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json")
	resp, err := lyricsHTTPClient().Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, lyricsMaxBytes))
	return u, resp.StatusCode, body, nil
}

// fetchLyrics validates + fetches + classifies an arbitrary lyrics page URL. Any transport error
// (timeout, DNS, blocked dial) becomes {status:"error"} — never a 500.
func fetchLyrics(ctx context.Context, rawURL string) lyricsResult {
	ctx, cancel := context.WithTimeout(ctx, lyricsFetchTimeout)
	defer cancel()
	u, status, body, err := guardedGet(ctx, rawURL)
	if err != nil {
		if errors.Is(err, errBlockedHost) {
			return lyricsResult{Status: "error", Reason: errBlockedHost.Error()}
		}
		if errors.Is(err, errBadScheme) || errors.Is(err, app.ErrInvalidInput) {
			return lyricsResult{Status: "error", Reason: err.Error()}
		}
		return lyricsResult{Status: "error", Reason: "could not reach the site"}
	}
	return classifyFetch(u.Hostname(), status, body)
}

// fetchLyricsOvh queries lyrics.ovh for {artist, title} — the "search by song" path. Same guarded
// GET; JSON parse instead of HTML extraction.
func fetchLyricsOvh(ctx context.Context, artist, title string) lyricsResult {
	artist, title = strings.TrimSpace(artist), strings.TrimSpace(title)
	if artist == "" || title == "" {
		return lyricsResult{Status: "error", Reason: "artist and title are required"}
	}
	base := lyricsOvhBase()
	if base == "" {
		return lyricsResult{Status: "error", Reason: "lyrics search is disabled on this server"}
	}
	// A "/" in a field (e.g. "AC/DC") would split the path — space it out; PathEscape the rest.
	u := base +
		url.PathEscape(strings.ReplaceAll(artist, "/", " ")) + "/" +
		url.PathEscape(strings.ReplaceAll(title, "/", " "))
	// Dot-segment guard: PathEscape does NOT escape "." and Go doesn't normalize request paths,
	// so a field of ".." would build /v1/../admin and escape the API prefix — dangerous when the
	// base points at a self-hosted mirror. Reject if the cleaned path leaves the base's prefix;
	// legitimate dots ("R.E.M.", "St. Vincent") are single segments and survive path.Clean.
	if !withinBasePath(base, u) {
		return lyricsResult{Status: "error", Reason: "invalid artist or title"}
	}
	ctx, cancel := context.WithTimeout(ctx, lyricsFetchTimeout)
	defer cancel()
	_, status, body, err := guardedGet(ctx, u)
	if err != nil {
		if errors.Is(err, errBlockedHost) {
			return lyricsResult{Status: "error", Reason: errBlockedHost.Error()}
		}
		return lyricsResult{Status: "error", Reason: "could not reach the lyrics API"}
	}
	return parseLyricsOvh(status, body)
}

// parseLyricsOvh maps a lyrics.ovh response to a lyricsResult. Pure + testable. A 404 (unknown
// song) is a normal {status:"error"}, not a failure.
func parseLyricsOvh(status int, body []byte) lyricsResult {
	if status == http.StatusNotFound {
		return lyricsResult{Status: "error", Reason: "no lyrics found for that artist/title"}
	}
	if status < 200 || status >= 300 {
		return lyricsResult{Status: "error", Reason: fmt.Sprintf("lyrics API returned HTTP %d", status)}
	}
	var payload struct {
		Lyrics string `json:"lyrics"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return lyricsResult{Status: "error", Reason: "lyrics API returned an unexpected response"}
	}
	text := normalizeLyrics(payload.Lyrics)
	if payload.Error != "" || strings.TrimSpace(text) == "" {
		return lyricsResult{Status: "error", Reason: "no lyrics found for that artist/title"}
	}
	return lyricsResult{Status: "ok", Text: text}
}

func isAzlyricsHost(host string) bool {
	host = strings.ToLower(host)
	return host == "azlyrics.com" || strings.HasSuffix(host, ".azlyrics.com")
}

// ---- parsers (host-dispatched) --------------------------------------------

var (
	tagRE = regexp.MustCompile(`(?is)<[^>]+>`)
	// RE2 has no backreferences, so each chrome tag is matched open→close explicitly.
	scriptRE = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>` +
		`|<style\b[^>]*>.*?</style>` +
		`|<head\b[^>]*>.*?</head>` +
		`|<nav\b[^>]*>.*?</nav>` +
		`|<header\b[^>]*>.*?</header>` +
		`|<footer\b[^>]*>.*?</footer>` +
		`|<noscript\b[^>]*>.*?</noscript>`)
	brRE         = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockCloseRE = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li)>`)
	pRE          = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	// azlyrics marks the lyrics div with a stable comment; the lyrics follow until the
	// next </div>. This is the site's landmark, not an evasion — just where the text is.
	azMarkerRE = regexp.MustCompile(`(?is)<!--\s*Usage of azlyrics\.com.*?-->(.*?)</div>`)
)

// stripTags turns an HTML fragment into text: <br> and block-closers become newlines,
// remaining tags are dropped, entities are unescaped.
func stripTags(frag string) string {
	frag = brRE.ReplaceAllString(frag, "\n")
	frag = blockCloseRE.ReplaceAllString(frag, "\n")
	frag = tagRE.ReplaceAllString(frag, "")
	return html.UnescapeString(frag)
}

func extractAzlyrics(htmlBody string) string {
	m := azMarkerRE.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return stripTags(m[1])
}

// extractGeneric is a readability-ish fallback: drop script/style/chrome, then take the
// joined <p> text (the text-dense block); if there are no paragraphs, strip the whole body.
func extractGeneric(htmlBody string) string {
	cleaned := scriptRE.ReplaceAllString(htmlBody, " ")
	ps := pRE.FindAllStringSubmatch(cleaned, -1)
	if len(ps) > 0 {
		var b strings.Builder
		for _, p := range ps {
			t := strings.TrimSpace(stripTags(p[1]))
			if t != "" {
				b.WriteString(t)
				b.WriteString("\n\n")
			}
		}
		if strings.TrimSpace(b.String()) != "" {
			return b.String()
		}
	}
	return stripTags(cleaned)
}

// ---- normalizer (mirrored minimally on the client for the paste path) ------

var (
	trailingWSRE = regexp.MustCompile(`[ \t]+\n`)
	blankRunRE   = regexp.MustCompile(`\n{3,}`)
	cruftRE      = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^submit corrections?\.?$`),
		regexp.MustCompile(`(?i)^writer\(s\):.*$`),
		regexp.MustCompile(`(?i)^thanks to .* for (these lyrics|correcting these lyrics).*$`),
		regexp.MustCompile(`(?i)^\d+ contributors?$`),
	}
)

// normalizeLyrics is deliberately minimal: CRLF→LF, trim outer whitespace, collapse 3+
// blank lines to a single section break, and drop ONLY lines matching a tiny, exact
// site-cruft blacklist. When in doubt it KEEPS the line — it never touches section
// labels, chords, brackets, or case.
func normalizeLyrics(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = trailingWSRE.ReplaceAllString(s, "\n")
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		if isCruft(strings.TrimSpace(ln)) {
			continue
		}
		kept = append(kept, ln)
	}
	out := strings.Join(kept, "\n")
	out = blankRunRE.ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}

func isCruft(line string) bool {
	if line == "" {
		return false
	}
	for _, re := range cruftRE {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// ---- handler --------------------------------------------------------------

// lyricsImport (POST /api/bands/{bandId}/lyrics-import) is band-scoped: only a member may
// spend the server's egress. It never 500s on a blocked/unreachable upstream — the outcome
// rides in the body so the UI can fall back to paste.
func (a *WebAPI) lyricsImport(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	if _, _, err := a.svc.GetBand(u, bandID); err != nil { // membership + existence
		writeErr(w, err)
		return
	}
	var in struct {
		URL    string `json:"url"`
		Artist string `json:"artist"`
		Title  string `json:"title"`
	}
	if !decode(w, r, &in) {
		return
	}
	// "Search by song": artist+title → lyrics.ovh. Otherwise a URL to fetch+scrape.
	if strings.TrimSpace(in.Artist) != "" && strings.TrimSpace(in.Title) != "" {
		writeJSON(w, http.StatusOK, fetchLyricsOvh(r.Context(), in.Artist, in.Title))
		return
	}
	if strings.TrimSpace(in.URL) == "" {
		writeErr(w, app.ErrInvalidInput)
		return
	}
	writeJSON(w, http.StatusOK, fetchLyrics(r.Context(), in.URL))
}
