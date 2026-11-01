# TroubaStage desktop icon

`troubastage.ico` is the Windows app icon used by the MSI (`iconFile` in `../build.gradle.kts`). It is the
TroubaStage launcher mark — the same adaptive icon the Android app uses (`BRAND05`: the play chip / layers on
the dark squircle), composited from the two brand layers and written as a multi-size `.ico` (256/128/64/48/32/24/16).

Regenerate it (from the repo root) with:

```sh
rsvg-convert -w 1024 -h 1024 docs/brand/dist/troubastage-adaptive-background.svg -o /tmp/ico-bg.png
rsvg-convert -w 1024 -h 1024 docs/brand/dist/troubastage-adaptive-foreground.svg -o /tmp/ico-fg.png
magick /tmp/ico-bg.png /tmp/ico-fg.png -layers flatten \
  \( -clone 0 -resize 256x256 \) \( -clone 0 -resize 128x128 \) \( -clone 0 -resize 64x64 \) \
  \( -clone 0 -resize 48x48 \) \( -clone 0 -resize 32x32 \) \( -clone 0 -resize 24x24 \) \( -clone 0 -resize 16x16 \) \
  -delete 0 app/desktopApp/icons/troubastage.ico
```

(`rsvg-convert` is librsvg; `magick` is ImageMagick 7. The adaptive-foreground layer is already inset to the
launcher safe circle, so composited over the background it is the full squircle mark.)
