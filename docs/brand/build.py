#!/usr/bin/env python3
"""Assemble the TroubaStack icon family from src/ bricks into dist/.

One set of bricks; every asset is a recipe over them. Nothing is hand-edited
in dist/ — regenerate instead.

    python3 docs/brand/build.py            # SVGs only
    python3 docs/brand/build.py --png      # + PNG ladder (needs rsvg-convert)
"""
import math
import re
import subprocess
import sys
from collections import Counter
from xml.etree import ElementTree
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SRC, DIST = ROOT / "src", ROOT / "dist"


# --- the three layer colours ------------------------------------------------
# Read out of _defs rather than restated. The palette panel used to name
# #EAAD55, #D131A7 and #1563C7, and NONE of the three was still present in the
# gradients after the faces were re-measured: the panel had drifted under a
# caption promising it could not. Index 3 is the stop that carries on a dark
# ground; the darker ends of each ramp disappear against the tile.
LAYER_STOPS = 6                  # each face ramp is measured in six bins


def _layer(n: int, stop: int = 3) -> str:
    txt = (SRC / "_defs.svg").read_text()
    block = re.search(rf'id="gLayer{n}".*?</linearGradient>', txt, re.S).group(0)
    stops = re.findall(r'stop-color="(#[0-9A-Fa-f]{6})"', block)
    # Indexing a ramp by position only means anything if the ramp still has the
    # shape it was measured with. Re-bin the faces into five stops and index 3
    # silently becomes a different colour — which is how the official swatch
    # would start naming a neighbour without anything failing.
    if len(stops) != LAYER_STOPS:
        raise SystemExit(f"gLayer{n}: expected {LAYER_STOPS} stops, found "
                         f"{len(stops)} - re-check which one carries the swatch")
    return stops[stop]


LAYER = {n: _layer(n) for n in (1, 2, 3)}
LAYER[3] = _layer(3, 4)          # layer 3's mid stop is too dark on the tile


