package app

import (
	"fmt"
	"strings"
	"unicode"
)

// T203 stage 4 — the per-concert console patch (Setlist.patch). Editing it is admin (it is concert production
// config, the same bar as archive/live). The channel keys are band-global (voice:<m> / inst:<m>:<p> /
// shared:<p>), so "copy from" another concert matches by key.

// SetPatch replaces a setlist's console patch. Entries are trimmed; an input ≤ 0 or an empty channel is dropped.
func (s *Service) SetPatch(caller User, bandID, setlistID string, patch []ChannelPatch) (Setlist, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return Setlist{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Setlist{}, err
	}
	clean := cleanPatch(patch)
	if a, b, dup := duplicateInput(clean); dup {
		return Setlist{}, fmt.Errorf("%w: two channels share input %d (%s and %s)", ErrInvalidInput, clean[a].Input, clean[a].Channel, clean[b].Channel)
	}
	sl.Patch = clean
	if err := s.repo.UpdateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

// duplicateInput reports the first pair of entries that share an input number (a console can't patch two
// sources to one input). Returns their indices.
func duplicateInput(patch []ChannelPatch) (int, int, bool) {
	byInput := map[int]int{}
	for i, c := range patch {
		if j, ok := byInput[c.Input]; ok {
			return j, i, true
		}
		byInput[c.Input] = i
	}
	return 0, 0, false
}

// CopyPatchFrom copies another concert's patch into this one, matching by channel key (the keys are band-global).
func (s *Service) CopyPatchFrom(caller User, bandID, setlistID, fromSetlistID string) (Setlist, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return Setlist{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Setlist{}, err
	}
	from, err := s.getSetlistForMember(caller, bandID, fromSetlistID)
	if err != nil {
		return Setlist{}, err
	}
	sl.Patch = cleanPatch(from.Patch)
	if err := s.repo.UpdateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

// AutoNumber fills input numbers 1..N over the amplified channels in the current column order, and PROPOSES a
// strip name where one is empty — never overwriting an existing name (Fable addition 2). Unamplified channels
// get no entry.
func (s *Service) AutoNumber(caller User, bandID, setlistID string, order SheetOrder) (Setlist, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return Setlist{}, err
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Setlist{}, err
	}
	model, err := s.BuildSheet(caller, bandID, setlistID, order)
	if err != nil {
		return Setlist{}, err
	}
	band, _, err := s.GetBand(caller, bandID)
	if err != nil {
		return Setlist{}, err
	}
	partByID := make(map[string]Part, len(band.Parts))
	for _, p := range band.Parts {
		partByID[p.ID] = p
	}
	views, err := s.Musicians(caller, bandID)
	if err != nil {
		return Setlist{}, err
	}
	name := make(map[string]string, len(views))
	for _, v := range views {
		name[v.ID] = v.DisplayName
	}
	existing := make(map[string]ChannelPatch, len(sl.Patch))
	for _, c := range sl.Patch {
		existing[c.Channel] = c
	}
	var next []ChannelPatch
	n := 0
	for _, sc := range model.Subcols {
		if !sc.Amplified {
			continue
		}
		n++
		strip := strings.TrimSpace(existing[sc.Key].Name)
		if strip == "" {
			strip = proposeStrip(sc, name, partByID) // propose only where empty; never overwrite
		}
		next = append(next, ChannelPatch{Channel: sc.Key, Input: n, Name: strip})
	}
	sl.Patch = next
	if err := s.repo.UpdateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

func cleanPatch(in []ChannelPatch) []ChannelPatch {
	var out []ChannelPatch
	for _, c := range in {
		key := strings.TrimSpace(c.Channel)
		if key == "" || c.Input <= 0 {
			continue
		}
		out = append(out, ChannelPatch{Channel: key, Input: c.Input, Name: strings.TrimSpace(c.Name)})
	}
	return out
}

// proposeStrip is the auto-named console strip: "A V" (voice), "A GTR" (a channel), "BASS" (shared).
func proposeStrip(sc SheetSubcol, name map[string]string, partByID map[string]Part) string {
	if sc.Kind == "shared" {
		return strings.ToUpper(partByID[sharedPartID(sc.Key)].Name)
	}
	if sc.ChannelKey == "voice" {
		return initial(name[sc.OwnerID]) + " V"
	}
	return strings.TrimSpace(initial(name[sc.OwnerID]) + " " + strings.ToUpper(sc.Channel)) // "A GTR" / "A DI"
}

func initial(displayName string) string {
	for _, r := range firstName(displayName) {
		return string(unicode.ToUpper(r))
	}
	return "?"
}
