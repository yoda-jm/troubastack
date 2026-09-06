// Command recover-annotations is the T159 ONE-SHOT recovery tool. The 2026-09-05 re-seed changed band ids
// and therefore song ids; annotation streams are keyed by song id, so three archived streams never
// re-attached — silent data loss. The songs still exist (by title), so this copies each orphaned stream's
// objects onto its live song.
//
// It matches by (band, title), NOT id (ids are what churned); refuses an ambiguous or missing match;
// copies only objects whose UUID is absent live (safe to run twice); preserves each object EXACTLY and
// NEVER anchors it (T159 ⚠ — anchoring is T145's separate decision). Reports by id/index, never title (this
// repo is public). Unlike migrate-anchors it does NO rendering, so it needs no special build.
//
// SAFETY: dry-run by DEFAULT. --apply writes. Run against a COPY of the store with the server STOPPED
// (filerepo is single-writer whole-file).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"troubastack/core/internal/annrecover"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
)

type stores struct {
	eng  *engine.Engine
	repo *filerepo.Repo
}

func open(dir string) (*stores, error) {
	ha, ok := filestore.New(dir).(store.HistoryAware)
	if !ok {
		return nil, fmt.Errorf("%s: store is not HistoryAware", dir)
	}
	repo, err := filerepo.New(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: open app repo: %w", dir, err)
	}
	return &stores{eng: engine.New(ha), repo: repo}, nil
}

