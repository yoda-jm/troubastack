package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"troubastack/core/internal/app"
)

// The admin socket (OPS07) lets an operator command act on the RUNNING server
// instead of writing behind its back. filerepo keeps the whole dataset in memory
// and rewrites app.json whole, so a CLI that writes the file directly while the
// server runs is overwritten on the next flush — the original reset-password bug.
// With the socket, `troubacore reset-password <user>` (e.g. via `docker compose
// exec`) asks the live server to mint the token: one command, no downtime.
//
// It is a unix socket inside the data dir (0700 dir, 0600 socket): reachable
// only by the server's own OS user on the same machine/volume — never the
// network. Only the server that holds the data dir's lock (filerepo.LockDir)
// creates it, so removing a stale one at startup is safe.
const adminSockName = "admin.sock"

func adminSockPath(dataDir string) string { return filepath.Join(dataDir, adminSockName) }

type resetReply struct {
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
	Token       string `json:"token"`
	Error       string `json:"error,omitempty"`
}

// serveAdminSocket listens on <dataDir>/admin.sock and serves the operator
// commands against the live service. Best-effort: a failure is returned for the
// caller to log, never fatal to serving.
func serveAdminSocket(dataDir string, svc *app.Service) error {
	path := adminSockPath(dataDir)
	_ = os.Remove(path) // stale from an unclean exit; we hold the dir lock
	ln, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("admin socket %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return fmt.Errorf("admin socket %s: %w", path, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /reset-password", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username string `json:"username"`
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(resetReply{Error: "a username is required"})
			return
		}
		u, token, err := svc.IssuePasswordResetForUser(in.Username)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, app.ErrNotFound) {
				status = http.StatusNotFound
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(resetReply{Error: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(resetReply{DisplayName: u.DisplayName, Username: u.Username, Token: token})
	})
	go func() { _ = (&http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}).Serve(ln) }()
	return nil
}

// errNoAdminSocket: the server holds the lock but answers on no socket — an
// older server binary, or a socket it could not create.
var errNoAdminSocket = errors.New("the running server has no admin socket")

// requestResetViaSocket asks the running server to mint a reset for username.
func requestResetViaSocket(dataDir, username string) (resetReply, error) {
	path := adminSockPath(dataDir)
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}},
	}
	body, _ := json.Marshal(map[string]string{"username": username})
	resp, err := client.Post("http://troubacore/reset-password", "application/json", bytes.NewReader(body))
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return resetReply{}, errNoAdminSocket
		}
		return resetReply{}, err
	}
	defer resp.Body.Close()
	var out resetReply
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return resetReply{}, fmt.Errorf("admin socket: bad reply (%s): %w", resp.Status, err)
	}
	if resp.StatusCode != http.StatusOK {
		return resetReply{}, errors.New(out.Error)
	}
	return out, nil
}
