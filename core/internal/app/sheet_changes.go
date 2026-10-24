package app

import (
	"sort"
	"strings"
)

// sheetChanges is the sound sheet's "changes" column for one song vs the previous one. It names who comes in /
// out, a voice change ("A: L→B"), a shared instrument changing player ("Bass: C→V (same channel)" — no
// repatch) and a personal instrument moving between people ("repatch: Gtr A→B"). Labels come from the central
// list, so the wording is one edit. Emitted in a stable category order (out, in, voice, shared, repatch),
// alphabetical within each.
func sheetChanges(prev, now Song, partByID map[string]Part, name map[string]string) []string {
	nm := func(mid string) string { return firstName(name[mid]) }
	var out []string

	pPresent, nPresent := presenceSet(prev), presenceSet(now)
	var outs, ins []string
	for mid := range pPresent {
		if !nPresent[mid] {
			outs = append(outs, nm(mid)+sheetLabelOutSuffix)
		}
	}
	for mid := range nPresent {
		if !pPresent[mid] {
			ins = append(ins, nm(mid)+sheetLabelInSuffix)
		}
	}
	sort.Strings(outs)
	sort.Strings(ins)
	out = append(out, outs...)
	out = append(out, ins...)

	// voice changes for a musician present in both
	pv, nv := voiceShortsByMus(prev, partByID, name), voiceShortsByMus(now, partByID, name)
	var voice []string
	for mid := range nPresent {
		if pPresent[mid] && pv[mid] != nv[mid] && (pv[mid] != "" || nv[mid] != "") {
			voice = append(voice, nm(mid)+": "+orDash(pv[mid])+"→"+orDash(nv[mid]))
		}
	}
	sort.Strings(voice)
	out = append(out, voice...)

	// shared instrument: player change on one channel → "same channel"
	ps, ns := playersByPart(prev, partByID, name, true), playersByPart(now, partByID, name, true)
	out = append(out, partChangeLines(ps, ns, partByID, func(short, from, to string) string {
		return short + ": " + from + "→" + to + " (" + SheetLabelSameChannel + ")"
	})...)

	// personal instrument: player change → "repatch"
	pi, ni := playersByPart(prev, partByID, name, false), playersByPart(now, partByID, name, false)
	out = append(out, partChangeLines(pi, ni, partByID, func(short, from, to string) string {
		return SheetLabelRepatch + ": " + short + " " + from + "→" + to
	})...)

	return out
}

func partChangeLines(prev, now map[string]string, partByID map[string]Part, fmtLine func(short, from, to string) string) []string {
	keys := map[string]bool{}
	for k := range prev {
		keys[k] = true
	}
	for k := range now {
		keys[k] = true
	}
	var lines []string
	ks := make([]string, 0, len(keys))
	for k := range keys {
		ks = append(ks, k)
	}
	// sort by the part's short for stable output
	sort.SliceStable(ks, func(i, j int) bool { return PartShort(partByID[ks[i]]) < PartShort(partByID[ks[j]]) })
	for _, pid := range ks {
		if prev[pid] != now[pid] {
			lines = append(lines, fmtLine(PartShort(partByID[pid]), orDash(prev[pid]), orDash(now[pid])))
		}
	}
	return lines
}

func presenceSet(sg Song) map[string]bool {
	m := map[string]bool{}
	for _, e := range sg.Lineup {
		if len(e.PartIDs) > 0 {
			m[e.MusicianID] = true
		}
	}
	return m
}

func voiceShortsByMus(sg Song, partByID map[string]Part, _ map[string]string) map[string]string {
	out := map[string]string{}
	for _, e := range sg.Lineup {
		var shorts []string
		for _, pid := range e.PartIDs {
			if p, ok := partByID[pid]; ok && p.Kind == PartVoice {
				shorts = append(shorts, PartShort(p))
			}
		}
		if len(shorts) > 0 {
			out[e.MusicianID] = strings.Join(shorts, "+")
		}
	}
	return out
}

// playersByPart maps a part id → the joined first names who play it in the song, for shared parts (shared=true)
// or personal instrument parts (shared=false). Voice parts are excluded.
func playersByPart(sg Song, partByID map[string]Part, name map[string]string, shared bool) map[string]string {
	players := map[string][]string{}
	for _, e := range sg.Lineup {
		for _, pid := range e.PartIDs {
			p, ok := partByID[pid]
			if !ok || p.Kind == PartVoice || p.Shared != shared {
				continue
			}
			players[pid] = append(players[pid], firstName(name[e.MusicianID]))
		}
	}
	out := make(map[string]string, len(players))
	for pid, ns := range players {
		sort.Strings(ns)
		out[pid] = strings.Join(ns, "+")
	}
	return out
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
