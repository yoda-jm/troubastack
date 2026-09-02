#!/usr/bin/env python3
"""Resolve objectBoundingBox gradients (gMono/gRim/gWhite — on stroked paths, which picosvg
left unresolved and s2v turns into NaN) into per-path userSpaceOnUse viewport coordinates.
objectBoundingBox: gradient coords are fractions of the referencing path's geometric bbox:
  userX = bx + fracX*bw ; userY = by + fracY*bh.
Since one gradient id is shared by several paths with different bboxes, each such path gets its
own resolved gradient (id suffixed by index)."""
import re, sys
from picosvg.svg_types import SVGPath

src, dst = sys.argv[1], sys.argv[2]
svg = open(src).read()

OBB_IDS = {"gMono", "gRim", "gWhite"}

# capture the stop lists of the OBB gradient templates
templates = {}
for gid in OBB_IDS:
    m = re.search(rf'<linearGradient id="{gid}"[^>]*>(.*?)</linearGradient>', svg, re.S)
    if m:
        attrs = dict(re.findall(r'(\w+)="([^"]*)"', m.group(0).split('>')[0]))
        templates[gid] = {
            "x1": float(attrs.get("x1", 0)), "y1": float(attrs.get("y1", 0)),
            "x2": float(attrs.get("x2", 0)), "y2": float(attrs.get("y2", 0)),
            "stops": m.group(1),
        }

new_defs = []
counter = {gid: 0 for gid in OBB_IDS}

def path_repl(m):
    tag = m.group(0)
    ref = re.search(r'url\(#(gMono|gRim|gWhite)\)', tag)
    if not ref:
        return tag
    gid = ref.group(1)
    d = re.search(r'\sd="([^"]*)"', tag).group(1)
    bb = SVGPath(d=d).bounding_box()  # geometric bbox: .x .y .w .h
    t = templates[gid]
    ux1 = bb.x + t["x1"] * bb.w; uy1 = bb.y + t["y1"] * bb.h
    ux2 = bb.x + t["x2"] * bb.w; uy2 = bb.y + t["y2"] * bb.h
    nid = f"{gid}_r{counter[gid]}"; counter[gid] += 1
    f = lambda v: f"{v:.3f}".rstrip('0').rstrip('.')
    new_defs.append(
        f'<linearGradient id="{nid}" x1="{f(ux1)}" y1="{f(uy1)}" x2="{f(ux2)}" y2="{f(uy2)}" '
        f'gradientUnits="userSpaceOnUse">{t["stops"]}</linearGradient>')
    return tag.replace(f"url(#{gid})", f"url(#{nid})")

svg = re.sub(r'<path\b[^>]*/>', path_repl, svg)
svg = re.sub(r'<path\b[^>]*>', path_repl, svg)  # in case of non-self-closing

# drop the original OBB templates, inject resolved defs before </defs>
for gid in OBB_IDS:
    svg = re.sub(rf'<linearGradient id="{gid}"[^>]*>.*?</linearGradient>\s*', '', svg, flags=re.S)
svg = svg.replace('</defs>', '\n'.join(new_defs) + '\n</defs>')

open(dst, "w").write(svg)
print("resolved:", {k: v for k, v in counter.items() if v})
print("residual OBB refs:", len(re.findall(r'url\(#(?:gMono|gRim|gWhite)\)', svg)))