def tile_colour() -> str:
    """The tile ground, as one swatch: gTile's two stops averaged. The palette
    used to name #202C37, which the gradient has not held since it became a
    ramp."""
    txt = (SRC / "_defs.svg").read_text()
    block = re.search(r'id="gTile".*?</linearGradient>', txt, re.S).group(0)
    a, b_ = re.findall(r'stop-color="#([0-9A-Fa-f]{6})"', block)[:2]
    mix = [(int(a[i:i + 2], 16) + int(b_[i:i + 2], 16)) // 2 for i in (0, 2, 4)]
    return "#%02X%02X%02X" % tuple(mix)

# --- the four marks --------------------------------------------------------
# The chip is the only differentiator. Each takes its own layer's hue: Stage
# yellow like layer 1, Studio pink like layer 2, Stack blue like layer 3.
# TroubaStack had no chip until VLL added one.
# `hl` is the MINIMAL variant's single stroke, in the same colour.
MARKS = {
    "troubastack":  {"chip": "chip-stack",  "hl": "#5A6674", "core": "#C8D4E0",
                     "min": [LAYER[1], LAYER[2], LAYER[3]]},
    "troubastudio": {"chip": "chip-pencil", "hl": LAYER[2],  "core": "#F87EE0"},
    "troubastage":  {"chip": "chip-play",   "hl": LAYER[1],  "core": "#FEE36A"},
    "troubacore":   {"chip": "chip-core",   "hl": LAYER[3],  "core": "#BEE4FF"},
}

# --- the three levels of detail -------------------------------------------
# MINIMAL drops everything that dies below ~48px and keeps the two shapes that
# survive: the layer stack and the highlighter.
# Draw order matters, and it changed once the reference was measured: the
# staff rules sit OVER the highlighter, not under it. The swipe is nearly
# opaque there, yet the rules cross it unbroken, which only works if they
# are printed on top. That is also the physical model: highlighter ink
# goes over the paper and the print shows through.
VARIANTS = {
    "full":    ["tile", "layers", "highlighter", "staff-full",    "notes-full",
                "monogram", "CHIP"],
    "compact": ["tile", "layers", "highlighter", "staff-compact", "notes-compact",
                "monogram", "CHIP"],
    "minimal": ["tile", "layers", "highlighter"],
}

# Highlighter colour carries the identity at MINIMAL, where no chip exists.
# Above MINIMAL every mark shares the same dark-gold stroke.
SHARED_HL, SHARED_CORE = "#FEE963", "#FFF9CE"

# 192 belonged to BOTH full and compact, and the filename carries no variant, so
# the compact render silently overwrote the full one — 32 renders, 28 files. The
# ranges are the README's own: full is 512 and up, compact owns 192.
PNG_SIZES = {"full": [1024, 512], "compact": [192, 96], "minimal": [48, 32, 16]}

# --- the Android adaptive foreground ---------------------------------------
# The launcher masks the 108dp canvas down to a 66dp circle on most devices, so
# the artwork has to sit inside 66/108 of it. The scale used to be a hand-typed
# 0.66 — which is not that ratio (66/108 = 0.611), and in any case a canvas
# ratio says nothing about where the ARTWORK ends. Measured: the tile-less
# compact art reached 403.5 units from centre against a 312.9-unit safe radius,
# a 29% overshoot, so every round-masked launcher clipped its corners off.
#
# What bounds the artwork is its MINIMAL ENCLOSING CIRCLE, not its extent from
# the middle of the canvas. The art is not centred there — the planes sit high,
# the monogram low and left — so measuring the radius from the canvas centre
# charges us for empty space on the opposite side. Measured at 2048px on the
# ink (alpha > 8), hull then Welzl: the enclosing circle is centred 21.9 units
# left and 98.3 units BELOW centre, radius 522.1 against 611.4 measured the
# naive way. Re-centring on it and scaling to the same safe circle buys 17%.
#
# So the foreground translates the art's own circle onto the canvas centre
# first, then scales. Identical for all four marks: only the chip differs, and
# the chip is not what the circle rests on.
#
# These stay CONSTANTS rather than measurements performed during the build, and
# that is deliberate: build.py is stdlib-only and deterministic, which is what
# lets CI diff dist/ as a drift guard. Rasterising here would make the committed
# SVGs depend on the rsvg build and installed fonts of whoever ran it — the same
# reason sheet.py is NOT guarded in CI. The --png path re-measures instead and
# fails if the constants ever drift from the artwork, so they cannot go stale.
SAFE_FRACTION = 66 / 108
FG_ART_CENTRE = (490.1, 610.3)   # centre of the art's enclosing circle, scale 1
FG_ART_RADIUS = 522.1            # its radius, in 1024-box units
FG_MARGIN = 0.98                 # leave 2% rather than resting on the circle
FG_SCALE = round(SAFE_FRACTION * 512 / FG_ART_RADIUS * FG_MARGIN, 4)

TEMPLATE = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" \
width="1024" height="1024" role="img" aria-label="{label}">
<title>{label}</title>
{defs}
{body}
</svg>
"""


# --- wordmark lockups ------------------------------------------------------
# NOTE: these use live <text>, so they render with whatever font the viewer has.
# Before shipping to the website, outline the type in a vector editor (or embed
# a licensed webfont) — otherwise the lockup drifts per machine.
WORDMARKS = {
    "troubastack":  ("Stack",  "THE PLATFORM FOR MUSICIANS"),
    "troubastudio": ("Studio", "READ. HIGHLIGHT. ANNOTATE. CREATE."),
    "troubastage":  ("Stage",  "PRACTICE. PLAY. PERFORM."),
    "troubacore":   ("Core",   "SCALE. SYNC. SERVE."),
}
# Only Stage was asked to change, so only Stage changed.
# BRAND06: an accent PAIR per mark, one per wordmark ground. A single accent cannot survive both
# grounds — the same hue fails the 3:1 large-text bar on one of them (Stack was 2.43 on the dark
# tile, Stage 2.65 on paper), which is why the project page had to hand-correct them. Each value
# keeps the mark's hue+saturation and moves lightness only until it clears the bar on its ground;
# the measured ratio is recorded beside it (measured once, not at build time — build.py stays
# stdlib-only and deterministic). The two page-live corrections (#AEBAC6, #936B1F) are adopted
# verbatim; the guard in main() enforces the bar with teeth.
WORDMARK_GROUNDS = {"dark": "#202C37", "paper": "#FFFFFF"}  # the two grounds a wordmark renders on
ACCENT = {
    "troubastack":  {"dark": "#AEBAC6", "paper": "#5A6674"},  # 7.20 / 5.85
    "troubastudio": {"dark": "#D62A8A", "paper": "#D62A8A"},  # 3.09 / 4.61
    "troubastage":  {"dark": "#C8912A", "paper": "#936B1F"},  # 5.11 / 4.81
    "troubacore":   {"dark": "#3E89EA", "paper": "#1769D1"},  # 4.04 / 5.28
}
FONT = "Inter, 'Helvetica Neue', Helvetica, Arial, sans-serif"


def _rel_luminance(hexc: str) -> float:
    """WCAG relative luminance of a #RRGGBB colour."""
    chan = []
    for i in (1, 3, 5):
        c = int(hexc[i:i + 2], 16) / 255
        chan.append(c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4)
    r, g, b = chan
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def _contrast(a: str, b: str) -> float:
    """WCAG contrast ratio between two #RRGGBB colours."""
    la, lb = _rel_luminance(a), _rel_luminance(b)
    return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)

