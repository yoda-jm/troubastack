#!/usr/bin/env bash
# DEMO-VID Part D — assemble the WEB walkthrough (Part 1) with voiceover.
#
# Joins the web-scene narration (S0–S14) into one track, time-aligns the silent walkthrough
# recording to it, prepends a title card, and muxes → a watchable MP4. The full film also needs
# Part C (the app capture, scenes 15–21) concatenated on the end — a follow-up once that lands.
#
# Usage:  docs/video/tools/assemble.sh <narration-dir> <walkthrough.webm> [out.mp4]
set -euo pipefail
NARR="${1:?narration dir with S*.wav + timings.json}"
VIDEO="${2:?path to the walkthrough .webm}"
OUT="${3:-docs/video/output/walkthrough-web.mp4}"
TITLE="TroubaShare"
SUB="rehearse together · perform offline"
W=1920; H=1080
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT

# 1. Concat the WEB-scene narration (S0..S14) with a short gap between scenes.
sil="$work/sil.wav"
ffmpeg -y -f lavfi -i anullsrc=r=22050:cl=mono -t 0.5 "$sil" -loglevel error
: > "$work/list.txt"
for n in $(seq 0 14); do
  w="$NARR/S$n.wav"; [ -f "$w" ] || continue
  echo "file '$(realpath "$w")'" >> "$work/list.txt"
  echo "file '$(realpath "$sil")'" >> "$work/list.txt"
done
ffmpeg -y -f concat -safe 0 -i "$work/list.txt" -ar 44100 -ac 2 "$work/narration.wav" -loglevel error
NDUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$work/narration.wav")
VDUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$VIDEO")
echo "narration ${NDUR}s · video ${VDUR}s"

# 2. Time-stretch the silent walkthrough to the narration length (both progress through the
#    same scenes in order → coherent sync), normalize to 1920x1080 h264.
FACTOR=$(python3 -c "print(f'{$NDUR/$VDUR:.6f}')")
ffmpeg -y -i "$VIDEO" -vf "setpts=${FACTOR}*PTS,scale=${W}:${H}:force_original_aspect_ratio=decrease,pad=${W}:${H}:(ow-iw)/2:(oh-ih)/2:color=0xEDEAE3,fps=30" -an "$work/body.mp4" -loglevel error

# 3. A 3s title card (S0 narration plays over it).
ffmpeg -y -f lavfi -i "color=c=0xEDEAE3:s=${W}x${H}:d=3" \
  -vf "drawtext=text='${TITLE}':fontcolor=0x3B3A54:fontsize=120:x=(w-tw)/2:y=(h)/2-110:box=0,drawtext=text='${SUB}':fontcolor=0x6B6A78:fontsize=48:x=(w-tw)/2:y=(h)/2+40" \
  -r 30 -pix_fmt yuv420p "$work/title.mp4" -loglevel error

# 4. Concat title + body (video), then mux the narration.
printf "file '%s'\nfile '%s'\n" "$work/title.mp4" "$work/body.mp4" > "$work/vlist.txt"
ffmpeg -y -f concat -safe 0 -i "$work/vlist.txt" -c copy "$work/silent.mp4" -loglevel error
# narration starts after the title card (title = 3s); pad the front by 3s of silence
ffmpeg -y -f lavfi -i anullsrc=r=44100:cl=stereo -t 3 "$work/pre.wav" -loglevel error
printf "file '%s'\nfile '%s'\n" "$(realpath "$work/pre.wav")" "$(realpath "$work/narration.wav")" > "$work/alist.txt"
ffmpeg -y -f concat -safe 0 -i "$work/alist.txt" "$work/audio.wav" -loglevel error
mkdir -p "$(dirname "$OUT")"
ffmpeg -y -i "$work/silent.mp4" -i "$work/audio.wav" -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p -c:a aac -b:a 160k -shortest "$OUT" -loglevel error
echo "→ $OUT ($(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$OUT")s)"
