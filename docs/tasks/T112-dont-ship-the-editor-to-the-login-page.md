# T112 — Don't ship the editor to the login page

**Priority:** normal, **after T108** · **Size:** S · **Area:** `web/studio` (Web & Core lane).
From the 2026-08-25 project audit, §4.3 frontend delivery. VLL also reports the build itself is
complaining — `vite.config.ts` sets no `manualChunks` and no `chunkSizeWarningLimit`, so the default
500 kB warning is firing on a 755 kB chunk.

## 1. The problem

Every visitor downloads a **755 kB JS chunk plus a 1.34 MB pdf.js worker** — including someone who
only reaches `/login`. There is no `React.lazy` and no `manualChunks` anywhere. This is a product
designed to be served off a garage laptop over band-practice Wi-Fi, to phones and tablets, and the
first screen costs ~2 MB before anyone has typed a password.

## 2. What to build

**(a) Route-level splitting.** `React.lazy` the heavy routes — the editor/viewer first — behind
`Suspense` with a deliberate fallback.

**(b) Get pdf.js off the initial path.** It is only needed once a PDF is actually opened.

**(c) A vendor chunk** so app code and dependencies cache independently across deploys.

**(d) Then set `chunkSizeWarningLimit` honestly** — to a number the build now meets, not one chosen to
silence it.

## 3. Rules

- **Measure, don't assert.** Report the initial-load bytes for `/login` before and after, from the real
  build output. "Smaller" is not a result; a number is.
- **A lazy boundary is a new failure mode.** A chunk that fails to load must not leave a blank screen —
  decide what the fallback and the error path look like.
- **The e2e suite is the safety net and it must stay green** — count reconciled (206, or T108's number
  if that lands first). Watch for specs that raced ahead of a `Suspense` boundary; a fixed sleep is not
  the fix (T93 removed all 39 of them; don't reintroduce one).
- Don't restructure routing itself — this is a loading change, not an IA change.

## 4. Acceptance criteria

- `/login` initial transfer measured before and after, both numbers reported.
- pdf.js is not fetched until a PDF is opened.
- Build emits no chunk-size warning, with the limit set to a met number.
- Full `make e2e` green, count reconciled.
- A failed chunk load shows something honest, not a blank page.

## 5. Out of scope

Bundle-analyzer tooling as a permanent gate; service workers; SSR; image/asset optimization.
