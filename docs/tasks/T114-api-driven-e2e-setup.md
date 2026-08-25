# T114 — Set up over the API, click only what's under test

**Priority:** last of the audit pack — after T112 and T110 · **Size:** M · **Area:** `web/studio/e2e`
(Web & Core lane). Split out of **T108** at the gate on 2026-08-25: T108's §2 listed this as part (b),
Web-Core deliberately deferred it, and the reviewer agreed and re-filed it here. **T108 is the baseline
this measures against.**

## 1. Why it's a separate task

T108 consolidated the setup into `e2e/setup-helpers.ts` and was *provably* behaviour-neutral — 169
assertions relocated, reconciling 77/51/41 against the helper, zero gained. That proof is only possible
because nothing about **what gets clicked** changed.

This task changes exactly that, and it carries the runtime win T108 correctly declined to claim.

## 2. What to build

**Create fixture state over `page.request` wherever the flow is not what the spec is testing.** A spec
that needs *a user with a band and a song* should get one over the API in a second; only a spec that is
*about* registering, creating a band, or creating a song should click through it.

Also finish the piece T108 stated as partial: **`uploadPdf`** is exported from the helper but 47 specs
still carry local copies, held back by four behavioural variants and a `PDF_PATH`/`fileURLToPath`
cascade. Converge them, or state which stay and why.

## 3. Rules

- **The failure mode here is coverage, not correctness.** A spec whose setup moves to the API stops
  exercising the UI path it used to walk. That is the *point* — but it means the flows that lose their
  last UI walker must still be covered somewhere deliberate. Before converting a path wholesale, check
  what still clicks it; if the answer is nothing, keep one spec on the UI and say which.
- **Keep the assertions.** Setup assertions that move to the API should assert the API's answer (a 200,
  a returned id), not vanish. T108's arithmetic must still close: relocated, not dropped.
- **Session state is the trap.** Registering over the API must leave the page as authenticated as the UI
  path did — cookie, origin, and any client-side state the app reads on first render. A spec that starts
  half-logged-in fails in ways that look like product bugs.
- **The count is 206.** If it moves, say why in the same sentence you report it.
- **No sleeps.** T93 removed all 39 `waitForTimeout`s; API setup is faster and therefore more tempting to
  paper over with one. Wait for observable state.

## 4. Acceptance criteria

- Fixture state is created over the API in the specs where the UI flow isn't under test; the specs that
  *are* about those flows still click them, and are listed.
- Any flow that loses its last UI walker is named, with what covers it now.
- `uploadPdf` converged, or the remainder listed with reasons.
- **Before/after full-suite wall-clock, both measured**, against T108's consolidated suite as the
  baseline. This is the task where a speedup claim is legitimate — so it must be a measurement, not an
  argument.
- Full `make e2e` green, count reconciled against 206.

## 5. Out of scope

`retries`/`workers` changes and a smoke/full split — worth doing once this task's runtime number exists,
not before. Adding or changing assertions beyond relocating setup ones.
