package discovery

import "testing"

// The opt-out env fully disables advertising and returns a safe no-op stop.
func TestAdvertise_disabledByEnv(t *testing.T) {
	t.Setenv("TROUBA_NO_MDNS", "1")
	stop := Advertise(8080, "")
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
	stop := Advertise(8080, "test-instance")
	if stop == nil {
		t.Fatal("Advertise returned a nil stop func")
	}
	stop() // safe whether or not advertising actually started
	stop() // idempotent-safe: calling twice must not panic
}
