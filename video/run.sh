#!/usr/bin/env bash
# Build one episode end to end (docs/video/README.md §4): music → narration → filming server → takes →
# edit → render → master. Every step is re-runnable; takes and narration are cached in video/out.
#
#   video/run.sh 03               # everything
#   STEPS="edit render master" video/run.sh 03   # only some steps
set -euo pipefail
EP="$(printf %02d "$((10#${1:?episode number}))")"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STEPS="${STEPS:-music tts server takes edit render master}"
V="$ROOT/video"
has() { [[ " $STEPS " == *" $1 "* ]]; }
cd "$ROOT"
has music  && { [ -s "$V/out/music/bed.wav" ] || "$V/music/render.sh"; }
has tts    && "${TROUBA_VIDEO_CACHE:-$HOME/.cache/troubastack-video}/venv/bin/python" "$V/tts/synth.py" "$EP"
has server && { "$V/server.sh" build; "$V/server.sh" start; }
if has takes; then
  case "$EP" in
    03) ( cd "$V/takes" && node web-get-the-app.mjs ../out/takes )
        "$V/takes/emulator.sh" fresh
        ( cd "$V/takes" && python3 e03_tablet.py ../out/takes ) ;;
    *)  echo "no take scripts for E$EP yet" >&2; exit 1 ;;
  esac
fi
has edit   && python3 "$V/build_edit.py" "$EP"
has render && ( cd "$V/remotion" && npx remotion render src/index.ts "ep$EP" "out/ep$EP-raw.mp4" --codec=h264 --crf=18 --audio-bitrate=192k --timeout=120000 --log=error )
has master && "$V/master.sh" "$EP"
