#!/usr/bin/env python3
"""Resolve an episode's edit (video/episodes/epNN.json) against its real takes and narration into the
frame-exact plan Remotion renders (docs/video/README.md §4.3), and stage the assets Remotion reads.

    python3 video/build_edit.py 03

SYNC ENGINE. A narration line is a list of sentences (its segments). A scene may ANCHOR a sentence to a beat
of its take (`"sync": [{"seg": 2, "mark": "tap:Import", "offset": -0.4}]`): the footage plays at normal speed
up to that beat, and if the voice is still busy with the previous sentence the footage HOLDS on that frame
until it is free — so the action happens as the narrator names it, never before. Unanchored sentences follow
the previous one. Stretches with no voice over them are sped up (≤ MAX_RATE) so silence never drags; an
optional `skip` drops a dead stretch (an install spinner) outright.
"""
import json
import shutil
import subprocess
import sys
from pathlib import Path

FPS = 30
LEAD, TAIL, GAP = 0.6, 1.2, 0.25
MAX_RATE = 1.6
SILENT_TARGET = 2.0   # a voiceless stretch is compressed toward this length (never past MAX_RATE)
ROOT = Path(__file__).resolve().parent.parent


def probe(path):
    return float(subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)],
                                capture_output=True, text=True, check=True).stdout)


class Timeline:
    """Output-ordered footage steps: play (src_from → src_to at a rate) and hold (a frame for N seconds)."""

    def __init__(self, skip):
        self.items, self.t, self.skip = [], 0.0, skip  # skip: (src_a, src_b) or None
        self.src_to_out = []  # (src_from, src_to, out_from, rate) — for mapping marks to output time

    def play(self, a, b, rate):
        ranges = [(a, b)]
        if self.skip and a < self.skip[1] and b > self.skip[0]:
            s0, s1 = self.skip
            ranges = [(p, q) for p, q in ((a, min(b, s0)), (max(a, s1), b)) if q - p > 1e-3]
        for p, q in ranges:
            self.items.append({"kind": "play", "from": round(p, 3), "to": round(q, 3), "rate": round(rate, 3)})
            self.src_to_out.append((p, q, self.t, rate))
            self.t += (q - p) / rate

    def hold(self, at, seconds):
        if seconds > 1e-3:
            self.items.append({"kind": "hold", "at": round(at, 3), "seconds": round(seconds, 3)})
            self.t += seconds

    def out_time(self, src):
        for p, q, o, r in self.src_to_out:
            if p - 1e-3 <= src <= q + 1e-3:
                return round(o + (src - p) / r, 3)
        return None

    def kept(self, a, b):
        """seconds of source between a and b that survive the skip"""
        if not self.skip:
            return b - a
        s0, s1 = self.skip
        return (b - a) - max(0.0, min(b, s1) - max(a, s0))