WORDMARK_TEMPLATE = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 220" \
width="900" height="220" role="img" aria-label="Trouba{tail}">
<title>Trouba{tail}</title>
{ground}<text x="40" y="118" font-family="{font}" font-size="88" font-weight="700">\
<tspan fill="{base}">Trouba</tspan><tspan fill="{accent}">{tail}</tspan></text>
<text x="44" y="168" font-family="{font}" font-size="25" font-weight="500" \
letter-spacing="3.4" fill="#A7ACB5">{tagline}</text>
</svg>
"""


def wordmark(mark: str, tail: str, tagline: str, ground: str | None,
             base: str) -> str:
    rect = (f'<rect x="0" y="0" width="900" height="220" rx="26" fill="{ground}"/>\n'
            if ground else "")
    accent = ACCENT[mark]["paper" if ground is None else "dark"]
    return WORDMARK_TEMPLATE.format(tail=tail, tagline=tagline, ground=rect,
                                    base=base, accent=accent, font=FONT)


def check(path: Path) -> None:
    """Fail at write time, not at raster time. A double hyphen inside an XML
    comment is invalid; rsvg only reports it later, pointing at the assembled
    file rather than the brick the text came from.

    Also: ids must be unique. Well-formed is not enough — the adaptive
    foregrounds parsed cleanly for weeks while carrying two <defs> blocks and
    nineteen duplicate gradient ids. Which definition a renderer picks is
    unspecified, and VectorDrawable conversion is exactly the consumer that
    may pick differently from the browser you checked in."""
    text = path.read_text()
    try:
        ElementTree.fromstring(text)
    except ElementTree.ParseError as exc:
        raise SystemExit(f"{path.name}: malformed SVG - {exc}") from exc
    dupes = sorted(i for i, n in Counter(re.findall(r'\bid="([^"]+)"', text)).items()
                   if n > 1)
    if dupes:
        raise SystemExit(f"{path.name}: {len(dupes)} duplicate id(s) - "
                         f"{', '.join(dupes[:6])}{' ...' if len(dupes) > 6 else ''}")


def brick(name: str, **subs: str) -> str:
    text = (SRC / f"{name}.svg").read_text().rstrip()
    for key, val in subs.items():
        text = text.replace("{{%s}}" % key, val)
    if "{{" in text:
        raise SystemExit(f"unsubstituted token in brick {name!r}")
    return text


# --- the marker stroke ------------------------------------------------------
# ONE pattern, instanced three times. VLL: every stroke must be the same shape,
# differing only by translation, rotation and scale. So the band is authored
# once in its own frame, running from (0,0) to (LEN,0), and each instance is a
# transform. Its gradient is userSpaceOnUse in that same local frame, so it
# travels with the instance instead of being restretched per shape.
LEN, HW, RAD = 655, 52, 9       # 20 -> 14 -> 9: less rounded again, per VLL
# 40% shorter, taken off BOTH ends so the pattern keeps its centre. Shortening
# from the left end instead would move every instance's centre, and the
# placements below are expressed about those centres: they would all shift.
SHORT = 0.60
X0, X1 = LEN * (1 - SHORT) / 2, LEN * (1 + SHORT) / 2
# Negative sagitta puts the circle's centre FAR BELOW the stroke, so the arc
# bows gently upward. Positive put it just above, which curved it the other way.
SAG = -8


def _circle(p0, p1, sag):
    mx, my = (p0[0] + p1[0]) / 2, (p0[1] + p1[1]) / 2
    dx, dy = p1[0] - p0[0], p1[1] - p0[1]
    c = math.hypot(dx, dy)
    R = (c * c / 4 + sag * sag) / (2 * abs(sag))
    nx, ny = -dy / c, dx / c
    h = math.sqrt(max(0.0, R * R - c * c / 4))
    k = -1 if sag > 0 else 1
    return (mx + nx * h * k, my + ny * h * k), R


def arc(p0, p1, sag):
    _, R = _circle(p0, p1, sag)
    return (f"M {p0[0]:.1f} {p0[1]:.1f} A {R:.0f} {R:.0f} 0 0 "
            f"{1 if sag > 0 else 0} {p1[0]:.1f} {p1[1]:.1f}")


def band(p0, p1, sag, hw, r):
    """A closed band with four rounded corners. Not a stroke: a stroke's ends
    are butt, round or square, with no corner radius in between."""
    C, R = _circle(p0, p1, sag)
    a0 = math.atan2(p0[1] - C[1], p0[0] - C[0])
    a1 = math.atan2(p1[1] - C[1], p1[0] - C[0])
    if a1 < a0:
        a1 += 2 * math.pi
    if a1 - a0 > math.pi:
        a0, a1 = a1 - 2 * math.pi, a0
    Ro, Ri = R + hw, R - hw
    P = lambda rad, a: (C[0] + rad * math.cos(a), C[1] + rad * math.sin(a))
    o_s, o_e = P(Ro, a0 + r / Ro), P(Ro, a1 - r / Ro)
    i_s, i_e = P(Ri, a0 + r / Ri), P(Ri, a1 - r / Ri)
    co_s, co_e, ci_s, ci_e = P(Ro, a0), P(Ro, a1), P(Ri, a0), P(Ri, a1)
    on = lambda A, B, t: (A[0] + (B[0] - A[0]) * t, A[1] + (B[1] - A[1]) * t)
    ts = r / (2 * hw)
    es_o, es_i = on(co_s, ci_s, ts), on(ci_s, co_s, ts)
    ee_o, ee_i = on(co_e, ci_e, ts), on(ci_e, co_e, ts)
    # The normalisation above leaves 0 <= a1-a0 <= pi for any pair atan2 can
    # return, so both arc flags are constants: never the large arc, always the
    # positive sweep on the outer edge and the negative one coming back along
    # the inner. They used to be written as conditionals, which read like cases
    # that occur. Asserted rather than assumed, since the whole stroke silently
    # turns inside out if it ever stops holding.
    assert -1e-9 <= a1 - a0 <= math.pi + 1e-9, f"arc span out of range: {a1 - a0}"
    la, sw = 0, 1
    return (f"M {o_s[0]:.1f} {o_s[1]:.1f} "
            f"A {Ro:.0f} {Ro:.0f} 0 {la} {sw} {o_e[0]:.1f} {o_e[1]:.1f} "
            f"Q {co_e[0]:.1f} {co_e[1]:.1f} {ee_o[0]:.1f} {ee_o[1]:.1f} "
            f"L {ee_i[0]:.1f} {ee_i[1]:.1f} "
            f"Q {ci_e[0]:.1f} {ci_e[1]:.1f} {i_e[0]:.1f} {i_e[1]:.1f} "
            f"A {Ri:.0f} {Ri:.0f} 0 {la} {1 - sw} {i_s[0]:.1f} {i_s[1]:.1f} "
            f"Q {ci_s[0]:.1f} {ci_s[1]:.1f} {es_i[0]:.1f} {es_i[1]:.1f} "
            f"L {es_o[0]:.1f} {es_o[1]:.1f} "
            f"Q {co_s[0]:.1f} {co_s[1]:.1f} {o_s[0]:.1f} {o_s[1]:.1f} Z")


# Three instances of that one pattern. `base` aims it the way the artwork needs;
# `user` is what VLL set in the placement page, in canvas space, so it is applied
# OUTSIDE the staff's rotation exactly as the page applied it.
# `users` is a STACK of placements, oldest first, each rotating and scaling about
# the shape's centre AT THE TIME it was set — which is what the placement page
# does. A round's translation moves that centre, so the next round's centre is
# the previous one plus that translation; rotation and scale leave it fixed.
STROKES = [
    dict(base=(58, 786, -15.14, 1.000),
         users=[(184.7, -125.1, 0.00, 1.000), (23.0, 3.4, 2.37, 0.975),
                (-119.2, 61.3, 0.00, 1.000)]),
    dict(base=(468, 706, -11.58, 0.760),
         users=[(-14.5, -28.1, 0.00, 1.000), (161.7, -57.9, 0.55, 1.163),
                (-126.0, 56.2, 0.00, 1.000)]),
    dict(base=(70, 806, -10.08, 0.558),
         users=[(-105.5, 53.6, 9.36, 0.692), (31.5, -1.7, 0.00, 1.311),
                (2.6, 11.9, 0.00, 1.212)]),
]


def _centre(bx, by, ba, bs):
    """The instance's centre in tile space, needed because the placement page
    rotates and scales about each shape's OWN centre. Composing those about the
    origin instead threw the third stroke against the tile's left edge."""
    a = math.radians(ba)
    px, py = LEN / 2 * bs, -SAG / 2 * bs   # LEN/2 stays the centre: see SHORT
    rx, ry = px * math.cos(a) - py * math.sin(a), px * math.sin(a) + py * math.cos(a)
    x, y = bx + rx, by + ry
    t = math.radians(-14)
    dx, dy = x - 512, y - 640
    return (512 + dx * math.cos(t) - dy * math.sin(t),
            640 + dx * math.sin(t) + dy * math.cos(t))


