# TroubaStack

Collaborative sheet-music & lyrics annotation for bands and ensembles.

**One product (`TroubaShare`), three layers, one contract.** You *compose and annotate*
scores in a reactive web editor (**TroubaStudio**); a server (**TroubaCore**) holds the single
authoritative truth and publishes performable concerts; an offline, dumb presenter
(**TroubaStage**, inside the mobile app) *performs* them on stage.

> The name is a troubadour pun, and it maps onto the architecture:
> a **troubadour** *composes* (the editor), a **joglar** *performs* (the presenter).
> See [`docs/glossary.md`](docs/glossary.md).

---

## Read this first

| If you want to… | Read |
|---|---|
| Understand the **non-negotiable rules** | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) ← the constitution |
| Understand a specific subsystem | [`docs/design/`](docs/design/) |
| Know why a decision was made | [`docs/adr/`](docs/adr/) |

**The golden rule:** [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) lists numbered
**invariants (I1…In)**. They are enforced, not aspirational. Everything in `docs/design/`
and every line of code *derives from* them. If code and an invariant disagree, the code is wrong.

---

## Monorepo map

```
troubastack/
├── proto/        the CONTRACT — domain types + wire protocol (protobuf).
│                 Single source of truth. All clients GENERATE from it. [I1]
├── core/         TroubaCore — Go. Authoritative state, realtime sync,
│                 bake orchestration, serves the Studio SPA. [I6]
├── web/          npm workspace — all browser/JS code.
│   ├── ink/      @troubastack/ink — THE one stroke renderer. [I8]
│   ├── studio/   TroubaStudio — the canonical editor SPA. [I10]
│   └── bake/     bake worker (Node) — reuses web/ink for pixel-parity. [I8]
└── app/          TroubaShare — Kotlin/Compose Multiplatform mobile app.
                  Hosts Studio in a webview + the TroubaStage presenter.
                  Native code kept to 3 seams only. [I15]
```

Dependencies point **toward the contract only**: `core`, `web/studio`, `web/bake`, and `app`
all depend on `proto`; nothing depends on a sibling client. [I14]

## Status

Pre-implementation scaffold. The first build step is the **TroubaStudio web-ink spike**
(validate stylus latency on a real tablet) — see [`docs/design/03-rendering-and-ink.md`](docs/design/03-rendering-and-ink.md).

## Toolchains

Go 1.26+ · Node 24+ / npm 11+ · JDK 25+ (Kotlin Multiplatform) · protoc 34+ / buf.
