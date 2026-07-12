package app_test

import (
	"errors"
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// setlistLiveFixture builds a service with a controllable clock, an admin + a
// non-admin member of a band, and one setlist. Returns the pieces the tests drive.
func setlistLiveFixture(t *testing.T, clock *time.Time) (*app.Service, app.User, app.User, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New()).WithClock(func() time.Time { return *clock })

	admin, err := svc.Register("amy", "Amy", "pass1234", "")
	if err != nil {
		t.Fatalf("admin register: %v", err)
	}
	band, err := svc.CreateBand(admin, "The Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	member, err := svc.Register("moe", "Moe", "pass1234", "")
	if err != nil {
		t.Fatalf("member register: %v", err)
	}
	// Add moe as a plain member: the admin invites by username, moe accepts.
	inv, err := svc.Invite(admin, band.ID, member.Username, app.KindUsername)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := svc.AcceptInvite(member, inv.ID); err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Sat @ The Anchor", "", "", "")
	if err != nil {
		t.Fatalf("create setlist: %v", err)
	}
	return svc, admin, member, band.ID, sl.ID
}

func TestSetSetlistLive_toggleAndExpiry(t *testing.T) {
	base := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	clock := base
	svc, admin, _, bandID, slID := setlistLiveFixture(t, &clock)

	// Off by default.
	sl, err := svc.Setlist(admin, bandID, slID)
	if err != nil {
		t.Fatalf("get setlist: %v", err)
	}
	if svc.SetlistLiveNow(sl.Setlist) {
		t.Fatal("new setlist should not be live")
	}

	// Admin enables → live now, LiveUntil = now + window.
	got, err := svc.SetSetlistLive(admin, bandID, slID, true)
	if err != nil {
		t.Fatalf("enable live: %v", err)
	}
	if !svc.SetlistLiveNow(got) {
		t.Fatal("setlist should be live right after enabling")
	}
	if want := base.Add(app.LiveModeWindow); !got.LiveUntil.Equal(want) {
		t.Fatalf("LiveUntil = %v, want %v", got.LiveUntil, want)
	}

	// It PERSISTS: a fresh read still reads live.
	reread, err := svc.Setlist(admin, bandID, slID)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if !svc.SetlistLiveNow(reread.Setlist) {
		t.Fatal("persisted setlist should still be live")
	}

	// Advance PAST the window → auto-expires with no explicit off, no sweeper.
	clock = base.Add(app.LiveModeWindow + time.Minute)
	if svc.SetlistLiveNow(reread.Setlist) {
		t.Fatal("setlist should auto-expire after the window")
	}

	// Just BEFORE the deadline it is still live (boundary).
	clock = base.Add(app.LiveModeWindow - time.Second)
	if !svc.SetlistLiveNow(reread.Setlist) {
		t.Fatal("setlist should be live one second before the deadline")
	}

	// Explicit OFF zeroes it.
	clock = base
	off, err := svc.SetSetlistLive(admin, bandID, slID, false)
	if err != nil {
		t.Fatalf("disable live: %v", err)
	}
	if !off.LiveUntil.IsZero() || svc.SetlistLiveNow(off) {
		t.Fatalf("after off: LiveUntil=%v live=%v, want zero/false", off.LiveUntil, svc.SetlistLiveNow(off))
	}
}

func TestSetSetlistLive_adminOnly(t *testing.T) {
	clock := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	svc, _, member, bandID, slID := setlistLiveFixture(t, &clock)

	// A non-admin member cannot toggle live mode.
	if _, err := svc.SetSetlistLive(member, bandID, slID, true); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member enable live err = %v, want ErrForbidden", err)
	}
	// And it stayed off.
	sl, err := svc.Setlist(member, bandID, slID)
	if err != nil {
		t.Fatalf("get setlist: %v", err)
	}
	if svc.SetlistLiveNow(sl.Setlist) {
		t.Fatal("setlist should still be off after a forbidden toggle")
	}
}

// TestSetlistLive_pure covers the exported liveness predicate directly.
func TestSetlistLive_pure(t *testing.T) {
	now := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	if app.SetlistLive(app.Setlist{}, now) {
		t.Fatal("zero LiveUntil must read not-live")
	}
	if !app.SetlistLive(app.Setlist{LiveUntil: now.Add(time.Hour)}, now) {
		t.Fatal("future LiveUntil must read live")
	}
	if app.SetlistLive(app.Setlist{LiveUntil: now.Add(-time.Second)}, now) {
		t.Fatal("past LiveUntil must read not-live")
	}
}
