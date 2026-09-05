package bake

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder for image.DecodeConfig
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// Config carries the external toolchain locations + output root. Zero values fall
// back to env (TROUBA_PDFTOPPM / TROUBA_NODE / TROUBA_BAKE_CLI) then sane defaults.
type Config struct {
	BakesDir string // where bundles are written (default <data>/bakes)
	Pdftoppm string // pdftoppm binary
	Node     string // node binary
	BakeCLI  string // path to web/bake/dist/cli.js
	DPI      int    // raster DPI (default 150)
	// CacheDir enables the T120 content-keyed render cache at this dir. Empty ⇒ caching OFF — which is
	// how mem-backed cores (e2e/tests/demo) stay isolated for free; production sets it (a subdir of the
	// data dir, or TROUBA_RENDER_CACHE). A cache test sets it to a per-test temp dir explicitly.
	CacheDir string
	// Raster/Overlays inject the two shell-out steps — DEPENDENCY INJECTION, nil in production (the real
	// poppler + node CLI are used). A test uses this to substitute a controllable rasterizer, e.g. one
	// that blocks so the T103 "a hung-up client does not cancel the bake" race has an observable window.
	Raster   Rasterizer
	Overlays OverlayRenderer
}

// Baker turns a setlist into a downloadable .tstage (I11): resolve songs →
// rasterize the default file's pages (poppler) → render per-layer overlays via
// web/bake (I8) → assemble a ConcertBundle dir + zip. It NEVER renders strokes.
type Baker struct {
	svc      *app.Service
	eng      *engine.Engine
	raster   Rasterizer
	overlays OverlayRenderer
	cache    *renderCache // T120 render cache (nil ⇒ disabled); held for PurgeCache
	bakesDir string
	now      func() int64 // injectable clock (unix seconds) for deterministic tests
	// afterNextRev, if set, is called once right after nextRev returns — a TEST SEAM
	// (nil in production) letting the B08 test deterministically publish a concurrent
	// rev in the window between our nextRev and our claim/publish.
	afterNextRev func()
	// progress is the T96 bake-progress registry (nil ⇒ progress publishing is a no-op,
	// e.g. a bare struct-literal Baker in a legacy test). newBakeID mints the per-bake id
	// (nil ⇒ the package default); onProgress is a TEST SEAM (nil in production) that
	// observes every publish so a test can assert the running sequence, not just the end.
	progress   *progressRegistry
	newBakeID  func() string
	onProgress func(id string, p BakeProgress)
	// logf writes the FULL internal failure detail (stderr, resolved binary/script paths, stack
	// frames) to the server log — and nowhere else (T102). Injectable so a test can capture it;
	// defaults to log.Printf.
	logf func(format string, args ...any)
}

// New builds a Baker with the real poppler + web/bake shell-out steps. A missing
// binary is not detected here — it surfaces as a clear per-bake error (I: never
// crash the server), so the server still starts without the toolchain installed.
func New(svc *app.Service, eng *engine.Engine, cfg Config) *Baker {
	dpi := cfg.DPI
	if dpi == 0 {
		dpi = 150
	}
	var raster Rasterizer = popplerRasterizer{bin: cfg.Pdftoppm, dpi: dpi}
	if cfg.Raster != nil {
		raster = cfg.Raster
	}
	var overlays OverlayRenderer = nodeOverlayRenderer{node: cfg.Node, cli: cfg.BakeCLI}
	if cfg.Overlays != nil {
		overlays = cfg.Overlays
	}
	// T120: wrap both render stages with the content-keyed cache when it's enabled. A cache-construct
	// error (bad dir) degrades to no cache rather than failing the server — never crash on bake config.
	var cache *renderCache
	if c, err := newRenderCache(cfg.CacheDir); err == nil && c != nil {
		cache = c
		raster = cachingRasterizer{inner: raster, cache: cache, dpi: dpi, popplerVer: popplerVersion(cfg.Pdftoppm)}
		overlays = cachingOverlayRenderer{inner: overlays, cache: cache, inkVer: inkVersion(cfg.BakeCLI)}
	}
	return &Baker{
		svc:       svc,
		eng:       eng,
		raster:    raster,
		overlays:  overlays,
		cache:     cache,
		bakesDir:  cfg.BakesDir,
		now:       func() int64 { return time.Now().Unix() },
		progress:  newProgressRegistry(nil),
		newBakeID: newBakeID,
		logf:      log.Printf,
	}
}

// bakeError is a bake failure split in two (T102): Error() is the SHORT, user-safe message that may
// reach the browser + the app (one line, no paths, no stack frames), while the full internal detail
// was already logged server-side by Baker.fail. A band member reading Error() learns whether it's
// their problem or the server's — and never sees a stderr blob.
type bakeError struct{ human string }

func (e *bakeError) Error() string { return e.human }

// fail logs the full internal detail to the server log and returns a bakeError carrying only `human`.
// Use it at a failure point where the cause is understood; the returned error can be wrapped freely —
// humanize finds it through errors.As.
func (b *Baker) fail(human string, detail error) error {
	if b.logf != nil {
		b.logf("bake failed: %s — %v", human, detail)
	}
	return &bakeError{human: human}
}

