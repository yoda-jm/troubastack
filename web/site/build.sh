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
# Where this build will be served from, with NO trailing slash. Only the og: tags need it,
# and they need it absolute: unfurlers do not resolve a relative og:image against the page.
# Override to move the site — a custom domain is this one variable, set here or in the
# workflow, and nothing in index.html changes.
SITE_URL="${SITE_URL:-https://yoda-jm.github.io/troubastack}"
# The QR is for the APP: a direct link to the debug APK on the rolling `latest` release
# (OPS05), so a phone downloads the raw .apk and taps into the installer — no GitHub
# sign-in, no zip. The explicit-tag URL is stable across rolling builds (fixed tag + asset
# name), and survives the release being a prerelease (the /latest/ shortcut would not).
APK_URL="https://github.com/yoda-jm/troubastack/releases/download/latest/troubastage-debug.apk"
# Google Search Console ownership token. NOT a secret — it is SERVED publicly at
# $SITE_URL/$GSC_TOKEN.html, which is the whole mechanism, and Google's own instructions are to
# leave it in place forever ("don't remove the file, even after verification succeeds"). So do
# not delete it as a leaked credential during a secret sweep: removing it un-verifies the
# property and the sitemap stops being re-read.
#
# It is GENERATED rather than committed as a downloaded asset, because its content is entirely
# determined by its name — one source, and no opaque blob in the tree.
GSC_TOKEN="google59f46b47ac995647"

rm -rf "$OUT"
mkdir -p "$OUT/assets"
cp "$SITE/index.html" "$OUT/"
# Substitute the origin into the og: tags, then refuse to ship a page that still holds a
# token — a silent miss here is invisible until someone pastes the link somewhere.
sed -i "s|{{SITE_URL}}|$SITE_URL|g" "$OUT/index.html"
if grep -q '{{SITE_URL}}' "$OUT/index.html"; then
  echo "error: {{SITE_URL}} survived substitution in dist/index.html" >&2; exit 1
fi
# The APK link is tappable on the phone itself (download → install), not only scannable from
# another screen — so the same URL the QR encodes is also an href. Same no-stale-token guard.
sed -i "s|{{APK_URL}}|$APK_URL|g" "$OUT/index.html"
if grep -q '{{APK_URL}}' "$OUT/index.html"; then
  echo "error: {{APK_URL}} survived substitution in dist/index.html" >&2; exit 1
fi
grep -q "<meta property=\"og:image\" content=\"$SITE_URL/assets/" "$OUT/index.html" || {
  echo "error: og:image is not absolute under $SITE_URL" >&2; exit 1; }
echo "  site url: $SITE_URL"

# --- sitemap + robots -------------------------------------------------------
# GENERATED, not hand-written: the site owns one hand-written file, and an address
# written twice is an address that will disagree with itself. Both come out of the
# same $SITE_URL the og: tags use.
#
# lastmod is the last commit that touched index.html, NOT the build date. A lastmod
# that moves on every build tells a crawler the page changed when it did not, and a
# site that cries wolf gets crawled less. If git is unavailable the field is omitted
# entirely — no date is honest, a wrong date is not.
LASTMOD="$(git -C "$ROOT" log -1 --format=%cs -- web/site/index.html 2>/dev/null || true)"
{
  echo '<?xml version="1.0" encoding="UTF-8"?>'
  echo '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'
  echo "  <url>"
  echo "    <loc>$SITE_URL/</loc>"
  [ -n "$LASTMOD" ] && echo "    <lastmod>$LASTMOD</lastmod>"
  echo "  </url>"
  echo '</urlset>'
} > "$OUT/sitemap.xml"

# robots.txt names the sitemap ABSOLUTELY — a relative Sitemap: line is ignored.
#
# ⚠ AND IT DOES NOTHING TODAY (VLL, 2026-09-28). robots.txt is honoured ONLY at the HOST
# ROOT, and this is a PROJECT page: crawlers read https://yoda-jm.github.io/robots.txt,
# which 404s (there is no user-page repo). The file we write under /troubastack/ is never
# fetched, so its Sitemap: line discovers nothing. It is written anyway because it becomes
# live for free the day SITE_URL is a custom domain — then the site IS the host root. Until
# then the ONLY way Google learns of the sitemap is Search Console, by hand: the ping
# endpoint Google offered for this was retired at the end of 2023 and now 404s.
printf 'User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n' "$SITE_URL" > "$OUT/robots.txt"

# The guards, in the shape the og: tags already use: refuse to ship an address that
# disagrees with itself, or a file still holding a token.
CANON="$(grep -oE '<link rel="canonical" href="[^"]+"' "$OUT/index.html" | grep -oE 'href="[^"]+"' | cut -d'"' -f2)"
OGURL="$(grep -oE '<meta property="og:url" content="[^"]+"' "$OUT/index.html" | grep -oE 'content="[^"]+"' | cut -d'"' -f2)"
[ -n "$CANON" ] || { echo "error: no canonical in dist/index.html" >&2; exit 1; }
[ "$CANON" = "$OGURL" ] || {
  echo "error: canonical ($CANON) and og:url ($OGURL) disagree — one page, one address" >&2; exit 1; }
grep -q "^Sitemap: $SITE_URL/sitemap.xml$" "$OUT/robots.txt" || {
  echo "error: robots.txt does not name the sitemap absolutely" >&2; exit 1; }
grep -q "<loc>$SITE_URL/</loc>" "$OUT/sitemap.xml" || {
  echo "error: sitemap does not carry $SITE_URL/" >&2; exit 1; }
# The namespace is the one thing in this file a typo makes INVALID rather than wrong — a
# crawler rejects the document whole. I shipped `sitemap.org` for `sitemaps.org` while
# writing this; the guard is cheaper than the next person doing it too.
grep -q 'xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"' "$OUT/sitemap.xml" || {
  echo "error: sitemap namespace is not the sitemaps.org 0.9 schema" >&2; exit 1; }
echo "  canonical: $CANON${LASTMOD:+ · sitemap lastmod $LASTMOD}"

# --- Search Console ownership ------------------------------------------------
# Why this file exists at all: a PROJECT page cannot be discovered automatically. robots.txt is
# read only at the host root (which 404s here) and Google's sitemap ping endpoint was retired in
# 2023 — so Search Console, verified by this file, is the ONLY path by which the sitemap above
# gets looked at. It is the working half of OPS03.
GSC_BODY="google-site-verification: $GSC_TOKEN.html"
printf '%s' "$GSC_BODY" > "$OUT/$GSC_TOKEN.html"
# Byte-exact against what Google hands you: that string and nothing else.
#
# The expectation is derived from the FILENAME ON DISK, never from $GSC_BODY — comparing the
# output to the variable that wrote it is a check that cannot fail, which is how the first
# version of this guard passed a deliberately wrong body. The file's own name is the truth:
# Google's file says "google-site-verification: <its own filename>".
GSC_FILE="$OUT/$GSC_TOKEN.html"
[ "$(cat "$GSC_FILE")" = "google-site-verification: $(basename "$GSC_FILE")" ] || {
  echo "error: $(basename "$GSC_FILE") does not carry its own filename" >&2; exit 1; }
[ "$(wc -c < "$GSC_FILE")" = "${#GSC_BODY}" ] || {
  echo "error: $GSC_TOKEN.html has trailing bytes (Google's file has no newline)" >&2; exit 1; }
echo "  search console: $GSC_TOKEN.html"

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
