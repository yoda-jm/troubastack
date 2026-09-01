#!/usr/bin/env python3
"""Regenerate the placement page from the CURRENT artwork.

    python3 docs/brand/tools/make-placement.py

Five shapes, each movable by translation, rotation and scale about its own
centre; VLL copies the resulting values back and they are appended to the
placement stacks in build.py.

Written as a script rather than retyped each round, for the same reason the
marker stroke became a brick: a tool that only exists in a throwaway command
cannot be trusted to reproduce what it produced last time. It always starts
from the CURRENT state, so each round's output is a delta on the one before.

The page is not committed: it embeds two rasters of the artwork. The generator
is, since it is small and regenerates them.
"""
import base64
import re
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
import build as B  # noqa: E402

OUT = Path(sys.argv[1]) if len(sys.argv) > 1 else ROOT / "dist" / "placement.html"
SHAPES = [("trait1", "1er trait des notes", "#F5A623"),
          ("trait2", "2e trait des notes", "#4FC3F7"),
          ("trait3", "trait du TS", "#E4508F"),
          ("T", "lettre T", "#9CCC65"),
          ("S", "lettre S", "#BA8CF0")]


def raster(body: str) -> str:
    defs = B.brick("_defs").replace("</defs>", B.ink("troubastack", "full") + "\n</defs>")
    with tempfile.NamedTemporaryFile(suffix=".svg", delete=False) as fh:
        svg = Path(fh.name)
    svg.write_text(B.TEMPLATE.format(label="x", defs=defs, body=body))
    png = svg.with_suffix(".png")
    r = subprocess.run(["rsvg-convert", "-w", "1024", "-h", "1024", "-o", str(png), str(svg)],
                       capture_output=True, text=True)
    if r.returncode:
        raise SystemExit("rsvg failed: " + r.stderr[:300])
    data = base64.b64encode(png.read_bytes()).decode()
    svg.unlink(); png.unlink()
    return data


def stroke_groups() -> list:
    """Each stroke with its whole placement stack already applied."""
    d = B.band((B.X0, 0), (B.X1, 0), B.SAG, B.HW, B.RAD)
    streaks = "".join(
        f'<path d="{B.arc((B.X0 + a, y), (B.X1 - b, y), B.SAG)}" '
        f'stroke-width="{w}" stroke-opacity="{o}"/>'
        for a, b, y, w, o in ((26, 24, -16, 19, 1.0), (34, 18, 8, 10, 0.60),
                              (22, 34, -34, 8, 0.45)))
    shape = (f'<path d="{d}" fill="url(#gInk)"/>'
             f'<g fill="none" stroke="url(#gCore)" stroke-linecap="round">{streaks}</g>')
    out = []
    for i, s in enumerate(B.STROKES):
        bx, by, ba, bs = s["base"]
        cx, cy = B._centre(bx, by, ba, bs)
        w = (f'<g transform="rotate(-14 512 640) translate({bx},{by}) '
             f'rotate({ba}) scale({bs})">{shape}</g>')
        for ux, uy, ua, us in s["users"]:
            w = (f'<g transform="translate({ux},{uy}) translate({cx:.1f},{cy:.1f}) '
                 f'rotate({ua}) scale({us}) translate({-cx:.1f},{-cy:.1f})">{w}</g>')
            cx, cy = cx + ux, cy + uy
        out.append(f'<g id="u-{SHAPES[i][0]}">{w}</g>')
    return out


def letter_groups() -> list:
    mono = (ROOT / "src" / "monogram.svg").read_text()
    g = re.findall(r'(<g transform="translate\(-?[\d.]+,-?[\d.]+\).*?)\n</g>\n', mono, re.S)
    if len(g) != 2:
        raise SystemExit(f"expected two letter groups in monogram.svg, found {len(g)}")
    return [f'<g id="u-T">{g[0]}</g></g>', f'<g id="u-S">{g[1]}</g></g>']


def main() -> None:
    back = raster(B.brick("tile") + "\n" + B.brick("layers"))
    front = raster(B.brick("staff-full", HL=B.SHARED_HL, SFX="") + "\n"
                   + B.brick("notes-full", HL=B.SHARED_HL, SFX=""))
    strokes, letters = stroke_groups(), letter_groups()
    art = (f'<image x="0" y="0" width="1024" height="1024" '
           f'href="data:image/png;base64,{back}"/>' + "".join(strokes)
           + f'<image x="0" y="0" width="1024" height="1024" '
             f'href="data:image/png;base64,{front}"/>' + "".join(letters))
    defs = B.brick("_defs").replace("</defs>", B.ink("troubastack", "full") + "\n</defs>")
    tpl = (Path(__file__).parent / "placement.tpl.html").read_text()
    shapes_js = ",".join(f'["{k}","{lab}","{col}"]' for k, lab, col in SHAPES)
    OUT.parent.mkdir(exist_ok=True)
    OUT.write_text(tpl.replace("/*SHAPES*/", shapes_js)
                      .replace("<!--SVG-->", defs + art))
    print(f"wrote {OUT} ({OUT.stat().st_size // 1024} KB)")


if __name__ == "__main__":
    main()