def rate_for(chunk, spoken_over):
    """1× while a voice talks over the footage; a voiceless stretch is compressed (≤ MAX_RATE)."""
    if chunk - spoken_over < 1.5:
        return 1.0
    return min(MAX_RATE, max(1.0, chunk / (spoken_over + SILENT_TARGET)))


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
        if not (pub / name).exists():
            shutil.copyfile(src, pub / name)
        return f"ep{ep}/{name}"

    scenes, t = [], 0.0
    for sc in spec["scenes"]:
        r = {"id": sc["id"], "start": round(t, 3)}
        narr = lines.get(sc.get("narration", ""))
        voice = []  # one entry per placed sentence: {src, from, to, at, text} — `at` in scene seconds
        segs_out = []
        if "card" in sc:
            r["card"] = sc["card"]
            if narr:
                src = stage(out / "audio" / narr["file"], f"{sc['id']}.wav")
                for sg in narr["segments"]:
                    at = 0.8 + sg["start"]
                    voice.append({"src": src, "from": sg["start"], "to": sg["end"], "at": round(at, 3), "text": sg["text"]})
                    segs_out.append([round(at, 3), round(at + sg["end"] - sg["start"], 3)])
            dur = max(sc.get("seconds", 5), (narr["duration"] + 1.6) if narr else 0)
        else:
            take = json.loads((takes_dir / f"{sc['take']}.marks.json").read_text())
            marks = {m["label"]: m["t"] for m in take["marks"]}
            video = takes_dir / f"{sc['take']}.mp4"
            vdur = probe(video)
            a = max(0.0, marks[sc["from"]] - 0.3)
            b = min(vdur, marks.get(sc.get("to", ""), vdur))
            skip = None
            if "skip" in sc:
                s0, s1 = marks[sc["skip"][0]] + 0.8, marks[sc["skip"][1]] - 0.6
                skip = (s0, s1) if a < s0 < s1 < b else None
            tl = Timeline(skip)
            segs = narr["segments"] if narr else []
            anchors = {x["seg"]: max(a, min(b, marks[x["mark"]] + x.get("offset", -0.3))) for x in sc.get("sync", [])}
            src_line = stage(out / "audio" / narr["file"], f"{sc['id']}.wav") if narr else None
            cursor, voice_free = a, LEAD
            for i, sg in enumerate(segs):
                anchored = i in anchors and anchors[i] >= cursor
                if anchored and anchors[i] > cursor:
                    chunk = tl.kept(cursor, anchors[i])
                    tl.play(cursor, anchors[i], rate_for(chunk, max(0.0, voice_free - tl.t)))
                    cursor = anchors[i]
                start = max(voice_free + (GAP if i else 0), tl.t) if anchored else voice_free + (GAP if i else 0)
                if anchored and start > tl.t:
                    tl.hold(cursor, start - tl.t)  # the action waits for the narrator
                length = sg["end"] - sg["start"]
                voice.append({"src": src_line, "from": sg["start"], "to": sg["end"], "at": round(start, 3), "text": sg["text"]})
                segs_out.append([round(start, 3), round(start + length, 3)])
                voice_free = start + length
            if b > cursor:  # the rest of the take
                chunk = tl.kept(cursor, b)
                tl.play(cursor, b, rate_for(chunk, max(0.0, voice_free - tl.t)))
            end = max(tl.t, voice_free + TAIL)
            tl.hold(b, end - tl.t)  # hold the last frame for the tail
            dur = end
            r["take"] = {"src": stage(video, f"{sc['take']}.mp4"), "timeline": tl.items, "device": sc.get("device", "browser")}
            r["marks"] = {k: tl.out_time(v) for k, v in marks.items() if tl.out_time(v) is not None}
            for key in ("lower", "zoom", "callouts"):
                if key in sc:
                    r[key] = sc[key]
        r["voice"] = voice
        r["segs"] = segs_out
        r["duration"] = round(dur, 3)
        scenes.append(r)
        t += dur

    total = t
    for name in ("sting", "bed", "outro"):
        stage(music / f"{name}.wav", f"{name}.wav")
    end_start = next(s["start"] for s in scenes if s.get("card") == "end")

    # captions from the known text (never speech recognition, §3.1): plain characters every player decodes,
    # lines of at most 42 characters, at most two lines per cue, each sentence's time shared out by length
    def plain(txt):
        for k, v in {"“": '"', "”": '"', "‘": "'", "’": "'", "«": '"', "»": '"', "—": " - ", "–": " - ", "…": "...", " ": " "}.items():
            txt = txt.replace(k, v)
        return " ".join(txt.split())

    def wrap(txt, width=42):
        out_l, cur = [], ""
        for w in txt.split():
            if cur and len(cur) + 1 + len(w) > width:
                out_l.append(cur); cur = w
            else:
                cur = f"{cur} {w}".strip()
        return out_l + ([cur] if cur else [])

    cues = []
    for s in scenes:
        for v, (a0, a1) in zip(s["voice"], s["segs"]):
            lns = wrap(plain(v["text"]))
            chunks = ["\n".join(lns[i:i + 2]) for i in range(0, len(lns), 2)]
            chars = sum(len(c) for c in chunks) or 1
            tt = s["start"] + a0
            for c in chunks:
                d = (a1 - a0) * len(c) / chars
                cues.append((round(tt, 3), round(tt + d - 0.04, 3), c))
                tt += d

    plan = {
        "id": ep, "title": spec["title"], "series": spec["series"], "arc": spec["arc"], "next": spec["next"],
        "fps": FPS, "durationInFrames": int(round(total * FPS)), "scenes": scenes,
        "cues": [{"from": a, "to": b, "text": c} for a, b, c in cues],
        "music": {"sting": f"ep{ep}/sting.wav", "bed": f"ep{ep}/bed.wav", "outro": f"ep{ep}/outro.wav",
                  "bedFrom": scenes[1]["start"], "bedTo": end_start, "outroAt": end_start},
    }
    (pub / "edit.json").write_text(json.dumps(plan, indent=1))

    def ts(x, sep=","):
        h, m = int(x // 3600), int(x % 3600 // 60)
        return f"{h:02d}:{m:02d}:{x % 60:06.3f}".replace(".", sep)
    # NOT beside the video: players (VLC, mplayer) auto-load a same-named .srt and would show the captions
    # twice over the burned-in ones. These files are for YouTube's caption track (uploaded with a clean render).
    cap = out / "captions"
    cap.mkdir(exist_ok=True)
    for stale in (out / f"ep{ep}.srt", out / f"ep{ep}.vtt"):
        stale.unlink(missing_ok=True)
    (cap / f"ep{ep}.srt").write_text("﻿" + "".join(f"{i + 1}\n{ts(a)} --> {ts(b)}\n{txt}\n\n" for i, (a, b, txt) in enumerate(cues)), encoding="utf-8")
    (cap / f"ep{ep}.vtt").write_text("WEBVTT\n\n" + "".join(f"{ts(a, '.')} --> {ts(b, '.')}\n{txt}\n\n" for a, b, txt in cues), encoding="utf-8")
    print(f"E{ep}: {len(scenes)} scenes, {total:.1f} s ({plan['durationInFrames']} frames), {len(cues)} caption cues; "
          + ", ".join(f"{s['id']} {s['duration']:.1f}s" + (f" [{sum(1 for i in s['take']['timeline'] if i['kind'] == 'hold')} holds]" if 'take' in s else "") for s in scenes))


if __name__ == "__main__":
    main()
