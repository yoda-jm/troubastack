package app

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// T203 — the band's Part vocabulary. Line-ups reference a part by ID (M1), so a rename is metadata-only and
// MERGE is the only band-wide rewrite; that removes the rename-vs-line-up-PATCH race.

func foldName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// latinFold maps a precomposed Latin letter to its base, so a short derived from an accented name is plain
// ASCII (the Studio foldText folds the same accents in search). Stdlib has no NFD, so this is an explicit table
// of the Latin-1 Supplement + the common Latin Extended-A letters — a FIXED standard set, not a growing domain
// enumeration. A rune not in the table is left as-is.
var latinFold = map[rune]rune{
	'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A', 'Ā': 'A', 'Ă': 'A', 'Ą': 'A',
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ă': 'a', 'ą': 'a',
	'Ç': 'C', 'Ć': 'C', 'Č': 'C', 'ç': 'c', 'ć': 'c', 'č': 'c',
	'Ď': 'D', 'ď': 'd', 'Đ': 'D', 'đ': 'd',
	'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E', 'Ē': 'E', 'Ė': 'E', 'Ę': 'E', 'Ě': 'E',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e',
	'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I', 'Ī': 'I', 'Į': 'I',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i', 'į': 'i',
	'Ñ': 'N', 'Ń': 'N', 'Ň': 'N', 'ñ': 'n', 'ń': 'n', 'ň': 'n',
	'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O', 'Ø': 'O', 'Ō': 'O', 'Ő': 'O',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o', 'ō': 'o', 'ő': 'o',
	'Ř': 'R', 'ř': 'r',
	'Ś': 'S', 'Š': 'S', 'Ş': 'S', 'ś': 's', 'š': 's', 'ş': 's', 'ß': 's',
	'Ť': 'T', 'ť': 't',
	'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U', 'Ū': 'U', 'Ů': 'U', 'Ű': 'U',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u', 'ů': 'u', 'ű': 'u',
	'Ý': 'Y', 'Ÿ': 'Y', 'ý': 'y', 'ÿ': 'y',
	'Ź': 'Z', 'Ž': 'Z', 'Ż': 'Z', 'ź': 'z', 'ž': 'z', 'ż': 'z',
}

// foldAccents replaces precomposed Latin letters with their base form; other runes pass through unchanged. It
// does NOT change case (that is deriveShort's job).
func foldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if base, ok := latinFold[r]; ok {
			b.WriteRune(base)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// firstRune returns the first rune of s (upper-cased), or "" when empty — rune-safe, so a multi-byte accented
// first letter is not sliced mid-codepoint.
func firstRune(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r))
	}
	return ""
}

// deriveShort is the SINGLE source of a part's fallback abbreviation (T203 stage 2, Fable ruling 3): Studio
// shows the server's effective short and never re-derives, so the grid and the sound sheet can never disagree.
// Accents are folded (NFD) first. A multi-word name → the upper-cased initials ("Lead vocal" → "LV"); a short
// single word (≤3 runes) → title-cased as-is ("Bg" → "Bg"); a longer single word → first rune upper + the next
// two ("Guitar" → "Gui").
func deriveShort(name string) string {
	name = strings.TrimSpace(foldAccents(name))
	fields := strings.Fields(name)
	if len(fields) > 1 {
		var b strings.Builder
		for _, f := range fields {
			b.WriteString(firstRune(f))
		}
		return b.String()
	}
	rs := []rune(name)
	if len(rs) == 0 {
		return ""
	}
	if len(rs) <= 3 {
		return strings.Title(name)
	}
	return string(unicode.ToUpper(rs[0])) + string(rs[1:3])
}

// PartShort returns a part's sheet abbreviation (explicit override, else derived). The HTTP layer fills this
// into every served Part so Studio need not derive.
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
	// Fill the EFFECTIVE short into every served part (ruling 3): Studio displays it and never derives, so the
	// grid and the sound sheet can never drift.
	for i := range parts {
		parts[i].Short = PartShort(parts[i])
	}
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

// PartSpec is the creation-time shape of a part: kind + short + input + shared may all be set AT creation (so
// a member can make a voice or a shared part in one call, and "Add common parts" works for members). An empty
// Kind defaults to a personal instrument. Later metadata EDITS (rename/kind/short/merge) stay admin (UpdatePart).
type PartSpec struct {
	Kind   string
	Short  string
	Input  string
	Shared bool
}

// CreatePart adds a vocabulary entry. ANY member may create one (while filling a line-up), as members create
// tags (gate ruling, reaffirmed T203 stage 2 ruling 2). An existing part with the same folded name is returned
// as-is (no duplicate).
func (s *Service) CreatePart(caller User, bandID, name string, spec PartSpec) (Part, error) {
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
	kind := strings.TrimSpace(spec.Kind)
	switch kind {
	case PartVoice, PartInstrument, PartOther:
	case "":
		kind = PartInstrument // personal instrument by default
	default:
		return Part{}, fmt.Errorf("%w: unknown part kind %q", ErrInvalidInput, spec.Kind)
	}
	p := Part{
		ID:     s.newID(),
		Name:   name,
		Kind:   kind,
		Short:  strings.TrimSpace(spec.Short),
		Input:  strings.TrimSpace(spec.Input),
		Shared: spec.Shared,
	}
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
