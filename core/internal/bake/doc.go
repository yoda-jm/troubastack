// Package bake orchestrates baking: it resolves the requesting admin's role and
// scope, invokes the Node web/bake worker to flatten pages, and mints the
// resulting ConcertBundle as appended revisions.
//
// Invariants served: I11 (a performable revision exists ONLY via an explicit
// admin bake — editing never auto-publishes), I12 (the output is flattened
// images: per page a PDF raster + a transparent annotation overlay, consumed by
// a dumb offline presenter).
//
// CRITICAL (I8): core MUST NOT render strokes itself. Stroke geometry lives once
// in web/ink. Bake SHELLS OUT to the web/bake Node worker (the only sanctioned
// server-side JS runtime) which reuses web/ink, guaranteeing pixel parity with
// Studio. A second Go renderer here would be drift — forbidden.
//
// Boundary:
//   - MAY import: domain, engine, app (the relational facade, for setlist/song/
//     file resolution scoped to the admin actor), stdlib (os/exec to invoke the
//     poppler + web/bake subprocesses).
//   - MUST NOT import: httpapi, sync, web/app source, or any stroke-rendering
//     code. Orchestration only — the pixels come from poppler + web/bake (B02).
//
// Baker (baker.go) is the real orchestrator; bundle.go is the ConcertBundle Go
// mirror + .tstage writer; render.go holds the poppler/web-bake shell-out steps
// (interfaces, so the orchestrator is unit-testable with fakes).
package bake
