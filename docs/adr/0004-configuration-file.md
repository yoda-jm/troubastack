# 0004 — Configuration file (INI) with layered precedence

- **Status:** Accepted (2026-07-07)
- **Relates to:** I6 (server is the single authority), I14 (boundaries — the
  composition root owns wiring), ADR 0002 (subsystems receive resolved values).

## Context
Operators had exactly one way to configure a self-hosted core: a dozen `TROUBA_*`
environment variables read ad-hoc in `cmd/troubacore/main.go`. That is fine for
containers and CI but awkward for a hand-run self-host — no single documented place
to see or set the knobs, and env-only config is easy to get subtly wrong. VLL asked
for a config file (2026-07-07). SMTP is **not** needed: T21 password reset is
deliberately email-free (admins/operators mint one-time links out-of-band).

## Decision
A single **INI** file (`gopkg.in/ini.v1` — mature, comment-friendly, sectioned;
the flat ~12-knob surface doesn't need TOML's typing, and JSON has no comments),
resolved by a small `internal/config` package with a strict precedence:

> **built-in defaults < INI file < `TROUBA_*` env vars < CLI flags** — most specific wins.

- Every existing `TROUBA_*` env var keeps working unchanged as an override, so
  containers/CI/Makefile are untouched. With **no** file present, behavior is
  byte-for-byte the previous env-only behavior.
- The file lives at `./troubacore.ini` by default (working directory, **not** under
  the data dir — the data dir is itself a config value). Override with `--config
  <path>` or `TROUBA_CONFIG`. A missing *default* file is silently fine; a missing
  *explicitly named* file is a fatal startup error (fail loud on operator intent).
- Loading lives in / is called from the composition root; every subsystem still
  receives already-resolved values (ADR 0002's spirit). Nothing outside `main.go`
  reads the environment — the last holdout, `TROUBA_NO_MDNS` inside
  `discovery.Advertise`, was hoisted: `Advertise` now takes an `enabled bool`.
- A single ordered **knob table** in `internal/config` is the source of truth for
  both loading and the generated example file. `troubacore --print-default-config`
  emits the fully-commented example (every knob at its default, commented out); it
  is committed as `core/troubacore.example.ini` and a **byte-equality test** pins
  the committed file to the generator, so the docs cannot rot.
- `[smtp]` is present but **fully commented and read by no code** — a documented
  forward hook for a future self-service "forgot password" flow, shape only.

## Why / rejected alternatives
- **TOML** — rejected: the knob surface is flat `key = value`; TOML's richer typing
  buys nothing here. **JSON** — rejected: no comments, and the example file's value
  is its inline documentation. **`.properties`** — no strong maintained Go library.
- **First config-file dependency in core** — deliberate, mirroring the B06 zeroconf
  precedent: an external dep is added when the matching subsystem lands. `ini.v1`
  is pure Go, so core still builds offline as a single static binary.
- **Env vars still win over the file** — so secret injection via the environment
  stays first-class; `database_url` (and any future `smtp.pass`) can carry
  credentials, so the example header recommends `chmod 600` if they live in the file.
- **No hot reload, no per-knob flags, no Studio/app config** — out of scope; config
  is server-side startup only.
