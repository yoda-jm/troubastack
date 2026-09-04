# T128 — the bake toolchain fails at the gig, not at boot

**Lane:** web-core (`core/internal/config`, `core/cmd/troubacore`, `deploy/`). **Size:** S/M.
**Status:** **CODE LANDED** — 4 commit(s), `ac5f5f4a`…`1dab9311` (last 2026-09-03). This line previously said "spec, not started"; it is a SECONDARY copy of a fact the review gate owns (`docs/handoff/reviews.md`), and it rotted. Corrected 2026-09-04 from the git history. **Not re-verified against this spec's own done-when** — "code landed" is what was checked, not "every criterion met".
**Asked by:** VLL, 2026-09-02 — after hitting it on his own running instance, hours before it
mattered.

## What happened, and why it is a product bug rather than a misconfiguration

A core running from a directory that is not the repo's `core/` had every bake fail with
*"The annotation renderer isn't available on the server. Ask an admin to check the bake setup."*
The cause, from the server log:

```
web/bake worker (node ../web/bake/dist/cli.js): exit status 1:
  Error: Cannot find module '/home/<user>/web/bake/dist/cli.js'
```

`TROUBA_BAKE_CLI` was unset, so the built-in default won —
`"../web/bake/dist/cli.js"` (`core/internal/config/config.go:114`), whose own help text admits the
shape of the trap: *"repo-relative default works when core runs from core/"*. Resolved against the
process's **working directory**, that default points at a sibling of `$HOME` on any real
deployment.

Two things make this worth fixing rather than documenting again:

