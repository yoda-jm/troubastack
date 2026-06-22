package filerepo

import (
	"testing"

	"troubastack/core/internal/app"
)

// A file-backed repo must persist the password hash across a RELOAD. app.User
// hides PasswordHash from the API (json:"-"), which previously dropped it from
// disk too — so after any restart (e.g. `make demo`'s seed→serve handoff) every
// login 401'd. Reproduce-first: this fails before the MarshalJSON/UnmarshalJSON
// fix (got.PasswordHash == "").
func TestPasswordHashSurvivesReload(t *testing.T) {
	dir := t.TempDir()

	r1, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	u := app.User{
		ID:           "u1",
		Username:     "marie",
		DisplayName:  "Marie",
		Email:        "marie@example.com",
		PasswordHash: "$2a$10$bcrypthashvalueplaceholder",
	}
	if err := r1.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Reopen from the same dir — what a fresh process / restart does.
	r2, err := New(dir)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	got, err := r2.GetUser("u1")
	if err != nil {
		t.Fatalf("GetUser after reload: %v", err)
	}
	if got.PasswordHash != u.PasswordHash {
		t.Fatalf("password hash lost across reload: got %q want %q", got.PasswordHash, u.PasswordHash)
	}
	if got.Username != "marie" || got.Email != "marie@example.com" {
		t.Fatalf("user fields lost across reload: %+v", got)
	}
}
