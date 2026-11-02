#!/usr/bin/env bash
# DESK01 D2 — wrap Compose's Linux app image (the TroubaStage folder: bin/, lib/app, the bundled Java runtime) into one
# `troubastage-linux-x86_64.AppImage`. CI runs it; so can anyone on Linux:
#
#   make-appimage.sh <app-image dir> <out.AppImage> <appimagetool> <runtime-x86_64>
#
# <appimagetool> and <runtime-x86_64> are the pinned, digest-checked downloads (see the `desktop` job in ci.yml). The
# runtime is passed explicitly so the build never fetches one of its own: it is AppImage's static type-2 runtime, which
# needs no libfuse2 on the user's machine (only FUSE itself; without it, `--appimage-extract-and-run` still works).
set -euo pipefail

app=$1 out=$2 tool=$3 runtime=$4
here=$(cd "$(dirname "$0")" && pwd)
[ -x "$app/bin/TroubaStage" ] || { echo "not a TroubaStage app image: $app" >&2; exit 1; }

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
appdir="$dir/TroubaStage.AppDir"
mkdir -p "$appdir"
cp -a "$app" "$appdir/TroubaStage"
# The launcher finds its lib/ from its own real path (/proc/self/exe), so AppRun can simply point at it.
ln -s TroubaStage/bin/TroubaStage "$appdir/AppRun"
cp "$here/troubastage.desktop" "$appdir/troubastage.desktop"
cp "$here/../../icons/troubastage.png" "$appdir/troubastage.png"
ln -s troubastage.png "$appdir/.DirIcon"

# appimagetool is itself an AppImage: extract-and-run, so the build machine needs no FUSE.
ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$tool" --no-appstream --runtime-file "$runtime" "$appdir" "$out"
chmod +x "$out"
echo "AppImage: $out ($(stat -c %s "$out") bytes)"