1. **`deploy/README.md:96` already says to point `TROUBA_BAKE_CLI`/`TROUBA_NODE`/`TROUBA_PDFTOPPM`
   at the host's copies**, and the `Dockerfile` already sets `TROUBA_BAKE_CLI=/app/web/bake/dist/cli.js`
   (`:73`). So the docs are not wrong and Docker users are fine. **The gap is the bare-binary
   self-hoster** — which is precisely who the project page now invites (*"a home server or a cheap
   VPS is plenty"*).
2. **Nothing checks any of it at startup.** `main.go:355` passes `cfg.Bake.CLI` straight through;
   there is no `Stat`, no `LookPath`, no log line. Grep for it: the only references to `Bake.CLI`
   and `Bake.Pdftoppm` outside tests are the config table and that one assignment. So a server can
   run for weeks looking healthy and only reveal that it cannot bake **the first time somebody
   tries to bake** — which, for this product, is the evening before a concert.

## Work

### 1. Resolve the default relative to the binary, not the shell

Replace the single cwd-relative default with an ordered candidate search, tried **only when the
operator has not set the value**. First existing file wins:

1. `<exeDir>/bake/dist/cli.js` — the renderer shipped next to the binary. Make this the
   **documented** non-Docker layout.
2. `<exeDir>/../web/bake/dist/cli.js` — the repo layout, for `core/bin/troubacore`.
3. `<cwd>/../web/bake/dist/cli.js` — **today's behaviour, kept last** so nothing that works now
   breaks.

If none exists, keep the last candidate as the value, so the error still names a concrete path
rather than an empty string.

**The design constraint to settle first:** CFG01's precedence machinery
(defaults < INI < `TROUBA_*` < flags, `config.Load`) hands back a *value*, not its *provenance* —
so "the operator set this" and "this is the built-in default" are not currently distinguishable.
Either thread provenance through, or run the search only when the resolved value is byte-equal to
the built-in default string. The second is simpler and acceptable; if you take it, **say so in a
comment**, because an operator who explicitly sets the exact default string will silently get the
search. Do not leave that as an undocumented accident.

**⚠ Ripple:** `core/internal/bake/rendercache.go:325` derives `inkVersion` from the **sha256 of the
deployed cli.js**. Changing which file gets found changes the cache key — that is correct (a
different renderer must not reuse another's cache) but it means existing render caches go cold on
upgrade. Name it in the commit message; do not let an operator discover it as a mystery slowdown.

### 2. Check at boot, and check by *running* it

Add a preflight at server startup that resolves `pdftoppm` (PATH lookup), `node`, and the bake CLI,
and logs **one line naming the absolute resolved path** of each.

**A `Stat` is not sufficient, and this is not a hypothesis.** On the incident above, the second
failure after the file was in place was `Cannot find native binding` — `@napi-rs/canvas` resolves
from the `node_modules` *next to the cli.js*, so the file existed and was still unusable. A stat
would have reported everything fine. **The preflight must spawn the worker** (`node <cli>` with no
arguments prints its usage line and exits non-zero) under a short timeout, and treat "the usage
line came back" as the pass condition.

**Warn and continue — do not refuse to start.** A core that cannot bake still serves charts, and
on the night of a gig a server that boots degraded beats a server that will not boot at all. The
warning must be loud, must name the absolute path, and must say which env var or INI key fixes it.

Note the interaction with **BRAND04**, which specs one identifying startup line (product, version,
page URL). These are two different lines with two different jobs; keep both, and keep each to one
line. Neither is a banner.

### 3. Document the supported layout

`deploy/README.md`'s systemd section becomes concrete: build the worker, copy `web/bake/dist`,
`web/bake/assets` and the required `node_modules` **next to the binary** as `bake/`, and the default
then resolves with no env var at all. State the layout explicitly, because it is load-bearing:

- `bake/dist/cli.js` loads `../assets/Roboto-Regular.ttf` **relative to itself**;
- `@napi-rs/canvas` is deliberately **external** to the bundle (native addon) and is resolved from
  the nearest `node_modules` — and it needs **both** `@napi-rs/canvas` **and** the platform package
  `@napi-rs/canvas-linux-x64-gnu`. Copying only the first is the "Cannot find native binding"
  failure above.

## Done when

- With `TROUBA_BAKE_CLI` unset, a binary run from an arbitrary working directory with `bake/` beside
  it **bakes successfully** — prove it by baking a real setlist and opening the resulting `.tstage`,
  not by reading the resolver.
- With the renderer absent, the server **still starts**, logs one warning naming the **absolute**
  path it looked for and the key that overrides it, and a bake still fails with T102's wording
  unchanged.
- With the cli.js present but its `node_modules` incomplete, the **preflight fails** — this is the
  case a `Stat`-only check would pass, and it is the one that actually happened.
- An explicitly set `TROUBA_BAKE_CLI` is used verbatim, with **no** search — assert it with a path
  that does not exist, so a silent fallback would be visible.
- **A discriminating unit vector:** a fixture where only candidate **#2** exists, so "always take
  the first candidate" and "take the first that exists" give different answers. A vector where the
  correct and the naive-wrong answers agree guards nothing.
- **Teeth-check:** restoring the plain `"../web/bake/dist/cli.js"` default makes the new test
  **fail**. Prove the guard by reintroducing the regression, not by editing the assertion.
- `gofmt -l core` clean, `go vet`, `make test` green. **Match the count.**

**One thing to notice while you are in here.** `core/internal/bake/overlay_skip_test.go:67,162`
reads `TROUBA_BAKE_CLI` **directly from the environment** and falls back to its own hard-coded
`"../../../web/bake/dist/cli.js"`, bypassing `config` entirely — and skips when that file is
absent. So the new resolver will not reach those tests, and, more to the point, **the suite never
exercised the resolution path that broke.** A green `go` job said nothing about whether a deployed
core could find its renderer, and after this task it still will not unless the resolver itself is
tested directly. That is why the discriminating vector above is the load-bearing part of this
task, not a formality.

## Out of scope

- Bundling or vendoring Node, or embedding the renderer in the binary.
- Changing the user-facing bake error text — T102 got that wording right and it is what surfaced
  this bug correctly.
- A `doctor` subcommand. The startup line is the whole of the diagnosis this task owes.
