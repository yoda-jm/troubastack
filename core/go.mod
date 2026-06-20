// TroubaCore Go module.
//
// Dependency rule (I14): core depends only on the contract in proto/ (whose
// generated Go lands under internal/gen). It imports NO sibling client (web,
// app) and contains NO UI.
//
// Stdlib-only on purpose: the scaffold must build offline (net/http embed.FS).
// External deps (Postgres driver, etc.) are added deliberately when the matching
// subsystem is implemented — see docs/design/06-tech-stack.md.
module troubastack/core

go 1.26
