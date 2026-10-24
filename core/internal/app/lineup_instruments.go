package app

import "fmt"

// T203 stage 2 (Fable ruling 1) — a musician's personal-instrument vocabulary, independent of any one song.
// The editor shows a column for each listed instrument (union with the instruments actually used), so a column
// never vanishes when its last usage is toggled off. Adding/removing one is a LINE-UP edit (any member), not a
// roster edit (the roster stays admin).

// ensureStoredMusician returns the index of the stored record for musicianID, materializing an auto-appearing
// member first (so the added instrument persists). -1 if the id is neither stored nor a known member account.
func (s *Service) ensureStoredMusician(band *Band, caller User, bandID, musicianID string) int {
	if _, idx := findMusician(band.Musicians, musicianID); idx >= 0 {
		return idx
	}
	mv, ok := s.memberByMusicianID(caller, bandID, musicianID)
	if !ok {
		return -1
	}
	band.Musicians = append(band.Musicians, Musician{ID: musicianID, UserID: mv.User.ID})
	return len(band.Musicians) - 1
}

// AddMusicianInstrument records that a musician plays a personal-instrument part in this band. The part must
// exist and be a non-shared instrument (voice shares one channel; a shared instrument routes to its own
// column — neither belongs in a person's instrument list). Idempotent; membership is enough.
func (s *Service) AddMusicianInstrument(caller User, bandID, musicianID, partID string) (Musician, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Musician{}, err
	}
	p, pi := findPart(band.Parts, partID)
	if pi < 0 {
		return Musician{}, fmt.Errorf("%w: no such part", ErrNotFound)
	}
	if p.Kind != PartInstrument || p.Shared {
		return Musician{}, fmt.Errorf("%w: a musician instrument must be a personal (non-shared) instrument", ErrInvalidInput)
	}
	idx := s.ensureStoredMusician(&band, caller, bandID, musicianID)
	if idx < 0 {
		return Musician{}, ErrNotFound
	}
	if !containsString(band.Musicians[idx].Instruments, partID) {
		band.Musicians[idx].Instruments = append(band.Musicians[idx].Instruments, partID)
	}
	if err := s.repo.UpdateBand(band); err != nil {
		return Musician{}, err
	}
	return band.Musicians[idx], nil
}

// RemoveMusicianInstrument drops a part from a musician's instrument list. If the musician still plays it in N
// songs and force is false, it is refused with the count N (the UI offers "clear N usages"); force also clears
// those N usages from the line-ups. Returns the usage count seen. A line-up edit (membership is enough).
func (s *Service) RemoveMusicianInstrument(caller User, bandID, musicianID, partID string, force bool) (int, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return 0, err
	}
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return 0, err
	}
	uses := 0
	for _, sg := range songs {
		for _, e := range sg.Lineup {
			if e.MusicianID == musicianID && containsString(e.PartIDs, partID) {
				uses++
			}
		}
	}
	if uses > 0 && !force {
		return uses, fmt.Errorf("%w: still played in %d song(s)", ErrInvalidInput, uses)
	}
	if force && uses > 0 {
		for _, sg := range songs {
			changed := false
			kept := make([]LineupEntry, 0, len(sg.Lineup))
			for _, e := range sg.Lineup {
				if e.MusicianID == musicianID && containsString(e.PartIDs, partID) {
					e.PartIDs = removeString(e.PartIDs, partID)
					changed = true
				}
				if len(e.PartIDs) > 0 || e.Note != "" { // prune an entry left with nothing
					kept = append(kept, e)
				} else {
					changed = true
				}
			}
			if changed {
				sg.Lineup = kept
				if err := s.repo.UpdateSong(sg); err != nil {
					return uses, err
				}
			}
		}
	}
	if _, idx := findMusician(band.Musicians, musicianID); idx >= 0 {
		band.Musicians[idx].Instruments = removeString(band.Musicians[idx].Instruments, partID)
		if err := s.repo.UpdateBand(band); err != nil {
			return uses, err
		}
	}
	return uses, nil
}

func removeString(ss []string, s string) []string {
	out := ss[:0]
	for _, x := range ss {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}
