# `view-resolution.vectors.json` — the P205 view-resolution contract (cross-lane)

This file is the **single source of truth** for "which baked layers does a viewer
see". It exists so the **printed PDF** (T57, `core/internal/bake` — `LayerVisible`)
and the **on-stage presenter** (P205 Stage 3, `app/shared`) resolve layers
identically: **print == screen by construction**, tested rather than hoped. It is
the [`glyphs.json`](../../../../web/ink/glyphs.json) pattern applied to *semantics*.

## Schema

```jsonc
{
  "cases": [
    {
      "name": "human description of the case",
      "layer":  { "mandatory": bool, "roleTag": string, "owner": string, "defaultOn": bool|null },
      "viewer": { "role": string, "memberId": string },
      "expectVisible": bool
    }
  ]
}
```

- `layer.owner` — `""` = shared/band content; a member id = that member's personal layer.
- `layer.defaultOn` — `null` = absent (bake had no dialog ⇒ legacy compute); `true`/`false` = captured bake-time default.
- `viewer.role` — the **explicit** print/view role; `""` = a fresh viewer (mandatory + untagged shared only).
- `viewer.memberId` — the identity whose own personal layers are included.

## The rule (highest precedence first)

1. `mandatory` → always visible (I12).
2. personal layer (`owner != ""`) → visible iff `owner == viewer.memberId` (identity outranks `default_on`).
3. shared layer (`owner == ""`) → `roleOK && defaultOK` where
   `roleOK = roleTag == "" || roleTag == viewer.role` and
   `defaultOK = defaultOn == null ? true : defaultOn`.

## Both lanes run these

- **web-core (now):** `TestLayerVisible_Vectors` in `viewfilter_test.go` reads this file and asserts every case.
- **mobile (P205 Stage 3a):** a `commonTest` reads the SAME cases (copied into
  `app/shared/src/commonTest/resources/`); a CI drift-guard (mirroring the
  glyphs.json/`CueGlyphData.kt` check) keeps the copies byte-identical. Deleting the
  `-mine` demo bridge + wiring this guard are named items on Stage 3a's checklist.