// humanize is the single choke point that guarantees no raw error reaches the wire: a *bakeError
// (however wrapped) passes through unchanged, and anything else is logged and replaced with a generic
// user-safe message. Applied to Bake's error return in the defer, so BOTH the POST body and
// BakeProgress.Error carry only human text — even from a failure point that didn't compose its own.
func (b *Baker) humanize(err error) error {
	if err == nil {
		return nil
	}
	var be *bakeError
	if errors.As(err, &be) {
		return be
	}
	if b.logf != nil {
		b.logf("bake failed (unsanitised): %v", err)
	}
	return &bakeError{human: "The bake failed. Ask an admin to check the server — the log has the details."}
}

// newBakeID mints an unguessable, per-bake identifier as a canonical v4 UUID. NOT the setlist id:
// concurrent bakes of one setlist are legal (B08/B09) and must not share a progress slot. The UUID
// SHAPE matters for T103: the async edge mints one via NewBakeID and hands it back to Bake as a
// self-supplied id, which is honoured only if ValidBakeID accepts it — so a minted id must itself be a
// well-formed UUID.
func newBakeID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// publish records the latest progress for a bake to the registry (if any) and notifies
// the test observer (if any). Non-blocking by construction: a map write under a short
// mutex, never I/O — a slow observer can't slow a bake (T96 §3.1).
func (b *Baker) publish(bakeID, bandID, setlistID string, p BakeProgress) {
	if b.progress != nil {
		b.progress.set(bakeID, bandID, setlistID, p)
	}
	if b.onProgress != nil {
		b.onProgress(bakeID, p)
	}
}

// NewBakeID mints a fresh, unguessable per-bake id — exported so the async HTTP edge (T103) can put
// the id in the 202 response BEFORE kicking the bake, then hand the SAME id to Bake as its (self-)
// supplied id. ValidBakeID reports whether a client-supplied id is a canonical UUID the edge may reuse.
func NewBakeID() string         { return newBakeID() }
func ValidBakeID(s string) bool { return validBakeID(s) }

// SetWarnings attaches T60 transpose warnings to a finished bake's terminal record so the polling
// client can read them once it sees `succeeded` (T103 — they no longer ride the async POST body).
func (b *Baker) SetWarnings(bandID, setlistID, bakeID string, warnings []string) {
	if b.progress != nil && len(warnings) > 0 {
		b.progress.setWarnings(bakeID, bandID, setlistID, warnings)
	}
}

// Progress returns a bake's latest progress, scoped to the band+setlist that owns it —
// an unknown/expired id or a cross-band request reports ok=false (the edge maps that to
// 404). Same authorisation as the bake: the HTTP edge additionally gates on band admin.
func (b *Baker) Progress(bandID, setlistID, bakeID string) (BakeProgress, bool) {
	if b.progress == nil {
		return BakeProgress{}, false
	}
	return b.progress.get(bakeID, bandID, setlistID)
}

// concertSep separates a setlist id from a member id in a per-member variant
// concert id (<setlistId>~<userId>, B07). Setlist and user ids are UUIDs (no
// '~'), so the split back into (setlist, user) is unambiguous.
const concertSep = "~"

// ParseConcertID splits a concert id into its base setlist id and, for a
// per-member variant, the owner's user id. isVariant is false for a shared band
// concert (id == setlist id). Used by the httpapi edge to scope listing +
// download (a member sees band concerts + only their OWN variants).
func ParseConcertID(id string) (setlistID, userID string, isVariant bool) {
	if i := strings.IndexByte(id, concertSep[0]); i >= 0 {
		return id[:i], id[i+1:], true
	}
	return id, "", false
}

