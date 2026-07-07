# CFG01 — A configuration file for troubacore

**Priority:** operator ergonomics, VLL-requested (2026-07-07) · **Size:** S/M ·
**Area:** `core/cmd/troubacore` (composition root) + a small loader package + docs

> **Status: AUTHORITATIVE — decisions fixed by arch 2026-07-07** (was: design question
> raised by Web-Core while VLL reviewed T21). Web-Core can implement.

## (a) Does forgotten-password need SMTP? — No (confirmed)

T21 is **deliberately email-free**: a band admin (or the operator via
`troubacore reset-password <user>`) mints a one-time link and hands it over
out-of-band — the same trust model as invite links. No mail code in core, none needed.
SMTP becomes relevant only if self-service "forgot password" is ever picked up (T21
out-of-scope). We **reserve a commented `[smtp]` section** in the config file as the
documented forward hook — shape only, no code behind it.

## (b) Configuration file — decisions (arch, 2026-07-07)

1. **Format: INI**, via `gopkg.in/ini.v1` (mature, comment-friendly, sectioned).
   VLL's first choice; the 12-knob surface is flat key=value and doesn't need TOML's
   typing. JSON ruled out (no comments). `.properties` has no strong maintained Go lib.
   (The raise said 13 knobs — verified by grep it's **12**: `TROUBA_DUMP_PDF` is a
   test-only debug hook in `cmd/seed`'s encoding test, not server config. Excluded.)
2. **Precedence: built-in defaults < config file < env vars < CLI flags.** Most
   specific wins. Every existing `TROUBA_*` env var keeps working unchanged as an
   override — tests/CI/Makefile untouched. (Today the only flag is `--config` itself;
   the precedence statement future-proofs any later flags.)
3. **Location:** default `./troubacore.ini` (working directory — NOT under the data
   dir: the data dir is itself a config value, chicken-and-egg). Override with
   `--config <path>` or `TROUBA_CONFIG`. A missing default file is fine (silent);
   a missing *explicitly named* file is a startup error (fail loud on operator intent).
4. **The example file is generated, never hand-maintained:**
   `troubacore --print-default-config` emits the full file — every knob present,
   **commented-out at its default** (per VLL), with a one-line comment per knob (reuse
   the meaning column below). Commit the output as `core/troubacore.example.ini` and
   add a Go test asserting the committed file byte-equals the generator output — the
   docs can't rot.
5. **Sections and keys** (ini key ↔ env var, 1:1, documented in the example file):
   - `[server]` `addr` ↔ `TROUBACORE_ADDR` · `secure_cookies` ↔ `TROUBA_SECURE_COOKIES`
   - `[storage]` `app_store` ↔ `TROUBA_APP_STORE` · `store` ↔ `TROUBA_STORE` ·
     `data_dir` ↔ `TROUBA_DATA_DIR` · `database_url` ↔ `TROUBA_DATABASE_URL`
   - `[mdns]` `enabled` (inverts `TROUBA_NO_MDNS`; file uses the positive form) ·
     `name` ↔ `TROUBA_MDNS_NAME`
   - `[bake]` `pdftoppm` ↔ `TROUBA_PDFTOPPM` · `node` ↔ `TROUBA_NODE` ·
     `cli` ↔ `TROUBA_BAKE_CLI`
   - `[dev]` `die_with_parent` ↔ `TROUBA_DIE_WITH_PARENT`
   - `[smtp]` — fully commented, reserved: `host/port/from/user/pass/tls`, header
     comment "not read by any code yet; forward hook for self-service password reset".
6. **Secrets note (document in the example file header):** `database_url` (and any
   future `smtp.pass`) may carry credentials — recommend `chmod 600`, and note that env
   vars still win, so secret-injection via environment remains first-class.
7. **Dependency + ADR:** first config-file lib in core — deliberate, mirrors the B06
   zeroconf precedent. Write **ADR 0004** (config file + precedence): loading lives in
   the composition root (`main.go` + the small loader pkg); everything else keeps
   receiving resolved values, per ADR 0002's spirit.

## Changes

1. `core/internal/config` (or `cmd/troubacore/config.go` if it stays tiny): load
   defaults → ini → env; expose the resolved struct; `PrintDefault()` for the generator.
2. `main.go`: replace the ad-hoc `os.Getenv` reads with the resolved config; add
   `--config` and `--print-default-config`. All reads already live in `main.go`
   except `TROUBA_NO_MDNS` (checked inside `discovery.Advertise`) — hoist that
   decision to the composition root (pass enabled/name in; keep Advertise never-fatal).
3. `core/troubacore.example.ini` (generated) + the byte-equality test.
4. ADR 0004; README operator note (file → env → flags, where the file lives).

## Acceptance criteria

- With no file, behavior is byte-for-byte today's (env-only) — existing tests + e2e
  green untouched. With a file, its values apply; env overrides file (test both
  directions on at least `addr` + `data_dir`); explicit `--config` to a missing path
  fails startup with a clear error.
- `--print-default-config` output == committed `troubacore.example.ini` (test-enforced);
  every knob in the table appears, commented, with its default and a meaning comment;
  `[smtp]` present, commented, marked not-yet-read.
- `make test` + e2e green; no `TROUBA_*` name changes.

## Out of scope

- SMTP code; hot reload; config for the app/Studio (server-side only); flags for
  individual knobs; changing any default.

### Current config surface (env vars in `cmd/troubacore/main.go`) — reference

| Env var | Meaning | Current default |
|---|---|---|
| `TROUBACORE_ADDR` | listen address | `:8080` |
| `TROUBA_APP_STORE` | relational backend | `mem` (`mem`\|`file`) |
| `TROUBA_STORE` | annotation store backend | `file` (`mem`\|`file`\|`git`\|`pg`) |
| `TROUBA_DATA_DIR` | data root | `./troubadata` |
| `TROUBA_DATABASE_URL` | Postgres URL (when `pg`) | — |
| `TROUBA_SECURE_COOKIES` | Secure flag on session cookie | off (set behind TLS) |
| `TROUBA_NO_MDNS` | disable LAN mDNS advertise | off |
| `TROUBA_MDNS_NAME` | mDNS instance name | host name |
| `TROUBA_PDFTOPPM` | poppler binary | `pdftoppm` |
| `TROUBA_NODE` | node binary (bake) | `node` |
| `TROUBA_BAKE_CLI` | web/bake CLI path | `../web/bake/dist/cli.js` |
| `TROUBA_DIE_WITH_PARENT` | dev: exit when parent dies | off |

(`TROUBA_DUMP_PDF`, listed in the original raise, is a test-only debug hook in
`core/cmd/seed/pdf_encoding_test.go` — not server config, not in the file.)
