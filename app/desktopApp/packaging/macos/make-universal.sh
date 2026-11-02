#!/usr/bin/env bash
# DESK01 D3 — one TroubaStage.app for Apple silicon and Intel Macs, from the two app images Compose built (arm64 with
# the native JDK, x86_64 with an x64 JDK under Rosetta):
#
#   make-universal.sh <arm64 TroubaStage.app> <x86_64 TroubaStage.app> <out TroubaStage.app>
#
# Layout (Fable's GO b67486a3, layout B): the runtimes are never merged — each arch keeps the runtime jpackage built
# for it, so nothing in it can disagree with the CPU it runs on:
#   Contents/MacOS/TroubaStage          universal stub (universal-launcher.c): execs the launcher of its own slice
#   Contents/MacOS/TroubaStage-<arch>   jpackage's launcher from that arch's build
#   Contents/app/TroubaStage-<arch>.cfg the build's cfg + app.runtime=$ROOTDIR/Contents/runtime-<arch>
#   Contents/runtime-<arch>             that arch's runtime, as built
#   Contents/app                        ONCE: the same app in both builds (check-app-trees.py), both Skiko libraries
# Then ad-hoc signed inside-out (native libraries, runtimes, launchers, stub, the app); --deep only to verify.
set -euo pipefail

arm=$1 x64=$2 out=$3
here=$(cd "$(dirname "$0")" && pwd)
archs=(arm64 x86_64)
src() { [ "$1" = arm64 ] && echo "$arm" || echo "$x64"; }

for a in "${archs[@]}"; do
  [ -x "$(src "$a")/Contents/MacOS/TroubaStage" ] || { echo "not a TroubaStage app: $(src "$a")" >&2; exit 1; }
  got=$(lipo -archs "$(src "$a")/Contents/MacOS/TroubaStage")
  [ "$got" = "$a" ] || { echo "the $a build's launcher is [$got]" >&2; exit 1; }
done

# The merge rule: one app/ folder is only sound if both builds hold the same app.
python3 -I "$here/check-app-trees.py" "$arm/Contents/app" "$x64/Contents/app"

rm -rf "$out"; mkdir -p "$(dirname "$out")"
ditto "$arm" "$out"
rm -rf "$out/Contents/runtime" "$out/Contents/_CodeSignature" "$out/Contents/MacOS/TroubaStage" "$out/Contents/app/TroubaStage.cfg"
for a in "${archs[@]}"; do
  ditto "$(src "$a")/Contents/runtime" "$out/Contents/runtime-$a"
  cp "$(src "$a")/Contents/MacOS/TroubaStage" "$out/Contents/MacOS/TroubaStage-$a"
  { echo "[Application]"; echo "app.runtime=\$ROOTDIR/Contents/runtime-$a"; tail -n +2 "$(src "$a")/Contents/app/TroubaStage.cfg"; } \
    > "$out/Contents/app/TroubaStage-$a.cfg"
done
# Both Skiko libraries (each build extracted its own); Skiko loads the one for os.arch.
for f in "$x64/Contents/app"/libskiko-macos-*.dylib*; do [ -e "$out/Contents/app/$(basename "$f")" ] || cp "$f" "$out/Contents/app/"; done
clang -arch arm64 -arch x86_64 -O2 -Wall -Werror -mmacosx-version-min=11.0 -o "$out/Contents/MacOS/TroubaStage" "$here/universal-launcher.c"

# Every piece is what it claims to be.
[ "$(lipo -archs "$out/Contents/MacOS/TroubaStage")" = "x86_64 arm64" ] || { echo "the stub is not universal" >&2; exit 1; }
for a in "${archs[@]}"; do
  for f in "MacOS/TroubaStage-$a" "runtime-$a/Contents/Home/lib/server/libjvm.dylib" "app/libskiko-macos-${a/x86_64/x64}.dylib"; do
    got=$(lipo -archs "$out/Contents/$f")
    [ "$got" = "$a" ] || { echo "$f is [$got], expected $a" >&2; exit 1; }
    echo "$f: $got"
  done
done

# Ad-hoc signing, inside-out: Apple silicon refuses unsigned code; nothing here is signed with an identity.
while IFS= read -r -d '' f; do
  if file -b "$f" | grep -q 'Mach-O'; then codesign --force --sign - "$f"; fi
done < <(find "$out/Contents/app" "$out/Contents/runtime-arm64" "$out/Contents/runtime-x86_64" -type f -print0)
for a in "${archs[@]}"; do codesign --force --sign - "$out/Contents/runtime-$a"; done
codesign --force --sign - "$out/Contents/MacOS/TroubaStage-arm64" "$out/Contents/MacOS/TroubaStage-x86_64"
codesign --force --sign - "$out"
codesign --verify --deep --strict "$out"
sig=$(codesign -dv "$out" 2>&1 | grep -E '^Signature=')
[ "$sig" = "Signature=adhoc" ] || { echo "signature: [$sig]" >&2; exit 1; }
echo "universal app: $out — stub $(lipo -archs "$out/Contents/MacOS/TroubaStage"), $sig, $(du -sh "$out" | cut -f1)"
