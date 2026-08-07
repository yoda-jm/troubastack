#!/usr/bin/env python3
"""
DEMO-VID Part D — narration synthesis.

Parses docs/video/script.md, synthesizes one WAV per scene with Piper TTS, and writes a
timings manifest (scene -> seconds). The walkthrough (Part B) reads the WEB-scene timings so
each on-screen beat matches its narration; the assembler (assemble.sh) muxes the WAVs onto the
video. WAVs/manifest are generated artifacts (git-ignored) — this script is the reproducible
source.

Usage (from repo root, with the Piper venv active + a voice model):
  python3 docs/video/tools/synth.py \
    --piper <path/to/piper> --voice <path/to/voice.onnx> \
    --script docs/video/script.md --out docs/video/output/narration
Outputs: <out>/S<n>.wav for every scene + <out>/timings.json  ({ "S0": 7.9, ... }).
"""
import argparse, json, os, re, subprocess, sys, wave

SCENE_RE = re.compile(r"^\*\*S(\d+)\b.*?\*\*")  # **S0 — Title card** · ~8s


def parse_scenes(script_path):
    """Return [(id, text)] in order. Narration = the quoted paragraph under each **S<n>** head."""
    scenes, cur, buf = [], None, []
    for raw in open(script_path, encoding="utf-8"):
        line = raw.rstrip("\n")
        m = SCENE_RE.match(line.strip())
        if m:
            if cur is not None:
                scenes.append((cur, " ".join(buf).strip()))
            cur, buf = f"S{m.group(1)}", []
            continue
        if cur is None:
            continue
        if line.strip().startswith(("##", "**S", ">")):  # next section/scene/aside ends it
            if line.strip().startswith("**S"):
                m2 = SCENE_RE.match(line.strip())
                if m2:
                    scenes.append((cur, " ".join(buf).strip()))
                    cur, buf = f"S{m2.group(1)}", []
                    continue
            scenes.append((cur, " ".join(buf).strip()))
            cur, buf = None, []
            continue
        if line.strip():
            buf.append(line.strip())
    if cur is not None:
        scenes.append((cur, " ".join(buf).strip()))
    # strip surrounding smart/straight quotes from the narration
    out = []
    for sid, text in scenes:
        text = text.strip().strip('"“”').strip()
        if text:
            out.append((sid, text))
    return out


def wav_seconds(path):
    with wave.open(path, "rb") as w:
        return round(w.getnframes() / float(w.getframerate()), 3)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--piper", default="piper")
    ap.add_argument("--voice", required=True)
    ap.add_argument("--script", default="docs/video/script.md")
    ap.add_argument("--out", default="docs/video/output/narration")
    a = ap.parse_args()
    os.makedirs(a.out, exist_ok=True)
    scenes = parse_scenes(a.script)
    if not scenes:
        print("no scenes parsed", file=sys.stderr)
        sys.exit(1)
    timings = {}
    for sid, text in scenes:
        wav = os.path.join(a.out, f"{sid}.wav")
        subprocess.run([a.piper, "-m", a.voice, "-f", wav], input=text.encode("utf-8"),
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        timings[sid] = wav_seconds(wav)
        print(f"{sid}: {timings[sid]:>5}s  {text[:60]}…")
    json.dump(timings, open(os.path.join(a.out, "timings.json"), "w"), indent=2)
    total = round(sum(timings.values()), 1)
    print(f"\n{len(timings)} scenes, {total}s total narration → {a.out}/timings.json")


if __name__ == "__main__":
    main()
