#!/usr/bin/env python3
"""Generate the family sheet as a single SVG, from the same bricks as the assets.

The point of generating it: the sheet cannot drift from what actually ships.
Every icon on it is the real artwork, and the size-ladder cells are drawn at
their true pixel size — so rendering this sheet 1:1 renders a genuine 16px icon.
The magnified proof strip embeds real rasterised PNGs at image-rendering:pixelated,
so it shows what the pixels actually do rather than a drawing of what they might.

    python3 docs/brand/sheet.py     # -> dist/family-sheet.svg
"""
import base64
import subprocess
import tempfile
from pathlib import Path

import build as B

W, H = 1680, 1372
INK, MUTED, RULE, PAPER = "#141A1F", "#6B7580", "#DDE2E7", "#FCFCFD"
F = "Inter, 'Helvetica Neue', Helvetica, Arial, sans-serif"
NAMES = {"troubastack": "Stack", "troubastudio": "Studio",
         "troubastage": "Stage", "troubacore": "Core"}
# Derived from build.py, never typed here: a hard-coded copy silently went stale
# once already, on the one panel that claims the sheet cannot drift.
# Derived, never typed. A hard-coded copy went stale twice: once silently, and
# once naming three layer colours that no longer existed in the gradients at
# all. Each mark reuses its layer's colour exactly, so the palette holds one
# yellow, one pink and one blue rather than four near-identical yellows.
TAKES = {1: "Stage — chip and stroke", 2: "Studio — chip and stroke",
         3: "Core — chip and stroke"}
SWATCHES = [(B.tile_colour(), "Tile ground")] + [
    (B.LAYER[n], f"Layer {n} · {TAKES[n]}") for n in (1, 2, 3)
] + [
    (B.SHARED_HL, "Highlighter — the two strokes over the notes"),
    (B.MARKS["troubastack"]["hl"],
     "TroubaStack — neutral; its stroke runs all three layers"),
]
# Every swatch must exist in the paint the artwork actually uses, or it is a
# claim about a colour nothing draws. This is the check that was missing when
# the panel drifted.
_paint = (B.SRC / "_defs.svg").read_text() + B.SHARED_HL + B.tile_colour()
_ghosts = [h for h, _ in SWATCHES if h not in _paint]
if _ghosts:
    raise SystemExit(f"palette names colours nothing uses: {_ghosts}")
o: list[str] = []


def txt(x, y, s, size=15, fill=INK, weight=400, anchor="start", ls=0):
    o.append(f'<text x="{x}" y="{y}" font-family="{F}" font-size="{size}" '
             f'font-weight="{weight}" fill="{fill}" text-anchor="{anchor}" '
             f'letter-spacing="{ls}">{s}</text>')


def icon(mark, variant, x, y, size):
    # sfx keys the highlighter gradient per (mark, variant): the same mark uses a
    # different ink at MINIMAL, and SVG ids are document-global.
    o.append(f'<g transform="translate({x},{y}) scale({size / 1024:.6f})">'
             f'{B.body(mark, variant, sfx=f"-{mark}-{variant}")}</g>')


def rule(y, x0=44, x1=W - 44):
    o.append(f'<line x1="{x0}" y1="{y}" x2="{x1}" y2="{y}" stroke="{RULE}" stroke-width="1"/>')


def png_b64(mark, variant, size):
    with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as fh:
        tmp = Path(fh.name)
    subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), "-o", str(tmp),
                    str(B.DIST / f"{mark}-{variant}.svg")], check=True)
    data = base64.b64encode(tmp.read_bytes()).decode()
    tmp.unlink()
    return data


# ---------------------------------------------------------------- header
o.append(f'<rect x="0" y="0" width="{W}" height="{H}" fill="{PAPER}"/>')
txt(44, 62, "TroubaStack icon family", 34, INK, 700)
txt(44, 92, "One mark, four products. Generated from docs/brand/src — this sheet "
            "cannot drift from the shipped assets.", 16, MUTED)
rule(118)

# ---------------------------------------------------------------- the marks
SUB = {"troubastack": "whirl chip — the sum of the three",
       "troubastudio": "pencil chip, layer 2 pink",
       "troubastage": "play chip, layer 1 yellow",
       "troubacore": "circuit chip, layer 3 blue"}
for i, mark in enumerate(B.MARKS):
    x = 52 + i * 296
    icon(mark, "full", x, 148, 258)
    txt(x, 448, f'<tspan fill="{INK}">Trouba</tspan>'
                f'<tspan fill="{B.ACCENT[mark]}">{NAMES[mark]}</tspan>', 26, INK, 700)
    txt(x, 472, SUB[mark], 13, MUTED)

# ---------------------------------------------------------------- palette
px = 1256
txt(px, 168, "PALETTE", 13, MUTED, 700, ls=1.6)
txt(px, 188, "measured off the reference art, not invented", 12, MUTED)
for i, (hexv, label) in enumerate(SWATCHES):
    y = 206 + i * 38
    o.append(f'<rect x="{px}" y="{y}" width="32" height="32" rx="8" fill="{hexv}" '
             f'stroke="{RULE}"/>')
    txt(px + 44, y + 14, hexv, 13, INK, 600)
    txt(px + 44, y + 29, label, 12, MUTED)
rule(516)

