# T121 — `TestDefault` silently skips any knob with an empty default

**Priority:** normal · **Size:** XS · **Area:** `core/internal/config` (Web-Core lane).
Found while verifying T120's landing condition (`a457115`).

## 1. Measured today

`TestDefault` iterates the authoritative `knobs` table and compares each knob's default against a `want`
map (`config_test.go:45`):

```go
for _, k := range knobs {
    if got := k.get(&c); got != want[k.env] { … }
}
```

A knob **missing from `want`** yields `want[k.env] == ""` — the zero value. So any knob whose default is
also `""` passes **without being listed at all**. The test cannot distinguish "asserted to default to
empty" from "nobody wrote it down".

**Two of fifteen knobs are in that state right now:**

| knob | |
|---|---|
| `TROUBA_APPS_DIR` | pre-existing |
| `TROUBA_RENDER_CACHE` | added by T120 |

Both default to `""`, so both pass vacuously. `TestDefault` is green and covers 13 of 15.

## 2. Why it's worth an XS

The `knobs` table is the repo's single source of truth for three things at once — ini parsing, the
generated `troubacore.example.ini`, and this test. A knob added with a wrong default, or dropped from the
table, is exactly what `TestDefault` exists to catch — and today it catches neither for the empty-default
case. This is a test passing for the wrong reason, which is the failure this repo's whole review standard
is built around.

## 3. What to build

- Add the two missing entries to `want`.
- **Make the omission impossible to repeat:** assert every knob is listed — e.g. fail if
  `len(want) != len(knobs)`, or better, fail naming any `k.env` not present as a key (use a
  two-value map lookup, `v, ok := want[k.env]`, so a missing key is distinguishable from an empty one).
  Prefer the `ok`-check: it fixes the actual defect rather than counting.

## 4. Rules

- **Teeth-check, reported:** delete one entry from `want` and show the test reddens **naming that knob**.
  Under today's code that deletion is silent for an empty-default knob — that contrast is the point of
  the task, so show both: silent before, named after.
- No behaviour change to `config.go` itself; this is test integrity. If you find a knob whose *default*
  is actually wrong while listing them, raise it separately rather than folding a fix in here.

## 5. Acceptance criteria

- All 15 knobs are asserted; a missing entry fails and names the knob.
- The teeth-check is reported in both directions (silent before / named after).
- `go test ./internal/config` green; `gofmt -l core` clean.

## 6. Out of scope

Changing any default. The `knobs` table's contents. Adding new configuration.
