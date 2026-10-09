#!/usr/bin/env python3
"""Narration for one episode (docs/video/README.md §4.4): the spec's narration lines → one WAV per line +
timings.json. The narrator is Kokoro `am_eric` (D1). Run with the venv from setup.sh:

    ~/.cache/troubastack-video/venv/bin/python video/tts/synth.py 03 [OUTDIR]

- The text comes from the spec itself (`### E03 —` section, `> **3.N** …` lines): one source of truth.
- `[beat]` / `[pause]` become 0.35 s / 0.8 s of silence; their positions are kept in timings.json, so the
  edit and the captions can cut on them.
- Each line is cached by hash(text, voice, speed, model): editing one sentence re-voices one line.
- lexicon.tsv (word → spoken form) fixes pronunciations before synthesis; captions keep the written form.
"""
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

import numpy as np
import soundfile as sf
from kokoro_onnx import EspeakConfig, Kokoro

VOICE, SPEED, LANG = "am_eric", 0.8, "en-us"
RATE = 24000
GAP = {"beat": 0.35, "pause": 0.8}
HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
CACHE = Path(os.environ.get("TROUBA_VIDEO_CACHE", Path.home() / ".cache/troubastack-video"))


def narration(ep: str) -> dict:
    spec = (ROOT / "docs/video/README.md").read_text()
    start = spec.index(f"\n### E{ep} — ")
    end = spec.find("\n### E", start + 10)
    sec = spec[start:end if end > 0 else None]
    sec = sec[sec.index("**Narration**"):]
    lines, cur = {}, None
    for raw in sec.splitlines():
        if not raw.startswith(">"):
            cur = None if raw.strip() else cur
            continue
        body = raw[1:].strip()
        m = re.match(r"\*\*(\d+\.\d+)\*\*\s*(.*)", body)
        if m:
            cur = m.group(1)
            lines[cur] = m.group(2)
        elif cur and body and not body.startswith("*("):
            lines[cur] += " " + body
    return lines


def lexicon() -> list:
    p = HERE / "lexicon.tsv"
    out = []
    if p.exists():
        for row in p.read_text().splitlines():
            if row.strip() and not row.startswith("#"):
                word, spoken = row.split("\t", 1)
                out.append((re.compile(rf"(?<![\w-]){re.escape(word)}(?![\w-])"), spoken.strip()))
    return out


def spoken(text: str, lex) -> str:
    text = text.replace("“", '"').replace("”", '"').replace("’", "'").replace("—", ", ")
    for rx, say in lex:
        text = rx.sub(say, text)
    return text


def main():
    ep = sys.argv[1].zfill(2)
    out = Path(sys.argv[2] if len(sys.argv) > 2 else ROOT / f"video/out/ep{ep}/audio")
    out.mkdir(parents=True, exist_ok=True)
    k = Kokoro(str(CACHE / "kokoro/kokoro-v1.0.onnx"), str(CACHE / "kokoro/voices-v1.0.bin"),
               espeak_config=EspeakConfig(lib_path=os.environ.get("ESPEAK_LIB", "/usr/lib64/libespeak-ng.so.1"),
                                          data_path=os.environ.get("ESPEAK_DATA", "/usr/share/espeak-ng-data")))
    lex = lexicon()
    timings = {"episode": ep, "voice": VOICE, "speed": SPEED, "rate": RATE, "lines": []}
    for lid, text in narration(ep).items():
        parts = re.split(r"\[(beat|pause)\]", text)
        key = hashlib.sha256(json.dumps([text, VOICE, SPEED, "kokoro-v1.0", [(r.pattern, s) for r, s in lex]]).encode()).hexdigest()[:16]
        wav = out / f"{lid}.wav"
        segs, audio, t = [], [], 0.0
        cached = (out / f"{lid}.json").exists() and json.loads((out / f"{lid}.json").read_text()).get("key") == key and wav.exists()
        if cached:
            segs = json.loads((out / f"{lid}.json").read_text())["segments"]
        else:
            for p in parts:
                if p in GAP:
                    audio.append(np.zeros(int(GAP[p] * RATE), dtype=np.float32)); t += GAP[p]; continue
                p = p.strip()
                if not p:
                    continue
                a, _ = k.create(spoken(p, lex), voice=VOICE, speed=SPEED, lang=LANG)
                segs.append({"text": p, "start": round(t, 3), "end": round(t + len(a) / RATE, 3)})
                audio.append(a); t += len(a) / RATE
            sf.write(wav, np.concatenate(audio), RATE)
            (out / f"{lid}.json").write_text(json.dumps({"key": key, "segments": segs}))
        dur = sf.info(wav).duration
        timings["lines"].append({"id": lid, "file": wav.name, "duration": round(dur, 3), "segments": segs})
        print(f"  {lid}: {dur:5.1f}s {'(cached)' if cached else ''}")
    (out / "timings.json").write_text(json.dumps(timings, indent=1))
    total = sum(l["duration"] for l in timings["lines"])
    print(f"E{ep}: {len(timings['lines'])} lines, {total:.1f} s of narration → {out}")


if __name__ == "__main__":
    main()
