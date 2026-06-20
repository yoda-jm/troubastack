// Command troubacore is the authoritative TroubaStack server (I6): the realtime
// sync hub, REST surface, persistence + GC policy, bake orchestration, and the
// host that serves the embedded TroubaStudio SPA. It has NO UI of its own.
//
// Invariants served here: I6 (server is the single authority), I10 + I14 (serve
// the canonical web editor's BUILT assets from an embedded FS, depending only on
// the contract — never on a sibling client). See docs/ARCHITECTURE.md.
//
// This scaffold only starts an http.Server with /healthz and a placeholder SPA
// file server, wiring the internal subsystem constructors as stubs.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/httpapi"
	"troubastack/core/internal/session"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
	"troubastack/core/internal/store/gitstore"
	"troubastack/core/internal/store/memstore"
	"troubastack/core/internal/store/pgstore"
	syncpkg "troubastack/core/internal/sync"
)

func main() {
	// Wire the subsystems (all stubs in this scaffold).
	st, err := openStore() // swappable backend; default file, zero infra (ADR 0002)
	if err != nil {
		log.Fatalf("troubacore: open store: %v", err)
	}
	// The apply engine is the per-song annotation authority over the store (design/07).
	// Every shipped backend is at least HistoryAware (store doc), so this holds.
	ha, ok := st.(store.HistoryAware)
	if !ok {
		log.Fatalf("troubacore: store backend is not history-aware")
	}
	eng := engine.New(ha)     // per-song annotation apply engine (I4, I5, I6)
	hub := syncpkg.New()      // realtime WebSocket hub (I6, I2)
	sessions := session.New() // auth + roles (I6, I11)
	baker := bake.New()       // bake orchestration; delegates rendering to web/bake (I8, I11)

	// Relational ("normal web") domain: users/sessions, bands, members, invites,
	// songs. Backend is swappable behind app.Repo (R8, ADR 0002).
	appRepo, err := openAppRepo()
	if err != nil {
		log.Fatalf("troubacore: open app repo: %v", err)
	}
	svc := app.NewService(appRepo)
	// Song-file bytes live in a content-addressed blob store. The file backend
	// persists under <TROUBA_DATA_DIR>/blobs/; otherwise it is in-memory.
	blobs, err := openBlobStore()
	if err != nil {
		log.Fatalf("troubacore: open blob store: %v", err)
	}
	svc.WithBlobStore(blobs)

	// Secure cookies only when explicitly told we're behind TLS (TROUBA_SECURE_COOKIES=1).
	secureCookies := os.Getenv("TROUBA_SECURE_COOKIES") == "1"

	handler, err := httpapi.Router(svc, eng, secureCookies, hub, sessions, baker)
	if err != nil {
		log.Fatalf("troubacore: build router: %v", err)
	}

	addr := os.Getenv("TROUBACORE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("troubacore: listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("troubacore: server error: %v", err)
	}
}

// openStore picks a persistence backend from TROUBA_STORE (default: file — zero
// infra for local dev). Swapping backends touches only this function; the rest of
// core depends on the store.Store interface alone (I14, ADR 0002).
func openStore() (store.Store, error) {
	kind := store.Kind(os.Getenv("TROUBA_STORE"))
	if kind == "" {
		kind = store.DefaultKind
	}
	dir := os.Getenv("TROUBA_DATA_DIR")
	if dir == "" {
		dir = "./troubadata"
	}
	log.Printf("troubacore: store backend = %s", kind)
	switch kind {
	case store.KindMemory:
		return memstore.New(), nil
	case store.KindFile:
		return filestore.New(dir), nil
	case store.KindGit:
		return gitstore.New(dir)
	case store.KindPostgres:
		return pgstore.New(os.Getenv("TROUBA_DATABASE_URL"))
	default:
		return nil, fmt.Errorf("unknown TROUBA_STORE %q (want mem|file|git|pg)", kind)
	}
}

// openAppRepo picks the relational backend from TROUBA_APP_STORE (mem|file,
// default mem for now). file persists to <TROUBA_DATA_DIR>/app.json (zero infra).
// Postgres is a later step. Swapping backends touches only this function; the
// rest of core depends on app.Repo alone (I14).
func openAppRepo() (app.Repo, error) {
	kind := os.Getenv("TROUBA_APP_STORE")
	if kind == "" {
		kind = "mem"
	}
	dir := os.Getenv("TROUBA_DATA_DIR")
	if dir == "" {
		dir = "./troubadata"
	}
	log.Printf("troubacore: app store backend = %s", kind)
	switch kind {
	case "mem":
		return memrepo.New(), nil
	case "file":
		return filerepo.New(dir)
	default:
		return nil, fmt.Errorf("unknown TROUBA_APP_STORE %q (want mem|file)", kind)
	}
}

// openBlobStore picks the song-file blob backend, matching the app repo backend:
// TROUBA_APP_STORE=file persists blobs under <TROUBA_DATA_DIR>/blobs/; anything
// else (mem/default) keeps them in memory. Swapping touches only this function.
func openBlobStore() (blob.Store, error) {
	if os.Getenv("TROUBA_APP_STORE") != "file" {
		return blob.NewMem(), nil
	}
	dir := os.Getenv("TROUBA_DATA_DIR")
	if dir == "" {
		dir = "./troubadata"
	}
	return blob.NewFile(filepath.Join(dir, "blobs"))
}
