#!/usr/bin/env python3
"""The series music, as code (docs/video/README.md §3.4) — dreamy, major, atmospheric.

Third take. VLL rejected a piano/guitar General-MIDI draft ("too cheesy, we hear the piano") and then a
synthesised ambient one ("feels minor and not nice"), pointing at how his other project scores its quiet
scenes. This follows that recipe: MIDI written in code, rendered through FluidR3_GM, but only SOFT colours —
a warm pad, slow harp rolls, celesta and music-box "stars", an upright bass — with major-seventh voicings
(root in the bass, colour tones above) and generous reverb sends. No piano, no drums, no tune to hum: the
motif appears only as a few sparse celesta notes.

    python3 video/music/theme.py OUTDIR      # → OUTDIR/{sting,bed,bed-light,outro}.mid

G major, 72 bpm, 4/4. The bed is 32 bars (≈ 107 s) and loops seamlessly (render.sh folds the tail).
"""
import struct
import sys
from pathlib import Path

BPM = 72
PPQ = 480
BEAT = PPQ
BAR = 4 * PPQ

# (channel, GM program, volume CC7, pan CC10, reverb send CC91)
PARTS = {
    "pad": (0, 89, 74, 64, 85),      # Pad 2 (warm)
    "harp": (1, 46, 82, 42, 70),     # Orchestral harp
    "celesta": (2, 8, 92, 78, 80),   # Celesta — the "stars"
    "musicbox": (3, 10, 80, 54, 85), # Music box — intro/sting sparkle
    "bass": (4, 32, 96, 60, 35),     # Acoustic (upright) bass
    "strings": (5, 49, 60, 72, 80),  # Slow strings, very low, for warmth in the B section
}

# (bass root, voicing) — rootless sevenths around middle C, as in the reference recipe.
Gmaj7 = (43, [59, 62, 66, 69])
Cmaj7 = (48, [59, 62, 64, 67])
Em7 = (40, [55, 59, 62, 66])
Am7 = (45, [60, 64, 67, 71])
Bm7 = (47, [57, 62, 66, 69])
D6 = (38, [54, 59, 62, 66])
Dsus = (38, [55, 57, 62, 67])
Gadd9 = (43, [57, 59, 62, 67])

A = [Gmaj7, Cmaj7, Em7, Dsus, Gmaj7, Cmaj7, Am7, D6]
B = [Cmaj7, Bm7, Am7, Gmaj7, Cmaj7, D6, Em7, Dsus]
BED = A + B + A + [Cmaj7, Bm7, Am7, Gadd9, Cmaj7, D6, Cmaj7, Dsus]  # 32 bars, the last turns back to bar 1

# Sparse celesta fragments (beat offset in the bar, MIDI note, length in beats), placed every 4 bars.
STARS = [
    [(0, 78, 1.5), (1.5, 81, 1.5), (3, 79, 3)],   # F#5 A5 G5
    [(1, 74, 1), (2, 76, 1), (3, 71, 3)],         # D5 E5 B4
    [(0.5, 83, 2), (2.5, 81, 3)],                 # B5 A5
    [(0, 79, 2), (2, 78, 1), (3, 74, 4)],         # G5 F#5 D5
]


class Track:
    def __init__(self):
        self.events = []

    def note(self, part, pitch, start, beats, vel):
        ch = PARTS[part][0]
        dur = max(1, int(beats * BEAT) - 10)
        self.events.append((int(start), 1, bytes([0x90 | ch, pitch, max(1, min(127, vel))])))
        self.events.append((int(start) + dur, 0, bytes([0x80 | ch, pitch, 0])))

    def raw(self, tick, data):
        self.events.append((int(tick), -1, data))

    def encode(self):
        out, last = bytearray(), 0
        for tick, _, data in sorted(self.events, key=lambda e: (e[0], e[1])):
            out += varlen(tick - last) + data
            last = tick
        return bytes(out + varlen(0) + b"\xff\x2f\x00")


def varlen(n):
    buf = [n & 0x7F]
    while n > 0x7F:
        n >>= 7
        buf.insert(0, (n & 0x7F) | 0x80)
    return bytes(buf)


