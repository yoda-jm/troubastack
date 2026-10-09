#!/usr/bin/env bash
# Master an episode (docs/video/README.md §3.1, §4.5): two-pass EBU R128 loudness to −16 LUFS / −1.5 dBTP,
# AAC 192k, the MP4 + a VP9/Opus .webm for the site, and the render checks. Captions (.srt/.vtt) were
# written from the known text by build_edit.py.
#
#   video/master.sh 03
set -euo pipefail
EP="$(printf %02d "$((10#${1:?episode number}))")"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RAW="$ROOT/video/remotion/out/ep$EP-raw.mp4"
OUT="$ROOT/video/out/ep$EP"
test -s "$RAW" || { echo "error: render first ($RAW)" >&2; exit 1; }
mkdir -p "$OUT"
# pass 1: measure
J="$(ffmpeg -hide_banner -nostats -i "$RAW" -af loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json -f null - 2>&1 | sed -n '/^{/,/^}/p')"
m() { python3 -c "import json,sys; print(json.loads(sys.argv[1])['$1'])" "$J"; }
# pass 2: apply, linear
ffmpeg -v error -y -i "$RAW" -c:v copy -af "loudnorm=I=-16:TP=-1.5:LRA=11:measured_I=$(m input_i):measured_TP=$(m input_tp):measured_LRA=$(m input_lra):measured_thresh=$(m input_thresh):offset=$(m target_offset):linear=true,aresample=48000" \
  -c:a aac -b:a 192k -movflags +faststart "$OUT/ep$EP.mp4"
ffmpeg -v error -y -i "$OUT/ep$EP.mp4" -c:v libvpx-vp9 -crf 32 -b:v 0 -row-mt 1 -c:a libopus -b:a 128k "$OUT/ep$EP.webm"

# checks (§4.5)
I="$(ffmpeg -hide_banner -nostats -i "$OUT/ep$EP.mp4" -af ebur128=peak=true -f null - 2>&1 | /usr/bin/grep -A12 'Summary' )"
LUFS="$(echo "$I" | /usr/bin/grep -oE 'I: +-?[0-9.]+' | /usr/bin/grep -oE -- '-?[0-9.]+$')"
TP="$(echo "$I" | /usr/bin/grep -oE 'Peak: +-?[0-9.]+' | /usr/bin/grep -oE -- '-?[0-9.]+$')"
DUR="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$OUT/ep$EP.mp4")"
RES="$(ffprobe -v error -select_streams v -show_entries stream=width,height,r_frame_rate -of csv=p=0 "$OUT/ep$EP.mp4")"
CUES="$(/usr/bin/grep -c -- '-->' "$OUT/ep$EP.srt")"
ok=1
python3 -c "import sys; sys.exit(0 if abs(float('$LUFS') + 16) <= 1 else 1)" || { echo "FAIL loudness $LUFS LUFS (want -16 ±1)"; ok=0; }
python3 -c "import sys; sys.exit(0 if float('$TP') <= -1.0 else 1)" || { echo "FAIL true peak $TP dBTP (want ≤ -1)"; ok=0; }
[ "$RES" = "1920,1080,30/1" ] || { echo "FAIL video $RES (want 1920x1080 @30)"; ok=0; }
[ "$CUES" -gt 0 ] || { echo "FAIL no captions"; ok=0; }
echo "E$EP master: $OUT/ep$EP.mp4 — ${DUR%.*} s, $RES, $LUFS LUFS, true peak $TP dBTP, $CUES caption cues (+ .webm, .srt, .vtt)"
[ "$ok" = 1 ]
