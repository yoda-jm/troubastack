package bake

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"strconv"

	"troubastack/core/internal/app"
)

// CropPreviewPage (T198 U1b) renders ONE page of an uploaded PDF as it would BAKE under a proposed (draft)
// crop — the raster cropped + the annotations composited on top — and returns it as a PNG. It is the server
// side of Studio's "Tablet view": no client-side transform, and it reuses the bake's own pieces (previewPage
// → renderCropPage, applyCropToDoc, the overlay worker, composeLayers) so the preview matches the tablet.
//
// Read-only: no bake, no rev, no sidecar, nothing written. The member gate + P4 (a generated chart has no
// crop) are the same as the crop save. `visible` decides which layers are painted (as FILEPDF does).
func (b *Baker) CropPreviewPage(ctx context.Context, bandID, songID, fileID string, actor app.User, page int, draft app.Crop, visible func(layerID string) bool) (out []byte, err error) {
	defer func() {
		if err != nil && !isAppSentinel(err) {
			err = b.humanize(err)
		}
	}()
	if b.preview == nil {
		return nil, fmt.Errorf("preview renderer not configured")
	}
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
	if file.Generated {
		return nil, fmt.Errorf("%w: a generated chart has no crop to preview", app.ErrInvalidInput) // P4
	}
	if page < 0 {
		return nil, fmt.Errorf("%w: page index must not be negative", app.ErrInvalidInput)
	}
	// Validate the DRAFT the same way the save does (policy A + geometry + finite numbers), so a non-square
	// rect or a NaN is a 400 rather than reaching cropWindowPx raw. NormalizeCrop mutates draft in place.
	if err := app.NormalizeCrop(&draft); err != nil {
		return nil, err
	}
	snap, err := b.eng.Head(songID)
	if err != nil {
		return nil, err
	}
	_, pdf, err := b.svc.DownloadSongFile(actor, fileID)
	if err != nil {
		return nil, err
	}

	// The crop for THIS page (if any). crops drives both the raster render and applyCropToDoc so the marks
	// land on the same music; an uncropped page previews whole.
	crops := pageCrops{}
	var cr *cropRect
	if r, ok := draft.Pages[strconv.Itoa(page)]; ok {
		rc := cropRect{x: r.X, y: r.Y, w: r.W, h: r.H}
		cr = &rc
		crops[page] = rc
	}

	rasterPNG, err := b.preview.previewPage(ctx, pdf, page, cr)
	if err != nil {
		return nil, err
	}
	raster, _, derr := image.Decode(bytes.NewReader(rasterPNG))
	if derr != nil {
		return nil, fmt.Errorf("decode preview raster: %w", derr)
	}
	wN, hN := raster.Bounds().Dx(), raster.Bounds().Dy()

	// Overlay for page N only, at the cropped raster's size (OverlayWidth = wN), with the marks transformed
	// into the cropped frame. A page with no marks skips the worker entirely.
	doc := snapshotToDoc(snap, fileID, nil) // uploaded PDF: no chart projection
	doc = applyCropToDoc(doc, crops)
	var layers []composeLayer
	hasPageObjects := false
	for _, o := range doc.Objects {
		if o.Page == page {
			hasPageObjects = true
			break
		}
	}
	if hasPageObjects {
		req := overlaySong{
			Key:          "preview",
			Doc:          doc,
			Pages:        []pageSize{{Index: page, Width: wN, Height: hN}},
			OverlayWidth: wN,
		}
		byKey, oerr := b.overlays.RenderBatch(ctx, []overlaySong{req})
		if oerr != nil {
			return nil, b.fail("The annotation renderer isn't available on the server. Ask an admin to check the bake setup.", oerr)
		}
		for _, ov := range byKey["preview"] {
			if ov.Page != page || len(ov.PNG) == 0 || (visible != nil && !visible(ov.LayerID)) {
				continue
			}
			pngBytes := ov.PNG
			layers = append(layers, composeLayer{order: ov.Order, load: func() (image.Image, error) {
				img, _, e := image.Decode(bytes.NewReader(pngBytes))
				return img, e
			}})
		}
	}

	composed, cerr := composeLayers(raster, layers)
	if cerr != nil {
		return nil, cerr
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, composed); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
