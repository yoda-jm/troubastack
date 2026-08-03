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
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/config"
	"troubastack/core/internal/discovery"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/httpapi"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
	"troubastack/core/internal/store/gitstore"
	"troubastack/core/internal/store/memstore"
	"troubastack/core/internal/store/pgstore"
)

func main() {
	// Operator subcommands (no args = run the server, the overwhelming default).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset-password":
			runResetPassword(os.Args[2:])
			return
		case "gc":
			runGC(os.Args[2:])
			return
		case "repair-blobs":
			runRepairBlobs(os.Args[2:])
			return
		}
		// Anything else falls through to flag parsing below (flags start with '-');
		// an unknown non-flag arg is rejected there.
	}

	cfg := loadConfig() // defaults < troubacore.ini < TROUBA_* env < flags (ADR 0004)

	// Wire the subsystems (all stubs in this scaffold).
	st, err := openStore(cfg) // swappable backend; default file, zero infra (ADR 0002)
	if err != nil {
		log.Fatalf("troubacore: open store: %v", err)
	}
	// The apply engine is the per-song annotation authority over the store (design/07).
	// Every shipped backend is at least HistoryAware (store doc), so this holds.
	ha, ok := st.(store.HistoryAware)
	if !ok {
		log.Fatalf("troubacore: store backend is not history-aware")
	}
	eng := engine.New(ha) // per-song annotation apply engine (I4, I5, I6)

	// Relational ("normal web") domain: users/sessions, bands, members, invites,
	// songs. Backend is swappable behind app.Repo (R8, ADR 0002).
	appRepo, err := openAppRepo(cfg)
	if err != nil {
		log.Fatalf("troubacore: open app repo: %v", err)
	}
	svc := app.NewService(appRepo)
	// Song-file bytes live in a content-addressed blob store. The file backend
	// persists under <data_dir>/blobs/; otherwise it is in-memory.
	blobs, err := openBlobStore(cfg)
	if err != nil {
		log.Fatalf("troubacore: open blob store: %v", err)
	}
	svc.WithBlobStore(blobs)

	// Bake orchestration (I11): resolves a setlist and shells out to poppler
	// (pdftoppm) + the web/bake worker (Node) to flatten a .tstage under the data
	// dir. A missing binary fails the individual bake, not the server — so this is
	// safe to wire even where the toolchain isn't installed.
	baker := bake.New(svc, eng, bakeConfig(cfg))

	// Server-lifetime context for background workers (P201 autobaker); runs for the
	// whole process.
	handler, err := httpapi.Router(context.Background(), svc, eng, baker, cfg.Server.SecureCookies, cfg.Server.AppsDir)
	if err != nil {
		log.Fatalf("troubacore: build router: %v", err)
	}

	addr := cfg.Server.Addr

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Dev convenience: when launched by `make dev/run/demo` (which set
	// TROUBA_DIE_WITH_PARENT=1 and exec us as a direct child), exit if our parent
	// goes away. This guarantees the server never outlives `make` even when the
	// terminal/runner delivers the interrupt to make ALONE (a non-tty / IDE
	// terminal sends SIGINT to make, not the whole process group), which would
	// otherwise orphan us holding the port. Off by default (prod parent is
	// init/systemd). Portable getppid poll — no PR_SET_PDEATHSIG thread caveats.
	if cfg.Dev.DieWithParent {
		watchParent()
	}

	// LAN discovery (B06): advertise this core as _troubacore._tcp so the app's
	// Connect screen can offer it without the user typing an IP. Best-effort —
	// never blocks serving. The enabled/name decision is resolved in the config
	// (mdns.enabled / TROUBA_NO_MDNS, mdns.name / TROUBA_MDNS_NAME).
	if _, portStr, err := net.SplitHostPort(addr); err == nil {
		if port, perr := strconv.Atoi(portStr); perr == nil {
			defer discovery.Advertise(cfg.MDNS.Enabled, port, cfg.MDNS.Name)()
		}
	}

	log.Printf("troubacore: listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("troubacore: server error: %v", err)
	}
}