// Bake renders the setlist into a bundle dir + .tstage under bakesDir and returns
// the manifest. Admin authorization is the CALLER's responsibility (the httpapi
// edge gates it, mirroring T08); `actor` scopes the data reads and is recorded as
// bakedBy.
//
// The bake is keyed by the setlist id alone (byte-identical to B02/B04); concert_rev
// bumps monotonically. (The per-member "?scope=mine" variant, B07, was retired — the
// band-wide bake is THE bake, P205.)
// layerDefaults is the P205 bake-dialog capture: layer NAME → default-on. nil means
// the dialog didn't run (legacy — LayerImage.DefaultOn stays absent so the viewer
// computes as today). When non-nil, every overlay gets an explicit DefaultOn
// (mandatory layers are forced on regardless). Keyed by name (the concert-level view
// the dialog shows: "Cues · Form · My notes").
func (b *Baker) Bake(ctx context.Context, bandID, setlistID string, actor app.User, layerDefaults map[string]bool, suppliedBakeID string) (bundle ConcertBundle, bakeID string, err error) {
	// T96 — publish "song N of M" progress as we go. The POST contract is unchanged; this is
	// a side channel keyed by a per-bake id (returned to the caller, exposed via GET …/progress).
	mint := b.newBakeID
	if mint == nil {
		mint = newBakeID
	}
	bakeID = mint()
	// T99/B: a caller may supply its OWN bake id, so it can poll progress from the instant it fires the
	// POST rather than only after this blocking bake returns (the response header — T96's original
	// mechanism — arrives too late to watch a running bake). Honour it only if it's a well-formed UUID
	// AND currently free; a malformed or in-flight-colliding id is ignored in favour of the server-minted
	// one — never an error (a progress hint must not fail a bake, nor clobber another bake's readout).
	if b.progress != nil && validBakeID(suppliedBakeID) && b.progress.claim(suppliedBakeID, bandID, setlistID) {
		bakeID = suppliedBakeID
	}
	var done, total int
	var curSong string
	// Terminal state on EVERY exit: an error, or a client disconnect that cancels ctx, must
	// never leave the bake stuck on "running" forever (the A39 stall). Fires on each return.
	defer func() {
		if err != nil {
			// T102: sanitise the escaping error ONCE, here — it's the single point both channels read
			// from (the returned err → POST body via writeErr; err.Error() → BakeProgress.Error). So a
			// failed bake never ships a stderr blob or a server path to a band member, on either path —
			// even from a failure point (assemble, packaging) that didn't compose its own message.
			err = b.humanize(err)
			b.publish(bakeID, bandID, setlistID, BakeProgress{State: BakeFailed, Done: done, Total: total, Song: curSong, Error: err.Error()})
			return
		}
		b.publish(bakeID, bandID, setlistID, BakeProgress{State: BakeSucceeded, Done: total, Total: total, Song: curSong})
	}()

	detail, err := b.svc.Setlist(actor, bandID, setlistID)
	if err != nil {
		return ConcertBundle{}, bakeID, err
	}
	total = len(detail.Items)
	// An immediate running snapshot so a poller that arrives before the first song sees
	// "running 0/N" rather than a 404. An empty setlist emits only this + the terminal
	// succeeded (0/0), never a "song 1 of 0".
	b.publish(bakeID, bandID, setlistID, BakeProgress{State: BakeRunning, Done: 0, Total: total})
	// The band-wide bake is THE bake (P205); the personal "?scope=mine" variant was
	// retired. ParseConcertID still reads old `${setlistID}~${userID}` variant concerts
	// for listing/download (read-compat), but no new ones are minted.
	concertID := setlistID
	name := detail.Setlist.Name
	concertDir := filepath.Join(b.bakesDir, concertID)
	if err := os.MkdirAll(concertDir, 0o700); err != nil {
		return ConcertBundle{}, bakeID, err
	}

	// Claim a rev ATOMICALLY (B04 finding 1): create the staging dir `<rev>.tmp`
	// with os.Mkdir (fails if it exists — unlike MkdirAll). On collision, bump the
	// number and retry; `nextRev` counts only PUBLISHED numeric dirs (not in-flight
	// `.tmp` claims), so a concurrent baker would otherwise re-pick the same number
	// forever — hence the local increment rather than a plain re-scan. No mutex.
	rev, err := b.nextRev(concertID)
	if err != nil {
		return ConcertBundle{}, bakeID, err
	}
	if b.afterNextRev != nil {
		b.afterNextRev()
	}
	var stageDir string
	for {
		revName := strconv.FormatUint(rev, 10)
		// A rev that is already PUBLISHED counts as taken too (B08): `nextRev` may have
		// run before a concurrent same-setlist bake published this number, so its `.tmp`
		// is gone and the Mkdir below would otherwise succeed on an already-used rev.
		if _, statErr := os.Stat(filepath.Join(concertDir, revName)); statErr == nil {
			rev++
			continue
		}
		stageDir = filepath.Join(concertDir, revName+".tmp")
		mkErr := os.Mkdir(stageDir, 0o700)
		if mkErr == nil {
			break
		}
		if !os.IsExist(mkErr) {
			return ConcertBundle{}, bakeID, mkErr
		}
		rev++ // another bake holds this rev's stage — try the next number
	}
	// Clean the staging dir if we don't reach a successful publish (rename clears it).
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stageDir)
		}
	}()

	blobsDir := filepath.Join(stageDir, "blobs")
	if err := os.MkdirAll(blobsDir, 0o700); err != nil {
		return ConcertBundle{}, bakeID, err
	}

	bundle = ConcertBundle{
		ConcertID:  concertID,
		Name:       name,
		ConcertRev: rev,
		BakedAt:    b.now(),
		BakedBy:    actor.DisplayName,
	}

	// T98 — two phases so the overlay worker spawns ONCE, not once per song. Phase 1: stage every
	// song (metadata + rasters) and collect the overlay requests for songs that have something to
	// draw. Phase 2: one RenderBatch for the whole bake. Phase 3: assemble each song with its
	// overlays. Kept SEQUENTIAL — the win is O(1) spawns, not parallelism (T97 §3.3), and this path
	// has a known race (TestBake_ConcurrentSameSetlist_distinctRevs) we must not agitate.
	staged := make([]stagedSong, 0, len(detail.Items))
	var batch []overlaySong
	for si, item := range detail.Items {
		// Publish BEFORE the song's work (poppler dominates a bake, T97/T98) so the readout
		// names the song being baked, not the one just finished: done advances 1..N here.
		done, curSong = si+1, item.SongTitle
		b.publish(bakeID, bandID, setlistID, BakeProgress{State: BakeRunning, Done: done, Total: total, Song: curSong})
		st, reqs, berr := b.stageSong(ctx, si, bandID, actor, item)
		if berr != nil {
			return ConcertBundle{}, bakeID, fmt.Errorf("song %s: %w", item.SongID, berr)
		}
		staged = append(staged, st)
		batch = append(batch, reqs...) // T137: one request per pool file that draws something
	}
	// T96/T98 tail: phase 1 is done (done==total), but the bake is NOT finished — the single
	// overlay RenderBatch (T98) + assembly + .tstage packaging still run (~2.4s of a ~13.5s bake).
	// Publish a running update that CLEARS the song, so a poller reads "finishing", not "still baking
	// song N". done==total must never imply finished; only the terminal state (below) says finished.
	curSong = ""
	b.publish(bakeID, bandID, setlistID, BakeProgress{State: BakeRunning, Done: total, Total: total})
	overlaysByKey, oerr := b.overlays.RenderBatch(ctx, batch)
	if oerr != nil {
		// VLL's exact failure (the overlay CLI missing): the raw error is a Node stack trace + absolute
		// server paths. Log that; tell the user the renderer is unavailable and whose problem it is.
		return ConcertBundle{}, bakeID, b.fail("The annotation renderer isn't available on the server. Ask an admin to check the bake setup.", oerr)
	}
	for _, st := range staged {
		song, aerr := b.assembleSong(st, overlaysByKey, blobsDir, layerDefaults)
		if aerr != nil {
			done, curSong = st.si+1, detail.Items[st.si].SongTitle // name the failing song in the terminal state
			return ConcertBundle{}, bakeID, fmt.Errorf("song %s: %w", st.song.SongID, aerr)
		}
		bundle.Songs = append(bundle.Songs, song)
	}

	// T124: derive the terminal state from the ARTEFACT, not from reaching the end of the code. A bake
	// that produced no songs — an empty setlist, or one whose every item dropped out — is nothing,
	// however cleanly the pipeline returned; publishing it as a concert is a false success. The studio
	// also guards the empty setlist at the button, but the honesty belongs here so every consumer of the
	// terminal record (studio, tests, anything later) inherits it. (A well-formed .tstage is still written
	// atomically below, B04/B09, so "wrote a zero-byte bundle" is not reachable on a clean return; the
	// reachable "produced nothing" is exactly this song-less case.)
	if len(bundle.Songs) == 0 {
		return ConcertBundle{}, bakeID, b.fail("This setlist has no songs to bake.", fmt.Errorf("bake produced no songs (%d setlist items)", total))
	}

	// P205: carry the band roster so the viewer resolves identity at view time
	// (logged-in Connect match ⇒ auto; anonymous ⇒ one-tap pick). Additive metadata;
	// a roster lookup failure must not fail the bake (older/shared bakes had none).
	if members, merr := b.svc.Members(actor, bandID); merr == nil {
		for _, m := range members {
			name := m.User.DisplayName
			if name == "" {
				name = m.User.Username
			}
			bundle.Roster = append(bundle.Roster, BundleMember{
				MemberID:    m.User.ID,
				DisplayName: name,
				Role:        string(m.Role),
			})
		}
	}

	// Publish (B04 atomic-rename + B08 re-claim + B09 two-phase .tstage).
	//
	// The `<rev>/` dir rename is the atomic arbiter: on a target-exists collision (a
	// same-setlist bake published this rev while we were baking, B08) we re-claim a
	// HIGHER rev and retry rather than failing, so concurrent bakes always land DISTINCT
	// published revs.
	//
	// The .tstage is published in TWO phases (B09): we write it under a name UNIQUE to
	// our exclusive `stageDir` (`<rev>.tmp` — only we hold it), win the dir rename, and
	// only THEN rename our staged .tstage onto the shared `<rev>.tstage`. So ONLY the
	// bake that wins `<rev>/` ever writes `<rev>.tstage` — a concurrent bake that also
	// re-claimed this rev writes its own unique staged file and, on losing the dir race,
	// removes only THAT (never the winner's), so it can neither delete nor content-
	// mismatch the published `.tstage`. (This narrows B04's "tstage strictly before dir"
	// to a sub-ms window where `<rev>/` exists just before `<rev>.tstage` lands, on the
	// rare re-claim path only — an accepted trade per the B09 ruling.)
	tstageStage := stageDir + ".tstage" // unique: we exclusively hold stageDir
	for {
		bundle.ConcertRev = rev
		manifest, err := bundle.MarshalCanonical()
		if err != nil {
			return ConcertBundle{}, bakeID, err
		}
		if err := os.WriteFile(filepath.Join(stageDir, "bundle.json"), manifest, 0o600); err != nil {
			return ConcertBundle{}, bakeID, err
		}
		if err := WriteTstage(tstageStage, stageDir, time.Unix(bundle.BakedAt, 0)); err != nil {
			return ConcertBundle{}, bakeID, err
		}
		revName := strconv.FormatUint(rev, 10)
		if err := os.Rename(stageDir, filepath.Join(concertDir, revName)); err == nil {
			// Won `<rev>/`. We are its sole owner, so we alone place `<rev>.tstage`.
			if err := os.Rename(tstageStage, filepath.Join(concertDir, revName+".tstage")); err != nil {
				return ConcertBundle{}, bakeID, err
			}
			published = true
			return bundle, bakeID, nil
		} else if _, statErr := os.Stat(filepath.Join(concertDir, revName)); statErr != nil {
			// Rename failed for a reason OTHER than the target already existing.
			_ = os.Remove(tstageStage)
			return ConcertBundle{}, bakeID, err
		}
		// `<rev>` was published by a concurrent bake — drop OUR uniquely-named staged
		// .tstage (never the shared published one) and re-claim the next free rev.
		_ = os.Remove(tstageStage)
		for {
			rev++
			if _, statErr := os.Stat(filepath.Join(concertDir, strconv.FormatUint(rev, 10))); statErr != nil {
				break // free (not yet published) — the rename below is the atomic claim
			}
		}
	}
}

