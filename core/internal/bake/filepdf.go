package bake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"strings"

	"troubastack/core/internal/app"
	"troubastack/core/internal/domain"
)

// FilePDF prints ONE file of a song's pool (a lead sheet, a tab) as an A4 PDF with its annotations drawn on
// it (FILEPDF). It reuses the bake pipeline's staging — download, rasterize (render cache included) and the
// overlay doc scoped to this file — but makes NO concert: nothing is written under bakesDir, no rev is
// claimed, no progress is published. Each page is composed in memory with the same flattening as the
// concert print (composeLayers) and written by the same A4 writer (writeA4PDF).
//
// visible decides, per layer id, whether a rendered overlay is painted; the caller resolves it (see
// FileLayerVisibility). An unknown file — or a file of ANOTHER song — is app.ErrNotFound; a non-member gets
// the service's authorization error. Other failures reach the caller as short human text (bakeError).
func (b *Baker) FilePDF(ctx context.Context, bandID, songID, fileID string, actor app.User, visible func(layerID string) bool) (out []byte, err error) {
	defer func() {
		if err != nil && !isAppSentinel(err) {
			err = b.humanize(err)
		}
	}()
	files, err := b.svc.SongFiles(actor, bandID, songID) // authorizes: member of the band, song in the band
	if err != nil {
		return nil, err
	}
	var file app.SongFile
	found := false
	for _, f := range files {
		if f.ID == fileID {
			file, found = f, true
			break
		}
	}
	if !found {
		return nil, app.ErrNotFound
	}
	song, err := b.svc.SongForMember(actor, bandID, songID)
	if err != nil {
		return nil, err
	}
	title := song.Title
	if title == "" {
		title = "Song"
	}
	snap, err := b.eng.Head(songID)
	if err != nil {
		return nil, err
	}
	item := app.SetlistItemView{SetlistItem: app.SetlistItem{SongID: songID}, SongTitle: title} // no transpose: the file as stored
	sf, req, err := b.stageFile(ctx, 0, 0, bandID, actor, item, snap, file)
	if err != nil {
		return nil, err
	}
	var rendered []renderedOverlay
	if req != nil && len(sf.rasters) > 0 { // no pages → nothing to draw on (and nothing to orphan-check)
		byKey, oerr := b.overlays.RenderBatch(ctx, []overlaySong{*req})
		if oerr != nil {
			return nil, b.fail("The annotation renderer isn't available on the server. Ask an admin to check the bake setup.", oerr)
		}
		rendered = byKey[req.Key]
	}
	overlaysByPage := map[int][]renderedOverlay{}
	for _, ov := range rendered {
		if ov.Page < 0 || ov.Page >= len(sf.rasters) {
			// Same T145 guard as the bake: never print a page with a mark silently dropped.
			return nil, b.fail(fmt.Sprintf("An annotation on %q sits on page %d, but the file has only %d page(s). Re-check the marks before printing.", title, ov.Page+1, len(sf.rasters)),
				fmt.Errorf("filepdf song %s file %s: overlay page %d of %d", songID, fileID, ov.Page, len(sf.rasters)))
		}
		overlaysByPage[ov.Page] = append(overlaysByPage[ov.Page], ov)
	}

	bandName := ""
	if bnd, _, gerr := b.svc.GetBand(actor, bandID); gerr == nil { // best effort: cosmetic footer text
		bandName = bnd.Name
	}
	fileName := FileDisplayName(file)
	footer := title
	if bandName != "" {
		footer = bandName + " · " + title
	}
	docTitle := title + " — " + fileName
	n := len(sf.rasters)
	return writeA4PDF(docTitle, docTitle, n, func(i int) (image.Image, string, string, error) {
		raster, _, derr := image.Decode(bytes.NewReader(sf.rasters[i]))
		if derr != nil {
			return nil, "", "", fmt.Errorf("decode page %d raster: %w", i, derr)
		}
		var layers []composeLayer
		for _, ov := range overlaysByPage[i] {
			if len(ov.PNG) == 0 || visible == nil || !visible(ov.LayerID) {
				continue
			}
			pngBytes := ov.PNG
			layers = append(layers, composeLayer{order: ov.Order, load: func() (image.Image, error) {
				img, _, e := image.Decode(bytes.NewReader(pngBytes))
				return img, e
			}})
		}
		composed, cerr := composeLayers(raster, layers)
		if cerr != nil {
			return nil, "", "", cerr
		}
		hdr := fmt.Sprintf("%s — %s — page %d/%d", title, fileName, i+1, n)
		return composed, hdr, footer, nil
	})
}

// FileDisplayName is how a pool file is named in a print and its download: the filename without a
// trailing ".pdf" ("Lead sheet.pdf" → "Lead sheet"), or "File" when nothing is left.
func FileDisplayName(f app.SongFile) string {
	name := strings.TrimSpace(f.Filename)
	if strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name = strings.TrimSpace(name[:len(name)-len(".pdf")])
	}
	if name == "" {
		return "File"
	}
	return name
}

// isAppSentinel reports whether err is one of the service's classified errors, which the HTTP edge maps to
// a status (404/403/401) and which are already user-safe — so FilePDF passes them through un-humanized.
func isAppSentinel(err error) bool {
	return errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrForbidden) || errors.Is(err, app.ErrUnauthorized)
}

// FileLayerVisibility resolves which layers of a song a single-file print paints (FILEPDF), from the song's
// CURRENT head snapshot. See fileLayerVisible for the rule.
func (b *Baker) FileLayerVisibility(songID, viewerID string, requested []string, explicit bool) (func(layerID string) bool, error) {
	snap, err := b.eng.Head(songID)
	if err != nil {
		return nil, err
	}
	return fileLayerVisible(snap.Layers, viewerID, requested, explicit), nil
}

// fileLayerVisible is the layer choice for a single-file print. A mandatory layer is ALWAYS painted — it
// can't be hidden anywhere else either (I12). Otherwise:
//
//   - explicit (the client sent the layers it shows on screen): exactly the requested ids; ids that are not
//     layers of this song are ignored, and an empty request means "no optional annotations".
//   - not explicit: the default view of a fresh viewer — LayerVisible with role "" and the viewer's own id:
//     shared untagged layers, plus the viewer's own personal layers.
//
// A layer id absent from the snapshot (created after it was read) is not painted.
func fileLayerVisible(layers []domain.Layer, viewerID string, requested []string, explicit bool) func(layerID string) bool {
	want := make(map[string]bool, len(requested))
	for _, id := range requested {
		want[id] = true
	}
	on := make(map[string]bool, len(layers))
	for _, l := range layers {
		switch {
		case l.Mandatory:
			on[l.ID] = true
		case explicit:
			on[l.ID] = want[l.ID]
		default:
			on[l.ID] = LayerVisible(LayerImage{LayerID: l.ID, Mandatory: l.Mandatory, RoleTag: l.RoleTag, Owner: bakedOwner(l)}, "", viewerID)
		}
	}
	return func(layerID string) bool { return on[layerID] }
}
