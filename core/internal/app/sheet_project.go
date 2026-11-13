package app

import (
	"sort"
	"strconv"
	"strings"
)

// projMusician is the minimal musician shape the pure projection needs: id, resolved display name, stage order.
type projMusician struct {
	ID     string
	Name   string
	Order  int
	Hidden bool // T210: band-hidden — filterHidden drops it (and its line-up entries) before projecting.
}

// filterHidden removes the excluded musicians AND their line-up entries from the inputs, so the WHOLE sheet
// projection — columns, shared-part players, cells, the changes column, the input list and auto-number —
// treats them as absent, "as if they don't exist" (T210). excluded is the union of band-hidden musicians
// (Musician.Hidden) and a concert's AbsentMusicianIDs. This is the ONE filter: it lives inside the shared
// projection (projectColumns here, buildGrid in the TS editor), never in a caller, so none of the four sheet
// outputs can drift. Pure — it returns fresh copies and never mutates the inputs; an excluded id that is not a
// musician simply matches no one. No exclusions ⇒ the inputs are returned unchanged (byte-identical pre-T210).
func filterHidden(mus []projMusician, songs []Song, excluded map[string]bool) ([]projMusician, []Song) {
	if len(excluded) == 0 {
		return mus, songs
	}
	fmus := make([]projMusician, 0, len(mus))
	for _, m := range mus {
		if !excluded[m.ID] {
			fmus = append(fmus, m)
		}
	}
	fsongs := make([]Song, len(songs))
	for i, sg := range songs {
		fsongs[i] = sg
		le := make([]LineupEntry, 0, len(sg.Lineup))
		for _, e := range sg.Lineup {
			if !excluded[e.MusicianID] {
				le = append(le, e)
			}
		}
		fsongs[i].Lineup = le
	}
	return fmus, fsongs
}

// projExcluded is the exclusion set for a projection: the band-hidden musicians plus the concert-absent ids.
func projExcluded(mus []projMusician, absent []string) map[string]bool {
	ex := map[string]bool{}
	for _, m := range mus {
		if m.Hidden {
			ex[m.ID] = true
		}
	}
	for _, id := range absent {
		ex[id] = true
	}
	return ex
}