// loadConfig parses the server flags and resolves the configuration. Precedence
// (ADR 0004): built-in defaults < INI file < TROUBA_* env vars < flags. --config
// (or TROUBA_CONFIG) points at the file; --print-default-config emits the fully
// commented example and exits (that output IS the committed troubacore.example.ini).
func loadConfig() config.Config {
	fs := flag.NewFlagSet("troubacore", flag.ExitOnError)
	configPath := fs.String("config", os.Getenv("TROUBA_CONFIG"), "path to the INI config file (default ./troubacore.ini; env TROUBA_CONFIG)")
	printDefault := fs.Bool("print-default-config", false, "print the fully-commented example config to stdout and exit")
	_ = fs.Parse(os.Args[1:])

	if *printDefault {
		fmt.Print(config.PrintDefault())
		os.Exit(0)
	}

	// An explicitly-named file (via --config or TROUBA_CONFIG) must exist; a missing
	// default file is silently fine.
	explicit := *configPath != ""
	cfg, err := config.Load(*configPath, explicit)
	if err != nil {
		log.Fatalf("troubacore: %v", err)
	}
	return cfg
}

// runResetPassword is the server operator's out-of-band recovery tool (T21): it
// mints a one-time reset link for a user by username and prints the relative
// path to hand over. This is the ONLY way to reset a password with no band
// admin available — it covers the "the only admin forgot their password"
// bootstrap. It opens the SAME app repo the server uses (TROUBA_APP_STORE), so
// run it with the same env — and, on the file backend, while the server is
// STOPPED: filerepo is a single-writer whole-file store, so a running server
// would overwrite the freshly written token on its next flush.
func runResetPassword(args []string) {
	if len(args) != 1 || args[0] == "" {
		log.Fatalf("usage: troubacore reset-password <username>")
	}
	// Resolve config so we open the SAME app repo the server uses (backend + data
	// dir), honoring the file + env; this subcommand takes no flags of its own.
	cfg, err := config.Load(os.Getenv("TROUBA_CONFIG"), os.Getenv("TROUBA_CONFIG") != "")
	if err != nil {
		log.Fatalf("troubacore: %v", err)
	}
	appRepo, err := openAppRepo(cfg)
	if err != nil {
		log.Fatalf("troubacore: open app repo: %v", err)
	}
	svc := app.NewService(appRepo)
	u, token, err := svc.IssuePasswordResetForUser(args[0])
	if err != nil {
		log.Fatalf("troubacore: reset-password %q: %v", args[0], err)
	}
	fmt.Printf("Password reset issued for %s (@%s).\n", u.DisplayName, u.Username)
	fmt.Println("Hand this one-time link to them — valid 24h, single use:")
	fmt.Printf("  <your-server-origin>/reset-password/%s\n", token)
}

// runGC is the server operator's out-of-band retention pass (P202): it prunes OLD
// baked concert outputs, keeping the newest `bake.keep_revs` per setlist (0 = keep
// all, the DEFAULT → no-op) and never a final-locked rev. It reclaims the real
// disk-growth source — the raster + overlay PNGs of every bake — while annotation
// history is left untouched (that pruning is deferred; see docs/tasks/P202). Like
// reset-password it resolves the SAME config the server uses, so run it with the same
// env; on the file backend prefer a maintenance window (a live bake or an in-flight
// .tstage download could race a pruned rev).
func runGC(args []string) {
	if len(args) != 0 {
		log.Fatalf("usage: troubacore gc  (retention set by bake.keep_revs / TROUBA_BAKE_KEEP_REVS)")
	}
	cfg, err := config.Load(os.Getenv("TROUBA_CONFIG"), os.Getenv("TROUBA_CONFIG") != "")
	if err != nil {
		log.Fatalf("troubacore: %v", err)
	}
	if cfg.Bake.KeepRevs <= 0 {
		fmt.Println("gc: bake.keep_revs=0 (keep all) — nothing to prune. Set it to opt into retention.")
		return
	}
	bakesDir := bakeConfig(cfg).BakesDir
	stats, err := bake.PruneOutputs(bakesDir, cfg.Bake.KeepRevs)
	if err != nil {
		log.Fatalf("troubacore: gc: %v", err)
	}
	fmt.Printf("gc: keep_revs=%d — scanned %d concert(s), pruned %d old bake revision(s), freed %d bytes.\n",
		cfg.Bake.KeepRevs, stats.ConcertsScanned, stats.RevsDeleted, stats.BytesFreed)
}

