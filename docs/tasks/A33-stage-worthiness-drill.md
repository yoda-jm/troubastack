# A33 — Stage-worthiness hardening drill (roadmap #2, VLL green-light)

**Priority:** high · **Size:** M (scaffolding that pays forever) · **Area:** app +
a checklist doc + scripted scenarios. Mobile lane. The product's point is the 90
minutes on stage; every real-device drill this month found exactly one true bug —
this systematizes that.

## The drills (each: script it where possible, run on the tablet, file findings)

1. **Airplane mode mid-set**: toggle mid-song; page turns, layers, cues, drawer
   all keep working (I12); live-mode toggles degrade silently (no error toasts
   mid-performance); recovery when connectivity returns.
2. **Process death → restore INTO the page**: kill the app mid-song (adb), reopen
   → land back in the same concert, same song/page/fit (nav survives per A27/A31;
   Stage POSITION should too — likely the finding: add rememberSaveable/KV for
   the stage position).
3. **Battery saver + screen-dim**: throttled schedulers vs the P201 poll + N9
   prefetch; keep-screen-on policy in Stage (is there one? probably the finding).
4. **Big bundle**: a generated 25-song/60-page bundle — open time, drawer scroll,
   memory (the A19/T44-era budgets under real load), turn latency budget.
5. **Reconnect storm during live mode**: server restarts mid-rehearsal; the poll
   loop + probe recover; no duplicate-apply (R10 remap under repeated rev bumps).
6. **Storage pressure**: import with low disk; a clean failure, never a corrupted
   bundle (the atomic-swap promise under its real failure mode).

## Progress (2026-08-25)

First session: **drill 2 → finding (A46)**; **drill 3's predicted finding DISPROVED** (keep-screen-on
exists on both platforms). Drills 1, 4, 5, 6 not yet run. Live sheet:
`docs/handoff/STAGE-WORTHINESS.md`. **Drill 1 cannot be driven over wireless adb** — airplane mode kills
the connection the harness runs on; use USB or a self-recovering script.

## Output

- `docs/handoff/STAGE-WORTHINESS.md`: the checklist with per-drill PASS/finding,
  re-runnable (the ACCEPTANCE-P205 pattern).
- One gate-filed finding per defect (small fixes may ride the drill branch with
  the finding documented; anything structural comes to me first).
- The bundle-generator script for drill 4 committed (useful forever).

## Acceptance

All six drills executed on the real tablet with recorded results; findings filed
or fixed; the checklist committed; no drill left "assumed".
