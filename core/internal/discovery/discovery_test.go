package discovery

import "testing"

// enabled=false fully disables advertising and returns a safe no-op stop (the
// composition root resolves TROUBA_NO_MDNS into this flag now — CFG01).
func TestAdvertise_disabled(t *testing.T) {
	stop := Advertise(false, 8080, "")
	if stop == nil {
		t.Fatal("Advertise returned a nil stop func")
	}
	stop() // must not panic
}

// Advertise NEVER fails the caller: it returns a callable stop whether the mDNS
// socket bound or not (bind failures are swallowed — serving must not depend on
// discovery). This exercises the default (enabled) path; in a sandbox the register
// may bind or may fail, and either outcome is fine.
func TestAdvertise_neverFatal(t *testing.T) {
	stop := Advertise(true, 8080, "test-instance")
	if stop == nil {
		t.Fatal("Advertise returned a nil stop func")
	}
	stop() // safe whether or not advertising actually started
	stop() // idempotent-safe: calling twice must not panic
}
