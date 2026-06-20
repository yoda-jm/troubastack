package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the embedded SPA with HTML5-history fallback: an existing file
// is served as-is; any other (non-/api) path falls back to index.html so the
// client-side router can handle deep links and refreshes (e.g. GET /bands). This
// matches what dev-mode Vite does — the prod single-binary path must behave the same
// (I10). Unknown /api/* paths are NOT rewritten to index.html; they stay real 404s.
func spaHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)

		// API namespace never falls back to the SPA — an unknown API route is a 404.
		if clean == "/api" || strings.HasPrefix(clean, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Serve the real asset when it exists.
		if p := strings.TrimPrefix(clean, "/"); p != "" {
			if f, err := assets.Open(p); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback: hand the SPA shell to the client router (no-cache so a new build
		// is never pinned by a stale index referencing old hashed assets).
		index, err := assets.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = index.Close() }()
		b, err := io.ReadAll(index)
		if err != nil {
			http.Error(w, "read index", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(b)
	})
}
