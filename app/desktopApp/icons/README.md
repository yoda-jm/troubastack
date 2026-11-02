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

## Linux and macOS (DESK01)

`troubastage.png` (512×512) is the same mark for Linux — the AppImage's icon, the jpackage app image's, and the
window's own icon on every OS (copied into the app's resources). `troubastage.icns` is the macOS app icon, the same
mark at the macOS sizes, each entry a PNG. Regenerate both from the 1024 px composite above (`/tmp/ico-1024.png`, the
`-layers flatten` result written to a file):

```sh
magick /tmp/ico-1024.png -resize 512x512 -strip PNG32:app/desktopApp/icons/troubastage.png
for z in 16 32 64 128 256 512 1024; do magick /tmp/ico-1024.png -resize ${z}x${z} -strip PNG32:/tmp/icns-$z.png; done
python3 - <<'PY'
import struct
entries = [("icp4",16),("icp5",32),("icp6",64),("ic07",128),("ic08",256),("ic09",512),("ic10",1024),
           ("ic11",32),("ic12",64),("ic13",256),("ic14",512)]
body = b"".join(t.encode() + struct.pack(">I", len(p) + 8) + p
                for t, z in entries for p in [open(f"/tmp/icns-{z}.png", "rb").read()])
open("app/desktopApp/icons/troubastage.icns", "wb").write(b"icns" + struct.pack(">I", len(body) + 8) + body)
PY
```
