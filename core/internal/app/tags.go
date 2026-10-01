package app

import "sort"

// TagCount is one of a band's tags with how many songs carry it. The count is the load-bearing half of
// T180 ⟨D2⟩: it is what lets a member tell an established convention from someone's one-off typo at the
// moment of typing, which is the only thing that keeps a free-text vocabulary from fragmenting into
// `encore` / `Encore` / `encores`.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TagCounts is the band-wide tag vocabulary with usage counts — ONE call per editor (T180 ⟨D4⟩), the same
// shape and reason as PendingRehearsalNotes: never one call per song, and never derived on the client from
// a song list it may not hold in full.
//
// Counts are over the DISTINCT STORED spelling. Two spellings that differ only by case or accent are two
// entries here, on purpose: the cloud then shows the fragmentation honestly, and the typing path folds to
// match (⟨D2⟩/⟨D3⟩) so the next tag joins the established spelling rather than minting a third. Merging the
// spellings here would hide the very thing the count exists to surface.
//
// Ordered most-used first (ties broken by the tag, so the result is deterministic for the cloud's "top N"
// and for a test).
func (s *Service) TagCounts(caller User, bandID string) ([]TagCount, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return nil, err
	}
	byTag := map[string]int{}
	for _, sg := range songs {
		// A song's own tags are a set; a duplicate within one song must not count twice.
		seen := map[string]bool{}
		for _, t := range sg.Tags {
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			byTag[t]++
		}
	}
	out := make([]TagCount, 0, len(byTag))
	for t, c := range byTag {
		out = append(out, TagCount{Tag: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out, nil
}
