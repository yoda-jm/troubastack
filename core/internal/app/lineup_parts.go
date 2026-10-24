package app

import (
	"fmt"
	"sort"
	"strings"
)

// T203 — the band's Part vocabulary. Line-ups reference a part by ID (M1), so a rename is metadata-only and
// MERGE is the only band-wide rewrite; that removes the rename-vs-line-up-PATCH race.

func foldName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// deriveShort is the fallback sheet abbreviation when a Part has no explicit Short: initials for a multi-word
// name ("Lead vocal" → "LV"), else the first three letters ("Guitar" → "Gui"). Admins set the nice ones.
func deriveShort(name string) string {
	fields := strings.Fields(name)
	if len(fields) > 1 {
		var b strings.Builder
		for _, f := range fields {
			b.WriteString(strings.ToUpper(f[:1]))
		}
		return b.String()
	}
	name = strings.TrimSpace(name)
	if len(name) <= 3 {
		return strings.Title(name)
	}
	return strings.ToUpper(name[:1]) + name[1:3]
}

// PartShort returns a part's sheet abbreviation (explicit or derived).
func PartShort(p Part) string {
	if strings.TrimSpace(p.Short) != "" {
		return p.Short
	}
	return deriveShort(p.Name)
}

// Parts lists the band's part vocabulary (membership-gated). Sorted by Order then name.
func (s *Service) Parts(caller User, bandID string) ([]Part, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, err
	}
	parts := append([]Part(nil), band.Parts...)
	sort.SliceStable(parts, func(i, j int) bool {
		oi, oj := parts[i].Order, parts[j].Order
		if (oi == 0) != (oj == 0) {
			return oi != 0 // explicit order before unset
		}
		if oi != oj {
			return oi < oj
		}
		return foldName(parts[i].Name) < foldName(parts[j].Name)
	})
	return parts, nil
}

// CreatePart adds a vocabulary entry. ANY member may create one (while filling a line-up) — it defaults to a
// personal instrument, as members create tags (gate ruling). An existing part with the same folded name is
// returned as-is (no duplicate). Metadata edits beyond creation are admin-only (UpdatePart).
func (s *Service) CreatePart(caller User, bandID, name string) (Part, error) {
	band, _, err := s.GetBand(caller, bandID) // membership is enough to create
	if err != nil {
		return Part{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Part{}, fmt.Errorf("%w: a part needs a name", ErrInvalidInput)
	}
	for _, p := range band.Parts {
		if foldName(p.Name) == foldName(name) {
			return p, nil // idempotent: reuse the existing vocabulary entry
		}
	}
	p := Part{ID: s.newID(), Name: name, Kind: PartInstrument} // personal instrument by default
	band.Parts = append(band.Parts, p)
	if err := s.repo.UpdateBand(band); err != nil {
		return Part{}, err
	}
	return p, nil
}

// PartPatch edits a part's metadata (admin): rename + kind/short/input/shared/order. Rename is metadata-only
// (line-ups reference the id).
type PartPatch struct {
	Name   *string
	Kind   *string
	Short  *string
	Input  *string
	Shared *bool
	Order  *int
}

func (s *Service) UpdatePart(caller User, bandID, partID string, p PartPatch) (Part, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Part{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Part{}, err
	}
	_, idx := findPart(band.Parts, partID)
	if idx < 0 {
		return Part{}, ErrNotFound
	}
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" {
			return Part{}, fmt.Errorf("%w: a part needs a name", ErrInvalidInput)
		}
		band.Parts[idx].Name = n
	}
	if p.Kind != nil {
		band.Parts[idx].Kind = *p.Kind
	}
	if p.Short != nil {
		band.Parts[idx].Short = strings.TrimSpace(*p.Short)
	}
	if p.Input != nil {
		band.Parts[idx].Input = *p.Input
	}
	if p.Shared != nil {
		band.Parts[idx].Shared = *p.Shared
	}
	if p.Order != nil {
		band.Parts[idx].Order = *p.Order
	}
	if err := s.repo.UpdateBand(band); err != nil {
		return Part{}, err
	}
	return band.Parts[idx], nil
}

// MergeParts folds fromID into toID — admin only. Every line-up referencing fromID is rewritten to toID
// (unioned per entry); then fromID is deleted. This is the ONLY band-wide rewrite the vocabulary does.
func (s *Service) MergeParts(caller User, bandID, fromID, toID string) error {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	if fromID == toID {
		return fmt.Errorf("%w: cannot merge a part into itself", ErrInvalidInput)
	}
	_, fi := findPart(band.Parts, fromID)
	_, ti := findPart(band.Parts, toID)
	if fi < 0 || ti < 0 {
		return ErrNotFound
	}
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return err
	}
	for _, sg := range songs {
		if remapped, changed := remapLineupPart(sg.Lineup, fromID, toID); changed {
			sg.Lineup = remapped
			if err := s.repo.UpdateSong(sg); err != nil {
				return err
			}
		}
	}
	band.Parts = append(band.Parts[:fi], band.Parts[fi+1:]...)
	return s.repo.UpdateBand(band)
}

// DeletePart removes a part. If it is referenced by any line-up, it is refused unless force is set, in which
// case it is first removed from every line-up. Admin only. Returns how many songs referenced it.
func (s *Service) DeletePart(caller User, bandID, partID string, force bool) (int, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return 0, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return 0, err
	}
	_, idx := findPart(band.Parts, partID)
	if idx < 0 {
		return 0, ErrNotFound
	}
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return 0, err
	}
	refs := 0
	for _, sg := range songs {
		if lineupHasPart(sg.Lineup, partID) {
			refs++
		}
	}
	if refs > 0 && !force {
		return refs, fmt.Errorf("%w: part is used by %d song(s)", ErrInvalidInput, refs)
	}
	for _, sg := range songs {
		if remapped, changed := remapLineupPart(sg.Lineup, partID, ""); changed { // "" drops it
			sg.Lineup = remapped
			if err := s.repo.UpdateSong(sg); err != nil {
				return refs, err
			}
		}
	}
	band.Parts = append(band.Parts[:idx], band.Parts[idx+1:]...)
	return refs, s.repo.UpdateBand(band)
}

func findPart(ps []Part, id string) (Part, int) {
	for i, p := range ps {
		if p.ID == id {
			return p, i
		}
	}
	return Part{}, -1
}

func lineupHasPart(l []LineupEntry, partID string) bool {
	for _, e := range l {
		for _, pid := range e.PartIDs {
			if pid == partID {
				return true
			}
		}
	}
	return false
}

// remapLineupPart rewrites a part id to another across a song's line-up (unioning within each entry); a toID
// of "" drops the part. Entries left with no parts are kept (a musician can be listed with a note only).
func remapLineupPart(l []LineupEntry, fromID, toID string) ([]LineupEntry, bool) {
	changed := false
	out := make([]LineupEntry, len(l))
	for i, e := range l {
		if !containsString(e.PartIDs, fromID) {
			out[i] = e
			continue
		}
		changed = true
		next := make([]string, 0, len(e.PartIDs))
		for _, pid := range e.PartIDs {
			if pid == fromID {
				if toID != "" && !containsString(next, toID) {
					next = append(next, toID)
				}
				continue
			}
			if !containsString(next, pid) {
				next = append(next, pid)
			}
		}
		e.PartIDs = next
		out[i] = e
	}
	return out, changed
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
