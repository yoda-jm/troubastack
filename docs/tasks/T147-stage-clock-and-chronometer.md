# T147 — A clock and a chronometer in Stage

**Lane:** mobile (A-track). **Size:** S/M. **Status:** spec, 2026-09-05. Enhancement #8 from the
rehearsal field report.

## What VLL asked for

*"dans Stage un chronometre (start/pause/reset) et une horloge"*, with the clock optionally **kept visible
bottom-right** during performance.

This came out of a real rehearsal, and it is worth reading literally: a band rehearsing to a schedule
needs to know **what time it is** and **how long this run-through has taken**, without leaving the sheet
they are playing from.

## Required

**The clock.**

- Current time, on the performance surface, **bottom-right**, toggleable.
- It must not steal the page or shift the music — it is an overlay, never part of the layout flow.
- Legible at arm's length in the dark, consistent with the existing performance palette. It is the
  smallest possible element that still reads across a room, not a decorative widget.

**The chronometer.**

- **start / pause / resume / reset.**
- Survives navigation between songs — it times the *session*, not the page.
- Survives a bundle update: `applyUpdate` swaps the bundle without moving the page (see T143), and it must
  not reset a running chrono either.

## The state question, which is where this will actually go wrong

A chronometer is a small state machine and a wall clock, and Android will suspend it. **Store a start
instant and accumulated elapsed, not a tick counter**, so the value stays correct across process death,
screen-off, and a configuration change. A chrono that quietly loses ten minutes because the tablet slept
is worse than no chrono — the musician would rather see nothing than a number they must not trust.

## ⟨R1⟩ Red first

- **The state machine**, tested without a UI: `start → pause → resume → reset` returns to `00:00`; a
  double `start` does not restart; `pause` twice does not double-count. Each of these must be **seen
  failing** against a stub that does the naive thing.
- **The suspend case, which is the one that matters:** advance the clock source by ten minutes with the
  chrono paused, and assert the elapsed value is **unchanged**; do the same while running and assert it
  advanced by exactly ten. Use an injectable time source — a test that sleeps proves nothing and is flaky.
- **Non-disruption:** toggling the clock does not change the page index or the rendered page's geometry.

**Teeth-check:** a tick-counter implementation must fail the suspend test. If it passes, the time source
is not being advanced the way a suspend actually behaves, and the test is guarding nothing.

## Acceptance

- Clock toggles, sits bottom-right, and does not move the music.
- Chrono survives song navigation, a bundle update, and a suspend.
- The state-machine and suspend tests were seen red first.
- Compile the iOS target before landing if any `shared` code is touched (`:shared:compileKotlinIosSimulatorArm64`).

## Out of scope

A metronome, and anything that makes sound. This is a display.
