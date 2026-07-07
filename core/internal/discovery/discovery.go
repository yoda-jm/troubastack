// Package discovery advertises the running core on the local network via
// mDNS/DNS-SD as `_troubacore._tcp`, so the mobile app's Connect screen can offer
// "tap the server that appeared" instead of typing the host's IP by hand (B06).
//
// SECURITY: this is an unauthenticated PREFILL convenience only. mDNS is
// unauthenticated, so a spoofed advertisement is possible — the risk equals typing
// a wrong URL, and the real mitigation is TLS (OPS01), not discovery. Nothing here
// auto-connects or sends a credential; the app shows host:port and the user still
// logs in.
//
// Best-effort: any failure (disabled by env, socket bind, no multicast interface)
// is logged and swallowed. Advertising must NEVER prevent the server from serving.
package discovery

import (
	"log"
	"os"
	"sync"

	"github.com/libp2p/zeroconf/v2"
)

// service is the DNS-SD service type the app browses for.
const service = "_troubacore._tcp"

// version is advertised in a TXT record (informational only).
const version = "dev"

// Advertise announces this core on the LAN when enabled (a LAN tool should be
// findable by default; a server behind real DNS/TLS opts out). The enabled/name
// decision is resolved by the composition root from config+env (CFG01) — nothing
// here reads the environment. name is the friendly instance name; empty falls back
// to the host name. It returns a shutdown func that is ALWAYS safe to call (a no-op
// when advertising didn't start).
func Advertise(enabled bool, port int, name string) func() {
	if !enabled {
		log.Printf("mDNS: disabled")
		return func() {}
	}
	if name == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			name = h
		} else {
			name = "TroubaCore"
		}
	}
	// TXT records: version (informational) + path (reserved for a future non-root
	// mount point; "/" today so consumers can rely on it existing).
	txt := []string{"version=" + version, "path=/"}
	server, err := zeroconf.Register(name, service, "local.", port, txt, nil)
	if err != nil {
		log.Printf("mDNS: advertise failed (continuing without discovery): %v", err)
		return func() {}
	}
	log.Printf("mDNS: advertising as %q (%s port %d)", name, service, port)
	// Wrap Shutdown in sync.Once so the returned stop is safe to call more than
	// once (e.g. a signal handler + a defer both firing).
	var once sync.Once
	return func() { once.Do(server.Shutdown) }
}