def setup(t, parts):
    t.raw(0, b"\xff\x51\x03" + (60_000_000 // BPM).to_bytes(3, "big"))
    for p in parts:
        ch, prog, vol, pan, rev = PARTS[p]
        t.raw(0, bytes([0xC0 | ch, prog]))
        for cc, v in ((7, vol), (10, pan), (91, rev), (93, 20)):
            t.raw(0, bytes([0xB0 | ch, cc, v]))


def pad(t, chord, at, bars=1, vel=54):
    for n in chord[1]:
        t.note("pad", n, at, 4 * bars, vel)


def harp_roll(t, chord, at, vel=48):
    """Slow broken chord: one note per beat, each ringing two beats, an octave up on the way."""
    root, v = chord
    seq = [root + 12, v[0], v[1], v[2] + 12 if len(v) > 2 else v[1] + 12]
    for i, n in enumerate(seq):
        t.note("harp", n, at + i * BEAT, 2, vel - (6 if i % 2 else 0))


def bass(t, chord, at, vel=58):
    root = chord[0]
    t.note("bass", root, at, 2, vel)
    t.note("bass", root + 7, at + 2 * BEAT, 2, vel - 14)


def stars(t, at, k, vel=46):
    for off, n, beats in STARS[k % len(STARS)]:
        t.note("celesta", n, at + off * BEAT, beats, vel)


def bed(light=False):
    t = Track()
    parts = ["pad", "harp"] + ([] if light else ["celesta", "bass", "strings"])
    setup(t, parts)
    for i, ch in enumerate(BED):
        at = i * BAR
        pad(t, ch, at, vel=50 if light else 54)
        harp_roll(t, ch, at, vel=40 if light else 48)
        if light:
            continue
        bass(t, ch, at)
        if 8 <= i < 16:  # the B section warms with a whisper of strings
            for n in ch[1][1:]:
                t.note("strings", n, at, 4, 36)
        if i % 4 == 1:
            stars(t, at, i // 4)
    t.raw(len(BED) * BAR + 2 * BAR, bytes([0xB0, 1, 0]))  # ring on past the loop; render.sh folds it back
    return t


def sting():
    t = Track()
    setup(t, ["pad", "harp", "celesta", "musicbox", "bass"])
    pad(t, Gmaj7, 0, bars=2, vel=58)
    t.note("bass", 43, 0, 6, 52)
    for i, n in enumerate([55, 59, 62, 66, 69, 74]):          # a harp sweep up through Gmaj9
        t.note("harp", n, i * BEAT // 3, 4, 52 + i * 2)
    for i, n in enumerate([86, 83, 79]):                      # music-box sparkle, D6 B5 G5
        t.note("musicbox", n, BEAT * 2 + i * BEAT // 2, 3, 50 - i * 4)
    t.note("celesta", 78, BEAT * 3, 4, 44)                    # a single high F#5 left hanging
    return t


def outro():
    t = Track()
    setup(t, ["pad", "harp", "celesta", "bass", "musicbox"])
    for i, ch in enumerate([Cmaj7, D6, Gmaj7]):
        pad(t, ch, i * BAR, bars=1 if i < 2 else 3, vel=54)
        harp_roll(t, ch, i * BAR, vel=46)
        t.note("bass", ch[0], i * BAR, 4 if i < 2 else 10, 56)
    stars(t, 0, 3, vel=44)                                     # G5 F#5 D5 …
    t.note("celesta", 79, 2 * BAR + 2 * BEAT, 6, 42)           # … and home on G5
    for i, n in enumerate([91, 86, 83, 79]):                   # music box closing the lid
        t.note("musicbox", n, 3 * BAR + i * BEAT, 3, 40 - i * 3)
    return t


def write(path: Path, track: Track):
    body = track.encode()
    path.write_bytes(b"MThd" + struct.pack(">IHHH", 6, 0, 1, PPQ) + b"MTrk" + struct.pack(">I", len(body)) + body)


def main():
    out = Path(sys.argv[1] if len(sys.argv) > 1 else "video/out/music")
    out.mkdir(parents=True, exist_ok=True)
    write(out / "sting.mid", sting())
    write(out / "bed.mid", bed())
    write(out / "bed-light.mid", bed(light=True))
    write(out / "outro.mid", outro())
    print(f"wrote 4 cues to {out} (G major, {BPM} bpm, bed = {len(BED)} bars = {len(BED) * 240 / BPM:.1f} s)")


if __name__ == "__main__":
    main()
