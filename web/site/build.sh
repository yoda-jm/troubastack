#!/usr/bin/env bash
# Assemble the public site into web/site/dist/.
#
# The site owns exactly one hand-written file: index.html. Everything else is
# COPIED or GENERATED from what already exists in the repo — the brand marks
# from docs/brand/dist, the screenshots from docs/screenshots, the marker swipe
# from the same band() the icons are drawn with, the QR from the repo URL. No
# asset is duplicated into the tree, so nothing here can drift from the source
# of truth the way a second copy always eventually does.
#
#   ./web/site/build.sh            # → web/site/dist/
#
# dist/ is generated and gitignored. The Pages workflow runs this and uploads
# ONLY dist/, which is the point: Pages must never be pointed at docs/, or the
# entire internal documentation tree — handoffs, reviews, task specs — becomes a
# published website.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SITE="$ROOT/web/site"
OUT="$SITE/dist"
REPO_URL="https://github.com/yoda-jm/troubastack"
# The QR is for the APP, and the only place an APK exists today is the CI run:
# there is no release, no registry image and no store listing. This lands on the
# successful main builds, whose android job attaches troubastage-debug-apk.
APK_URL="https://github.com/yoda-jm/troubastack/actions/workflows/ci.yml?query=branch%3Amain+is%3Asuccess"

rm -rf "$OUT"
mkdir -p "$OUT/assets"
cp "$SITE/index.html" "$OUT/"

# --- the brand marks --------------------------------------------------------
# Regenerated first, so the site can never ship an icon that no longer matches
# the bricks. build.py is stdlib-only; this costs nothing.
python3 "$ROOT/docs/brand/build.py" >/dev/null
# full for the social card; compact for the page — full's chip is 9px at card size.
for f in troubastack-full troubastack-compact troubastudio-compact troubacore-compact \
         troubastage-compact troubastack-minimal; do
  cp "$ROOT/docs/brand/dist/$f.svg" "$OUT/assets/"
done

# --- screenshots ------------------------------------------------------------
# An explicit allow-list, never a glob. docs/screenshots holds ~70 images, some
# showing a real band's material and one naming a copyrighted song and its
# artist; a public page must carry only what has been looked at. Every file
# below shows the synthetic demo cast (Marie/Leo/Sasha, "The Troubadours") and
# the original chart "The Open Road", which is marked free to ship.
for f in studio-editor band-overview stage-page stage-controls stage-concerts; do
  cp "$ROOT/docs/screenshots/$f.png" "$OUT/assets/"
done

# --- the marker swipe -------------------------------------------------------
# The section highlight is the PRODUCT's marker pass, not a CSS rectangle: the
# path band() emits, same half-width, corner radius and (negative) sagitta as
# the icon's stroke. The gold is baked into the file rather than applied with a
# CSS mask, because masks need a CORS-clean source and therefore paint NOTHING
# over file:// — which is how this page gets previewed before it ships.
python3 - "$ROOT" "$OUT/assets/swipe.svg" <<'PYEOF'
import sys, pathlib
root, out = sys.argv[1], sys.argv[2]
sys.path.insert(0, root + "/docs/brand")
import build as B
d = B.band((0, 0), (B.LEN, 0), B.SAG, B.HW, B.RAD)
pathlib.Path(out).write_text(
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -60 {B.LEN} 120" '
    f'preserveAspectRatio="none"><path d="{d}" fill="{B.SHARED_HL}"/></svg>\n')
print(f"  swipe: band() LEN={B.LEN} HW={B.HW} RAD={B.RAD} SAG={B.SAG} fill={B.SHARED_HL}")
PYEOF

# --- the QR -----------------------------------------------------------------
if command -v qrencode >/dev/null; then
  qrencode -t SVG -m 1 -o "$OUT/assets/qr-repo.svg" "$REPO_URL"
  qrencode -t SVG -m 1 -o "$OUT/assets/qr-apk.svg" "$APK_URL"
else
  echo "warn: qrencode missing — the QR codes will not render" >&2
fi

# --- the social card --------------------------------------------------------
# og:image wants a raster; SVG is not reliably honoured by link unfurlers.
if command -v rsvg-convert >/dev/null; then
  rsvg-convert -w 512 -h 512 -o "$OUT/assets/troubastack-512.png" \
    "$ROOT/docs/brand/dist/troubastack-full.svg"
else
  echo "warn: rsvg-convert missing — no og:image raster" >&2
fi

echo "site → $OUT ($(find "$OUT" -type f | wc -l) files, $(du -sh "$OUT" | cut -f1))"
