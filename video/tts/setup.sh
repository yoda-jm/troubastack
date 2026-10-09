#!/usr/bin/env bash
# One-time setup for the narration (docs/video/README.md §4.4, D1): a private venv with kokoro-onnx and the
# Kokoro v1.0 model files, under ~/.cache/troubastack-video (outside the repo — ~350 MB). Needs the system
# espeak-ng (the phonemiser). The torch build of Kokoro is avoided on purpose: its spaCy dependency does not
# build on Python 3.13.
set -euo pipefail
CACHE="${TROUBA_VIDEO_CACHE:-$HOME/.cache/troubastack-video}"
mkdir -p "$CACHE/kokoro"
command -v espeak-ng >/dev/null || { echo "error: espeak-ng is required" >&2; exit 1; }
[ -x "$CACHE/venv/bin/python" ] || python3 -m venv "$CACHE/venv"
"$CACHE/venv/bin/pip" install -q --upgrade pip
"$CACHE/venv/bin/pip" install -q kokoro-onnx soundfile
for f in kokoro-v1.0.onnx voices-v1.0.bin; do
  [ -s "$CACHE/kokoro/$f" ] || curl -fsSL -o "$CACHE/kokoro/$f" \
    "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/$f"
done
"$CACHE/venv/bin/python" -c "import kokoro_onnx; print('kokoro-onnx ready in $CACHE')"
