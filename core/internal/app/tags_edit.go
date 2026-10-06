package app

import (
	"fmt"
	"strings"
)

// T182 — editing the band's tag vocabulary: rename (which merges), and delete, each applied to every
// song that carries the exact spelling, in ONE repo operation (⟨D5⟩). The authorisation is band
// membership — the same right as editing one song's tags — and the affected songs are computed from the
// store at write time, never trusted from the client.
//
// The fold-and-adopt OFFER of ⟨D4⟩ ("to" folds equal to a different existing spelling → offer that one)
// is the UI's, from TagCounts: by the time we are here `to` is the exact spelling the member chose, and we
// rename to it verbatim. Case-only renames (encore → Encore) are ordinary renames.

// RenameTag changes the spelling `from` to `to` on every song in the band carrying `from`. It returns the
// number of songs changed; ErrNotFound if none carried `from`; ErrInvalidInput if `to` is empty after
// trimming or contains a comma; ErrForbidden to a non-member.
func (s *Service) RenameTag(caller User, bandID, from, to string) (int, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return 0, err
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return 0, fmt.Errorf("%w: tag cannot be empty", ErrInvalidInput)
	}
	if strings.Contains(to, ",") {
		return 0, fmt.Errorf("%w: tag cannot contain a comma", ErrInvalidInput)
	}
	n, err := s.repo.RenameTag(bandID, from, to)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

// DeleteTag removes the spelling `tag` from every song in the band carrying it. It returns the number of
// songs changed; ErrNotFound if none carried it; ErrForbidden to a non-member.
func (s *Service) DeleteTag(caller User, bandID, tag string) (int, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return 0, err
	}
	n, err := s.repo.DeleteTag(bandID, tag)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

// RenameInTags returns a song's tags with `from` replaced by `to`, deduped so a song that carried both
// `from` and `to` ends with one `to` (⟨D4⟩ merge), position preserved at the first occurrence. `changed`
// is false — and the input is returned untouched — when the song did not carry `from`.
func RenameInTags(tags []string, from, to string) (out []string, changed bool) {
	has := false
	for _, t := range tags {
		if t == from {
			has = true
			break
		}
	}
	if !has {
		return tags, false
	}
	out = make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, t := range tags {
		v := t
		if t == from {
			v = to
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, true
}

// DeleteFromTags returns a song's tags with every `tag` removed; `changed` is false when it carried none.
func DeleteFromTags(tags []string, tag string) (out []string, changed bool) {
	has := false
	for _, t := range tags {
		if t == tag {
			has = true
			break
		}
	}
	if !has {
		return tags, false
	}
	out = make([]string, 0, len(tags))
	for _, t := range tags {
		if t != tag {
			out = append(out, t)
		}
	}
	return out, true
}