// runRepairBlobs re-materializes generated charts whose rendered PDF blob is missing from the
// store — orphaned historical data that 404s on download (T69). Charts re-render from their
// stored source; uploaded files whose bytes are gone are reported (unrecoverable — re-upload).
// Heals a box in one pass instead of waiting for each file to be viewed (the download-time
// auto-heal). Like the other subcommands it opens the SAME repo + blob store the server uses
// (TROUBA_APP_STORE + data dir); on the file backend run it with the server STOPPED (filerepo
// is a single-writer whole-file store — a running server would race the write-back).
func runRepairBlobs(args []string) {
	if len(args) != 0 {
		log.Fatalf("usage: troubacore repair-blobs")
	}
	cfg, err := config.Load(os.Getenv("TROUBA_CONFIG"), os.Getenv("TROUBA_CONFIG") != "")
	if err != nil {
		log.Fatalf("troubacore: %v", err)
	}
	appRepo, err := openAppRepo(cfg)
	if err != nil {
		log.Fatalf("troubacore: open app repo: %v", err)
	}
	blobs, err := openBlobStore(cfg)
	if err != nil {
		log.Fatalf("troubacore: open blob store: %v", err)
	}
	svc := app.NewService(appRepo).WithBlobStore(blobs)
	rep, err := svc.RepairMissingBlobs()
	if err != nil {
		log.Fatalf("troubacore: repair-blobs: %v", err)
	}
	fmt.Printf("repair-blobs: scanned %d file(s) — %d healthy, %d re-rendered from source, %d unrecoverable.\n",
		rep.Scanned, rep.Healthy, len(rep.Healed), len(rep.Unfixable))
	for _, f := range rep.Unfixable {
		fmt.Printf("  UNRECOVERABLE (re-upload needed): %q (id %s, song %s) — blob missing, not a generated chart.\n",
			f.Filename, f.ID, f.SongID)
	}
}

// watchParent exits the process once our original parent dies (PPID changes as
// we get reparented to init/a subreaper). Polling is cheap and avoids the
// per-thread semantics of PR_SET_PDEATHSIG under the Go runtime.
func watchParent() {
	orig := os.Getppid()
	if orig <= 1 {
		return // already orphaned (or no real parent) — nothing to watch
	}
	go func() {
		for range time.Tick(time.Second) {
			if os.Getppid() != orig {
				log.Printf("troubacore: parent %d exited — shutting down", orig)
				os.Exit(0)
			}
		}
	}()
}

// openStore picks a persistence backend from the resolved config (default: file —
// zero infra for local dev). Swapping backends touches only this function; the rest
// of core depends on the store.Store interface alone (I14, ADR 0002).
func openStore(cfg config.Config) (store.Store, error) {
	kind := store.Kind(cfg.Storage.Store)
	dir := cfg.Storage.DataDir
	log.Printf("troubacore: store backend = %s", kind)
	switch kind {
	case store.KindMemory:
		return memstore.New(), nil
	case store.KindFile:
		return filestore.New(dir), nil
	case store.KindGit:
		return gitstore.New(dir)
	case store.KindPostgres:
		return pgstore.New(cfg.Storage.DatabaseURL)
	default:
		return nil, fmt.Errorf("unknown store %q (want mem|file|git|pg)", kind)
	}
}

// openAppRepo picks the relational backend from the resolved config (mem|file).
// file persists to <data_dir>/app.json (zero infra). Postgres is a later step.
// Swapping backends touches only this function; the rest of core depends on
// app.Repo alone (I14).
func openAppRepo(cfg config.Config) (app.Repo, error) {
	kind := cfg.Storage.AppStore
	dir := cfg.Storage.DataDir
	log.Printf("troubacore: app store backend = %s", kind)
	switch kind {
	case "mem":
		return memrepo.New(), nil
	case "file":
		return filerepo.New(dir)
	default:
		return nil, fmt.Errorf("unknown app store %q (want mem|file)", kind)
	}
}

// bakeConfig resolves the bake toolchain from the config. Binaries default to bare
// names (found on PATH); the web/bake worker defaults to the repo-relative path that
// works when core runs from core/ (make dev/run/e2e). Bundles are written under
// <data>/bakes. Swapping paths touches only this fn.
func bakeConfig(cfg config.Config) bake.Config {
	return bake.Config{
		BakesDir: filepath.Join(cfg.Storage.DataDir, "bakes"),
		Pdftoppm: cfg.Bake.Pdftoppm,
		Node:     cfg.Bake.Node,
		BakeCLI:  filepath.FromSlash(cfg.Bake.CLI),
	}
}

// openBlobStore picks the song-file blob backend, matching the app repo backend:
// app_store=file persists blobs under <data_dir>/blobs/; anything else
// (mem/default) keeps them in memory. Swapping touches only this function.
func openBlobStore(cfg config.Config) (blob.Store, error) {
	if cfg.Storage.AppStore != "file" {
		return blob.NewMem(), nil
	}
	return blob.NewFile(filepath.Join(cfg.Storage.DataDir, "blobs"))
}
