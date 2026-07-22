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
	"context"
	"encoding/json"
	"net/http"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/buildinfo"
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
func Router(ctx context.Context, svc *app.Service, eng *engine.Engine, baker *bake.Baker, secureCookies bool, appsDir string) (http.Handler, error) {
	mux := http.NewServeMux()

	// Liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Build identity (T29): unauthenticated, like /healthz — what version is this
	// binary, when was it built, and does it carry a real embedded SPA? Diagnosis
	// (stale build / placeholder / SPA↔server mismatch) and the future app↔server
	// compatibility hook. Display only — NO version gating happens anywhere yet.
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     buildinfo.Version(),
			"builtAt":     buildinfo.BuiltAt(),
			"spaEmbedded": webassets.SPAEmbedded(),
		})
	})

	// Relational JSON API (auth/identity, bands, members, invites, songs).
	web := NewWebAPI(svc, secureCookies)
	web.Mount(mux)

	// Annotation API (view-only): read a song's materialized HEAD, import layers
	// + objects. Reuses the relational auth middleware so it shares one auth path.
	NewAnnotationsAPI(svc, eng).Mount(mux, web.auth)

	// Bake orchestration (I11): admin bakes a setlist → .tstage; members list +
	// download baked concerts. baker may be nil in tests that don't exercise bake.
	NewBakeAPI(svc, baker).Mount(mux, web.auth)

	// Downloadable native app binaries (OPS02): the server image can carry the app
	// so a band installs it from its own server. Unauthenticated (pre-account members
	// are the audience). appsDir == "" (dev / no embedded apps) ⇒ empty manifest.
	NewAppsAPI(appsDir, buildinfo.Version()).Mount(mux)

	// Rehearsal live mode autobaker (P201/I11 stage 1b): when a baker is wired, a
	// debounced autobaker turns annotation commits on a live setlist's songs into
	// auto-bakes. The bake closure wraps baker.Bake (a system bake as the enabling
	// admin), so the app package needs no bake import. Started for the process
	// lifetime; a nil baker (most unit tests) skips it entirely.
	var onCommit func(songID string)
	if baker != nil {
		ab := app.NewAutoBaker(svc, func(ctx context.Context, bandID, setlistID string, actor app.User) error {
			_, err := baker.Bake(ctx, bandID, setlistID, actor, nil) // shared band bake (P205: no dialog → legacy defaults)
			return err
		}, nil, 0)
		go ab.Run(ctx, time.Second) // stops when ctx is cancelled (server shutdown / test cleanup)
		onCommit = ab.Notify
	}

	// Realtime annotation sync: a per-song WebSocket over the SAME engine, so live
	// edits and the REST read see one consistent HEAD (I6). onCommit (if wired) fires
	// the autobaker after each accepted apply.
	mountWS(mux, svc, eng, onCommit)

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
