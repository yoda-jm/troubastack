# T119 — Test what only a DOM can reach

**Priority:** normal, after T116 · **Size:** M · **Area:** `web/studio` (Web & Core lane).
The half T110 scoped out. T110 chose `environment: "node"` deliberately — *"a node environment is enough
— the first suite is pure functions, no DOM"* (`vitest.config.ts`) — which was right for a first suite
and leaves a named gap.

## 1. The gap, concretely

The clearest example is one this pack created. `RouteErrorBoundary` (T112) is covered only at
`getDerivedStateFromError` — the static method that decides. **Nothing asserts the error screen actually
renders**, and the e2e suite structurally cannot reach it: a lazy-chunk fetch failure isn't something the
dev server produces on demand.

So the honest position today is: if that component's `render()` returned `null`, every test would pass
and a user on dropped Wi-Fi would get the blank page T112 exists to prevent.

## 2. What to build

**(a) A jsdom test environment** alongside the node one — not replacing it. The pure-function suite
should stay in node; it's faster and it proves it needs nothing.

**(b) Component tests for what only a DOM can assert**, starting with `RouteErrorBoundary`: given a
child that throws, the error screen renders, says something honest, and offers the reload affordance.

**(c) Pick the rest by the same bar T110 used** — where a wrong answer is silent. A component that
renders the wrong thing is silent in a way a pure function usually isn't, so this is where the bar bites.

## 3. Rules

- **Don't duplicate e2e.** If Playwright already walks it against a real server, that is better coverage
  than jsdom. This suite is for what e2e *cannot reach* — error boundaries, failure states, branches
  behind a network condition you can't stage.
- **Per-test teeth-check, reported**, same as T110.
- **Keep the two environments separate and both fast.** If the jsdom suite starts costing minutes, say so
  — the whole argument for unit tests here was 1.4s against 19 minutes.
- **A render test asserts what the user sees, not that a component mounted.** "Renders without crashing"
  is the canonical hollow guard.

## 4. Acceptance criteria

- jsdom environment added without moving the existing node suite.
- `RouteErrorBoundary`'s render path covered: throwing child → honest error screen + reload affordance.
- Teeth-check per test reported (including: making `render()` return null reddens it).
- Both suites run from `make` and in CI; combined wall-clock reported.

## 5. Out of scope

Visual regression / `toHaveScreenshot` — separate, and the audit prices it separately. Coverage
measurement. Testing library choice is yours; don't spend the task on it.
