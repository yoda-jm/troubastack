# T117 — One flake should not red the push

**Priority:** normal · **Size:** M · **Area:** `web/studio` (Web & Core lane).
The half of the audit's §4.3 testing bullet that T110 did **not** take. T108 and T114 both deferred it
explicitly, on the grounds that it should wait until the suite's runtime was known. **It is now known:
19.0 min, 199 tests** (T114's measurement).

## 1. The problem

`playwright.config.ts:25,27` — `workers: 1`, `retries: 0`. One flaky test reds the whole push, and the
suite is a 19-minute serial run, so the cost of that is a 19-minute re-run to learn nothing.

The audit named this alongside the sleeps, and it survived the two deflake passes because those attacked
the *causes* of flake. This one is about what the suite does when a flake happens anyway.

## 2. What to build

**(a) `retries` above zero** — and this is the part that needs care, see the rules.

**(b) A smoke/full split.** A fast subset that runs on every push and the full suite on whatever trigger
you argue for. Which tests are "smoke" is a judgement call: propose the criterion, not just the list.

## 3. Rules — this task's danger is the opposite of a deflake's

- **`retries` can hide a real flake.** A test that passes on retry is not a passing test; it is a
  flaky test that stopped telling you. Whatever retry count you choose, the run must still make a
  retried pass **visible** — a reporter line, an annotation, something a human sees. A green tick that
  silently swallowed a retry is worse than the red it replaced.
- **Say what retries are FOR.** If the answer is "so CI is less annoying", that is a reason to fix the
  flake instead. If it is "so an infrastructure blip doesn't cost 19 minutes", that is legitimate — and
  it implies retries belong on CI and not necessarily locally.
- **A smoke subset that never fails is worthless.** The criterion must be defensible: what class of
  regression would the smoke set catch, and what would only the full run catch? State it.
- Don't touch `workers` without measuring — the suite's serialisation may be load-bearing for the shared
  fixture ports.

## 4. Acceptance criteria

- Retry behaviour chosen with a stated rationale; a retried pass is visible in the output, demonstrated.
- Smoke/full split with the selection criterion written down, plus what each catches.
- Measured: smoke wall-clock, full wall-clock, and the count each covers.
- Full `make e2e` green, count reconciled against 199.

## 5. Out of scope

New tests. Deflaking (done — 32 sleeps remain, all documented). Parallelism changes beyond what a
measurement justifies.
