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

Known cosmetic issue inherited from the seed data: the seeded PDFs render an em-dash as
`â€"` in page titles (an encoding bug in `core/cmd/seed`'s PDF text — worth a small fix).
