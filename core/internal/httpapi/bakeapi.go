package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
)

// BakeAPI is the HTTP edge for bake orchestration (I11): an admin bakes a setlist
// into a downloadable .tstage, and band members list + download baked concerts.
// The actual flattening lives in internal/bake (which shells out to poppler +
// web/bake); this adapter does auth/scoping and streams the result.
//
// A concert's id IS its setlist id (a setlist bakes to one concert, rev-bumped per
// bake). Scoping: bake is admin-only (mirrors T08's import gate); list/download are
// member-only and filtered to setlists that belong to the caller's band.
type BakeAPI struct {
	svc   *app.Service
	baker *bake.Baker
	// ctx is the SERVER's lifetime context (T103). The async bake runs on it, NOT the request's, so a
	// client that hangs up — a long bake over venue/power-saving wifi — no longer cancels its own bake.
	ctx context.Context
}

func NewBakeAPI(ctx context.Context, svc *app.Service, baker *bake.Baker) *BakeAPI {
	return &BakeAPI{svc: svc, baker: baker, ctx: ctx}
}

func (a *BakeAPI) Mount(mux *http.ServeMux, authed func(authedHandler) http.HandlerFunc) {
	mux.HandleFunc("POST /api/bands/{bandId}/setlists/{setlistId}/bake", authed(a.bake))
	mux.HandleFunc("GET /api/bands/{bandId}/setlists/{setlistId}/bakes/{bakeId}/progress", authed(a.bakeProgress))
	mux.HandleFunc("GET /api/bands/{bandId}/concerts", authed(a.listConcerts))
	mux.HandleFunc("GET /api/bands/{bandId}/concerts/{concertId}/bundle", authed(a.downloadBundle))
	mux.HandleFunc("GET /api/bands/{bandId}/concerts/{concertId}/pdf", authed(a.concertPDF))
}

// concertView is the client-facing projection of a baked concert. It is the proto
// AvailableConcert shape (docs/design/08 canonical JSON — 64-bit ints as STRINGS,
// so the app deserializes it with A02's AvailableConcert Kotlin mirror verbatim,
// B03), plus two extras the mirror ignores as unknown fields: `bakedBy` and the
// `downloadUrl` for the .tstage. Per-song `rev` is the song's source (annotation)
// revision — the "did song X change" signal for future granular updates (B03).
type songRevView struct {
	SongID string `json:"songId"`
	Rev    uint64 `json:"rev,string"`
}

type concertView struct {
	ConcertID   string        `json:"concertId"`
	Name        string        `json:"name"`
	CurrentRev  uint64        `json:"currentRev,string"`
	UpdatedAt   int64         `json:"updatedAt,string"`
	FinalLocked bool          `json:"finalLocked,omitempty"`
	Songs       []songRevView `json:"songs"`
	BakedBy     string        `json:"bakedBy,omitempty"` // extra (studio history)
	DownloadURL string        `json:"downloadUrl"`       // extra (studio + app download)
	// Warnings surface per-song bake issues in the Studio bake dialog. Set only on the
	// bake POST (omitempty → absent for listConcerts). T60: items that asked to transpose
	// but whose conditions didn't hold at bake time (baked untransposed, not failed).
	Warnings []string `json:"warnings,omitempty"`
}

func viewOf(bandID string, cb bake.ConcertBundle) concertView {
	songs := make([]songRevView, 0, len(cb.Songs))
	for _, s := range cb.Songs {
		songs = append(songs, songRevView{SongID: s.SongID, Rev: s.SourceRevision})
	}
	return concertView{
		ConcertID:   cb.ConcertID,
		Name:        cb.Name,
		CurrentRev:  cb.ConcertRev,
		UpdatedAt:   cb.BakedAt,
		FinalLocked: cb.FinalLocked,
		Songs:       songs,
		BakedBy:     cb.BakedBy,
		DownloadURL: "/api/bands/" + bandID + "/concerts/" + cb.ConcertID + "/bundle",
	}
}

