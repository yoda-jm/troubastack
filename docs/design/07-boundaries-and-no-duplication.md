# Design: Boundaries, separation of concerns, no duplication

Derives from **I1, I8, I10, I14, I15**. This doc is the "what may depend on what / what may be
copied" rulebook.

## Layering (I14)

```
            ┌─────────┐
            │  proto  │   the contract — imports nothing of ours
            └────┬────┘
     ┌───────────┼───────────┬───────────┐
     ▼           ▼           ▼           ▼
  ┌──────┐   ┌────────┐  ┌──────┐    ┌──────┐
  │ core │   │studio  │  │ bake │    │ app  │
  └──────┘   └────────┘  └──────┘    └──────┘
```

- Everyone depends on **`proto`** and nothing else cross-layer.
- **No client imports another client.** `app` does not import `studio` *source* — it **embeds the
  built Studio** in a webview (I10). `core` serves Studio's built assets but shares no UI code.
- `core` has **no UI**; `proto` has **no logic**.

## The single allowed duplication (I8)
Stroke rendering exists once in `web/ink`. The **native ink overlay** is the *only* sanctioned
re-implementation, because it must run on a native low-latency surface. It is:
- **isolated** (one file/seam in `app`),
- **parity-tested** (golden image vs `web/ink`),
- **never a third copy** (Go must not render strokes — bake calls `web/bake` → `web/ink`).

Everything else: **if you're about to copy logic between layers, stop** — it belongs in `proto`
(types/protocol) or a shared package.

## Keep native to the strict minimum (I15)
The mobile app is a thin shell. Native code is allowed **only** at three `expect/actual` seams:
WebView host · low-latency ink overlay · storage. Reviewer test: *"Could this have been shared
Kotlin or lived in the webview?"* If yes, it must not be native.

## Separation of concerns — one-liners
- **proto** — *what things are and how they travel.*
- **core** — *what is true* (authority, sync, persistence, bake orchestration).
- **studio** — *how you compose & annotate* (the canonical, reactive editor).
- **ink** — *how a stroke looks* (once).
- **bake** — *how truth becomes a flat, performable artifact* (reusing ink).
- **app** — *how it runs on a device*: host Studio, perform Stage, download — minimal native glue.

## "Reactive enough Studio"
Studio is the only place with rich interactive state (tools, selection, optimistic objects, zoom).
Keep that state **in Studio**, model it reactively, and let `core` stay a calm authority that just
accepts/echoes objects (I6). Don't push UI reactivity into `core`, and don't push authority into
Studio.

## The apply-engine ⟂ store border
The cleanest seam in `core`. The **apply engine** (the `sync` layer) owns the live, concurrent
state: the in-memory **HEAD**, per-song **serialization** (single writer), **LWW** (I5), **`seq`**,
optimistic **echo**, and the durable ordered log (**WAL**). The **store** is a *passive,
history-aware sink* — the engine **hydrates** HEAD from it at startup and **persists** the
already-ordered, already-reconciled stream to it (async for slow backends).

**Concurrency stops at this border** — the store never sees a race, never does LWW, never merges.
That is exactly why every backend stays dumb and swappable (mem/file/git/pg), and why git never
conflicts. HEAD, serialization, and the WAL belong to the engine, **never** inside a store backend
(so they are shared across all backends, not reimplemented per backend). It is the
per-aggregate-actor + event-store pattern: **authority above, durable history below.**