// bakeSong bakes one setlist item: pick its default shared-pool PDF, rasterize its
// pages, render per-layer overlays, and assemble the BakedSong (overrides ride as
// metadata). A song with no viewable PDF bakes to zero pages (loaders tolerate it).
// effectiveTempo is the setlist tempo override when set (>0), else the song's base
// tempo (T86). 0 has always meant "no override", so override semantics are preserved.
func effectiveTempo(item app.SetlistItemView) int {
	if item.TempoOverride > 0 {
		return item.TempoOverride
	}
	return item.SongTempo
}

// effectiveKey is the setlist key override when set, else the song's base key (T86).
func effectiveKey(item app.SetlistItemView) string {
	if item.KeyOverride != "" {
		return item.KeyOverride
	}
	return item.SongKey
}

// stagedFile is one source file's rasters + overlay wiring within a song's shared page POOL (T137). A
// song with no divergent member selections has exactly one — the default — and bakes byte-identically to
// before (the pool IS that file, no member_pages).
type stagedFile struct {
	fileID       string
	rasters      [][]byte
	nameByLayer  map[string]string
	ownerByLayer map[string]string
	overlayKey   string // "" if this file has nothing to draw
}

// stagedSong is a song after phase 1 (metadata + rasters), waiting for its overlays from the one
// batch render before phase 3 assembles the blobs (T98).
type stagedSong struct {
	si   int
	song BakedSong
	// T137: the pool sources in a stable order; files[0] is the default (the "" / anonymous reader's
	// sequence). One entry ⇒ today's single-file bundle, byte-identical.
	files         []stagedFile
	selections    []app.MemberFileSelection // per-member ordered file choices → member_pages (⟨D1⟩)
	defaultFileID string                    // files[0].fileID, or "" if metadata-only / no default PDF
	metadataOnly  bool
}