def highlighter(sfx: str, variant: str = "full") -> str:
    """The hero element: three instances of the one marker pattern.

    MINIMAL keeps ONE of them, run across the full width under the stack and
    coloured for the mark, because at 16px three overlapping strokes are a
    smudge and the chip that would otherwise tell the marks apart is gone."""
    d = band((X0, 0), (X1, 0), SAG, HW, RAD)
    streaks = "\n".join(
        f'  <path d="{arc((X0 + a, y), (X1 - b, y), SAG)}" stroke-width="{w}"'
        + (f' stroke-opacity="{o}"' if o < 1 else "") + "/>"
        for a, b, y, w, o in ((26, 24, -16, 19, 1.0), (34, 18, 8, 10, 0.60),
                              (22, 34, -34, 8, 0.45)))
    shape = brick("stroke", D=d, GRAD="gInk", STREAKS=streaks, SFX=sfx)
    # The third stroke lies over the monogram and takes the MARK's ramp, the
    # same one the minimal variant uses: a mark declaring three colours gets
    # them over the TS as well. The two over the notes stay gold.
    shape_mark = brick("stroke", D=d, GRAD="gInkMin", STREAKS=streaks, SFX=sfx)
    if variant == "minimal":
        # NON-uniform: x stretches the pattern across the tile, y is held near 1.
        # A uniform scale thickened it in proportion to the stretch and it read
        # as a bar rather than a stroke.
        mini = brick("stroke", D=d, GRAD="gInkMin", STREAKS=streaks, SFX=sfx)
        return (f'<g transform="translate(512,748) rotate(-9) '
                f'scale({900 / (LEN * SHORT):.3f},1.15) '
                f'translate({-LEN / 2:.0f},0)">\n{mini}\n</g>')
    out = []
    for s in STROKES:
        bx, by, ba, bs = s["base"]
        inner = (f'rotate(-14 512 640) translate({bx},{by}) rotate({ba}) '
                 f'scale({bs})')
        cx, cy = _centre(bx, by, ba, bs)
        body_ = shape_mark if s is STROKES[2] else shape
        wrapped = f'<g transform="{inner}">\n{body_}\n</g>'
        for ux, uy, ua, us in s["users"]:
            wrapped = (f'<g transform="translate({ux},{uy}) '
                       f'translate({cx:.1f},{cy:.1f}) rotate({ua}) scale({us}) '
                       f'translate({-cx:.1f},{-cy:.1f})">\n{wrapped}\n</g>')
            cx, cy = cx + ux, cy + uy
        out.append(wrapped)
    return "\n".join(out)


