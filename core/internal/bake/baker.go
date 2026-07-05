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
	"time"

	"troubastack/core/internal/app"
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

// Bake renders the setlist into a bundle dir + .tstage under bakesDir and returns
// the manifest. Admin authorization is the CALLER's responsibility (the httpapi
// edge gates it, mirroring T08); `actor` scopes the data reads and is recorded as
// bakedBy. concertId is the setlist id (stable per setlist); concert_rev bumps
// monotonically per setlist across bakes.
func (b *Baker) Bake(ctx context.Context, bandID, setlistID string, actor app.User) (ConcertBundle, error) {
	detail, err := b.svc.Setlist(actor, bandID, setlistID)
	if err != nil {
		return ConcertBundle{}, err
	}
	concertID := setlistID
	rev, err := b.nextRev(concertID)
	if err != nil {
		return ConcertBundle{}, err
	}

	revDir := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10))
	blobsDir := filepath.Join(revDir, "blobs")
	if err := os.MkdirAll(blobsDir, 0o755); err != nil {
		return ConcertBundle{}, err
	}

	bundle := ConcertBundle{
		ConcertID:  concertID,
		Name:       detail.Setlist.Name,
		ConcertRev: rev,
		BakedAt:    b.now(),
		BakedBy:    actor.DisplayName,
	}

	for si, item := range detail.Items {
		song, berr := b.bakeSong(ctx, si, bandID, actor, item, blobsDir)
		if berr != nil {
			return ConcertBundle{}, fmt.Errorf("song %s: %w", item.SongID, berr)
		}
		bundle.Songs = append(bundle.Songs, song)
	}

	manifest, err := bundle.MarshalCanonical()
	if err != nil {
		return ConcertBundle{}, err
	}
	if err := os.WriteFile(filepath.Join(revDir, "bundle.json"), manifest, 0o644); err != nil {
		return ConcertBundle{}, err
	}
	// Zip to a sibling .tstage (outside revDir, so it isn't zipped into itself).
	tstage := filepath.Join(b.bakesDir, concertID, strconv.FormatUint(rev, 10)+".tstage")
	if err := WriteTstage(tstage, revDir, time.Unix(bundle.BakedAt, 0)); err != nil {
		return ConcertBundle{}, err
	}
	return bundle, nil
}

// bakeSong bakes one setlist item: pick its default shared-pool PDF, rasterize its
// pages, render per-layer overlays, and assemble the BakedSong (overrides ride as
// metadata). A song with no viewable PDF bakes to zero pages (loaders tolerate it).
func (b *Baker) bakeSong(ctx context.Context, si int, bandID string, actor app.User, item app.SetlistItemView, blobsDir string) (BakedSong, error) {
	song := BakedSong{
		SongID:       item.SongID,
		SongRev:      1,
		DisplayNotes: item.Notes,
		Key:          item.KeyOverride,
		Tempo:        int32(item.TempoOverride),
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

	file, ok, err := b.defaultFile(actor, bandID, item.SongID)
	if err != nil {
		return BakedSong{}, err
	}
	if !ok {
		return song, nil // no PDF to bake — metadata-only song
	}
	_, pdf, err := b.svc.DownloadSongFile(actor, file.ID)
	if err != nil {
		return BakedSong{}, err
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
	rendered, err := b.overlays.Render(ctx, cliRequest{
		Doc:          snapshotToDoc(snap),
		Pages:        pageSizes,
		OverlayWidth: overlayWidth,
	})
	if err != nil {
		return BakedSong{}, err
	}
	overlaysByPage := map[int][]renderedOverlay{}
	for _, ov := range rendered {
		overlaysByPage[ov.Page] = append(overlaysByPage[ov.Page], ov)
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
			page.Overlays = append(page.Overlays, LayerImage{
				LayerID:     ov.LayerID,
				ImageRef:    ref,
				ContentHash: ov.ContentHash,
				Order:       ov.Order,
				Mandatory:   ov.Mandatory,
				RoleTag:     ov.RoleTag,
			})
		}
		song.Pages = append(song.Pages, page)
	}
	return song, nil
}

// defaultFile is v1's single-file choice: the song's shared-pool file with the
// lowest DisplayOrder that is a viewable PDF — the same one Studio opens by
// default. (Per-member my-files bakes are a later product decision.)
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
