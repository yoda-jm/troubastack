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

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/session"
	syncpkg "troubastack/core/internal/sync"
	"troubastack/core/internal/webassets"
)

// Router builds the http.Handler for core: /healthz, the REST API, and the
// SPA file server (I10). Subsystems are injected so this layer stays a wiring
// seam with no logic of its own.
//
// The relational ("normal web") API — auth, bands, members, invites, songs —
// is mounted from svc via WebAPI under /api/*. secureCookies should be true
// behind TLS. The realtime /ws upgrade to the sync.Hub remains a TODO.
func Router(svc *app.Service, secureCookies bool, _ *syncpkg.Hub, _ *session.Manager, _ *bake.Baker) (http.Handler, error) {
	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Relational JSON API (auth/identity, bands, members, invites, songs).
	NewWebAPI(svc, secureCookies).Mount(mux)

	// Serve the embedded Studio SPA (I10) with HTML5-history fallback so client-side
	// routes (e.g. /bands) resolve to index.html. The catch-all "/" pattern has lower
	// precedence than the explicit /api/* and /healthz patterns under the Go 1.22
	// router, so API routes are matched first.
	assets, err := webassets.FS()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spaHandler(assets))

	return mux, nil
}