def _minstops(mark: str) -> str:
    """The minimal stroke's ramp.

    One colour unless the mark declares several. Several means EQUAL THIRDS
    with a thin blend between them, not one long ramp: VLL asked for the three
    colours each holding a third, the gradients only where they meet."""
    cols = MARKS[mark].get("min") or [MARKS[mark]["hl"]]
    n, blend, out = len(cols), 0.035, []
    for i, c in enumerate(cols):
        lo = 0.0 if i == 0 else i / n + blend
        hi = 0.92 if i == n - 1 else (i + 1) / n - blend
        out.append(f'  <stop offset="{lo:.3f}" stop-color="{c}" stop-opacity="0.86"/>')
        out.append(f'  <stop offset="{hi:.3f}" stop-color="{c}" stop-opacity="0.86"/>')
    out.append(f'  <stop offset="1" stop-color="{cols[-1]}" stop-opacity="0"/>')
    return "\n".join(out)


def ink(mark: str, variant: str, sfx: str = "") -> str:
    """The per-mark highlighter gradients. Must be in <defs> of the same document."""
    m = MARKS[mark]
    hl, core = ((m["hl"], m["core"]) if variant == "minimal"
                else (SHARED_HL, SHARED_CORE))
    return brick("_ink", HL=hl, CORE=core, MINSTOPS=_minstops(mark), SFX=sfx)


