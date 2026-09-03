# web/studio/public

Files here are copied verbatim into the build root by Vite.

## Favicons

The **SVG** favicon (`troubastudio-minimal.svg`) is NOT here on purpose: it has one source
of truth — `docs/brand/dist/troubastudio-minimal.svg` — and the `brandFavicon` plugin in
`vite.config.ts` serves it in dev and copies it into `dist/` at build. A committed second
copy would drift from the bricks the moment the brand changes (see `web/site/build.sh`).

The **PNG** rasters below are the fallback for browsers without SVG-favicon support, and
must be committed (Vite does not rasterize, and the build must not shell out to a converter
that CI may lack). Regenerate them from the same source whenever the mark changes:

```sh
SRC=docs/brand/dist/troubastudio-minimal.svg
rsvg-convert -w 180 -h 180 -o web/studio/public/apple-touch-icon.png "$SRC"
rsvg-convert -w  48 -h  48 -o web/studio/public/favicon.png          "$SRC"
```
