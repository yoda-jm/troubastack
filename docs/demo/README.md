# demo-concert.tstage — a real-music demo bundle (no server needed)

`demo-concert.tstage` (~90 KB) is a **hand-baked** concert bundle: two pages of the
seeded *Wonderwall — Vocals* part with its real annotations, packaged per
[`../design/08-bundle-container.md`](../design/08-bundle-container.md). Install the app
(root README → "The mobile app"), share/push this file to the device, **Import**, and
perform it fully offline.

It demos the layer semantics:

- **`sections`** (section labels + highlight bands) is `mandatory` — always composited,
  the viewer can't hide it.
- **`conductor-cues`** (the red "Watch me" markings) carries `roleTag: "conductor"` —
  hidden by default; set **Role → `conductor`** in Stage and they appear.

## Provenance (and why "hand-baked")

The real bake pipeline (server-side, invariants I8/I11) doesn't exist yet. This bundle
was assembled from the *running* seeded editor: the PDF raster canvases and the
transparent annotation-overlay canvases were extracted per layer from TroubaStudio
(`make demo` data, `marie`'s view), downscaled, and wrapped in a canonical-JSON
`bundle.json`. So the images are genuine studio renders, but the *packaging* is manual —
when the real bake lands, regenerate this file with it and delete this caveat.

Known cosmetic issue inherited from the seed data: this bundle's page-title raster shows
an em-dash rendered as `â€"`. The underlying seed bug (`core/cmd/seed` wrote UTF-8 into a
cp1252 PDF font) is **fixed** as of T16 — fresh `make demo` seeds now render `—`
correctly — but this file was hand-baked from the *old* seeds, so its raster keeps the
mojibake until the real bake (B02) regenerates it. Do not hand-rebake it for this.