// stageSong does everything up to the overlay render: metadata, snapshot, member cues, and — per file in
// the T137 POOL — resolve/transpose/rasterize + build the overlay doc. It returns the staged state plus
// one overlay request per file that has something to draw (T97's skip generalised: a file with nothing to
// draw never enters the batch, so it costs no spawn). One pool file ⇒ today's single-file song.
func (b *Baker) stageSong(ctx context.Context, si int, bandID string, actor app.User, item app.SetlistItemView) (stagedSong, []overlaySong, error) {
	song := BakedSong{
		SongID:       item.SongID,
		SongRev:      1,
		DisplayNotes: item.Notes,
		// T86: bake the EFFECTIVE key/tempo — the setlist override when set, else the
		// song's own value. Baking the override alone left tempo=0 (and key="") for every
		// song without a per-setlist override, so the Stage beat never appeared. 0/"" have
		// always meant "no override", so override semantics are unchanged.
		Key:    effectiveKey(item),
		Tempo:  int32(effectiveTempo(item)),
		OnCall: item.OnCall,     // bench membership rides into the bundle (T23)
		Title:  item.SongTitle,  // real title snapshot → kills the "Song N" fallback (T26)
		Artist: item.SongArtist, // P207: artist snapshot at bake time; empty is normal (no artist)
		Meter:  item.SongMeter,  // T86: the song's metre → the Stage beat (A35)
	}
	// source_revision = the song's CURRENT head annotation revision. (There is no
	// per-setlist-item revision pin today — SetlistItem carries no revision, and
	// domain.Pin is a store-level GC construct not wired to setlists. If setlist
	// pinning is added later, prefer the pin here.)
	snap, err := b.eng.Head(item.SongID)
	if err != nil {
		return stagedSong{}, nil, err
	}
	song.SourceRevision = snap.Revision

	// P205: the band-wide bake carries EVERY member's cues as `member_cues` (field 11),
	// keyed by member id; the viewer shows only its own identity's entry (Stage 3). The
	// personal-variant field 10 (`cues`) is retired with the scope=mine bake — it stays
	// empty so an old app degrades to no-cues, never wrong-cues (the ruled compat guard).
	mcs, mcerr := b.svc.AllMemberCues(actor, bandID, item.SongID)
	if mcerr != nil {
		return stagedSong{}, nil, mcerr
	}
	for _, mc := range mcs {
		cues := make([]SongCue, 0, len(mc.Cues))
		for _, c := range mc.Cues {
			cues = append(cues, SongCue{Icon: c.Icon, Color: c.Color})
		}
		song.MemberCues = append(song.MemberCues, MemberCues{MemberID: mc.MemberID, Cues: cues})
	}

	// T137: resolve the pool SOURCES. files[0] is the DEFAULT — the lowest-DisplayOrder viewable PDF, or
	// (⟨D3⟩: default-only) the generated chart when the item transposes (D1) — followed by the DISTINCT
	// files any member selected, in Members order. Everything resolves through SongFiles so a stale
	// selection ref (a deleted file) is skipped, never a bake failure.
	allFiles, err := b.svc.SongFiles(actor, bandID, item.SongID)
	if err != nil {
		return stagedSong{}, nil, err
	}
	byID := make(map[string]app.SongFile, len(allFiles))
	for _, f := range allFiles {
		byID[f.ID] = f
	}
	def, hasDefault, err := b.defaultFile(actor, bandID, item.SongID)
	if err != nil {
		return stagedSong{}, nil, err
	}
	if hasDefault && item.TransposeChords {
		if gen, genOK, gerr := b.generatedChart(actor, bandID, item.SongID); gerr != nil {
			return stagedSong{}, nil, gerr
		} else if genOK {
			def = gen // D1: transpose forces the DEFAULT to the generated chart (the previewed file)
		}
	}
	sels, serr := b.svc.AllFileSelections(actor, bandID, item.SongID)
	if serr != nil {
		return stagedSong{}, nil, serr
	}

	union := make([]app.SongFile, 0, 1+len(sels))
	seen := map[string]bool{}
	if hasDefault {
		union = append(union, def)
		seen[def.ID] = true
	}
	for _, sel := range sels {
		for _, fid := range sel.FileIDs {
			if seen[fid] {
				continue
			}
			if f, exists := byID[fid]; exists { // skip a stale/deleted selection ref
				union = append(union, f)
				seen[fid] = true
			}
		}
	}
	if len(union) == 0 {
		return stagedSong{si: si, song: song, metadataOnly: true}, nil, nil // no PDF anywhere
	}

	st := stagedSong{si: si, song: song, selections: sels}
	if hasDefault {
		st.defaultFileID = def.ID
	}
	var reqs []overlaySong
	for fi, f := range union {
		sf, req, ferr := b.stageFile(ctx, si, fi, bandID, actor, item, snap, f)
		if ferr != nil {
			return stagedSong{}, nil, ferr
		}
		st.files = append(st.files, sf)
		if req != nil {
			reqs = append(reqs, *req)
		}
	}
	return st, reqs, nil
}

