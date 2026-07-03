# Design: Bundle container format (`.tstage`)

Derives from **I11, I12** and [`05-distribution.md`](05-distribution.md). Defines the concrete
on-disk / on-the-wire layout of a **baked concert bundle** so the presenter (A04), the importer
(A05), and the server-side bake (later) all agree on the same bytes. The *messages* are defined
once in [`../../proto/troubastack/v1/bundle.proto`](../../proto/troubastack/v1/bundle.proto) (**I1** —
that file is the authority); this doc only pins the container around them.

## Directory layout

A bundle is a **directory**:

```
<bundle>/
├── bundle.json          ← the manifest: a ConcertBundle (see below)
└── blobs/
    ├── p0-raster.webp   ← page rasters + transparent per-layer overlays
    ├── p0-L1.webp
    └── …
```

- `bundle.json` sits at the bundle root.
- Every image is a **blob** under `blobs/`. Nothing else is required.

## `bundle.json` — proto3 canonical JSON of `ConcertBundle`

`bundle.json` is the **proto3 canonical JSON** encoding of the `ConcertBundle` message. That means
a future `buf`/protobuf-generated encoder produces byte-identical output, and the hand-written
Kotlin mirror (`app/shared/.../bundle/BundleModel.kt`) round-trips it. Concretely:

- **Field names are lowerCamelCase** of the proto field: `concert_id → concertId`,
  `concert_rev → concertRev`, `baked_at → bakedAt`, `page_raster_ref → pageRasterRef`,
  `image_ref → imageRef`, `final_locked → finalLocked`, `role_tag → roleTag`, etc.
- **64-bit integers (`int64`/`uint64`) are JSON strings**, not numbers — e.g.
  `"concertRev":"7"`, `"bakedAt":"1700000000"`, `"sourceRevision":"3"`. 32-bit `order` stays a
  JSON number. (This is the proto3 canonical-JSON rule; the loader also tolerates a bare number on
  read.)
- **Default-valued fields may be omitted** (canonical JSON drops zero/false/""/[]). Consumers must
  treat an absent field as its proto default, so every mirror field has a default.
- **Unknown fields are ignored** on read (`ignoreUnknownKeys`) for forward compatibility.

Shape (see the proto for the authoritative field set):

```jsonc
{
  "concertId": "c1", "name": "Spring Gig", "concertRev": "7",
  "bakedAt": "1700000000", "bakedBy": "maestro", "finalLocked": true,
  "songs": [{
    "songId": "s1", "sourceRevision": "3", "songRev": "1",
    "pages": [{
      "pageRasterRef": "blobs/p0-raster.webp", "rasterHash": "…",
      "overlays": [
        { "layerId": "L1", "imageRef": "blobs/p0-L1.webp", "contentHash": "…", "order": 1, "mandatory": false, "roleTag": "" }
      ]
    }]
  }]
}
```

## Blob refs

`pageRasterRef` and each overlay `imageRef` are **opaque relative paths** resolved against the
bundle root (e.g. `blobs/p0-raster.webp`). Rules:

- The ref **carries the real file extension**; consumers must not assume a codec. Production rasters
  are WebP, but the ref is the source of truth for the actual bytes.
- A ref is just a path — no directory structure beyond `blobs/` is implied or required.
- **Resilience (I12):** a ref that points at a missing or empty (0-byte) file does **not** fail the
  load. The bundle loads and that page is flagged so the presenter can show a placeholder for *only*
  that page. A malformed/missing `bundle.json`, by contrast, fails the whole load.
- Within a page, overlays are composited in ascending `order`; a repeated `layerId` on the same page
  is dropped (first wins) and flagged.

## Transport form: `.tstage`

For sharing/import, the bundle directory is **zipped**, with `bundle.json` at the **zip root** (not
nested under a top-level folder) and `blobs/` alongside it. The file extension is **`.tstage`**.
Unzipping yields exactly the directory layout above.

> Producing/unzipping `.tstage` is **A05**'s job (via the Storage seam); this doc only specifies it.
> Image decoding/compositing is **A04**. This task ships only the model + the resilient loader.
