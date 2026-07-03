# Agent handoffs

Per-agent handoff notes for the Claude Code agents working this repo. Point a fresh session at the
file for its lane to continue seamlessly — each captures *how that agent works* and *what's done*,
not the code (read the code + `docs/` for that).

| Role | File | Lane |
|---|---|---|
| **Architect / Reviewer** | [`architect-reviewer.md`](architect-reviewer.md) | writes the task specs in `docs/tasks/`, reviews every task commit/PR by re-verifying acceptance criteria with tools, makes design decisions inside specs, owns README/docs |
| **Mobile-app developer** | [`mobile-app-agent.md`](mobile-app-agent.md) | the **A/IOS tracks** (Kotlin/Compose Multiplatform app in `app/`) |
| **Core / webservice developer** | *(adds their file here)* | the **T/B/OPS tracks** (Go core, web studio/ink/bake, CI) |

Shared ground rules live in [`../tasks/README.md`](../tasks/README.md) — read that first
regardless of role. These are living docs: keep the "what's done / remaining" sections current
as tasks land. No secrets in these files, ever.