def body(mark: str, variant: str, *, tile: bool = True, sfx: str = "") -> str:
    """The artwork only — no <svg> wrapper, no <defs>. Reusable inside a sheet."""
    cfg, parts = MARKS[mark], []
    hl = cfg["hl"] if variant == "minimal" else SHARED_HL
    for name in VARIANTS[variant]:
        if name == "CHIP":
            if cfg["chip"]:
                parts.append(brick(cfg["chip"]))
            continue
        if name == "highlighter":
            parts.append(highlighter(sfx, variant))
            continue
        if name == "tile" and not tile:
            continue
        parts.append(brick(name, HL=hl, SFX=sfx))
    return "\n".join(parts)


def compose(mark: str, variant: str, *, tile: bool = True) -> str:
    defs = brick("_defs").replace("</defs>", ink(mark, variant) + "\n</defs>")
    return TEMPLATE.format(label=f"{mark} icon ({variant})", defs=defs,
                           body=body(mark, variant, tile=tile))


# --- per-brick previews -----------------------------------------------------
# The bricks are bare fragments: no <svg> root, no viewBox, and they depend on
# the gradients in _defs. That is what makes them composable, but it also means
# nothing can display one on its own. SVG has no include directive to fix that
# from the other side: XML external entities are disabled by renderers, external
# <use href="other.svg#id"> is blocked by many of them, and the Android
# VectorDrawable conversion needs one self-contained file. So assembly happens
# here, and the build ALSO emits a viewable copy of each brick.
def previews() -> list:
    out_dir = DIST / "bricks"
    out_dir.mkdir(exist_ok=True)
    defs = brick("_defs").replace("</defs>", ink("troubastack", "full") + "\n</defs>")
    written = []
    for src in sorted(SRC.glob("*.svg")):
        name = src.stem
        if name.startswith("_"):
            continue
        if name == "stroke":
            art = highlighter("")
        else:
            art = brick(name, HL=SHARED_HL, SFX="")
        ground = "" if name == "tile" else brick("tile") + "\n"
        path = out_dir / f"{name}.svg"
        path.write_text(TEMPLATE.format(label=f"brick: {name}", defs=defs,
                                        body=ground + art))
        check(path)
        written.append(path)
    return written


def check_safe_circle(render: int = 1024) -> None:
    """Every adaptive foreground must fit the launcher's mask.

    Rasterises rather than reasoning about geometry, because what gets clipped
    is pixels: strokes, the plane borders and the marker's soft tail all put ink
    outside any path's nominal bounds. Lives in the --png path, which already
    depends on rsvg — the SVG build stays stdlib-only so CI can diff dist/.

    This is also what keeps FG_ART_EXTENT honest: re-measure the art, and if a
    brick has grown past the constant the build fails instead of shipping a
    foreground that the mask eats."""
    try:
        import numpy as np
        from PIL import Image
    except ImportError as exc:
        print(f"  safe-circle check skipped: {exc.name} not installed")
        return
    import tempfile
    safe = SAFE_FRACTION * render / 2
    centre = (render - 1) / 2
    # The furthest INK pixel, not a bounding box: a bbox corner need not be
    # opaque, and taking one as the extent would fail the build on empty space.
    worst = []
    for mark in MARKS:
        path = DIST / f"{mark}-adaptive-foreground.svg"
        with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as fh:
            tmp = Path(fh.name)
        subprocess.run(["rsvg-convert", "-w", str(render), "-h", str(render),
                        "-o", str(tmp), str(path)], check=True)
        alpha = np.array(Image.open(tmp).convert("RGBA"))[:, :, 3]
        tmp.unlink()
        ys, xs = np.nonzero(alpha > 8)
        far = float(np.hypot(xs - centre, ys - centre).max()) if len(xs) else 0.0
        if far > safe:
            worst.append(f"{path.name}: art reaches {far:.1f}px of a {safe:.0f}px "
                         f"safe circle (+{(far / safe - 1) * 100:.0f}%)")
    if worst:
        raise SystemExit("adaptive foreground escapes the launcher mask:\n  "
                         + "\n  ".join(worst))
    print(f"  safe-circle check: all {len(MARKS)} foregrounds inside "
          f"{SAFE_FRACTION:.3f} of the canvas")


