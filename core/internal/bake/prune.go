package bake

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// PruneStats reports what PruneOutputs reclaimed (printed by the `gc` subcommand).
type PruneStats struct {
	ConcertsScanned int
	RevsDeleted     int
	BytesFreed      int64
}

// PruneOutputs is the bake side of P202's retention (I7): it reclaims disk by dropping
// OLD baked concert revisions — a whole `<concert>/<rev>/` output dir plus its sibling
// `<concert>/<rev>.tstage` — keeping the newest keepN per concert. Bake outputs are the
// real disk-growth source (raster + overlay PNGs per rev); unlike annotation history
// they are NOT a delta chain, so a whole rev is safe to delete outright.
//
// Safety rules — I7 never breaks a reference the server can still be asked for:
//   - keepN <= 0 is a no-op (keep everything). This is the default, so a standard
//     configuration prunes nothing and behavior is byte-identical to before P202.
//   - A rev whose bundle.json has FinalLocked=true is NEVER deleted and does NOT count
//     toward keepN — a locked concert is an explicit "keep this" marker.
//   - The server cannot know which revs a device is still pinned to (no server-side
//     device-pin record exists), so keeping the newest keepN (+ locked) is the policy.
//     A client mid-download of a pruned rev would 404; run this during a maintenance
//     window (the `gc` subcommand documents the "server stopped / same env" caveat).
//
// Staging dirs (`<rev>.tmp`) and any non-numeric entry are ignored, matching the
// baker's rev scanners (nextRev/latestRev).
func PruneOutputs(bakesDir string, keepN int) (PruneStats, error) {
	var stats PruneStats
	if keepN <= 0 {
		return stats, nil // keep-all: the default, byte-identical to no GC
	}
	concerts, err := os.ReadDir(bakesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil // no bakes yet
		}
		return stats, err
	}
	for _, ce := range concerts {
		if !ce.IsDir() {
			continue
		}
		stats.ConcertsScanned++
		concertDir := filepath.Join(bakesDir, ce.Name())
		revs, rerr := numericRevs(concertDir)
		if rerr != nil {
			return stats, rerr
		}
		// Walk newest→oldest, keeping the first keepN NON-locked revs; locked revs are
		// always kept and skipped over, never consuming a keep slot.
		kept := 0
		for i := len(revs) - 1; i >= 0; i-- {
			rev := revs[i]
			if bundleFinalLocked(concertDir, rev) {
				continue
			}
			if kept < keepN {
				kept++
				continue
			}
			freed, derr := deleteRev(concertDir, rev)
			if derr != nil {
				return stats, derr
			}
			stats.RevsDeleted++
			stats.BytesFreed += freed
		}
	}
	return stats, nil
}

// numericRevs returns a concert's published rev numbers, ascending. Non-numeric
// entries (including `<rev>.tmp` staging dirs) are ignored — the baker's rule.
func numericRevs(concertDir string) ([]uint64, error) {
	entries, err := os.ReadDir(concertDir)
	if err != nil {
		return nil, err
	}
	var revs []uint64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, perr := strconv.ParseUint(e.Name(), 10, 64); perr == nil {
			revs = append(revs, n)
		}
	}
	sort.Slice(revs, func(i, j int) bool { return revs[i] < revs[j] })
	return revs, nil
}

// bundleFinalLocked reports whether a rev's bundle.json marks it FinalLocked. A
// missing/corrupt bundle is treated as NOT locked (an incomplete bake is prunable),
// mirroring ListConcerts' skip-don't-fail stance.
func bundleFinalLocked(concertDir string, rev uint64) bool {
	data, err := os.ReadFile(filepath.Join(concertDir, strconv.FormatUint(rev, 10), "bundle.json"))
	if err != nil {
		return false
	}
	var cb ConcertBundle
	if json.Unmarshal(data, &cb) != nil {
		return false
	}
	return cb.FinalLocked
}

// deleteRev removes a concert rev's output dir and its sibling `<rev>.tstage`,
// returning the bytes reclaimed.
func deleteRev(concertDir string, rev uint64) (int64, error) {
	name := strconv.FormatUint(rev, 10)
	revDir := filepath.Join(concertDir, name)
	freed := dirSize(revDir)
	if err := os.RemoveAll(revDir); err != nil {
		return 0, err
	}
	tstage := filepath.Join(concertDir, name+".tstage")
	if fi, err := os.Stat(tstage); err == nil {
		freed += fi.Size()
		if err := os.Remove(tstage); err != nil {
			return freed, err
		}
	}
	return freed, nil
}

// dirSize sums the sizes of regular files under dir (best-effort — walk errors are
// ignored so a partially-removed tree still reports what it can).
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, ferr := d.Info(); ferr == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}
