#!/usr/bin/env python3
"""Resolve an episode's edit (video/episodes/epNN.json) against its real takes and narration into the
frame-exact plan Remotion renders (docs/video/README.md §4.3), and stage the assets Remotion reads.

    python3 video/build_edit.py 03

The narration is the clock: a scene lasts LEAD + its narration + TAIL (or a card's fixed length). Footage
plays from its start mark; when it is longer than the voice it is sped up, but never past MAX_RATE (then
cut at the end of the scene), and when it is shorter its last frame is held — the voice never waits for the
picture and the picture never drags behind the voice.
"""
import json
import shutil
import subprocess
import sys
from pathlib import Path

FPS = 30
LEAD, TAIL = 0.6, 1.2
MAX_RATE = 1.6
ROOT = Path(__file__).resolve().parent.parent


def probe(path):
    return float(subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)],
                                capture_output=True, text=True, check=True).stdout)


def main():
    ep = sys.argv[1].zfill(2)
    spec = json.loads((ROOT / f"video/episodes/ep{ep}.json").read_text())
    out = ROOT / f"video/out/ep{ep}"
    takes_dir = ROOT / "video/out/takes"
    music = ROOT / "video/out/music"
    timings = json.loads((out / "audio/timings.json").read_text())
    lines = {l["id"]: l for l in timings["lines"]}
    pub = ROOT / f"video/remotion/public/ep{ep}"
    if pub.exists():
        shutil.rmtree(pub)
    pub.mkdir(parents=True)

    def stage(src: Path, name: str):
        dst = pub / name
        shutil.copyfile(src, dst)
        return f"ep{ep}/{name}"

    scenes, t = [], 0.0
    for sc in spec["scenes"]:
        r = {"id": sc["id"], "start": round(t, 3)}
        narr = lines.get(sc.get("narration", ""))
        if narr:
            r["narration"] = {"src": stage(out / "audio" / narr["file"], f"{sc['id']}.wav"),
                              "at": LEAD if "take" in sc else 0.8, "duration": narr["duration"],
                              "segments": narr["segments"]}
        if "card" in sc:
            r["card"] = sc["card"]
            dur = max(sc.get("seconds", 5), (narr["duration"] + 1.6) if narr else 0)
        else:
            take = json.loads((takes_dir / f"{sc['take']}.marks.json").read_text())
            marks = {m["label"]: m["t"] for m in take["marks"]}
            video = takes_dir / f"{sc['take']}.mp4"
            vdur = probe(video)
            a = max(0.0, marks[sc["from"]] - 0.3)
            b = marks.get(sc.get("to", ""), vdur)
            clip = b - a
            dur = LEAD + (narr["duration"] if narr else 0) + TAIL
            rate = min(MAX_RATE, max(1.0, clip / dur))
            src_name = f"{sc['take']}.mp4"
            if not (pub / src_name).exists():
                stage(video, src_name)
            r["take"] = {"src": f"ep{ep}/{src_name}", "from": round(a, 3), "to": round(b, 3), "rate": round(rate, 3),
                         "device": sc.get("device", "browser")}
            # marks relative to the scene (seconds of scene time), so overlays can hang on beats
            r["marks"] = {k: round((v - a) / rate, 3) for k, v in marks.items() if a <= v <= b}
            for key in ("lower", "zoom", "callouts"):
                if key in sc:
                    r[key] = sc[key]
        r["duration"] = round(dur, 3)
        scenes.append(r)
        t += dur

    total = t
    for name in ("sting", "bed", "outro"):
        stage(music / f"{name}.wav", f"{name}.wav")
    end_start = next(s["start"] for s in scenes if s.get("card") == "end")
    plan = {
        "id": ep, "title": spec["title"], "series": spec["series"], "arc": spec["arc"], "next": spec["next"],
        "fps": FPS, "durationInFrames": int(round(total * FPS)), "scenes": scenes,
        "music": {"sting": f"ep{ep}/sting.wav", "bed": f"ep{ep}/bed.wav", "outro": f"ep{ep}/outro.wav",
                  "bedFrom": scenes[1]["start"], "bedTo": end_start, "outroAt": end_start},
    }
    (pub / "edit.json").write_text(json.dumps(plan, indent=1))
    # captions from the known text (never speech recognition, §3.1) — written beside the render
    cues = []
    for s in scenes:
        n = s.get("narration")
        if not n:
            continue
        for seg in n["segments"]:
            cues.append((s["start"] + n["at"] + seg["start"], s["start"] + n["at"] + seg["end"], seg["text"]))
    def ts(x, sep=","):
        h, m = int(x // 3600), int(x % 3600 // 60)
        return f"{h:02d}:{m:02d}:{x % 60:06.3f}".replace(".", sep)
    (out / f"ep{ep}.srt").write_text("".join(f"{i + 1}\n{ts(a)} --> {ts(b)}\n{txt}\n\n" for i, (a, b, txt) in enumerate(cues)))
    (out / f"ep{ep}.vtt").write_text("WEBVTT\n\n" + "".join(f"{ts(a, '.')} --> {ts(b, '.')}\n{txt}\n\n" for a, b, txt in cues))
    print(f"E{ep}: {len(scenes)} scenes, {total:.1f} s ({plan['durationInFrames']} frames); "
          + ", ".join(f"{s['id']} {s['duration']:.1f}s" + (f" ×{s['take']['rate']}" if 'take' in s else "") for s in scenes))


if __name__ == "__main__":
    main()