def main() -> None:
    DIST.mkdir(exist_ok=True)
    written = []
    for mark in MARKS:
        for variant in VARIANTS:
            path = DIST / f"{mark}-{variant}.svg"
            path.write_text(compose(mark, variant))
            check(path)
            written.append(path)
        # Android adaptive: background is flat ground, foreground is art only,
        # scaled so the artwork itself fits the 66/108 safe circle (see FG_SCALE).
        bg = DIST / f"{mark}-adaptive-background.svg"
        bg.write_text(TEMPLATE.format(
            label=f"{mark} adaptive background", defs=brick("_defs"),
            body='<rect x="0" y="0" width="1024" height="1024" fill="url(#gTile)"/>'))
        check(bg)
        # Built from the bricks, NOT carved out of compose()'s output. String
        # surgery on the assembled document kept compose's <defs> and the
        # template then added a second one, so every gradient id was defined
        # twice in all four files. The ink gradients have to travel with the
        # art, hence the same defs assembly compose() does.
        inner = body(mark, "compact", tile=False)
        fg = DIST / f"{mark}-adaptive-foreground.svg"
        fg.write_text(TEMPLATE.format(
            label=f"{mark} adaptive foreground",
            defs=brick("_defs").replace("</defs>", ink(mark, "compact") + "\n</defs>"),
            body=f'<g transform="translate(512,512) scale({FG_SCALE}) '
                 f'translate({-FG_ART_CENTRE[0]},{-FG_ART_CENTRE[1]})">\n{inner}\n</g>'))
        check(fg)
        written += [bg, fg]

    # BRAND06 guard, with teeth: every accent must clear the 3:1 large-text bar on its ground
    # (the wordmark is 88px/700), or the family ships an unreadable lockup. This runs in build.py
    # because CI regenerates via build.py (not sheet.py); reverting one accent fails the build here.
    for mark, pair in ACCENT.items():
        for ground_name, bg in WORDMARK_GROUNDS.items():
            r = _contrast(pair[ground_name], bg)
            if r < 3.0:
                raise SystemExit(f"ACCENT[{mark!r}][{ground_name!r}] = {pair[ground_name]} is "
                                 f"{r:.2f}:1 on {bg} — below the 3:1 large-text bar")

    for mark, (tail, tagline) in WORDMARKS.items():
        for suffix, ground, base in (("", None, "#101418"), ("-dark", "#202C37", "#FFFFFF")):
            path = DIST / f"{mark}-wordmark{suffix}.svg"
            path.write_text(wordmark(mark, tail, tagline, ground, base))
            written.append(path)

    written += previews()
    print(f"{len(written)} SVGs -> {DIST} "
          f"(including {len(list((DIST / 'bricks').glob('*.svg')))} brick previews)")

    if "--png" in sys.argv:
        seen: dict[Path, str] = {}
        for mark in MARKS:
            for variant, sizes in PNG_SIZES.items():
                for size in sizes:
                    out = DIST / f"{mark}-{size}.png"
                    # The filename carries no variant, so two variants sharing a
                    # size would render twice and keep only the last silently.
                    if out in seen:
                        raise SystemExit(
                            f"{out.name}: written twice ({seen[out]} then {variant}) "
                            f"- two variants share a size in PNG_SIZES")
                    seen[out] = variant
                    subprocess.run(
                        ["rsvg-convert", "-w", str(size), "-h", str(size),
                         "-o", str(out), str(DIST / f"{mark}-{variant}.svg")],
                        check=True)
        print(f"PNG ladder rendered ({len(seen)} files, one per render)")
        check_safe_circle()


if __name__ == "__main__":
    main()