// stageFile stages ONE pool source: download, transpose (⟨D3⟩ — only when the item asks AND this file is
// itself eligible; a member's explicit non-chart pick is baked as selected, never silently overridden),
// rasterize, and build its overlay doc scoped to this file. [fi] is the file's position in the pool.
func (b *Baker) stageFile(ctx context.Context, si, fi int, bandID string, actor app.User, item app.SetlistItemView, snap domain.Snapshot, file app.SongFile) (stagedFile, *overlaySong, error) {
	_, pdf, err := b.svc.DownloadSongFile(actor, file.ID)
	if err != nil {
		return stagedFile{}, nil, err
	}
	// T60 surface 2: burn the chart transposed to the item's key override when the item asks AND this file
	// is eligible. A degraded case falls through to the stored PDF — the bake must NOT fail (a failed bake
	// the night before a gig, or a silent wrong-key page, are both worse; bakeapi surfaces a warning from
	// the same TransposeEligible check, which ⟨D3⟩ extends to name a member whose non-eligible pick reads
	// in the original key). Transpose preserves line count ⇒ identical geometry ⇒ layers stay anchored.
	if item.TransposeChords {
		if s, serr := b.svc.SongForMember(actor, bandID, item.SongID); serr == nil {
			if ok, _ := app.TransposeEligible(s.Key, item.KeyOverride, file.Generated); ok {
				if _, src, cerr := b.svc.ChartSource(actor, bandID, item.SongID, file.ID); cerr == nil {
					from, _ := chartpdf.ParseKey(s.Key)
					to, _ := chartpdf.ParseKey(item.KeyOverride)
					if t, terr := chartpdf.TransposeToKey(src, item.KeyOverride, from, to); terr == nil { // D5
						if tpdf, rerr := chartpdf.Render(t); rerr == nil {
							pdf = tpdf
						}
					}
				}
			}
		}
	}

	rasters, err := b.raster.Rasterize(ctx, pdf)
	if err != nil {
		// poppler's stderr + its binary path; the user gets a one-line, song-named reason instead.
		return stagedFile{}, nil, b.fail(fmt.Sprintf("Couldn't read the sheet music for %q — the file may be damaged.", item.SongTitle), err)
	}
	pageSizes := make([]pageSize, 0, len(rasters))
	for i, r := range rasters {
		cfg, _, derr := image.DecodeConfig(bytes.NewReader(r))
		if derr != nil {
			return stagedFile{}, nil, fmt.Errorf("decode page %d raster: %w", i, derr)
		}
		pageSizes = append(pageSizes, pageSize{Index: i, Width: cfg.Width, Height: cfg.Height})
	}
	// Overlays are drawn at the raster's pixel width so they composite 1:1 on the presenter.
	overlayWidth := 0
	if len(pageSizes) > 0 {
		overlayWidth = pageSizes[0].Width
	}
	// T53: the rendered overlay carries no name; look it up from the source snapshot's layers so the baked
	// LayerImage can label the layer. P205: also carry each layer's owner (member id, or "" for shared).
	nameByLayer := map[string]string{}
	ownerByLayer := map[string]string{}
	for _, l := range snap.Layers {
		nameByLayer[l.ID] = l.Name
		if l.OwnerID != domain.SharedOwner { // "" = band/shared; a member id = personal
			ownerByLayer[l.ID] = l.OwnerID
		}
	}
	sf := stagedFile{fileID: file.ID, rasters: rasters, nameByLayer: nameByLayer, ownerByLayer: ownerByLayer}
	// T97/T98: a file enters the overlay batch only if it has objects to draw (scoped to THIS file — a
	// multi-file song carries per-file layers, B11/T40). Zero objects → no spawn contribution.
	doc := snapshotToDoc(snap, file.ID)
	if len(doc.Objects) == 0 {
		return sf, nil, nil
	}
	sf.overlayKey = fmt.Sprintf("s%d-f%d", si, fi)
	return sf, &overlaySong{Key: sf.overlayKey, Doc: doc, Pages: pageSizes, OverlayWidth: overlayWidth}, nil
}

