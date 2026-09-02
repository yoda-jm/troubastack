#!/usr/bin/env python3
"""Bake gradientTransform matrices into gradient coordinates so s2v (which drops the
matrix silently) has only plain userSpaceOnUse viewport coords to copy.
- linear: transform both endpoints by the matrix.
- radial uniform (a==d, b==c==0): transform centre, scale r by |a|.
- radial non-uniform (only gRule, matrix(1 0 0 0.55)): can't be an ellipse in VectorDrawable,
  so emit the deliberate circle approximation centred on the staff (cy=640, r=235 = the oval's
  vertical semi-axis). Documented in the drawable header + BRAND05 verdict.
objectBoundingBox gradients (gMono/gRim/gWhite) are left untouched — s2v resolves those
correctly (Fable measured them non-degenerate).
"""
import re, sys

src = sys.argv[1]
dst = sys.argv[2]
with open(src) as f:
    svg = f.read()

def parse_matrix(m):
    nums = [float(x) for x in re.findall(r'-?[0-9.eE]+', m)]
    return nums  # a b c d e f

def apply(mat, x, y):
    a, b, c, d, e, f = mat
    return (a * x + c * y + e, b * x + d * y + f)

def fmt(v):
    return f"{v:.3f}".rstrip('0').rstrip('.')

def bake_grad(tag):
    attrs = dict(re.findall(r'(\w+)="([^"]*)"', tag))
    gid = attrs.get('id', '')
    gt = attrs.get('gradientTransform')
    if not gt:
        return tag  # nothing to bake (plain userSpaceOnUse or objectBoundingBox)
    mat = parse_matrix(gt)
    is_radial = tag.startswith('<radialGradient')
    if is_radial:
        a, b, c, d, e, f = mat
        cx, cy, r = float(attrs['cx']), float(attrs['cy']), float(attrs['r'])
        if abs(a - d) < 1e-6 and abs(b) < 1e-6 and abs(c) < 1e-6:
            ncx, ncy = apply(mat, cx, cy)
            nr = abs(a) * r
            new = f'<radialGradient id="{gid}" cx="{fmt(ncx)}" cy="{fmt(ncy)}" r="{fmt(nr)}" gradientUnits="userSpaceOnUse">'
        elif gid == 'gRule':
            new = f'<radialGradient id="{gid}" cx="515" cy="640" r="235" gradientUnits="userSpaceOnUse">'
        else:
            raise SystemExit(f"non-uniform radial {gid} not handled")
        return new
    else:
        x1, y1 = float(attrs['x1']), float(attrs['y1'])
        x2, y2 = float(attrs['x2']), float(attrs['y2'])
        nx1, ny1 = apply(mat, x1, y1)
        nx2, ny2 = apply(mat, x2, y2)
        new = f'<linearGradient id="{gid}" x1="{fmt(nx1)}" y1="{fmt(ny1)}" x2="{fmt(nx2)}" y2="{fmt(ny2)}" gradientUnits="userSpaceOnUse">'
        return new

def repl(m):
    return bake_grad(m.group(0))

svg = re.sub(r'<(?:linear|radial)Gradient[^>]*>', repl, svg)
with open(dst, 'w') as f:
    f.write(svg)

# report residual transforms + degenerate axes
resid = re.findall(r'gradientTransform', svg)
print(f"residual gradientTransform: {len(resid)}")
for g in re.findall(r'<(linear|radial)Gradient[^>]*>', svg):
    a = dict(re.findall(r'(\w+)="([^"]*)"', g))
    if 'x1' in a:
        dx, dy = float(a['x2']) - float(a['x1']), float(a['y2']) - float(a['y1'])
        L = (dx * dx + dy * dy) ** 0.5
        if L < 2:
            print(f"  DEGENERATE linear {a.get('id')}: axis len {L:.3f}")