// bake mints a new revision of a setlist's band-wide concert — ADMIN-only (I11, same
// pattern as T08's annotation import). This is THE bake (P205); the per-member
// "?scope=mine" variant (B07) was retired, so the query param no longer branches.
func (a *BakeAPI) bake(w http.ResponseWriter, r *http.Request, u app.User) {
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	bandID := r.PathValue("bandId")
	// The band-wide bake is THE bake (P205) and is ADMIN-only (I11); the personal
	// "?scope=mine" variant was retired. GetBand 403/404s a non-member.
	_, role, err := a.svc.GetBand(u, bandID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if role != app.RoleAdmin {
		writeErr(w, app.ErrForbidden)
		return
	}
	// P205 bake dialog: an OPTIONAL body carries the captured per-layer defaults
	// (name → default-on). Absent/empty body ⇒ nil ⇒ legacy (viewer computes as
	// today). Malformed JSON is ignored (treated as no capture), never a 400.
	var in struct {
		LayerDefaults map[string]bool `json:"layerDefaults"`
		// T120 render-cache controls (both optional, default off): force skips reads but still writes
		// ("I don't trust the cache right now"); noCache neither reads nor writes ("give me a cold path").
		Force   bool `json:"force"`
		NoCache bool `json:"noCache"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	// T103: baking is a KICK, not a held-open socket. Decide the bake id up front so the 202 can return
	// it (the client polls GET …/bakes/{id}/progress for the outcome — the single source of truth). A
	// long bake used to hold the connection for minutes and die when the client's wifi dropped it.
	setlistID := r.PathValue("setlistId")
	bakeID := r.Header.Get("X-Trouba-Bake-Id") // honour a valid client-supplied id (T99/B), now optional
	if !bake.ValidBakeID(bakeID) {
		bakeID = bake.NewBakeID()
	}
	// Run on the SERVER's context, not r.Context(): a disconnected client must NOT cancel the bake.
	// On success, publish T60's transpose warnings onto the terminal progress record (the client reads
	// them there now, since there's no synchronous response body to carry them).
	// T120: thread the render-cache mode onto the bake's context (still the SERVER ctx, so a dropped
	// client can't cancel the bake — WithCacheControl only annotates it).
	bakeCtx := bake.WithCacheControl(a.ctx, in.Force, in.NoCache)
	go func() {
		if _, _, berr := a.baker.Bake(bakeCtx, bandID, setlistID, u, in.LayerDefaults, bakeID); berr == nil {
			a.baker.SetWarnings(bandID, setlistID, bakeID, a.transposeWarnings(u, bandID, setlistID))
		}
	}()
	w.Header().Set("X-Trouba-Bake-Id", bakeID)
	writeJSON(w, http.StatusAccepted, map[string]string{"bakeId": bakeID})
}

// bakeProgress reports a running/finished bake's "song N of M" (T96). SAME authorisation
// as the bake it observes — band admin (a bake is admin-only, I11) — reusing GetBand, not
// a new access rule. An unknown, expired, or cross-band bake id is a 404: "no progress"
// and "not yours" are one answer, so this never confirms a bake id to an outsider.
func (a *BakeAPI) bakeProgress(w http.ResponseWriter, r *http.Request, u app.User) {
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	bandID := r.PathValue("bandId")
	_, role, err := a.svc.GetBand(u, bandID)
	if err != nil {
		writeErr(w, err)
		return
	}
	if role != app.RoleAdmin {
		writeErr(w, app.ErrForbidden)
		return
	}
	p, ok := a.baker.Progress(bandID, r.PathValue("setlistId"), r.PathValue("bakeId"))
	if !ok {
		// Distinct from an empty 200: "I lost/never had your progress" ≠ "running, 0 done".
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such bake"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// transposeWarnings names the items that asked for a chord transpose but weren't
// eligible at bake time (baked untransposed). Derived from current state via the same
// app.TransposeEligible the baker used, so the warning and the bake decision agree.
// Best-effort: any lookup error yields no warning (never blocks the bake response).
func (a *BakeAPI) transposeWarnings(u app.User, bandID, setlistID string) []string {
	detail, err := a.svc.Setlist(u, bandID, setlistID)
	if err != nil {
		return nil
	}
	var warns []string
	for _, item := range detail.Items {
		if !item.TransposeChords {
			continue
		}
		song, serr := a.svc.SongForMember(u, bandID, item.SongID)
		if serr != nil {
			continue
		}
		hasChart := false
		if files, ferr := a.svc.SongFiles(u, bandID, item.SongID); ferr == nil {
			for _, f := range files {
				if f.Generated {
					hasChart = true
					break
				}
			}
		}
		title := item.SongTitle
		if title == "" {
			title = song.Title
		}
		if ok, reason := app.TransposeEligible(song.Key, item.KeyOverride, hasChart); !ok {
			warns = append(warns, title+": chords not transposed — "+reason)
		} else if !a.svc.BakeTransposeSucceeds(u, bandID, item.SongID, item.KeyOverride) {
			// D3: eligible, but the transform errored at bake so the chart baked
			// UNTRANSPOSED — otherwise a silent wrong-key page.
			warns = append(warns, title+": chords not transposed — the chart could not be transposed at bake")
		}
	}
	return warns
}

// listConcerts returns the baked concerts whose setlist belongs to this band.
func (a *BakeAPI) listConcerts(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	setlists, err := a.svc.Setlists(u, bandID) // member-scoped; 403/404 if not our band
	if err != nil {
		writeErr(w, err)
		return
	}
	inBand := make(map[string]bool, len(setlists))
	for _, sl := range setlists {
		inBand[sl.ID] = true
	}
	views := []concertView{}
	if a.baker != nil {
		for _, cb := range a.baker.ListConcerts() {
			// A concert is visible if its base setlist is in this band AND, for a
			// per-member variant (B07), it belongs to the caller — never another
			// member's variant.
			base, owner, isVariant := bake.ParseConcertID(cb.ConcertID)
			if !inBand[base] {
				continue
			}
			if isVariant && owner != u.ID {
				continue
			}
			views = append(views, viewOf(bandID, cb))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"concerts": views})
}

// downloadBundle streams a concert's latest .tstage. Member-only; the setlist must
// belong to the caller's band (svc.Setlist enforces that scope).
func (a *BakeAPI) downloadBundle(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	concertID := r.PathValue("concertId")
	// The base setlist must belong to the caller's band (svc.Setlist enforces
	// scope). For a per-member variant (B07) the caller must also be its owner —
	// a member can fetch band concerts + their OWN variants, never another's.
	base, owner, isVariant := bake.ParseConcertID(concertID)
	if _, err := a.svc.Setlist(u, bandID, base); err != nil {
		writeErr(w, err) // not our band / unknown setlist → 403/404
		return
	}
	if isVariant && owner != u.ID {
		writeErr(w, app.ErrForbidden)
		return
	}
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	path := a.baker.BundlePath(concertID)
	if path == "" {
		writeErr(w, app.ErrNotFound)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, app.ErrNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+concertID+`.tstage"`)
	http.ServeContent(w, r, concertID+".tstage", info.ModTime(), f)
}

// concertPDF renders a concert's latest bake to a printable A4 PDF (T57 — VLL's
// paper fallback). Same gating as downloadBundle. The view is parameterized (never
// a live session's toggles, so a printed backup is reproducible): `?role=X` selects
// which role-tagged shared layers composite; personal layers resolve to the caller's
// identity, or to `?member=Y` when the caller is an ADMIN (printing on someone's
// behalf). The compositing rule is bake.LayerVisible — the shared P205 view-
// resolution contract, so print == screen.
func (a *BakeAPI) concertPDF(w http.ResponseWriter, r *http.Request, u app.User) {
	bandID := r.PathValue("bandId")
	concertID := r.PathValue("concertId")
	base, owner, isVariant := bake.ParseConcertID(concertID)
	if _, err := a.svc.Setlist(u, bandID, base); err != nil {
		writeErr(w, err) // not our band / unknown setlist → 403/404
		return
	}
	if isVariant && owner != u.ID {
		writeErr(w, app.ErrForbidden)
		return
	}
	if a.baker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bake not configured"})
		return
	}
	// Personal layers resolve to the caller, unless an admin prints on a member's
	// behalf via ?member= (admin-only — a member never prints another's view).
	viewerMemberID := u.ID
	if m := r.URL.Query().Get("member"); m != "" && m != u.ID {
		if _, role, err := a.svc.GetBand(u, bandID); err != nil {
			writeErr(w, err)
			return
		} else if role != app.RoleAdmin {
			writeErr(w, app.ErrForbidden)
			return
		}
		viewerMemberID = m
	}
	pdf, err := a.baker.ConcertPDF(concertID, r.URL.Query().Get("role"), viewerMemberID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeErr(w, app.ErrNotFound)
			return
		}
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+concertID+`.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	_, _ = w.Write(pdf)
}
