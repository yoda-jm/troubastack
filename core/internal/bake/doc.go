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
//   - MAY import: domain, store, session, stdlib (os/exec to invoke the worker).
//   - MUST NOT import: httpapi, sync, web/app source, or any stroke-rendering
//     code. Orchestration only — the pixels come from web/bake.
package bake

// Baker drives a bake: authorize (ADMIN, I11) → resolve scope → invoke the
// web/bake worker (subprocess) → mint a ConcertBundle revision (I4) → persist.
//
// TODO: hold store + session; locate the web/bake worker; run it via os/exec
// passing the resolved scope; collect flattened page images (I12); append the
// bundle revision. Never render strokes in Go (I8).
type Baker struct {
	// TODO: store handle, session handle, path to web/bake worker.
}

// New returns a placeholder Baker. TODO: wire dependencies + worker path.
func New() *Baker { return &Baker{} }
