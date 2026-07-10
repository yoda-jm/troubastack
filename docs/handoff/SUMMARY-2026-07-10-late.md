# Summary — Friday 2026-07-10, late session (architect/reviewer)

*Picks up where [`SUMMARY-2026-07-08-to-10.md`](SUMMARY-2026-07-08-to-10.md) ends
(that digest closed with the 07-10 audit that filed T31). Every claim below is backed
by a verdict in [`reviews.md`](reviews.md); commits are as-landed on `main` (linear,
CI-green — `514e8bd` and `1d545ac` confirmed all five jobs green, `f67d696` watched).*

## TL;DR

Two arcs closed in one afternoon. **T31** (the audit's HIGH find: the bake ignored
per-object z-order, so studio and Stage could disagree on stacking) was fixed
test-first by the architect per VLL — and the web-core lane, racing on the same task,
independently produced a **functionally identical patch**, which is corroboration,
not conflict. Then the **a11y viewport arc**: the lane executed audit note #3
(zoomable by default, `user-scalable=no` scoped to the editor route — WCAG 1.4.4),
survived an honestly-reported push slip that landed it a verdict early, got a KEEP
ruling on re-verified merit, and closed the ruling's one condition with a committed
drift-guard e2e that improved on the reviewer's own scratch spec. Plus one piece of
review-infra hardening: both CI watchers were found silently dead and fixed.

## T31 — bake z-order parity (`514e8bd` + docs `c567cda`)

- **The defect:** T27 stage 2 gave objects a within-layer z-order
  (`order → createdAt → uuid`, `compareObjectZ`), but `web/bake` still rendered each
  layer's objects in document/API order — its own comment claimed "matching studio's
  dry layer", which stage 2 had silently invalidated. A studio bring-to-front was
  therefore absent from the baked `.tstage` (the I8 class).
- **Test-first:** `web/bake/test/zorder.test.mjs` (pixel assertions on the rendered
  overlay PNG — order-inverts-document-order + createdAt/uuid tiebreaks) was written
  first and run RED against unfixed code (**pass 0 / fail 2**), then green post-fix.
  Full bake suite incl. the B01 golden pixel-parity stayed green.
- **The fix:** an `objectZ` comparator in `render.ts` mirroring `compareObjectZ`
  exactly, applied per (layer, page) bucket; stale comment corrected. Bake-local (the
  REST DTOs already carry `order`/`createdAt`); Kotlin/Stage untouched.
- **The race:** VLL had steered both the architect ("go ahead with T31") and the lane
  at T31 in the same hour. The lane's `aef7da2` arrived minutes after `514e8bd`
  landed — diffed: same comparator contract, same sort site, same pixel-test shape;
  only cosmetic type-export deltas. Two independent implementations converging on the
  same patch. The lane spotted the landing and deleted their branch; no work lost.

## The a11y viewport arc (audit note #3 → closed the same day)

1. **The change** (`50e0ce8`, lane): `index.html` ships the zoomable viewport default
   (the global `user-scalable=no` was a WCAG 1.4.4 barrier on the management pages,
   which have no in-app zoom); `Shell` re-adds the clamp dynamically only on the
   editor route (same predicate as `fullbleed`), where the T27 stage-4 pinch owns the
   gesture.
2. **The push slip, honestly recovered:** the lane meant to push only its gate-claim
   doc, but `git checkout main` fails **silently** in their checkout (`main` is held
   by the architect's review worktree) and the follow-up `push origin HEAD:main`
   carried the code commit to main under the very doc claiming it was held. Caught
   and reported by the lane within minutes, with root cause and a keep/revert ask.
   Lesson logged: never `checkout main` in this repo — it's worktree-locked.
3. **Ruling: KEEP** (`1d545ac`) — re-verified on merit before ruling: predicate
   byte-identical to `fullbleed`'s, and a scratch Playwright spec on the isolated
   stack walked the five-leg contract (management zoomable × 2 · editor clamped ·
   SPA back-nav restores · hard-load directly on the editor clamps on mount) —
   1 passed, `tsc -b studio` clean. Note-as-spec ruled sufficient for this XS
   (design was fully decided in the note). Explicitly **not** precedent for landing
   ahead of a verdict. Fix-forward in the same commit: `50e0ce8`'s stray root-level
   `test-results/.last-run.json` removed; root `.gitignore` now covers
   `test-results/`/`playwright-report/` at any depth.
4. **The guard e2e** (`f67d696`, approved in `3c749dd`): `viewport-a11y.spec.ts`
   gates all five legs — and is better than the reviewer's scratch spec
   (`expect.poll` absorbs the Shell-useEffect timing; the scratch version's
   synchronous read was a latent flake). Re-run independently: 1 passed;
   direct landing was authorized by the ruling itself ("fix-forward, not blocking").
5. **Device caveats recorded** (ride the attended T27 pass): iOS ignores
   `user-scalable=no` entirely (stage-4's `touch-action` is the real guard there);
   enter-while-browser-zoomed on Android Chrome unverified on hardware.

## Review-infra: the silently dead CI watchers

Both one-shot CI monitors this session produced zero events and timed out. Two
independent faults, both invisible under `curl -sf` + unguarded parsing: `jq` is not
installed on this box (parse with `python3 -c` instead), and the remote URL is the
token-only form (`https://TOKEN@github.com/...`) so a `user:token`-shaped sed
extraction silently yielded an empty token. Fixed, re-armed, and the lesson
("smoke-test one API query by hand before arming any CI monitor") recorded. The CI
verdicts above were backfilled by hand the moment the fault was found — nothing had
actually gone red.

## Decisions made

| Decision | Call |
|---|---|
| T31 lane race | Keep the landed fix; identical parallel patch = corroboration; noted, branch pruned by the lane |
| a11y early landing | KEEP on re-verified merit; honest self-report + immediate escalation is the model recovery; not precedent |
| Note-as-spec | Sufficient for XS changes whose design is fully decided in the audit note; ask first if a decision is open |
| Artifact hygiene | Root `.gitignore` now ignores Playwright output at any depth (janitorial fix-forward by the architect) |

## Current state

**Open (unchanged otherwise):** T26 app half + T23 drawer grouping + B06 app half
(mobile lane) · OPS01 (urgent per the audit) · P20x. **Attended:** T27 device pass
(now also carrying the two viewport device caveats) · T24 · B07 screenshot pair.
**On Vincent:** A07 stylus spike, IOS03 Mac + Apple ID, credential rotation, OPS01/TLS.

*Verdicts: `reviews.md` (five new entries this session). Queue:
`docs/tasks/README.md` § "Queue state".*
