// Package httpapi is the HTTP edge: the REST handlers (non-realtime ops — auth,
// list, download bundles, trigger bake) plus the file server that serves the
// embedded TroubaStudio SPA.
//
// Invariants served: I10 (serve the canonical web editor's built assets), I14
// (core serves Studio's built output but shares no UI source), I6 (REST is the
// non-realtime authority surface; the realtime path is internal/sync). See
// docs/design/02-sync-protocol.md.
//
// Boundary:
//   - MAY import: domain, store, session, sync, bake, webassets, proto types, stdlib.
//   - MUST NOT import: any client (web/app) source. It wires subsystems behind
//     HTTP; it holds no business logic of its own.
package httpapi

import (
	"net/http"

	"troubastack/core/internal/bake"
	"troubastack/core/internal/session"
	syncpkg "troubastack/core/internal/sync"
	"troubastack/core/internal/webassets"
)

// Router builds the http.Handler for core: /healthz, the REST API, and the
// SPA file server (I10). Subsystems are injected so this layer stays a wiring
// seam with no logic of its own.
//
// TODO: mount REST routes (auth, songs, bundles, POST bake → bake.Baker), and
// upgrade /ws to the sync.Hub. For now it serves /healthz and the embedded SPA.
func Router(_ *syncpkg.Hub, _ *session.Manager, _ *bake.Baker) (http.Handler, error) {
	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Serve the embedded Studio SPA (I10). TODO: SPA fallback to index.html for
	// client-side routes; mount the REST API and the /ws sync upgrade above it.
	assets, err := webassets.FS()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(assets)))

	return mux, nil
}