func annotatedSongIDs(dir string) ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(dir, "songs"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range ents {
		if n := e.Name(); strings.HasSuffix(n, ".jsonl") {
			ids = append(ids, strings.TrimSuffix(n, ".jsonl"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// liveIndex maps (band, title) → the live song ids that have it (from every song that has a file).
func liveIndex(s *stores) (map[string][]string, error) {
	files, err := s.repo.AllSongFiles()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	idx := map[string][]string{}
	for _, f := range files {
		if seen[f.SongID] {
			continue
		}
		seen[f.SongID] = true
		sg, err := s.repo.GetSong(f.SongID)
		if err != nil {
			continue
		}
		b, err := s.repo.GetBand(sg.BandID)
		if err != nil {
			continue
		}
		idx[annrecover.TargetKey(b.Name, sg.Title)] = append(idx[annrecover.TargetKey(b.Name, sg.Title)], f.SongID)
	}
	return idx, nil
}

func main() {
	live := flag.String("live", "", "live store dir (operate on a COPY, not the served store)")
	archive := flag.String("archive", "", "the frozen archive store dir (holds the orphaned streams)")
	apply := flag.Bool("apply", false, "persist the recovered objects (default: dry-run, no writes)")
	actor := flag.String("actor", "t159-recovery", "author id stamped on the write")
	flag.Parse()
	if *live == "" || *archive == "" {
		fmt.Fprintln(os.Stderr, "usage: recover-annotations --live <copyDir> --archive <archiveDir> [--apply]")
		os.Exit(2)
	}

	liveS, err := open(*live)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open live:", err)
		os.Exit(1)
	}
	archS, err := open(*archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open archive:", err)
		os.Exit(1)
	}
	idx, err := liveIndex(liveS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "index live:", err)
		os.Exit(1)
	}
	archSongs, err := annotatedSongIDs(*archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list archive songs:", err)
		os.Exit(1)
	}

	mode := "DRY-RUN (no writes)"
	if *apply {
		mode = "APPLY (restoring objects)"
	}
	fmt.Printf("T159 recovery — %s\n  live:    %s\n  archive: %s\n  archived annotated streams: %d\n\n", mode, *live, *archive, len(archSongs))

	var streamsRecovered, streamsAborted, streamsClean, streamsSkipped, objsCopied, layersCreated, writeErrors int

	for i, archSongID := range archSongs {
		snap, err := archS.eng.Head(archSongID)
		if err != nil {
			fmt.Printf("  stream #%d: ABORTED — archive head unreadable: %v\n", i, err)
			streamsAborted++
			continue
		}
		live := 0
		for _, o := range snap.LiveObjects() {
			_ = o
			live++
		}
		if live == 0 {
			// Every object in this stream is a tombstone (or it is empty): nothing to restore. Counted
			// so the summary arithmetic closes — these are marks VLL erased, NOT losses to recover.
			streamsSkipped++
			continue
		}
		sg, err := archS.repo.GetSong(archSongID)
		if err != nil {
			fmt.Printf("  stream #%d: ABORTED — archive song record missing\n", i)
			streamsAborted++
			continue
		}
		b, err := archS.repo.GetBand(sg.BandID)
		if err != nil {
			fmt.Printf("  stream #%d: ABORTED — archive band record missing\n", i)
			streamsAborted++
			continue
		}
		target, err := annrecover.MatchTarget(b.Name, sg.Title, idx)
		if err != nil {
			fmt.Printf("  stream #%d: ABORTED — %v\n", i, err)
			streamsAborted++
			continue
		}
		liveSnap, err := liveS.eng.Head(target)
		if err != nil {
			fmt.Printf("  stream #%d → %s: ABORTED — live target unreadable\n", i, target)
			streamsAborted++
			continue
		}
		plan := annrecover.BuildPlan(snap.LiveObjects(), snap.Layers, liveSnap.LiveObjects(), liveSnap.Layers)
		if len(plan.ObjectsToCopy) == 0 && len(plan.LayersToCreate) == 0 {
			streamsClean++
			continue
		}
		fmt.Printf("  stream #%d → live song %s: %d object(s) to restore, %d layer(s) to create\n",
			i, target, len(plan.ObjectsToCopy), len(plan.LayersToCreate))
		for _, o := range plan.ObjectsToCopy {
			// id + type + point-count + page only — never content (public repo).
			fmt.Printf("      · %s type=%d page=%d points=%d\n", o.UUID, o.Type, o.Page, len(o.Points))
		}
		if !*apply {
			streamsRecovered++
			objsCopied += len(plan.ObjectsToCopy)
			layersCreated += len(plan.LayersToCreate)
			continue
		}
		ok := true
		for _, l := range plan.LayersToCreate {
			nl := l
			if _, err := liveS.eng.Apply(target, domain.Mutation{
				Kind: domain.KindLayerCreate, Layer: &nl, AuthorID: *actor,
				Summary: "T159 restore orphaned layer " + nl.ID,
			}); err != nil {
				writeErrors++
				ok = false
				fmt.Printf("    [LAYER FAIL] %s: %v\n", nl.ID, err)
			} else {
				layersCreated++
			}
		}
		for _, o := range plan.ObjectsToCopy {
			no := o // preserved exactly — points/page/style/layer/owner; no Anchor, no PointsRenderHash
			if no.Version == 0 {
				no.Version = 1
			}
			if _, err := liveS.eng.Apply(target, domain.Mutation{
				Kind: domain.KindCreate, UUID: no.UUID, Object: &no, AuthorID: *actor,
				Summary: "T159 restore orphaned mark " + no.UUID,
			}); err != nil {
				writeErrors++
				ok = false
				fmt.Printf("    [OBJECT FAIL] %s: %v\n", no.UUID, err)
			} else {
				objsCopied++
			}
		}
		if ok {
			streamsRecovered++
		} else {
			streamsAborted++
		}
	}

	fmt.Printf("\nsummary\n  archived annotated streams   %d\n  streams recovered            %d\n  streams already clean         %d\n  streams skipped (all tombstoned/empty) %d\n  streams aborted              %d\n  objects restored             %d\n  layers created               %d\n",
		len(archSongs), streamsRecovered, streamsClean, streamsSkipped, streamsAborted, objsCopied, layersCreated)
	if *apply {
		fmt.Printf("  write errors        %d\n", writeErrors)
		if writeErrors > 0 {
			os.Exit(1)
		}
	}
}
