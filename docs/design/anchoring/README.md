# Annotation anchoring: spec, critique and red-first tests — DECIDED 2026-10-11

Written 2026-10-11 at VLL's request ("une vraie documentation… analyser de façon critique"), and reviewed by
Fable. **VLL decided on 2026-10-11:** option **G**, rigid follow-by-line with flags, and **yes to every
recommendation** in `critique.md` §8: margin marks follow vertically, multi-line marks never stretch
(flagged instead), marks scale with the type size, freeze now. The states are Linked / Check / Lost / Fixed.
Stage 0 is T193; stages 1–4 follow as their own tasks.
- `anchoring-spec.md`: Part A is today's behaviour, with code citations; Part B is the proposed rigid model.
  Properties are numbered P1… .
- `critique.md`: every option, cost, failure mode and test surface, the recommendation, and the staged plan.
- `coverage.md`: shape × event matrix → properties → existing tests and gaps.
- `red-first-baseline.md` and `red-first.patch`: 225 executable cells against main `a936c93d`. Fable re-ran
  them: 85 pass and 140 red with `ANCHOR_RED=1`; without it, the red cells skip and CI stays green. The patch
  lands after VLL's choice, because its assertions follow option G.
- `explication-fr.md`: the plain-French version for VLL (also kept as his private doc).

T191 is superseded: stage 2 reuses its apply.go seam. T192 becomes stage 3, simplified to the four states.
