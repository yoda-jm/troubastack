# web/ — TroubaStack browser/JS workspace

An npm workspace holding **all** browser/JS code. Three packages, one hard rule:
**stroke rendering exists exactly once.**

> Read the constitution first: [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).
> Everything here *derives from* its invariants (I1…I15). If code and an invariant
> disagree, the code is wrong.

## Workspace map

```
web/
├── ink/      @troubastack/ink   — THE one stroke renderer. [I8]
├── studio/   @troubastack/studio — TroubaStudio, the canonical editor SPA. [I10]
└── bake/     @troubastack/bake   — Node bake worker; server overlay raster. [I8]
```

Shared wire/domain types are **generated** into `web/proto-gen/` from `../proto/`
(I1) — never hand-written, git-ignored, regenerated in CI. Each package depends on
`proto` and (for studio/bake) on `ink`; **no package imports another client layer** (I14).

## The no-duplication rule (I8)

`@troubastack/ink` is the **single** stroke renderer (points/handles → geometry → draw
onto a canvas / `OffscreenCanvas`). It is consumed by **both**:

- **`studio`** — runs in the **browser** (dry layer + in-browser wet ink),
- **`bake`** — runs in **Node** (server-side transparent annotation overlays).

The mobile app's **native ink overlay is the only sanctioned re-implementation**, and it
must render **pixel-identically** — guarded by a golden-image **parity test**. There is
**no third copy**: Go never draws strokes (core's bake step calls `bake` → `ink`).
If you are about to copy rendering logic out of `ink`, stop.

## Studio is canonical (I10)

`studio` is the **complete** editor and runs standalone in any browser. The mobile app
**embeds it in a webview**; it is **never reimplemented natively**. The native overlay is
a feature-detected accelerator for wet stylus ink (I9) with a full in-browser fallback —
the in-browser wet path **always exists**.

## Build & serve (I14)

`studio` builds (Vite) to **static SPA assets**, which `core` embeds (`embed.FS`) and
serves directly — **no Node runtime in production**. `bake` is the *one* legitimate
server-side JS runtime: a Node worker that `core` invokes at bake time.

```
npm install      # not run yet — no network in this scaffold
npm run build    # builds all workspaces (--if-present)
```

## Toolchain

Node 24+ / npm 11+.
