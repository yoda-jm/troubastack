# Glossary & naming

The brand and component names come from the world of the **troubadours** — and the vocabulary
happens to map onto the architecture, because a *troubadour composed* and a *joglar performed*.

## Product & components

| Name | Is | Notes |
|---|---|---|
| **TroubaShare** | the shipped mobile product users install | hosts the two sections below |
| **TroubaStudio** | the editor (web SPA; also a section of the app via webview) | where you *compose & annotate* |
| **TroubaStage** | the offline concert presenter (a section of the app) | where you *perform* |
| **TroubaCore** | the backend server | source of truth, sync, baking |
| **TroubaStack** | this monorepo / umbrella | server + web + app |

## Domain terms

| Term | Meaning |
|---|---|
| **Object** | an annotation (freehand, line, shape, text) with a client UUID |
| **Stroke** | an immutable freehand object: points (in `[0,1]`) + style |
| **Revision** | a point on a song's linear, append-only history |
| **Pin** | a setlist's reference to a specific song revision |
| **Head** | the latest revision of a song (always retained) |
| **Bake** | the admin action that flattens a setlist into a TroubaStage concert bundle |
| **Bundle** | a self-contained, flattened-image concert for offline performance |
| **Wet layer** | the in-progress freehand stroke (native overlay, on tablet) |
| **Dry layer** | all committed objects (rendered in the web layer) |
| **Tombstone** | a terminal delete marker; only an explicit *restore* revives a UUID |

## Naming roots (for the curious)

*Trobar* (Occitan, "to compose verse") is the root of "troubadour". A *joglar/jongleur* performed
what the troubadour composed — i.e. **compose = Studio, perform = Stage**.
