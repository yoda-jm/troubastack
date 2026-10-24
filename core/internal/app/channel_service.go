package app

import (
	"fmt"
	"sort"
	"strings"
)

// T204 — the band's custom console-track groups (Channel) and the admin operations on them. Grouping a part
// onto a channel re-keys its per-concert patches by channel id (never a name), so a rename never unpatches;
// when grouping MERGES two already-patched tracks, the lower input is kept and reported.

// MergeNote records one patch merge caused by grouping (two tracks collapsing onto one channel in a concert).
type MergeNote struct {
	SetlistID    string `json:"setlistId"`
	SetlistName  string `json:"setlistName"`
	Channel      string `json:"channel"` // the resulting patch key
	KeptInput    int    `json:"keptInput"`
	DroppedInput int    `json:"droppedInput"`
}

// Channels lists the band's custom channel groups (membership-gated), sorted by Order then name.
func (s *Service) Channels(caller User, bandID string) ([]Channel, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, err
	}
	out := append([]Channel(nil), band.Channels...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return partOrderLess(out[i].Order, out[j].Order)
		}
		return foldName(out[i].Name) < foldName(out[j].Name)
	})
	return out, nil
}

func (s *Service) CreateChannel(caller User, bandID, name string) (Channel, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Channel{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Channel{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Channel{}, fmt.Errorf("%w: a channel needs a name", ErrInvalidInput)
	}
	max := 0
	for _, c := range band.Channels {
		if c.Order > max {
			max = c.Order
		}
	}
	c := Channel{ID: s.newID(), Name: name, Order: max + 1}
	band.Channels = append(band.Channels, c)
	if err := s.repo.UpdateBand(band); err != nil {
		return Channel{}, err
	}
	return c, nil
}

func (s *Service) RenameChannel(caller User, bandID, channelID, name string) (Channel, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Channel{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Channel{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Channel{}, fmt.Errorf("%w: a channel needs a name", ErrInvalidInput)
	}
	for i := range band.Channels {
		if band.Channels[i].ID == channelID {
			band.Channels[i].Name = name // id unchanged → patches keep resolving (no re-key)
			if err := s.repo.UpdateBand(band); err != nil {
				return Channel{}, err
			}
			return band.Channels[i], nil
		}
	}
	return Channel{}, ErrNotFound
}

// DeleteChannel removes a channel. Refused while any part references it (409 + count), unless force — which
// first clears those parts' ChannelID (they revert to their own p:<id> track).
func (s *Service) DeleteChannel(caller User, bandID, channelID string, force bool) (int, []MergeNote, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return 0, nil, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return 0, nil, err
	}
	idx := -1
	for i, c := range band.Channels {
		if c.ID == channelID {
			idx = i
		}
	}
	if idx < 0 {
		return 0, nil, ErrNotFound
	}
	var reverted []string // the part ids that were on this channel
	for _, p := range band.Parts {
		if p.ChannelID == channelID {
			reverted = append(reverted, p.ID)
		}
	}
	if len(reverted) > 0 && !force {
		return len(reverted), nil, fmt.Errorf("%w: channel used by %d part(s)", ErrInvalidInput, len(reverted))
	}
	for i := range band.Parts {
		if band.Parts[i].ChannelID == channelID {
			band.Parts[i].ChannelID = "" // revert to its own p:<id> track
		}
	}
	band.Channels = append(band.Channels[:idx], band.Channels[idx+1:]...)
	if err := s.repo.UpdateBand(band); err != nil {
		return len(reverted), nil, err
	}
	// Binding 2: a deleted group leaves no stale ch:<m>:g:<id> patch. With ONE reverting part, move the group's
	// patch to that part's p:<id> track; with several, drop it (ambiguous) and report. No g:<id> key may remain.
	notes, err := s.cleanupDeletedGroupPatches(bandID, channelID, reverted)
	return len(reverted), notes, err
}

// cleanupDeletedGroupPatches rewrites per-concert patches that still point at a deleted group's g:<channelID>:
// one reverting part → re-key to its p:<id>; otherwise drop + report.
func (s *Service) cleanupDeletedGroupPatches(bandID, channelID string, reverted []string) ([]MergeNote, error) {
	sls, err := s.repo.SetlistsOfBand(bandID)
	if err != nil {
		return nil, err
	}
	groupKey := "g:" + channelID
	var notes []MergeNote
	for _, sl := range sls {
		byKey := map[string]ChannelPatch{}
		for _, c := range sl.Patch {
			byKey[MigratePatchKey(c.Channel)] = c
		}
		changed := false
		for key, c := range byKey {
			mid, ck, ok := parsePatchKey(key)
			if !ok || ck != groupKey {
				continue
			}
			delete(byKey, key)
			changed = true
			if len(reverted) == 1 {
				target := PatchKey(mid, "p:"+reverted[0])
				if existing, dup := byKey[target]; dup {
					keep := existing
					if c.Input < existing.Input {
						keep = ChannelPatch{Channel: target, Input: c.Input, Name: c.Name}
					}
					byKey[target] = keep
					notes = append(notes, MergeNote{SetlistID: sl.ID, SetlistName: sl.Name, Channel: target, KeptInput: keep.Input, DroppedInput: existing.Input + c.Input - keep.Input})
				} else {
					byKey[target] = ChannelPatch{Channel: target, Input: c.Input, Name: c.Name}
				}
			} else {
				notes = append(notes, MergeNote{SetlistID: sl.ID, SetlistName: sl.Name, Channel: key, KeptInput: 0, DroppedInput: c.Input})
			}
		}
		if changed {
			sl.Patch = sortedPatch(byKey)
			if err := s.repo.UpdateSetlist(sl); err != nil {
				return notes, err
			}
		}
	}
	return notes, nil
}

// SetPartChannel groups a part onto a channel (or clears it with ""). It re-keys the part's per-concert patches
// from its old channel key to the new one (by id, so a rename is irrelevant); a collision with a track already
// patched on the new channel keeps the LOWER input and is reported. Admin.
func (s *Service) SetPartChannel(caller User, bandID, partID, channelID string) ([]MergeNote, error) {
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return nil, err
	}
	p, idx := findPart(band.Parts, partID)
	if idx < 0 {
		return nil, ErrNotFound
	}
	if channelID != "" {
		found := false
		for _, c := range band.Channels {
			if c.ID == channelID {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: no such channel", ErrNotFound)
		}
	}
	oldKey := PartChannelKey(p)
	band.Parts[idx].ChannelID = channelID
	newKey := PartChannelKey(band.Parts[idx])
	if err := s.repo.UpdateBand(band); err != nil {
		return nil, err
	}
	if oldKey == newKey || oldKey == "" || newKey == "" {
		return nil, nil // shared, or no change
	}
	// Binding 1: move the track's patch ONLY when the OLD track disappears (no other part still maps to it). A
	// part leaving a GROUP that keeps other parts must not steal the group's input — the group's patch stays
	// with the remaining parts; the moved part simply joins the new track (with whatever patch it already has).
	// Leaving a `p:<part>` track always qualifies (that track is unique to the one part).
	if channelKeyStillUsed(band.Parts, oldKey) {
		return nil, nil
	}
	return s.rekeyPatches(bandID, oldKey, newKey)
}

// channelKeyStillUsed reports whether any part maps to the given per-musician channel key.
func channelKeyStillUsed(parts []Part, key string) bool {
	for _, p := range parts {
		if PartChannelKey(p) == key {
			return true
		}
	}
	return false
}

// rekeyPatches moves every per-musician patch on oldKey to newKey across the band's setlists, keeping the lower
// input on a collision and reporting it. Legacy T203 keys are migrated first, so it is robust to un-migrated data.
func (s *Service) rekeyPatches(bandID, oldKey, newKey string) ([]MergeNote, error) {
	sls, err := s.repo.SetlistsOfBand(bandID)
	if err != nil {
		return nil, err
	}
	var notes []MergeNote
	for _, sl := range sls {
		byKey := map[string]ChannelPatch{}
		for _, c := range sl.Patch {
			byKey[MigratePatchKey(c.Channel)] = c
		}
		changed := false
		for key, c := range byKey {
			mid, ck, ok := parsePatchKey(key)
			if !ok || ck != oldKey {
				continue
			}
			target := PatchKey(mid, newKey)
			moved := ChannelPatch{Channel: target, Input: c.Input, Name: c.Name}
			if existing, dup := byKey[target]; dup {
				keep, drop := moved, existing
				if existing.Input <= c.Input {
					keep, drop = existing, moved
				}
				byKey[target] = keep
				notes = append(notes, MergeNote{SetlistID: sl.ID, SetlistName: sl.Name, Channel: target, KeptInput: keep.Input, DroppedInput: drop.Input})
			} else {
				byKey[target] = moved
			}
			delete(byKey, key)
			changed = true
		}
		if changed {
			sl.Patch = sortedPatch(byKey)
			if err := s.repo.UpdateSetlist(sl); err != nil {
				return notes, err
			}
		}
	}
	return notes, nil
}

// sortedPatch flattens a key→patch map to a slice in a STABLE order (by input, then channel key) — the map
// iteration order is random, which must not leak into the stored patch (Binding 3).
func sortedPatch(byKey map[string]ChannelPatch) []ChannelPatch {
	next := make([]ChannelPatch, 0, len(byKey))
	for _, c := range byKey {
		next = append(next, c)
	}
	sort.SliceStable(next, func(i, j int) bool {
		if next[i].Input != next[j].Input {
			return next[i].Input < next[j].Input
		}
		return next[i].Channel < next[j].Channel
	})
	return next
}

// parsePatchKey splits ch:<musicianId>:<channelKey>; the channelKey may itself contain ':'.
func parsePatchKey(key string) (musicianID, channelKey string, ok bool) {
	if !strings.HasPrefix(key, "ch:") {
		return "", "", false
	}
	rest := key[len("ch:"):]
	i := strings.IndexByte(rest, ':')
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}
