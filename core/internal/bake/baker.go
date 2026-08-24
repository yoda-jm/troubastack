package bake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png" // register PNG decoder for image.DecodeConfig
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
}

// Baker turns a setlist into a downloadable .tstage (I11): resolve songs →
// rasterize the default file's pages (poppler) → render per-layer overlays via
// web/bake (I8) → assemble a ConcertBundle dir + zip. It NEVER renders strokes.
type Baker struct {
	svc      *app.Service
	eng      *engine.Engine
	raster   Rasterizer
	overlays OverlayRenderer
	bakesDir string
	now      func() int64 // injectable clock (unix seconds) for deterministic tests
	// afterNextRev, if set, is called once right after nextRev returns — a TEST SEAM
	// (nil in production) letting the B08 test deterministically publish a concurrent
	// rev in the window between our nextRev and our claim/publish.
	afterNextRev func()
}

// New builds a Baker with the real poppler + web/bake shell-out steps. A missing
// binary is not detected here — it surfaces as a clear per-bake error (I: never
// crash the server), so the server still starts without the toolchain installed.
func New(svc *app.Service, eng *engine.Engine, cfg Config) *Baker {
	dpi := cfg.DPI
	if dpi == 0 {
		dpi = 150
	}
	return &Baker{
		svc:      svc,
		eng:      eng,
		raster:   popplerRasterizer{bin: cfg.Pdftoppm, dpi: dpi},
		overlays: nodeOverlayRenderer{node: cfg.Node, cli: cfg.BakeCLI},
		bakesDir: cfg.BakesDir,
		now:      func() int64 { return time.Now().Unix() },
	}
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
func (b *Baker) Bake(ctx context.Context, bandID, setlistID string, actor app.User, layerDefaults map[string]bool) (ConcertBundle, error) {
	detail, err := b.svc.Setlist(actor, bandID, setlistID)
	if err != nil {
		return ConcertBundle{}, err
	}
	// The band-wide bake is THE bake (P205); the personal "?scope=mine" variant was
	// retired. ParseConcertID still reads old `${setlistID}~${userID}` variant concerts
	// for listing/download (read-compat), but no new ones are minted.
	concertID := setlistID
	name := detail.Setlist.Name
	concertDir := filepath.Join(b.bakesDir, concertID)
	if err := os.MkdirAll(concertDir, 0o755); err != nil {
		return ConcertBundle{}, err
	}

	// Claim a rev ATOMICALLY (B04 finding 1): create the staging dir `<rev>.tmp`
	// with os.Mkdir (fails if it exists — unlike MkdirAll). On collision, bump the
	// number and retry; `nextRev` counts only PUBLISHED numeric dirs (not in-flight
	// `.tmp` claims), so a concurrent baker would otherwise re-pick the same number
	// forever — hence the local increment rather than a plain re-scan. No mutex.
	rev, err := b.nextRev(concertID)
	if err != nil {
		return ConcertBundle{}, err
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
		mkErr := os.Mkdir(stageDir, 0o755)
		if mkErr == nil {
			break
		}
		if !os.IsExist(mkErr) {
			return ConcertBundle{}, mkErr
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
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		return ConcertBundle{}, err
	}

	bundle := ConcertBundle{
		ConcertID:  concertID,
		Name:       name,
		ConcertRev: rev,
		BakedAt:    b.now(),
		BakedBy:    actor.DisplayName,
	}

	for si, item := range detail.Items {
		song, berr := b.bakeSong(ctx, si, bandID, actor, item, blobsDir, layerDefaults)
		if berr != nil {
			return ConcertBundle{}, fmt.Errorf("song %s: %w", item.SongID, berr)
		}
		bundle.Songs = append(bundle.Songs, song)
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
			return ConcertBundle{}, err
		}
		if err := os.WriteFile(filepath.Join(stageDir, "bundle.json"), manifest, 0o644); err != nil {
			return ConcertBundle{}, err
		}
		if err := WriteTstage(tstageStage, stageDir, time.Unix(bundle.BakedAt, 0)); err != nil {
			return ConcertBundle{}, err
		}
		revName := strconv.FormatUint(rev, 10)
		if err := os.Rename(stageDir, filepath.Join(concertDir, revName)); err == nil {
			// Won `<rev>/`. We are its sole owner, so we alone place `<rev>.tstage`.
			if err := os.Rename(tstageStage, filepath.Join(concertDir, revName+".tstage")); err != nil {
				return ConcertBundle{}, err
			}
			published = true
			return bundle, nil
		} else if _, statErr := os.Stat(filepath.Join(concertDir, revName)); statErr != nil {
			// Rename failed for a reason OTHER than the target already existing.
			_ = os.Remove(tstageStage)
			return ConcertBundle{}, err
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

func (b *Baker) bakeSong(ctx context.Context, si int, bandID string, actor app.User, item app.SetlistItemView, blobsDir string, layerDefaults map[string]bool) (BakedSong, error) {
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
		OnCall: item.OnCall,    // bench membership rides into the bundle (T23)
		Title:  item.SongTitle, // real title snapshot → kills the "Song N" fallback (T26)
		Meter:  item.SongMeter, // T86: the song's metre → the Stage beat (A35)
	}
	// source_revision = the song's CURRENT head annotation revision. (There is no
	// per-setlist-item revision pin today — SetlistItem carries no revision, and
	// domain.Pin is a store-level GC construct not wired to setlists. If setlist
	// pinning is added later, prefer the pin here.)
	snap, err := b.eng.Head(item.SongID)
	if err != nil {
		return BakedSong{}, err
	}
	song.SourceRevision = snap.Revision

	// P205: the band-wide bake carries EVERY member's cues as `member_cues` (field 11),
	// keyed by member id; the viewer shows only its own identity's entry (Stage 3). The
	// personal-variant field 10 (`cues`) is retired with the scope=mine bake — it stays
	// empty so an old app degrades to no-cues, never wrong-cues (the ruled compat guard).
	mcs, mcerr := b.svc.AllMemberCues(actor, bandID, item.SongID)
	if mcerr != nil {
		return BakedSong{}, mcerr
	}
	for _, mc := range mcs {
		cues := make([]SongCue, 0, len(mc.Cues))
		for _, c := range mc.Cues {
			cues = append(cues, SongCue{Icon: c.Icon, Color: c.Color})
		}
		song.MemberCues = append(song.MemberCues, MemberCues{MemberID: mc.MemberID, Cues: cues})
	}

	file, ok, err := b.defaultFile(actor, bandID, item.SongID)
	if err != nil {
		return BakedSong{}, err
	}
	if !ok {
		return song, nil // no PDF to bake — metadata-only song
	}
	// D1: when the item asks to transpose, bake the GENERATED chart — the exact file the
	// playlist preview shows and the bake-warning / Studio checkbox key on. Otherwise, with
	// an uploaded PDF at a lower DisplayOrder, the baker would bake that untransposed PDF
	// while the band previewed the transposed chart — a silently wrong gig bundle.
	if item.TransposeChords {
		if gen, genOK, gerr := b.generatedChart(actor, bandID, item.SongID); gerr != nil {
			return BakedSong{}, gerr
		} else if genOK {
			file = gen
		}
	}
	_, pdf, err := b.svc.DownloadSongFile(actor, file.ID)
	if err != nil {
		return BakedSong{}, err
	}

	// T60 surface 2: burn the chart transposed to the item's key override, band-wide,
	// when the item asks AND all conditions hold at bake time. A degraded case (key
	// edited to garbage, chart replaced) falls through to the stored PDF — the bake must
	// NOT fail (a failed bake the night before a gig, or a silent wrong-key page, are
	// both worse; bakeapi surfaces a warning from the same TransposeEligible check). The
	// transpose preserves line count ⇒ identical pagination/geometry ⇒ existing layer
	// annotations stay anchored (chartpdf Part A invariant). Any sub-step failing leaves
	// `pdf` as the stored bytes.
	if item.TransposeChords {
		if song, serr := b.svc.SongForMember(actor, bandID, item.SongID); serr == nil {
			if ok, _ := app.TransposeEligible(song.Key, item.KeyOverride, file.Generated); ok {
				if _, src, cerr := b.svc.ChartSource(actor, bandID, item.SongID, file.ID); cerr == nil {
					from, _ := chartpdf.ParseKey(song.Key)
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
		return BakedSong{}, err
	}
	pageSizes := make([]pageSize, 0, len(rasters))
	for i, r := range rasters {
		cfg, _, derr := image.DecodeConfig(bytes.NewReader(r))
		if derr != nil {
			return BakedSong{}, fmt.Errorf("decode page %d raster: %w", i, derr)
		}
		pageSizes = append(pageSizes, pageSize{Index: i, Width: cfg.Width, Height: cfg.Height})
	}
	// Overlays are drawn at the raster's pixel width so they composite 1:1 on the
	// presenter. v1 assumes uniform page width (typical for a single PDF).
	overlayWidth := 0
	if len(pageSizes) > 0 {
		overlayWidth = pageSizes[0].Width
	}
	// T97: the overlay worker (node + @napi-rs/canvas) costs ~0.6s of startup per spawn, and it was
	// spawned for EVERY song regardless of content — a concert with no annotations paid N × that to
	// draw nothing. Skip the spawn entirely when this file has no objects to draw. Byte-identical:
	// zero objects → the worker would have produced zero overlays anyway.
	doc := snapshotToDoc(snap, file.ID)
	var rendered []renderedOverlay
	if len(doc.Objects) > 0 {
		rendered, err = b.overlays.Render(ctx, cliRequest{
			Doc:          doc,
			Pages:        pageSizes,
			OverlayWidth: overlayWidth,
		})
		if err != nil {
			return BakedSong{}, err
		}
	}
	overlaysByPage := map[int][]renderedOverlay{}
	for _, ov := range rendered {
		overlaysByPage[ov.Page] = append(overlaysByPage[ov.Page], ov)
	}

	// T53: the rendered overlay carries no name; look it up from the source snapshot's
	// layers so the baked LayerImage can label the layer for the viewer.
	// P205: also carry each layer's owner (member id, or "" for band-shared) so a
	// band-wide bundle can be filtered to the viewer's identity at view time.
	nameByLayer := map[string]string{}
	ownerByLayer := map[string]string{}
	for _, l := range snap.Layers {
		nameByLayer[l.ID] = l.Name
		if l.OwnerID != domain.SharedOwner { // "" = band/shared; a member id = personal
			ownerByLayer[l.ID] = l.OwnerID
		}
	}

	for i, r := range rasters {
		rasterRef := fmt.Sprintf("blobs/s%d-p%d-raster.png", si, i)
		if err := os.WriteFile(filepath.Join(blobsDir, "..", filepath.FromSlash(rasterRef)), r, 0o644); err != nil {
			return BakedSong{}, err
		}
		page := PageImages{PageRasterRef: rasterRef, RasterHash: Sha256Hex(r)}
		ovs := overlaysByPage[i]
		sort.Slice(ovs, func(a, b int) bool { return ovs[a].Order < ovs[b].Order })
		for _, ov := range ovs {
			ref := fmt.Sprintf("blobs/s%d-p%d-%s.png", si, i, safeName(ov.LayerID))
			if err := os.WriteFile(filepath.Join(blobsDir, "..", filepath.FromSlash(ref)), ov.PNG, 0o644); err != nil {
				return BakedSong{}, err
			}
			li := LayerImage{
				LayerID:     ov.LayerID,
				ImageRef:    ref,
				ContentHash: ov.ContentHash,
				Order:       ov.Order,
				Mandatory:   ov.Mandatory,
				RoleTag:     ov.RoleTag,
				Name:        nameByLayer[ov.LayerID],
				Owner:       ownerByLayer[ov.LayerID], // P205: "" = shared; member id = personal
			}
			// P205 default_on capture (bake dialog): when the dialog ran, stamp an
			// explicit default-on per layer (mandatory always on); otherwise leave it
			// absent so the viewer computes as today.
			if layerDefaults != nil {
				on := ov.Mandatory || layerDefaults[nameByLayer[ov.LayerID]]
				li.DefaultOn = &on
			}
			page.Overlays = append(page.Overlays, li)
		}
		song.Pages = append(song.Pages, page)
	}
	return song, nil
}

// defaultFile is the shared-pool single-file choice: the song's file with the
// lowest DisplayOrder that is a viewable PDF — the same one Studio opens by
// default. (The retired per-member bake once picked from the member's my-files view.)
func (b *Baker) defaultFile(actor app.User, bandID, songID string) (app.SongFile, bool, error) {
	files, err := b.svc.SongFiles(actor, bandID, songID)
	if err != nil {
		return app.SongFile{}, false, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].DisplayOrder < files[j].DisplayOrder })
	for _, f := range files {
		if f.ContentType == "application/pdf" {
			return f, true, nil
		}
	}
	return app.SongFile{}, false, nil
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