// assembleSong writes the pool's rasters + batched overlays into the bundle blobs and returns the
// completed BakedSong. `overlaysByKey` is the whole RenderBatch result, keyed per pool file (T137).
func (b *Baker) assembleSong(st stagedSong, overlaysByKey map[string][]renderedOverlay, blobsDir string, layerDefaults map[string]bool) (BakedSong, error) {
	if st.metadataOnly {
		return st.song, nil
	}
	song := st.song
	// T137: the shared POOL — one PageImages entry per (pool file, page), each with its OWN overlays. ⟨D2⟩:
	// the raster BLOB is written once per content hash and its ref reused, so identical pages cost one
	// image; but the entries stay distinct, so two byte-identical files with DIFFERENT annotations (a part
	// duplicated "pour flûte") never merge their marks. `pagesByFile` records each file's entry indices so
	// per-member sequences (member_pages) can point into the pool.
	rasterRefByHash := map[string]string{}
	pagesByFile := map[string][]int32{}
	for _, sf := range st.files {
		overlaysByPage := map[int][]renderedOverlay{}
		for _, ov := range overlaysByKey[sf.overlayKey] {
			overlaysByPage[ov.Page] = append(overlaysByPage[ov.Page], ov)
		}
		// T145: an overlay whose page fell off the end of the (reflowed) render would be silently dropped
		// below — the loop only reads overlaysByPage[i] for pages that exist in the raster set. That is the
		// "one overlay vanished from the bundle" failure. Fail the BAKE, not the rehearsal: a mark on a page
		// the chart no longer has is a reflow orphan that must be re-anchored (T145), never shipped blank.
		for pg := range overlaysByPage {
			if pg >= len(sf.rasters) {
				return BakedSong{}, fmt.Errorf("bake %q: an annotation is on page %d but the chart rendered only %d page(s) — a reflow orphaned this overlay (T145); re-anchor the mark or re-check the chart before baking", song.Title, pg+1, len(sf.rasters))
			}
		}
		seq := make([]int32, 0, len(sf.rasters))
		for i, r := range sf.rasters {
			entryIdx := len(song.Pages)
			hash := Sha256Hex(r)
			rasterRef, ok := rasterRefByHash[hash]
			if !ok { // first time we see this raster — write the blob once and remember its ref (⟨D2⟩)
				rasterRef = fmt.Sprintf("blobs/s%d-p%d-raster.png", st.si, entryIdx)
				if err := os.WriteFile(filepath.Join(blobsDir, "..", filepath.FromSlash(rasterRef)), r, 0o600); err != nil {
					return BakedSong{}, err
				}
				rasterRefByHash[hash] = rasterRef
			}
			page := PageImages{PageRasterRef: rasterRef, RasterHash: hash}
			ovs := overlaysByPage[i]
			sort.Slice(ovs, func(a, b int) bool { return ovs[a].Order < ovs[b].Order })
			for _, ov := range ovs {
				ref := fmt.Sprintf("blobs/s%d-p%d-%s.png", st.si, entryIdx, safeName(ov.LayerID))
				if err := os.WriteFile(filepath.Join(blobsDir, "..", filepath.FromSlash(ref)), ov.PNG, 0o600); err != nil {
					return BakedSong{}, err
				}
				li := LayerImage{
					LayerID:     ov.LayerID,
					ImageRef:    ref,
					ContentHash: ov.ContentHash,
					Order:       ov.Order,
					Mandatory:   ov.Mandatory,
					RoleTag:     ov.RoleTag,
					Name:        sf.nameByLayer[ov.LayerID],
					Owner:       sf.ownerByLayer[ov.LayerID], // P205: "" = shared; member id = personal
				}
				// P205 default_on capture (bake dialog): when the dialog ran, stamp an explicit default-on
				// per layer (mandatory always on); otherwise leave it absent so the viewer computes as today.
				if layerDefaults != nil {
					on := ov.Mandatory || layerDefaults[sf.nameByLayer[ov.LayerID]]
					li.DefaultOn = &on
				}
				page.Overlays = append(page.Overlays, li)
			}
			song.Pages = append(song.Pages, page)
			seq = append(seq, int32(entryIdx))
		}
		pagesByFile[sf.fileID] = seq
	}
	// ⟨D1⟩: emit member_pages ONLY when the pool holds more than one source (members diverge) — then the ""
	// DEFAULT sequence (the default file's pool pages: the no-selection / anonymous reader) TOGETHER with a
	// per-member entry for each divergent member. One pool file ⇒ no member_pages, byte-identical to today.
	if len(st.files) > 1 {
		if def := pagesByFile[st.defaultFileID]; len(def) > 0 {
			song.MemberPages = append(song.MemberPages, MemberPages{MemberID: "", Page: def})
		}
		for _, sel := range st.selections {
			seq := make([]int32, 0, len(sel.FileIDs))
			for _, fid := range sel.FileIDs {
				seq = append(seq, pagesByFile[fid]...)
			}
			if len(seq) > 0 {
				song.MemberPages = append(song.MemberPages, MemberPages{MemberID: sel.MemberID, Page: seq})
			}
		}
	}
	return song, nil
}

