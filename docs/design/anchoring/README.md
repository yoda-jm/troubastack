# Annotation anchoring: spec, critique and red-first tests (proposal, awaiting VLL)

Written 2026-10-11 at VLL's request ("une vraie documentation… analyser de façon critique"), and reviewed by
Fable. **Nothing here is decided yet.** VLL chooses between rigid follow-by-line (recommended) and
never-link-plus-flag, then answers the six questions in `critique.md` §8.
- `anchoring-spec.md`: Part A is today's behaviour, with code citations; Part B is the proposed rigid model.
  Properties are numbered P1… .
- `critique.md`: every option, cost, failure mode and test surface, the recommendation, and the staged plan.
- `coverage.md`: shape × event matrix → properties → existing tests and gaps.
- `red-first-baseline.md` and `red-first.patch`: 225 executable cells against main `a936c93d`. Fable re-ran
  them: 85 pass and 140 red with `ANCHOR_RED=1`; without it, the red cells skip and CI stays green. The patch
  lands after VLL's choice, because its assertions follow option G.
- `explication-fr.md`: the plain-French version for VLL (also kept as his private doc).

T191 and T192 stay on hold until the decision (see `docs/handoff/reviews.md`).