# ---------------------------------------------------------------- variants
txt(44, 552, "LEVELS OF DETAIL", 13, MUTED, 700, ls=1.6)
txt(330, 552, "At MINIMAL there is no chip: one stroke in the mark's colour is "
              "what separates the four.", 13, MUTED)
for row, (variant, note) in enumerate((
        ("compact", "3 rules, 2 notes, heavier strokes — 96–192px"),
        ("minimal", "layer stack + one coloured stroke — 16–48px"))):
    y = 576 + row * 200
    txt(44, y + 88, variant.upper(), 18, INK, 700)
    txt(44, y + 110, note, 13, MUTED)
    for i, mark in enumerate(B.MARKS):
        icon(mark, variant, 330 + i * 186, y, 160)
        txt(330 + i * 186 + 80, y + 182, NAMES[mark], 12, MUTED, anchor="middle")
rule(1000)

# ------------------------------------------------- true-size ladder + proof
txt(44, 1036, "RENDERED AT TRUE SIZE", 13, MUTED, 700, ls=1.6)
txt(44, 1056, "the real icons at 48 / 32 / 16 px, not drawings of them", 12, MUTED)
for row, mark in enumerate(B.MARKS):
    y = 1076 + row * 56
    txt(44, y + 30, NAMES[mark], 13, INK, 600)
    for i, s in enumerate((48, 32, 16)):
        icon(mark, "minimal", 140 + i * 68, y + (48 - s) / 2, s)

txt(400, 1036, "SAME PIXELS, SCALED TO A COMMON WIDTH", 13, MUTED, 700, ls=1.6)
txt(400, 1056, "real rasterised output, nearest-neighbour", 12, MUTED)
for row, mark in enumerate(B.MARKS):
    y = 1072 + row * 60
    txt(400, y + 34, NAMES[mark], 13, INK, 600)
    for i, s in enumerate((48, 32, 16)):
        d = png_b64(mark, "minimal", s)
        o.append(f'<image x="{490 + i * 62}" y="{y}" width="54" height="54" '
                 f'image-rendering="pixelated" preserveAspectRatio="none" '
                 f'href="data:image/png;base64,{d}"/>')
for i, s in enumerate((48, 32, 16)):
    txt(490 + i * 62 + 27, 1072 + 4 * 60 + 16, f"{s}px", 11, MUTED, anchor="middle")

# ---------------------------------------------------------------- wordmarks
txt(760, 1036, "WORDMARK LOCKUPS", 13, MUTED, 700, ls=1.6)
txt(760, 1056, "live text — outline the type before the website ships", 12, MUTED)
for i, mark in enumerate(B.MARKS):
    x, y = 760 + (i % 2) * 300, 1076 + (i // 2) * 106
    o.append(f'<rect x="{x}" y="{y}" width="284" height="92" rx="14" fill="#202C37"/>')
    txt(x + 20, y + 48, f'<tspan fill="#FFFFFF">Trouba</tspan>'
                        f'<tspan fill="{B.ACCENT[mark]}">{NAMES[mark]}</tspan>',
        26, "#FFFFFF", 700)
    txt(x + 21, y + 70, B.WORDMARKS[mark][1], 9, "#A7ACB5", 500, ls=1.0)

inks = "\n".join(B.ink(m, v, f"-{m}-{v}") for m in B.MARKS for v in B.VARIANTS)
defs = B.brick("_defs").replace("</defs>", inks + "\n</defs>")
svg = (f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" '
       f'viewBox="0 0 {W} {H}" width="{W}" height="{H}">\n{defs}\n'
       + "\n".join(o) + "\n</svg>\n")
out = B.DIST / "family-sheet.svg"
out.write_text(svg)
print(f"wrote {out} ({len(svg) // 1024} KB)")

# --- render, and stack the generated sheet above the latest ChatGPT reference --
png = B.DIST / "family-sheet.png"
subprocess.run(["rsvg-convert", "-w", str(W), "-h", str(H), "-o", str(png), str(out)],
               check=True)

from PIL import Image, ImageDraw  # noqa: E402  (optional dependency, only for the compare)

REFS = sorted((B.ROOT / "reference").glob("*.png"))
if not REFS:
    # the exploration plates are not committed; without them there is nothing
    # to compare against and the sheet itself is the whole output
    print("no reference/ plate: skipping the side-by-side")
    raise SystemExit
REF = REFS[-1]                                            # newest reference
mine, ref = Image.open(png).convert("RGB"), Image.open(REF).convert("RGB")
width = max(mine.width, ref.width)


def scaled(im):
    return im.resize((width, round(im.height * width / im.width)), Image.LANCZOS)


mine, ref = scaled(mine), scaled(ref)
band, pad = 44, 16
canvas = Image.new("RGB", (width + pad * 2, band * 2 + mine.height + ref.height + pad * 3),
                   (255, 255, 255))
d = ImageDraw.Draw(canvas)
y = 0
d.text((pad, 16), "GENERATED FROM THE BRICKS  —  real renders, regenerable", fill=(30, 110, 60))
canvas.paste(mine, (pad, band)); y = band + mine.height + pad
d.text((pad, y + 16), f"CHATGPT REFERENCE  —  {REF.name}  (mockup)", fill=(150, 40, 40))
canvas.paste(ref, (pad, y + band))
cmp_path = B.DIST / "family-sheet-vs-reference.png"
canvas.save(cmp_path)
print(f"wrote {cmp_path} ({canvas.size[0]}x{canvas.size[1]})")
