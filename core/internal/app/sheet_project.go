package app

import (
	"sort"
	"strconv"
	"strings"
)

// projMusician is the minimal musician shape the pure projection needs: id, resolved display name, stage order.
type projMusician struct {
	ID    string
	Name  string
	Order int
}

// projectColumns is the PURE person-major column projection, in stage order, shared with the TS editor's
// buildGrid (sheet mode) and guarded by the shared test vectors. Only channels USED across the songs appear:
// one Voice column per singing musician, one per personal instrument they play (amplified before unamplified),
// and a shared instrument its OWN column after the people. Input text + shared-header players are filled by the
// caller (they need the concert's patch); here OwnerName is the bare musician / part name.
func projectColumns(songs []Song, mus []projMusician, partByID map[string]Part) []SheetSubcol {
	byID := make(map[string]projMusician, len(mus))
	for _, m := range mus {
		byID[m.ID] = m
	}
	musSings := map[string]bool{}
	musInst := map[string]map[string]bool{}
	sharedUsed := map[string]bool{}
	firstMus := map[string]int{}
	firstShared := map[string]int{}
	for si, sg := range songs {
		for _, e := range sg.Lineup {
			if len(e.PartIDs) == 0 {
				continue
			}
			if _, ok := firstMus[e.MusicianID]; !ok {
				firstMus[e.MusicianID] = si
			}
			for _, pid := range e.PartIDs {
				p, ok := partByID[pid]
				if !ok {
					continue
				}
				switch {
				case p.Kind == PartVoice:
					musSings[e.MusicianID] = true
				case p.Shared:
					sharedUsed[pid] = true
					if _, ok := firstShared[pid]; !ok {
						firstShared[pid] = si
					}
				default:
					if musInst[e.MusicianID] == nil {
						musInst[e.MusicianID] = map[string]bool{}
					}
					musInst[e.MusicianID][pid] = true
				}
			}
		}
	}

	usedMus := map[string]bool{}
	for m := range musSings {
		usedMus[m] = true
	}
	for m := range musInst {
		usedMus[m] = true
	}
	musIDs := make([]string, 0, len(usedMus))
	for m := range usedMus {
		musIDs = append(musIDs, m)
	}
	sort.SliceStable(musIDs, func(i, j int) bool {
		return orderThenFirst(byID[musIDs[i]].Order, byID[musIDs[j]].Order, firstMus[musIDs[i]], firstMus[musIDs[j]])
	})

	var subcols []SheetSubcol
	for _, mid := range musIDs {
		owner := byID[mid].Name
		if owner == "" {
			owner = "?"
		}
		if musSings[mid] {
			subcols = append(subcols, SheetSubcol{Key: VoiceChannelKey(mid), Kind: "voice", OwnerID: mid, OwnerName: owner, Channel: "Voice", Amplified: true})
		}
		instIDs := make([]string, 0, len(musInst[mid]))
		for pid := range musInst[mid] {
			instIDs = append(instIDs, pid)
		}
		sort.SliceStable(instIDs, func(i, j int) bool {
			pi, pj := partByID[instIDs[i]], partByID[instIDs[j]]
			ai, aj := pi.Input != InputNone, pj.Input != InputNone
			if ai != aj {
				return ai
			}
			if pi.Order != pj.Order {
				return partOrderLess(pi.Order, pj.Order)
			}
			return PartShort(pi) < PartShort(pj)
		})
		for _, pid := range instIDs {
			p := partByID[pid]
			subcols = append(subcols, SheetSubcol{Key: InstChannelKey(mid, pid), Kind: "inst", OwnerID: mid, OwnerName: owner, Channel: p.Name, Amplified: p.Input != InputNone})
		}
	}
	sharedIDs := make([]string, 0, len(sharedUsed))
	for pid := range sharedUsed {
		sharedIDs = append(sharedIDs, pid)
	}
	sort.SliceStable(sharedIDs, func(i, j int) bool {
		pi, pj := partByID[sharedIDs[i]], partByID[sharedIDs[j]]
		if pi.Order != pj.Order {
			return partOrderLess(pi.Order, pj.Order)
		}
		return firstShared[sharedIDs[i]] < firstShared[sharedIDs[j]]
	})
	for _, pid := range sharedIDs {
		p := partByID[pid]
		subcols = append(subcols, SheetSubcol{Key: SharedChannelKey(pid), Kind: "shared", OwnerID: pid, OwnerName: p.Name, Channel: p.Name, Amplified: p.Input != InputNone})
	}
	return subcols
}

// inputText is the console-input row (row 3) for a channel: "— acoustic" when unamplified, "In N · strip" when
// patched, else "not patched".
func inputText(patchByKey map[string]ChannelPatch, key string, amplified bool) string {
	if !amplified {
		return SheetLabelAcoustic
	}
	if c, ok := patchByKey[key]; ok && c.Input > 0 {
		if strip := strings.TrimSpace(c.Name); strip != "" {
			return "In " + strconv.Itoa(c.Input) + " · " + strip
		}
		return "In " + strconv.Itoa(c.Input)
	}
	return SheetLabelNotPatched
}
