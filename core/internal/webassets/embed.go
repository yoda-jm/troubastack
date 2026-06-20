// Package webassets holds the embedded TroubaStudio SPA so core can serve the
// canonical web editor with NO Node runtime in production.
//
// Invariants served: I10 (the web editor is canonical and runs standalone; core
// serves its built assets), I14 (dependencies point only toward the contract —
// core serves Studio's BUILT output and shares no UI source with it).
//
// Build wiring: TroubaStudio (web/studio) builds to a static SPA; its dist/ is
// copied into this package's dist/ directory before `go build`, and go:embed
// bakes it into the single static binary served by internal/httpapi. The dist/
// tree is git-ignored except for a committed placeholder index.html, so this
// package compiles offline even before Studio has been built.
package webassets

import (
	"embed"
	"io/fs"
)

// dist embeds the built Studio SPA. The placeholder index.html keeps the
// directory populated in a fresh checkout so this scaffold builds before any
// Studio output exists.
//
//go:embed dist
var dist embed.FS

// FS returns the embedded Studio assets rooted at dist/, ready to hand to an
// http file server (I10).
//
// TODO: once Studio is built into dist/, this serves the real SPA. For now it is
// just the placeholder directory.
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
