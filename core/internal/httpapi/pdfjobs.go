package httpapi

import (
	"strconv"
	"sync"
	"time"

	"troubastack/core/internal/bake"
)

// T202 — the async concert-PDF job registry. A job composes the member-scoped concert PDF in the background
// (mirroring the bake kick+poll), reporting {done,total} sheets; the finished bytes are held for download.
// In-memory, server-scoped: a job and its file expire 10 minutes after the last update, one job runs per
// (concert, member, role), and a job is visible only to its creator or a band admin.
const pdfJobTTL = 10 * time.Minute

type pdfJobState string

const (
	pdfQueued  pdfJobState = "queued"
	pdfRunning pdfJobState = "running"
	pdfDone    pdfJobState = "done"
	pdfFailed  pdfJobState = "failed"
)

type pdfJob struct {
	id          string
	bandID      string
	concertID   string
	rev         uint64
	memberID    string
	role        string
	creatorID   string
	state       pdfJobState
	done, total int
	errMsg      string
	bytes       []byte
	filename    string
	expires     time.Time
}

type pdfJobRegistry struct {
	mu    sync.Mutex
	byID  map[string]*pdfJob
	byKey map[string]string // (concert,member,role) → jobID, for dedup
	now   func() time.Time
}

func newPdfJobRegistry(now func() time.Time) *pdfJobRegistry {
	if now == nil {
		now = time.Now
	}
	return &pdfJobRegistry{byID: map[string]*pdfJob{}, byKey: map[string]string{}, now: now}
}

// pdfJobKey identifies a dedup bucket. It includes the REV (so a re-bake — a new rev — is a new job, never a
// stale reuse of the old PDF) and the CREATOR (so an admin's job for member M is not handed to M, who would
// then be denied by view and 404).
func pdfJobKey(concertID string, rev uint64, memberID, role, creatorID string) string {
	return concertID + "\x00" + strconv.FormatUint(rev, 10) + "\x00" + memberID + "\x00" + role + "\x00" + creatorID
}

// startOrGet reuses an IN-FLIGHT job (queued or running) for the key so a repeated request joins it; a
// terminal job (done or failed) is NOT reused — a fresh request after one finishes re-composes (identical
// bytes for the same rev), and a retry after a failure starts anew.
func (r *pdfJobRegistry) startOrGet(bandID, concertID string, rev uint64, memberID, role, creatorID, filename string) (id string, created bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	key := pdfJobKey(concertID, rev, memberID, role, creatorID)
	if jid, ok := r.byKey[key]; ok {
		if j, ok := r.byID[jid]; ok && (j.state == pdfQueued || j.state == pdfRunning) {
			return j.id, false
		}
	}
	jid := bake.NewBakeID()
	r.byID[jid] = &pdfJob{
		id: jid, bandID: bandID, concertID: concertID, rev: rev, memberID: memberID, role: role,
		creatorID: creatorID, state: pdfQueued, filename: filename, expires: r.now().Add(pdfJobTTL),
	}
	r.byKey[key] = jid
	return jid, true
}

func (r *pdfJobRegistry) setRunning(id string, done, total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.byID[id]; ok {
		j.state, j.done, j.total = pdfRunning, done, total
		j.expires = r.now().Add(pdfJobTTL)
	}
}

func (r *pdfJobRegistry) finish(id string, pdf []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.byID[id]; ok {
		j.state, j.bytes = pdfDone, pdf
		if j.total > 0 {
			j.done = j.total
		}
		j.expires = r.now().Add(pdfJobTTL)
	}
}

func (r *pdfJobRegistry) fail(id, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.byID[id]; ok {
		j.state, j.errMsg = pdfFailed, msg
		j.expires = r.now().Add(pdfJobTTL)
	}
}

// view returns a snapshot of the job for a poll/download, gated: it must belong to this band and the caller
// must be its creator or a band admin. A missing/expired job → (nil, false) → the handler 404s.
func (r *pdfJobRegistry) view(id, bandID, callerID string, isAdmin bool) (pdfJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	j, ok := r.byID[id]
	if !ok || j.bandID != bandID {
		return pdfJob{}, false
	}
	if j.creatorID != callerID && !isAdmin {
		return pdfJob{}, false
	}
	return *j, true
}

func (r *pdfJobRegistry) sweepLocked() {
	now := r.now()
	for id, j := range r.byID {
		if now.After(j.expires) {
			delete(r.byID, id)
			if k := pdfJobKey(j.concertID, j.rev, j.memberID, j.role, j.creatorID); r.byKey[k] == id {
				delete(r.byKey, k)
			}
		}
	}
}