// projectColumns is the PURE person-major column projection, in stage order, shared with the TS editor's
// buildGrid (sheet mode) and guarded by the shared test vectors. Only channels USED across the songs appear:
// one Voice column per singing musician, one per personal instrument they play (amplified before unamplified),
// and a shared instrument its OWN column after the people. Input text + shared-header players are filled by the
// caller (they need the concert's patch); here OwnerName is the bare musician / part name.
func projectColumns(songs []Song, mus []projMusician, parts []Part, channels []Channel, absent []string) []SheetSubcol {
	// T210: drop band-hidden and concert-absent musicians (and their line-up entries) at the source, so the
	// column set, a shared part left with nobody, the unused-column dropping and the changes column all follow.
	mus, songs = filterHidden(mus, songs, projExcluded(mus, absent))
	// T204: columns are per-musician CHANNELS (tracks). A channel groups the parts that share its key
	// (PartChannelKey): the default is "voice" for voice parts and "p:<id>" per instrument (so a band with no
	// custom groups is byte-identical to T203); a group is "g:<channelId>". Shared parts keep their own column.
	// Stable tie-break is the roster/vocabulary index, NEVER usage (T203-FIX2).
	partByID := make(map[string]Part, len(parts))
	partIndex := make(map[string]int, len(parts))
	for i, p := range parts {
		partByID[p.ID] = p
		partIndex[p.ID] = i
	}
	chanByID := make(map[string]Channel, len(channels))
	for _, c := range channels {
		chanByID[c.ID] = c
	}
	// band-wide non-shared parts grouped by channel key — drives a channel's label/order/amplified.
	channelParts := map[string][]Part{}
	for _, p := range parts {
		if p.Shared {
			continue
		}
		k := PartChannelKey(p)
		channelParts[k] = append(channelParts[k], p)
	}
	byID := make(map[string]projMusician, len(mus))
	rosterIndex := make(map[string]int, len(mus))
	for i, m := range mus {
		byID[m.ID] = m
		rosterIndex[m.ID] = i
	}

	musChan := map[string]map[string]bool{} // musicianId -> set of used channel keys
	sharedUsed := map[string]bool{}
	for _, sg := range songs {
		for _, e := range sg.Lineup {
			for _, pid := range e.PartIDs {
				p, ok := partByID[pid]
				if !ok {
					continue
				}
				if p.Shared {
					sharedUsed[pid] = true
					continue
				}
				if musChan[e.MusicianID] == nil {
					musChan[e.MusicianID] = map[string]bool{}
				}
				musChan[e.MusicianID][PartChannelKey(p)] = true
			}
		}
	}

	musIDs := make([]string, 0, len(musChan))
	for m := range musChan {
		musIDs = append(musIDs, m)
	}
	sort.SliceStable(musIDs, func(i, j int) bool {
		return orderThenFirst(byID[musIDs[i]].Order, byID[musIDs[j]].Order, rosterIndex[musIDs[i]], rosterIndex[musIDs[j]])
	})

	var subcols []SheetSubcol
	for _, mid := range musIDs {
		owner := byID[mid].Name
		if owner == "" {
			owner = "?"
		}
		cks := make([]string, 0, len(musChan[mid]))
		for ck := range musChan[mid] {
			cks = append(cks, ck)
		}
		sort.SliceStable(cks, func(i, j int) bool { return channelLess(cks[i], cks[j], channelParts, chanByID) })
		for _, ck := range cks {
			subcols = append(subcols, SheetSubcol{
				Key: PatchKey(mid, ck), Kind: "channel", OwnerID: mid, ChannelKey: ck,
				OwnerName: owner, Channel: channelLabel(ck, partByID, chanByID), Amplified: channelAmplified(ck, channelParts),
			})
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
		return partIndex[sharedIDs[i]] < partIndex[sharedIDs[j]]
	})
	for _, pid := range sharedIDs {
		p := partByID[pid]
		subcols = append(subcols, SheetSubcol{Key: SharedChannelKey(pid), Kind: "shared", OwnerID: pid, OwnerName: p.Name, Channel: p.Name, Amplified: p.Input != InputNone})
	}
	return subcols
}

// channelLess orders a musician's channels: the voice channel first, then by (amplified, order, short) so a
// default band (one "p:<id>" channel per instrument) matches T203's instrument order exactly.
func channelLess(a, b string, channelParts map[string][]Part, chanByID map[string]Channel) bool {
	if (a == "voice") != (b == "voice") {
		return a == "voice"
	}
	aa, ba := channelAmplified(a, channelParts), channelAmplified(b, channelParts)
	if aa != ba {
		return aa
	}
	ao, bo := channelOrder(a, channelParts, chanByID), channelOrder(b, channelParts, chanByID)
	if ao != bo {
		return partOrderLess(ao, bo)
	}
	return channelShort(a, channelParts) < channelShort(b, channelParts)
}

func channelAmplified(key string, channelParts map[string][]Part) bool {
	for _, p := range channelParts[key] {
		if p.Input != InputNone {
			return true
		}
	}
	return false
}

func channelOrder(key string, channelParts map[string][]Part, chanByID map[string]Channel) int {
	if strings.HasPrefix(key, "g:") {
		if c, ok := chanByID[strings.TrimPrefix(key, "g:")]; ok && c.Order != 0 {
			return c.Order
		}
	}
	min := 0
	for i, p := range channelParts[key] {
		if i == 0 || partOrderLess(p.Order, min) {
			min = p.Order
		}
	}
	return min
}

func channelShort(key string, channelParts map[string][]Part) string {
	min := ""
	for i, p := range channelParts[key] {
		s := PartShort(p)
		if i == 0 || s < min {
			min = s
		}
	}
	return min
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