// defaultFile is the shared-pool single-file choice for a member who has chosen nothing. T138 ⟨R1⟩: it is
// now the ONE shared rule (app.DefaultFile) — the lowest-DisplayOrder PDF, tie-broken by filename — the
// same definition Studio's my-files default uses, pinned by docs/contracts/default-file.vectors.json.
func (b *Baker) defaultFile(actor app.User, bandID, songID string) (app.SongFile, bool, error) {
	files, err := b.svc.SongFiles(actor, bandID, songID)
	if err != nil {
		return app.SongFile{}, false, err
	}
	f, ok := app.DefaultFile(files)
	return f, ok, nil
}

// generatedChart returns the song's generated text-chart (lowest DisplayOrder) if it has
// one — the file transpose + the playlist preview operate on. Baking THIS when an item asks
// to transpose (D1) keeps the gig bundle, the preview, the bake-warning, and the Studio
// checkbox all pointing at the same document; "a generated chart exists" is the single
// eligibility predicate the other three sites already use.
func (b *Baker) generatedChart(actor app.User, bandID, songID string) (app.SongFile, bool, error) {
	files, err := b.svc.SongFiles(actor, bandID, songID)
	if err != nil {
		return app.SongFile{}, false, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].DisplayOrder < files[j].DisplayOrder })
	for _, f := range files {
		if f.Generated {
			return f, true, nil
		}
	}
	return app.SongFile{}, false, nil
}

// nextRev returns 1 + the highest existing numeric rev dir for a concert (1 if none).
func (b *Baker) nextRev(concertID string) (uint64, error) {
	dir := filepath.Join(b.bakesDir, concertID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	var max uint64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Staging dirs are named `<rev>.tmp`; ParseUint rejects them, so in-flight
		// bakes never count as a published rev here (B04). Both scanners rely on this.
		if n, perr := strconv.ParseUint(e.Name(), 10, 64); perr == nil && n > max {
			max = n
		}
	}
	return max + 1, nil
}

// latestRev returns the highest numeric rev dir for a concert (0, false if none).
func (b *Baker) latestRev(concertID string) (uint64, bool) {
	entries, err := os.ReadDir(filepath.Join(b.bakesDir, concertID))
	if err != nil {
		return 0, false
	}
	var max uint64
	var found bool
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, perr := strconv.ParseUint(e.Name(), 10, 64); perr == nil {
			found = true
			if n > max {
				max = n
			}
		}
	}
	return max, found
}

// ListConcerts returns the latest baked manifest of every concert under bakesDir
// (the input to B03's "what's available" list). Missing/partial bakes are skipped,
// not errored — listing must never fail the whole request.
func (b *Baker) ListConcerts() []ConcertBundle {
	out := []ConcertBundle{}
	entries, err := os.ReadDir(b.bakesDir)
	if err != nil {
		return out // no bakes yet
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		concertID := e.Name()
		rev, ok := b.latestRev(concertID)
		if !ok {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10), "bundle.json"))
		if rerr != nil {
			continue
		}
		var cb ConcertBundle
		if json.Unmarshal(data, &cb) != nil {
			continue
		}
		out = append(out, cb)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConcertID < out[j].ConcertID })
	return out
}

// BundlePath returns the filesystem path of a concert's latest .tstage, or "" if
// no bake exists. The httpapi edge scopes the concert to the caller's band before
// calling this (concertID == setlistID).
func (b *Baker) BundlePath(concertID string) string {
	rev, ok := b.latestRev(concertID)
	if !ok {
		return ""
	}
	return filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10)+".tstage")
}

// safeName keeps a layer id usable as a blob filename.
func safeName(id string) string {
	out := make([]rune, 0, len(id))
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "layer"
	}
	return string(out)
}
