#!/usr/bin/env bash
# DEMO-VID Part D — assemble the WEB walkthrough (Part 1) with voiceover, SYNCED PER SCENE.
#
# The walkthrough records scene marks (docs/video/output/scene-marks.json: when each narration
# scene S1..S14 begins in the footage). This cuts the footage at those marks and time-fits each
# scene to its OWN narration length, so the picture tracks the words scene-by-scene instead of
# one global stretch. A title card (S0 narration) leads. The full film also needs Part C (the
# app capture, scenes 15-21) concatenated on the end — a follow-up once that lands.
#
# Usage:  docs/video/tools/assemble.sh <narration-dir> <walkthrough.webm> [out.mp4]
set -euo pipefail
NARR="${1:?narration dir with S*.wav + timings.json}"
VIDEO="${2:?path to the walkthrough .webm}"
OUT="${3:-docs/video/output/walkthrough-web.mp4}"
MARKS="$(dirname "$NARR")/scene-marks.json"
TITLE="TroubaShare"
SUB="rehearse together · perform offline"
W=1920; H=1080; BG=0xEDEAE3
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
dur() { ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "$1"; }

VF_NORM="scale=${W}:${H}:force_original_aspect_ratio=decrease,pad=${W}:${H}:(ow-iw)/2:(oh-ih)/2:color=${BG},fps=30,format=yuv420p"

# ---- 0. Title card: length = S0 narration -----------------------------------
S0DUR=$(dur "$NARR/S0.wav")
ffmpeg -y -f lavfi -i "color=c=${BG}:s=${W}x${H}:d=${S0DUR}" \
  -vf "drawtext=text='${TITLE}':fontcolor=0x3B3A54:fontsize=120:x=(w-tw)/2:y=(h)/2-110:box=0,drawtext=text='${SUB}':fontcolor=0x6B6A78:fontsize=48:x=(w-tw)/2:y=(h)/2+40,format=yuv420p" \
  -r 30 "$work/seg00.mp4" -loglevel error
echo "seg00 title  ${S0DUR}s"

# ---- 1. Normalize the (VFR browser) webm to CFR 30fps so per-scene setpts is exact ----
echo "normalizing footage to CFR 30fps…"
ffmpeg -nostdin -y -i "$VIDEO" -vf "${VF_NORM}" -an -r 30 -vsync cfr "$work/cfr.mp4" -loglevel error
VIDEO="$work/cfr.mp4"

# ---- 1b. Per-scene plan (start dur narration) from the marks ----------------
if [ ! -f "$MARKS" ]; then echo "no scene-marks.json at $MARKS — run the walkthrough first" >&2; exit 1; fi
VDUR=$(dur "$VIDEO")
python3 - "$MARKS" "$NARR/timings.json" "$VDUR" > "$work/plan.txt" <<'PY'
import json, sys, wave, os
marks = json.load(open(sys.argv[1]))
timings = json.load(open(sys.argv[2]))
vdur = float(sys.argv[3])
# marks: [{id,t}]; find S1..S14 + END; clamp to video duration.
mt = {m["id"]: m["t"] for m in marks}
order = [f"S{i}" for i in range(1, 15)]
for i, sid in enumerate(order):
    start = mt.get(sid)
    if start is None:
        continue
    end = mt["END"] if sid == "S14" else mt.get(order[i + 1], mt.get("END", vdur))
    start = max(0.0, min(start, vdur))
    end = max(start + 0.1, min(end, vdur))
    ndur = timings.get(sid)
    if ndur is None:
        continue
    print(f"{sid} {start:.3f} {end:.3f} {ndur:.3f}")
PY
cat "$work/plan.txt"

# ---- 2. Cut + time-fit each scene to its narration --------------------------
: > "$work/vlist.txt"
echo "file '$(realpath "$work/seg00.mp4")'" >> "$work/vlist.txt"
awavs=("$NARR/S0.wav")           # narration inputs, in order (S0 over the title card)
n=0
while read -r sid start end ndur; do
  n=$((n+1))
  seg="$work/seg$(printf '%02d' "$n").mp4"
  raw="$work/raw$(printf '%02d' "$n").mp4"
  segdur=$(python3 -c "print(f'{$end-$start:.3f}')")
  # Never slow-mo: play each scene at natural speed (factor 1), or SPEED IT UP if the footage
  # is longer than its narration (factor<1). When shorter, hold the last frame (tpad) to fill.
  factor=$(python3 -c "print(f'{min(1.0,$ndur/max(0.1,$end-$start)):.5f}')")
  pad=$(python3 -c "print(f'{max(0.3, $ndur-($end-$start)*min(1.0,$ndur/max(0.1,$end-$start))+0.5):.3f}')")
  # Stage A — cut [start, start+segdur] at natural speed. Fast+accurate seek (input-seek a few
  # seconds early, then a short output-seek): output-seek alone is empty on deep seeks (VFR),
  # input-seek alone is keyframe-coarse; combined they are both reliable AND frame-accurate.
  iss=$(python3 -c "print(f'{max(0.0,$start-4):.3f}')")
  oss=$(python3 -c "print(f'{$start-max(0.0,$start-4):.3f}')")
  ffmpeg -nostdin -y -ss "$iss" -i "$VIDEO" -ss "$oss" -t "$segdur" \
    -an -c:v libx264 -preset ultrafast -crf 20 -pix_fmt yuv420p "$raw" -loglevel error
  # Stage B — retime to exactly the narration length.
  ffmpeg -nostdin -y -i "$raw" \
    -vf "setpts=(PTS-STARTPTS)*${factor},fps=30,tpad=stop_mode=clone:stop_duration=${pad},format=yuv420p" \
    -an -r 30 -t "$ndur" "$seg" -loglevel error
  rm -f "$raw"
  echo "file '$(realpath "$seg")'" >> "$work/vlist.txt"
  awavs+=("$NARR/$sid.wav")
  printf "%s  seg %.1fs → %.1fs (×%.2f) = %.1fs\n" "$sid" "$segdur" "$ndur" "$factor" "$(dur "$seg")"
done < "$work/plan.txt"

# ---- 3. Concat video segments; build narration via the concat FILTER (robust) ----
ffmpeg -nostdin -y -f concat -safe 0 -i "$work/vlist.txt" -c copy "$work/silent.mp4" -loglevel error
ain=(); fc=""; i=0
for w in "${awavs[@]}"; do ain+=(-i "$w"); fc+="[${i}:a]"; i=$((i+1)); done
# Concat via the FILTER (each input decoded separately). NB: the concat *demuxer* must never be
# used on these WAVs — it byte-joins their 44-byte headers and decodes them as audio = white
# noise. Piper renders near 0 dBFS, so pull the level down before a safety limiter to avoid the
# AAC encoder clipping (which sounds like harsh noise).
ffmpeg -nostdin -y "${ain[@]}" \
  -filter_complex "${fc}concat=n=${i}:v=0:a=1,aresample=44100,volume=-4dB,alimiter=limit=0.9[a]" \
  -map "[a]" -ac 2 "$work/audio.wav" -loglevel error
mkdir -p "$(dirname "$OUT")"
ffmpeg -y -i "$work/silent.mp4" -i "$work/audio.wav" \
  -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p -c:a aac -b:a 160k -shortest "$OUT" -loglevel error
echo "→ $OUT ($(dur "$OUT")s)  [video $(dur "$work/silent.mp4")s · audio $(dur "$work/audio.wav")s]"
