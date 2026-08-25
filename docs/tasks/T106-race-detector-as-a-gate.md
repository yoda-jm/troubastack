# T106 — Run the race detector, and make it a gate

**Priority:** FIRST of the audit pack (T106–T113) · **Size:** S–M · **Area:** `core/`, `.github/` (Web & Core lane).
From the 2026-08-25 project audit, finding C4. VLL picked this first, and it is the right first move: it
may turn CI red, so everything else in the pack should land on top of it, not underneath.

## 1. Why this is a gate task, not a fix task

The audit reports a data race on `conn.dropped` — written under `r.mu` in `room.broadcast`
(`sync.go:157`), read without a lock in `readPump`'s teardown (`conn.go:221`), guarding a
`close(c.send)`. **Do not start by "fixing" that.** Reviewer's reading of main, before filing this:

`readPump`'s defer calls `c.hub.unregister(c)` *first*, and `unregister` (`sync.go:121`) acquires the
same `r.mu` and deletes `c` from `r.conns`. So every write to `c.dropped` either happens-before
`unregister` returns — and therefore before the read — or cannot happen at all, because `broadcast`'s
range no longer reaches `c`. The package's own comment at `sync.go:59` asserts exactly this
("register/unregister/broadcast never races the pumps"). The second send site, `sendTo`, already
guards with `recover()`.

**So the interesting question is empirical, not architectural, and the tool settles it.** That is the
task: install the arbiter, run it, and report what it actually says.

## 2. What to build

**(a) The gate.** `-race` in `make test` and in the CI `go` job. This is the deliverable even if it
finds nothing.

**(b) The report.** Run the full Go suite under `-race` and bring the *output* to the gate before
fixing anything. A known suspect: `TestBake_ConcurrentSameSetlist_distinctRevs` has flaked in CI for
weeks and is believed to be a real `baker.go` race — check it first on any red.

**(c) The fixes** that (b) actually justifies.

**(d) `conn.dropped`, regardless of what the detector says.** If `-race` is silent, the flag is still
*emergently* safe: its correctness depends on three separate facts holding together (the defer calling
`unregister` first, `unregister` taking `r.mu`, the delete happening there). Nothing states that at the
read site and nothing tests it — move one line and it becomes a double-close panic that takes the
process down. Make the safety **local** (an atomic, or close under the same lock that sets the flag) or
**pinned** (a test that reddens if the teardown order changes). Emergent is not good enough for a
panic path.

## 3. Rules

- **Report before you fix.** A race the detector names is worth more than a race either of us reasoned
  about. If the suite is green under `-race`, say so plainly — that is a real result, not a failure.
- **Do not loosen a test to get green.** If `-race` reddens something, the race is the bug.
- `-race` slows tests substantially; adjust CI `timeout-minutes` rather than trimming coverage.
- The Go CI job gofmt-gates *after* vet/test — run `gofmt -l core` before landing.

## 4. Acceptance criteria

- `make test` and the CI `go` job both run with `-race`.
- The full `-race` run's summary line is posted at the gate, pass or fail.
- Every race the detector reported is either fixed or filed with a named reason for deferring.
- `conn.dropped`'s safety is local or pinned by a test, and the comment at the read site says which.
- `gofmt -l core` clean.

## 5. Out of scope

The other audit findings; performance work in `filestore`/`gitstore`; anything in the app.
