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
//   - MAY import: domain, store, sync, bake, webassets, proto types, stdlib.
//     (Sessions/auth are owned by app.Service; there is no core/internal/session.)
//   - MUST NOT import: any client (web/app) source. It wires subsystems behind
//     HTTP; it holds no business logic of its own.
package httpapi

import (
	"net/http"

	"troubastack/core/internal/app"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/webassets"
)

// Router builds the http.Handler for core: /healthz, the REST API, and the
// SPA file server (I10). Subsystems are injected so this layer stays a wiring
// seam with no logic of its own.
//
// The relational ("normal web") API — auth, bands, members, invites, songs —
// is mounted from svc via WebAPI under /api/*. The per-song annotation engine
// (eng) backs the view-only annotation routes under .../songs/{songId}/annotations.
// secureCookies should be true behind TLS. The realtime /ws upgrade is wired here
// over a sync.Hub built on the SAME engine instance (in mountWS), so the live HEAD
// the hub mutates is exactly the one GET …/annotations reads.
func Router(svc *app.Service, eng *engine.Engine, secureCookies bool) (http.Handler, error) {
	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Relational JSON API (auth/identity, bands, members, invites, songs).
	web := NewWebAPI(svc, secureCookies)
	web.Mount(mux)

	// Annotation API (view-only): read a song's materialized HEAD, import layers
	// + objects. Reuses the relational auth middleware so it shares one auth path.
	NewAnnotationsAPI(svc, eng).Mount(mux, web.auth)

	// Realtime annotation sync: a per-song WebSocket over the SAME engine, so live
	// edits and the REST read see one consistent HEAD (I6).
	mountWS(mux, svc, eng)

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
