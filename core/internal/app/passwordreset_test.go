package app_test

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// TestIssuePasswordResetForUser exercises the operator/CLI path: mint a reset by
// username with no band scope, then consume it — the new password logs in, the
// old one and any prior session are dead.
func TestIssuePasswordResetForUser(t *testing.T) {
	svc := app.NewService(memrepo.New())
	if _, err := svc.Register("dave", "Dave", "oldpass1", ""); err != nil {
		t.Fatalf("register: %v", err)
	}

	// An active session that the reset must later invalidate.
	_, sessTok, err := svc.Login("dave", "oldpass1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Unknown user → ErrNotFound (no oracle for the operator to misread).
	if _, _, err := svc.IssuePasswordResetForUser("ghost"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("issue for unknown user err = %v, want ErrNotFound", err)
	}

	u, token, err := svc.IssuePasswordResetForUser("dave")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if u.Username != "dave" || token == "" {
		t.Fatalf("issue returned user=%q token=%q", u.Username, token)
	}

	// An empty new password is rejected (shared minPasswordLen floor) without
	// consuming the token.
	if err := svc.ConsumePasswordReset(token, ""); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty password err = %v, want ErrInvalidInput", err)
	}

	if err := svc.ConsumePasswordReset(token, "newpass1"); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Single-use: the token is spent.
	if err := svc.ConsumePasswordReset(token, "again123"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("second consume err = %v, want ErrNotFound", err)
	}

	// Old session invalidated.
	if _, err := svc.UserForToken(sessTok); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("old session err = %v, want ErrUnauthorized", err)
	}

	// Old password dead, new password lives.
	if _, _, err := svc.Login("dave", "oldpass1"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("old password err = %v, want ErrUnauthorized", err)
	}
	if _, _, err := svc.Login("dave", "newpass1"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}
