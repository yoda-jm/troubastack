package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// maxRehearsalNoteBytes caps one uploaded note. A 1600-wide transparent PNG of handwriting
// measures tens of KB (the first real one off VLL's tablet was 22,664 bytes for a 1600×2261
// canvas that is 99.9% transparent), so this is ~200× the real thing: generous enough never
// to reject honest handwriting, small enough to stop a mistake — a whole page raster, say.
const maxRehearsalNoteBytes = 4 << 20 // 4 MiB

// BakeLookup is the NARROW seam the service needs into the bake package to answer "has this
// page changed since the note was drawn?". It is an interface here, and implemented in
// httpapi (like chartAnchorer), so app keeps importing nothing from bake.
//
// PageRasterHash reports the raster hash of pageInSong of songID in concertID's LATEST bake.
// The two bools carry the three outcomes §3.4 needs and must not be collapsed: baked=false
// means no bake exists for that concert at all (→ "unknown", we cannot say), while
// baked=true, pageExists=false means the bake is there and the page is not (→ changed).
type BakeLookup interface {
	PageRasterHash(concertID, songID string, pageInSong int) (hash string, pageExists, baked bool)
	// ResolvePageSource answers T185 ⟨D2⟩: which file and 0-based page produced the raster with this hash,
	// in this song and bake rev. Matched on the hash, never the index. resolved=false when it cannot be
	// told (no source record for the rev, or no hash match) — the caller then leaves the fields empty and
	// Studio keeps the single-file rule. It never guesses.
	ResolvePageSource(concertID string, concertRev uint64, songID, rasterHash string) (fileID string, filePage int, resolved bool)
	// PageRasterByHash returns the baked page raster (PNG bytes) whose hash is rasterHash in this song —
	// the note's rev first, then any rev of the concert (T186 ⟨D1⟩). false when no rev the server holds has
	// it. The viewer draws the note over this page as it was when drawn, even for an unresolved note.
	PageRasterByHash(concertID string, concertRev uint64, songID, rasterHash string) ([]byte, bool)
}

// WithBakeLookup injects the bake seam. Nil is allowed and means "no bake knowledge": every
// note then reports pageChanged unknown, which is the honest answer rather than a cheerful
// false. Returns the Service for chaining.
func (s *Service) WithBakeLookup(b BakeLookup) *Service {
	s.bakes = b
	return s
}

// RehearsalNoteView is one note as the API returns it: the record plus the ONE derived fact
// the human needs. PageChanged is a *bool because it is genuinely three-valued — nil is
// "unknown" (§3.4), and nil is not false. Collapsing it would tell the musician their
// reference is current when nobody checked.
type RehearsalNoteView struct {
	RehearsalNote
	PageChanged *bool `json:"pageChanged"`
	// T185 ⟨D2⟩ — the file and 0-based page the note's raster was drawn on, resolved from the bake by hash.
	// Both are absent when unresolvable (no source record, no hash match); Studio then keeps today's
	// single-file rule. FileID present is the signal that FilePage is meaningful (page 0 is valid).
	FileID   string `json:"fileId,omitempty"`
	FilePage *int   `json:"filePage,omitempty"`
}

// RehearsalNoteMeta is what the sender tells us about a note. Everything here is the
// tablet's claim, not something the server derives; the one thing we do NOT take on trust is
// the content type (see PutRehearsalNote).
type RehearsalNoteMeta struct {
	RasterHash string
	ConcertID  string
	ConcertRev uint64
	TakenAs    string
	Width      int
	Height     int
	// CapturedAt is the zero time when the tablet has no wall clock for the note. That is the
	// case today: A70 stamps NoteEntry.updatedAt with SystemClock.elapsedRealtime(), a
	// boot-relative counter. Zero is stored as zero and rendered as "unknown, showing upload
	// date" — never as a date.
	CapturedAt time.Time
}

