# CFG01 — A configuration file for troubacore

> **Status: DESIGN QUESTION — raised by the Web-Core Agent for arch/reviewer decision
> (2026-07-07), not an authoritative spec.** VLL asked two things while reviewing T21:
> (a) does forgotten-password need an SMTP server?, and (b) we've never discussed
> configuration — he'd like a config file. This captures both for a verdict; once the
> arch fixes format + precedence, Web-Core can implement (it's a composition-root change
> in `cmd/troubacore/main.go` + a small loader package + docs).

## (a) Does forgotten-password need SMTP? — No (by design)

T21 (password reset) is **deliberately email-free**: a band admin (or the server
operator via `troubacore reset-password <user>`) mints a one-time link and hands it over
**out-of-band** — the same trust model as invite links. There is **no mail code in core**
and none is needed for the shipped feature.

SMTP only becomes relevant if we later add **self-service "forgot password"** (a user
triggers their own reset and receives it by email) — which T21's spec listed as
explicitly out-of-scope ("needs email — revisit if a mail pipeline ever exists"). So the
recommendation is: **don't wire SMTP now**, but **reserve a commented-out `[smtp]`
section** in the config file as a forward hook, so the shape is documented when/if
self-service reset is picked up.

## (b) Configuration file — the ask

Today **all** configuration is environment variables read ad-hoc in `main.go` (13 knobs,
below). There is no single place to see or set them, and no file. VLL's preferences:

- **Format: INI or `.properties` preferred; JSON explicitly *less* preferred** (can't
  comment JSON).
- **Defaults are the most relevant values, shipped *commented-out* in the file** — i.e.
  the config file itself documents every knob at its default; uncommenting overrides.
- (Implied) the file should be the single documented surface for operators.

### Current config surface (env vars in `cmd/troubacore/main.go`)

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
| `TROUBA_DUMP_PDF` | debug PDF dump | off |

## Open decisions for the arch

1. **Format.** VLL's first choice is INI. Candidate Go libs: `gopkg.in/ini.v1` (mature,
   comment-friendly, sections). Alternative worth a mention: **TOML** (`BurntSushi/toml`)
   — comment-friendly *and* typed/nested, very idiomatic in Go, but it's not INI/.properties.
   `.properties` (Java-style) has no strong maintained Go lib. **Recommend INI** unless the
   arch prefers TOML for typing. (JSON ruled out per VLL — no comments.)
2. **Precedence.** Proposed: built-in defaults **<** config file **<** env vars **<** CLI
   flags (most specific wins). This keeps every existing `TROUBA_*` working as an override
   and lets the file be the documented baseline.
3. **Location / discovery.** Proposed: default `./troubacore.ini` (or under the data dir?);
   `--config <path>` / `TROUBA_CONFIG` to point elsewhere. Ship a
   `troubacore.example.ini` (or generate `--print-default-config`) with **every knob
   present and commented at its default**, per VLL's "commented-out defaults" ask.
4. **First-cut scope.** Fold the 13 knobs above + reserve a commented `[smtp]` section
   (host/port/from/user/pass/tls) as the forgotten-password forward hook. Nothing else.
5. **New dependency.** This introduces the first config-file lib. Consistent with ADR
   0002's "composition root owns backend choice" — loading stays in `main.go`; the rest of
   core keeps depending only on the resolved values. Worth an ADR? (arch's call).

## Non-goals

No runtime-behavior change; env vars keep working (as overrides). Not touching the
`TROUBA_*` names the tests/CI/Makefile rely on.
