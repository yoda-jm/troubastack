#!/usr/bin/env bash
# Render the series music (docs/video/README.md §3.4): theme.py → MIDI → fluidsynth (FluidR3_GM, MIT) → WAV.
# The beds: the ringing tail past the loop point is folded onto the head, then cut at exactly 32 bars, so
# they loop without a seam. Levels set by RMS (−18.3 dB; bed-light −21) with a −1 dB peak ceiling — the
# recipe of VLL's other project. Output (gitignored): video/out/music/{sting,bed,bed-light,outro}.wav
#
#   video/music/render.sh [OUTDIR]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-$HERE/../out/music}"
SF2="${SF2:-/usr/share/sounds/sf2/FluidR3_GM.sf2}"
[ -s "$SF2" ] || { echo "error: SoundFont not found: $SF2 (set SF2=)" >&2; exit 1; }
mkdir -p "$OUT"
python3 "$HERE/theme.py" "$OUT"

for cue in sting bed bed-light outro; do
  fluidsynth -ni -q -g 0.6 -r 48000 -F "$OUT/$cue.raw.wav" "$SF2" "$OUT/$cue.mid" 2>/dev/null
  test -s "$OUT/$cue.raw.wav"
done

python3 - "$OUT" <<'PY'
import sys, wave, numpy as np
out = sys.argv[1]
BPM, BARS, RATE = 72, 32, 48000
def read(p):
    with wave.open(p) as w:
        return np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16).astype(float).reshape(-1, w.getnchannels())
def level(y, rms_db):
    scale = 0.89 * 32767 / (np.abs(y).max() or 1)                                   # −1 dB peak ceiling
    scale = min(scale, 10 ** (rms_db / 20) * 32767 / (np.sqrt(np.mean(y ** 2)) or 1))
    return y * scale
def write(p, y):
    with wave.open(p, "wb") as w:
        w.setnchannels(y.shape[1]); w.setsampwidth(2); w.setframerate(RATE)
        w.writeframes(np.clip(y, -32767, 32767).round().astype(np.int16).tobytes())
for cue, rms in (("bed", -18.3), ("bed-light", -21.0)):
    x = read(f"{out}/{cue}.raw.wav")
    n = int(round(BARS * 4 * 60 / BPM * RATE))
    if len(x) < n: x = np.vstack([x, np.zeros((n - len(x), x.shape[1]))])
    y = x[:n].copy(); tail = x[n:n + RATE * 4]; y[:len(tail)] += tail                # fold the ring onto the head
    y = level(y, rms)
    seam = np.abs(y[0] - y[-1]).max(); step = np.abs(np.diff(y, axis=0)).max()
    print(f"  {cue}.wav: {n / RATE:.1f} s loop, seam jump {seam:.0f} vs max step {step:.0f} -> {'OK' if seam <= step else 'CLICK'}")
    write(f"{out}/{cue}.wav", y)
for cue, secs in (("sting", 7.0), ("outro", 16.0)):
    x = read(f"{out}/{cue}.raw.wav")[: int(secs * RATE)]
    f = int(2.0 * RATE); x[-f:] *= np.linspace(1, 0, f)[:, None]                      # gentle fade-out
    write(f"{out}/{cue}.wav", level(x, -18.3))
PY
rm -f "$OUT"/*.raw.wav
for cue in sting bed bed-light outro; do
  printf '  %-14s %6.1f s  ' "$cue.wav" "$(soxi -D "$OUT/$cue.wav")"
  ffmpeg -hide_banner -nostats -i "$OUT/$cue.wav" -af ebur128 -f null - 2>&1 | /usr/bin/grep -A2 "Integrated loudness" | /usr/bin/grep -oE "I: +-?[0-9.]+ LUFS"
done
