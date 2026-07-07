package app

import (
	"sort"
	"strings"
)

// Deterministic listing order (T22). Every Repo listing method ranges a Go map,
// which randomizes order per call — VLL saw a song list that reshuffled on each
// request. These helpers are the single source of truth for list ordering so BOTH
// backends (mem + file) return identical, stable results. Sort keys are
// case-insensitive with an ID tiebreak (stable even for equal names).

// nameLess compares two records by a case-insensitive primary key with an ID
// tiebreak.
func nameLess(aKey, aID, bKey, bID string) bool {
	la, lb := strings.ToLower(aKey), strings.ToLower(bKey)
	if la != lb {
		return la < lb
	}
	return aID < bID
}

// SortBands orders bands by name (ci), then id.
func SortBands(b []Band) {
	sort.Slice(b, func(i, j int) bool { return nameLess(b[i].Name, b[i].ID, b[j].Name, b[j].ID) })
}

// SortSongs orders songs by title (ci), then id.
func SortSongs(s []Song) {
	sort.Slice(s, func(i, j int) bool { return nameLess(s[i].Title, s[i].ID, s[j].Title, s[j].ID) })
}

// SortSetlists orders setlists by name (ci), then id.
func SortSetlists(sl []Setlist) {
	sort.Slice(sl, func(i, j int) bool { return nameLess(sl[i].Name, sl[i].ID, sl[j].Name, sl[j].ID) })
}

// SortMembers orders memberships by join time, then user id — a stable founding
// order (memberships carry no name; the edge resolves display names).
func SortMembers(m []Membership) {
	sort.Slice(m, func(i, j int) bool {
		if !m[i].CreatedAt.Equal(m[j].CreatedAt) {
			return m[i].CreatedAt.Before(m[j].CreatedAt)
		}
		return m[i].UserID < m[j].UserID
	})
}

// SortFiles orders a song's files by DisplayOrder, then id — the intended pool
// order (matches what the default-file / my-files resolution already assumes).
func SortFiles(f []SongFile) {
	sort.Slice(f, func(i, j int) bool {
		if f[i].DisplayOrder != f[j].DisplayOrder {
			return f[i].DisplayOrder < f[j].DisplayOrder
		}
		return f[i].ID < f[j].ID
	})
}

// SortSetlistItems orders items by Position, then id — the performance order.
func SortSetlistItems(it []SetlistItem) {
	sort.Slice(it, func(i, j int) bool {
		if it[i].Position != it[j].Position {
			return it[i].Position < it[j].Position
		}
		return it[i].ID < it[j].ID
	})
}