// PutRehearsalNote stores (or replaces) the caller's note for one page of one song.
//
// The caller is the owner, always (§3.3): we never map TakenAs back to a user. Band
// membership is required, exactly as for a song-file upload; beyond that a note is private
// to its sender, and that is enforced on every read path by keying on the caller's own id
// rather than by filtering afterwards.
//
// overwrite=false and a note already there → ErrConflict, so the tablet can ask before it
// destroys the copy in Studio.
func (s *Service) PutRehearsalNote(caller User, bandID, songID string, pageInSong int, meta RehearsalNoteMeta, data []byte, overwrite bool) (RehearsalNote, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return RehearsalNote{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return RehearsalNote{}, ErrNotFound
	}
	if pageInSong < 0 {
		return RehearsalNote{}, fmt.Errorf("%w: page index must not be negative", ErrInvalidInput)
	}
	if len(data) == 0 {
		return RehearsalNote{}, fmt.Errorf("%w: empty note", ErrInvalidInput)
	}
	if len(data) > maxRehearsalNoteBytes {
		return RehearsalNote{}, fmt.Errorf("%w: a rehearsal note may be at most %d bytes, got %d", ErrTooLarge, maxRehearsalNoteBytes, len(data))
	}
	// Sniff the BYTES. The declared type is not consulted at all — not even as a fallback, the
	// way UploadSongFile allows one — because here exactly one format is acceptable and a
	// fallback is just a way to let a caller's claim decide. The T141 family of bugs all start
	// with trusting a field over the payload.
	if ct := http.DetectContentType(data); ct != "image/png" {
		return RehearsalNote{}, fmt.Errorf("%w: a rehearsal note must be a PNG (the bytes look like %q)", ErrUnsupportedMedia, ct)
	}
	prev, err := s.repo.GetRehearsalNote(caller.ID, songID, pageInSong)
	switch {
	case err == nil && !overwrite:
		return RehearsalNote{}, fmt.Errorf("%w: a note already exists for this song and page", ErrConflict)
	case err != nil && !errors.Is(err, ErrNotFound):
		return RehearsalNote{}, err
	}
	hadPrev := err == nil

	hash, err := s.blobs.Put(data)
	if err != nil {
		return RehearsalNote{}, err
	}
	n := RehearsalNote{
		ID:          s.newID(),
		BandID:      bandID,
		SongID:      songID,
		OwnerUserID: caller.ID,
		PageInSong:  pageInSong,
		RasterHash:  meta.RasterHash,
		ConcertID:   meta.ConcertID,
		ConcertRev:  meta.ConcertRev,
		TakenAs:     meta.TakenAs,
		BlobHash:    hash,
		Width:       meta.Width,
		Height:      meta.Height,
		CapturedAt:  meta.CapturedAt.UTC(),
		UploadedAt:  s.now().UTC(),
	}
	if hadPrev {
		n.ID = prev.ID // replacing a note keeps its identity; only the bytes and metadata move
	}
	if err := s.repo.CreateOrReplaceRehearsalNote(n); err != nil {
		return RehearsalNote{}, err
	}
	// Drop the replaced bytes. Re-sending an IDENTICAL note is the ordinary case (the musician
	// fixed nothing, just sent again) and content-addressing makes prev.BlobHash == hash there,
	// so this skips a pointless repo scan — it is an optimisation, NOT the guard: derefBlob
	// counts the notes on the hash and would decline anyway, which a teeth-check confirmed by
	// removing this condition and watching every test still pass. The guard is in derefBlob.
	if hadPrev && prev.BlobHash != hash {
		s.derefBlob(prev.BlobHash)
	}
	return n, nil
}

// ListRehearsalNotes returns the CALLER's notes for a song, page order, each decorated with
// §3.4's pageChanged. Another member's notes are not filtered out of a shared list — they are
// never fetched, because the repo is keyed by owner.
func (s *Service) ListRehearsalNotes(caller User, bandID, songID string) ([]RehearsalNoteView, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return nil, ErrNotFound
	}
	notes, err := s.repo.ListRehearsalNotes(caller.ID, songID)
	if err != nil {
		return nil, err
	}
	out := make([]RehearsalNoteView, 0, len(notes))
	for _, n := range notes {
		v := RehearsalNoteView{RehearsalNote: n, PageChanged: s.pageChanged(n)}
		if fileID, filePage, ok := s.resolveSource(n); ok {
			v.FileID = fileID
			fp := filePage
			v.FilePage = &fp
		}
		out = append(out, v)
	}
	return out, nil
}

// resolveSource answers T185 ⟨D2⟩ at list time: the file and page a note's raster was drawn on, resolved by
// hash through the bake seam. Unresolvable (no lookup wired, no hash, no source record, no match) leaves it
// to the caller to omit the fields — Studio then keeps the single-file rule. Resolving here, not at upload,
// also covers notes stored before T185.
func (s *Service) resolveSource(n RehearsalNote) (fileID string, filePage int, resolved bool) {
	if s.bakes == nil || n.ConcertID == "" || n.RasterHash == "" {
		return "", 0, false
	}
	return s.bakes.ResolvePageSource(n.ConcertID, n.ConcertRev, n.SongID, n.RasterHash)
}

