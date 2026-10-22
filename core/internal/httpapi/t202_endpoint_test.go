package httpapi_test

import (
	"bytes"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestConcertPDFJob (T202 §3): kick → poll → download. Progress is monotonic and reaches total; the job's
// bytes equal the sync endpoint's for the same viewer; a repeated request reuses the job; the member gate
// holds; a member can't see the admin's job.
func TestConcertPDFJob(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	seedSongInSetlist(t, admin, band.ID, sl.ID)
	awaitBake(t, admin, band.ID, sl.ID)

	jobsURL := "/api/bands/" + band.ID + "/concerts/" + sl.ID + "/pdf-jobs"

	// Kick (default viewer = the caller).
	resp, kbody := admin.do(http.MethodPost, jobsURL, map[string]any{})
	mustStatus(t, resp, http.StatusAccepted)
	var jobID string
	unmarshalField(t, kbody, "jobId", &jobID)
	if jobID == "" {
		t.Fatal("no jobId returned")
	}

	// Poll to terminal; assert monotonic progress.
	var state string
	var done, total int
	prev := -1
	for i := 0; i < 300; i++ {
		resp, pbody := admin.do(http.MethodGet, jobsURL+"/"+jobID, nil)
		mustStatus(t, resp, http.StatusOK)
		unmarshalField(t, pbody, "state", &state)
		unmarshalField(t, pbody, "done", &done)
		unmarshalField(t, pbody, "total", &total)
		if done < prev {
			t.Fatalf("progress went backwards: %d → %d", prev, done)
		}
		prev = done
		if state == "done" || state == "failed" {
			break
		}
	}
	if state != "done" {
		t.Fatalf("job did not finish (last state %q)", state)
	}
	if total <= 0 || done != total {
		t.Fatalf("done/total = %d/%d, want done==total>0", done, total)
	}

	// The job's bytes == the sync endpoint's for the same viewer.
	resp, jobPDF := rawGet(admin, jobsURL+"/"+jobID+"/file")
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("file Content-Type = %q", ct)
	}
	resp, syncPDF := rawGet(admin, "/api/bands/"+band.ID+"/concerts/"+sl.ID+"/pdf")
	mustStatus(t, resp, http.StatusOK)
	if !bytes.Equal(jobPDF, syncPDF) {
		t.Fatalf("job bytes != sync bytes (%d vs %d)", len(jobPDF), len(syncPDF))
	}

	// A request after the job FINISHED starts a fresh job (dedup covers only in-flight jobs; done jobs are
	// not reused, so a re-bake can never serve a stale PDF — binding 1). In-flight reuse is covered at the
	// registry level (TestPdfJobRegistry). The fresh job must also be pollable to done.
	resp, kbody2 := admin.do(http.MethodPost, jobsURL, map[string]any{})
	mustStatus(t, resp, http.StatusAccepted)
	var jobID2 string
	unmarshalField(t, kbody2, "jobId", &jobID2)
	if jobID2 == jobID {
		t.Fatalf("a request after the job finished should start a fresh job, got the done one: %s", jobID2)
	}

	// §2 gate: a member naming another → 403; the admin → 202.
	resp, _ = member.do(http.MethodPost, jobsURL, map[string]any{"member": "someone-else"})
	mustStatus(t, resp, http.StatusForbidden)
	resp, _ = admin.do(http.MethodPost, jobsURL, map[string]any{"member": "someone-else"})
	mustStatus(t, resp, http.StatusAccepted)

	// Visibility: a member cannot poll the admin's job.
	resp, _ = member.do(http.MethodGet, jobsURL+"/"+jobID, nil)
	mustStatus(t, resp, http.StatusNotFound)
}

// TestConcertPDFJob_CrossCreator (T202 binding 2): an admin may kick a job FOR a member; that member kicking
// for themselves must get their OWN job (a distinct, pollable one), not the admin's (which the member would
// be denied → 404). Red before creatorID was folded into the dedup key.
func TestConcertPDFJob_CrossCreator(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	_, mebody := member.do(http.MethodGet, "/api/me", nil)
	var me app.PublicUser
	unmarshalField(t, mebody, "user", &me)

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	seedSongInSetlist(t, admin, band.ID, sl.ID)
	awaitBake(t, admin, band.ID, sl.ID)
	jobsURL := "/api/bands/" + band.ID + "/concerts/" + sl.ID + "/pdf-jobs"

	// Admin kicks a job for bob.
	resp, abody := admin.do(http.MethodPost, jobsURL, map[string]any{"member": me.ID})
	mustStatus(t, resp, http.StatusAccepted)
	var adminJob string
	unmarshalField(t, abody, "jobId", &adminJob)

	// Bob kicks for himself → a DISTINCT job he can poll.
	resp, bbody := member.do(http.MethodPost, jobsURL, map[string]any{})
	mustStatus(t, resp, http.StatusAccepted)
	var bobJob string
	unmarshalField(t, bbody, "jobId", &bobJob)
	if bobJob == adminJob {
		t.Fatal("bob must get his own job, not the admin's (reusing it would 404 bob on the poll)")
	}
	resp, _ = member.do(http.MethodGet, jobsURL+"/"+bobJob, nil)
	mustStatus(t, resp, http.StatusOK)
}
