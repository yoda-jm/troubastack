package main

import (
	"os"
	"path/filepath"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// The reset minted over the socket must be a token the LIVE service honours —
// the property the original bug broke (the CLI's token never reached the server).
func TestResetViaAdminSocketMintsALiveToken(t *testing.T) {
	dir, err := os.MkdirTemp("", "as") // short path: unix socket paths are capped near 108 bytes
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	svc := app.NewService(memrepo.New())
	if _, err := svc.Register("marie", "Marie", "oldpassword1", ""); err != nil {
		t.Fatal(err)
	}
	if err := serveAdminSocket(dir, svc); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, adminSockName))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode: %v %v, want 0600", fi.Mode().Perm(), err)
	}
	r, err := requestResetViaSocket(dir, "marie")
	if err != nil {
		t.Fatalf("reset via socket: %v", err)
	}
	if r.Username != "marie" || r.Token == "" {
		t.Fatalf("reply = %+v", r)
	}
	if err := svc.ConsumePasswordReset(r.Token, "newpassword2"); err != nil {
		t.Fatalf("the live service rejects the token: %v", err)
	}
	if _, _, err := svc.Login("marie", "newpassword2"); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	if _, err := requestResetViaSocket(dir, "nobody"); err == nil {
		t.Fatal("reset for an unknown user succeeded")
	}
}

func TestResetViaAbsentSocketSaysSo(t *testing.T) {
	if _, err := requestResetViaSocket(t.TempDir(), "marie"); err != errNoAdminSocket {
		t.Fatalf("err = %v, want errNoAdminSocket", err)
	}
}

func TestResetLinkUsesThePublicURLWhenSet(t *testing.T) {
	for _, c := range []struct{ base, want string }{
		{"https://band.example.org", "https://band.example.org/reset-password/tok"},
		{" https://band.example.org/ ", "https://band.example.org/reset-password/tok"}, // trailing slash, spaces
		{"", "<your-server-origin>/reset-password/tok"},
	} {
		if got := resetLink(c.base, "tok"); got != c.want {
			t.Errorf("resetLink(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}