// pageChanged answers "does the page this note was drawn on still look like this?" — a LABEL
// for the human and nothing else. Nothing is re-placed, re-anchored or hidden on the strength
// of it; doing that would be T145's bug in a new coat.
//
// nil = unknown (no bake to compare against, or no lookup wired). true = the page at that
// index now hashes differently, or there is no such page any more. false = same hash.
func (s *Service) pageChanged(n RehearsalNote) *bool {
	if s.bakes == nil || n.ConcertID == "" || n.RasterHash == "" {
		return nil
	}
	hash, pageExists, baked := s.bakes.PageRasterHash(n.ConcertID, n.SongID, n.PageInSong)
	if !baked {
		return nil // the concert has never been baked here — we cannot say, so we do not
	}
	changed := !pageExists || hash != n.RasterHash
	return &changed
}

// PendingRehearsalNotes answers the ONE question the band's song list asks: which of my songs have a
// rehearsal note sitting in Studio, and how many? T173 ⟨D3⟩ — one call per band, never one per song.
//
// ⟨D6⟩, and it belongs in the code as much as in the UI copy: this counts notes IN STUDIO and nothing
// else. A note the tablet has not sent is invisible here (the tablet is the only surface that knows it
// exists, and the only one that can act on it), and a note removed with "Done, remove" disappears from
// this count while the tablet still holds its copy — removal never travels back (T170 §7). So an empty
// result means "nothing waiting in Studio", never "nobody has notes".
//
// Read-only by construction: it returns counts and no ids to act on (⟨D4⟩).
func (s *Service) PendingRehearsalNotes(caller User, bandID string) (map[string]int, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	return s.repo.CountRehearsalNotesByBand(caller.ID, bandID)
}

// RehearsalNoteBytes returns one of the caller's own notes and its PNG. A note belonging to
// someone else is ErrNotFound, NOT ErrForbidden: whether another member has drawn on a page
// is not information a non-owner is entitled to, and a 403 would leak exactly that.
func (s *Service) RehearsalNoteBytes(caller User, bandID, songID string, pageInSong int) (RehearsalNote, []byte, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return RehearsalNote{}, nil, err
	}
	n, err := s.repo.GetRehearsalNote(caller.ID, songID, pageInSong)
	if err != nil {
		return RehearsalNote{}, nil, ErrNotFound
	}
	if n.BandID != bandID {
		return RehearsalNote{}, nil, ErrNotFound
	}
	data, err := s.blobs.Get(n.BlobHash)
	if err != nil {
		return RehearsalNote{}, nil, ErrNotFound
	}
	return n, data, nil
}

// RehearsalNoteBackground serves the BAKE PAGE the note was drawn over (T186 ⟨D1⟩): the raster whose hash is
// the note's, found in the note's rev or any rev of the concert. Owner-only, same authorisation as the note.
// ErrNotFound (→ 404) when no rev the server still holds has that hash, and the viewer then shows the strokes
// on plain paper.
func (s *Service) RehearsalNoteBackground(caller User, bandID, songID string, pageInSong int) ([]byte, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	n, err := s.repo.GetRehearsalNote(caller.ID, songID, pageInSong)
	if err != nil || n.BandID != bandID {
		return nil, ErrNotFound
	}
	if s.bakes == nil {
		return nil, ErrNotFound
	}
	data, ok := s.bakes.PageRasterByHash(n.ConcertID, n.ConcertRev, n.SongID, n.RasterHash)
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

// DeleteRehearsalNote is the musician's "Done, remove": they have recopied the note into real
// annotations and no longer want the reference. There is no undo and none is wanted — it was a
// reference, and the tablet still has the original.
func (s *Service) DeleteRehearsalNote(caller User, bandID, songID string, pageInSong int) error {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return err
	}
	n, err := s.repo.GetRehearsalNote(caller.ID, songID, pageInSong)
	if err != nil || n.BandID != bandID {
		return ErrNotFound
	}
	if err := s.repo.DeleteRehearsalNote(caller.ID, songID, pageInSong); err != nil {
		return err
	}
	s.derefBlob(n.BlobHash)
	return nil
}
