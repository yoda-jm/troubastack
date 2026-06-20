# troubaproto — the contract

**The single source of truth for every domain type and wire message** (ARCHITECTURE.md **I1**).
`core` (Go), `web` (TS), and `app` (Kotlin) all **generate** their types from these `.proto`
files. Hand-writing a duplicate of any wire type is forbidden.

## Files

| File | Covers | Invariants |
|---|---|---|
| `common.proto` | normalized points, style, scope | I3 |
| `object.proto` | annotation objects | I2, I3, I5 |
| `sync.proto`   | realtime mutation/echo protocol | I2, I5, I6 |
| `song.proto`   | linear revisions, setlists, pins | I4, I7 |
| `bundle.proto` | baked concerts + availability manifest | I11, I12, I13 |

## Codegen

Schema is single-sourced here; we generate per language with [`buf`](https://buf.build).
See `buf.gen.yaml`. Targets:

- **Go** → consumed by `core`
- **TypeScript** → consumed by `web` (`studio`, `ink` types, `bake`)
- **Kotlin** → consumed by `app`

```sh
buf lint
buf generate
```

**Generated code is never edited by hand and is regenerated in CI.** JSON-on-the-wire is acceptable
if a transport prefers it, but the *definition* always lives here.
