// Command migrate-anchors is the T145 ONE-SHOT back-migration runner. It gives legacy annotation marks
// (drawn before the source-anchor model, so Anchor==nil) a SourceAnchor derived from the FROZEN 08-22
// render, so they re-project onto any later render instead of floating where the words used to be.
//
// Renderer-PINNED: build it from the pre-margin commit (bdfa19fc^ / e58a4fa1) so RenderWithAnchors
// reproduces the 08-22 geometry. Built from post-T146 main it would fail every byte-equality gate
// (harmlessly: 0 migrated) — the gate is what makes that safe.
//
// SAFETY: dry-run by DEFAULT. --apply writes. Run against a COPY of the store with the server STOPPED
// (filerepo is single-writer whole-file). Per Fable's BLOCKER 2 the correct-render manifest comes ONLY from
// a live source that reproduces the archived 08-22 blob byte-for-byte; a source edited since August is
// refused, never anchored against the current (reflowed) render.
//
// The live store was re-seeded since the 08-22 archive, so FILE IDS DIFFER between the two stores (the very
// T150 churn). We therefore match a live file to its 08-22 blob by the STABLE identity — band name + song
// title + filename — not by id.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
)

type stores struct {
	svc  *app.Service
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
	blobs, err := blob.NewFile(filepath.Join(dir, "blobs"))
	if err != nil {
		return nil, fmt.Errorf("%s: open blobs: %w", dir, err)
	}
	return &stores{svc: app.NewService(repo).WithBlobStore(blobs), eng: engine.New(ha), repo: repo}, nil
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// identity keys a file by band name + song title + filename — stable across re-seeds (which churn ids).
func (s *stores) identity(songID, fileID string) (string, bool) {
	f, err := s.repo.GetSongFile(fileID)
	if err != nil {
		return "", false
	}
	sg, err := s.repo.GetSong(songID)
	if err != nil {
		return "", false
	}
	b, err := s.repo.GetBand(sg.BandID)
	if err != nil {
		return "", false
	}
	// Strip the extension: the re-seed added ".txt" to generated-chart filenames (live "lyrics.txt" vs
	// archive "lyrics"), so match on the base name.
	fn := strings.TrimSuffix(f.Filename, filepath.Ext(f.Filename))
	return norm(b.Name) + "\x00" + norm(sg.Title) + "\x00" + norm(fn), true
}

// archiveBlobIndex maps the stable identity → the 08-22 rendered-blob hash, over every file in the archive.
func archiveBlobIndex(a *stores) (map[string]string, error) {
	files, err := a.repo.AllSongFiles()
	if err != nil {
		return nil, err
	}
	idx := map[string]string{}
	for _, f := range files {
		key, ok := a.identity(f.SongID, f.ID)
		if !ok || f.BlobHash == "" {
			continue
		}
		idx[key] = f.BlobHash
	}
	return idx, nil
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

func main() {
	live := flag.String("live", "", "live store dir (operate on a COPY, not the served store)")
	archive := flag.String("archive", "", "the frozen 08-22 archive store dir")
	apply := flag.Bool("apply", false, "persist the anchors (default: dry-run, no writes)")
	actor := flag.String("actor", "t145-migration", "author id stamped on the write")
	flag.Parse()
	if *live == "" || *archive == "" {
		fmt.Fprintln(os.Stderr, "usage: migrate-anchors --live <copyDir> --archive <archiveDir> [--apply]")
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
	archIdx, err := archiveBlobIndex(archS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "index archive:", err)
		os.Exit(1)
	}
	songIDs, err := annotatedSongIDs(*live)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list songs:", err)
		os.Exit(1)
	}

	mode := "DRY-RUN (no writes)"
	if *apply {
		mode = "APPLY (writing anchors)"
	}
	fmt.Printf("T145 back-migration — %s\n  live:    %s\n  archive: %s\n  archive files indexed: %d\n\n", mode, *live, *archive, len(archIdx))

	var (
		songsWithLegacy, songsMigrated, songsBlocked int
		marksMigrated, marksBlocked, marksOverNoRun  int
		writeErrors                                  int
	)

	for _, songID := range songIDs {
		snap, err := liveS.eng.Head(songID)
		if err != nil {
			continue
		}
		fileByLayer := map[string]string{}
		for _, l := range snap.Layers {
			fileByLayer[l.ID] = l.FileID
		}
		byFile := map[string][]domain.Object{}
		for _, o := range snap.LiveObjects() {
			if o.Anchor == nil {
				byFile[fileByLayer[o.LayerID]] = append(byFile[fileByLayer[o.LayerID]], o)
			}
		}
		if len(byFile) == 0 {
			continue
		}
		songsWithLegacy++
		songMigratedAny, songBlockedAny := false, false

		for fileID, marks := range byFile {
			block := func(why string) {
				marksBlocked += len(marks)
				songBlockedAny = true
				fmt.Printf("  [blocked] song %s file %s: %s\n", songID, fileID, why)
			}
			if fileID == "" {
				block("mark on an unresolved layer/file")
				continue
			}
			key, ok := liveS.identity(songID, fileID)
			if !ok {
				block("cannot resolve band/title/filename")
				continue
			}
			archHash := archIdx[key]
			if archHash == "" {
				block("no matching file in the 08-22 archive (by band/title/filename)")
				continue
			}
			_, liveSrc, err := liveS.svc.ChartSourceForFile(fileID)
			if err != nil {
				block(fmt.Sprintf("no source (%v)", err))
				continue
			}
			// BLOCKER 2 gate: the live source must reproduce the 08-22 blob byte-for-byte, else it was edited
			// since August and 08-22 is not the render its marks were drawn on — refuse.
			correctAnchors, ok := app.ChartAnchorsIfCurrent(liveSrc, archHash)
			if !ok {
				block("live source does not reproduce the 08-22 blob (edited since Aug)")
				continue
			}

			migrated, report := chartpdf.MigrateObjects(marks, correctAnchors, archHash)
			marksMigrated += report.Migrated
			marksOverNoRun += report.Unmigratable
			if report.Migrated > 0 {
				songMigratedAny = true
			}
			fmt.Printf("  [ok]      song %s file %s: %d migratable, %d over-no-run(kept)\n",
				songID, fileID, report.Migrated, report.Unmigratable)

			shown := 0
			for _, mo := range migrated {
				if mo.Anchor == nil || shown >= 2 {
					continue
				}
				if _, _, _, _, _, pok := chartpdf.Project(*mo.Anchor, correctAnchors); pok {
					fmt.Printf("            ↳ %s → anchored to %q\n", mo.UUID, mo.Anchor.RunText)
					shown++
				}
			}

			if *apply {
				for i := range migrated {
					mo := migrated[i]
					if mo.Anchor == nil {
						continue // over no run: keep frozen Points, do not write
					}
					cur := marks[i]
					mo.Version = cur.Version + 1
					m := domain.Mutation{
						Kind:        domain.KindSetStyle,
						UUID:        mo.UUID,
						Object:      &mo,
						BaseVersion: cur.Version,
						AuthorID:    *actor,
						Summary:     "T145 back-anchor legacy mark " + mo.UUID,
						ClientTS:    time.Now().UnixMilli(),
					}
					if _, err := liveS.eng.Apply(songID, m); err != nil {
						writeErrors++
						fmt.Printf("  [WRITE FAIL] song %s mark %s: %v\n", songID, mo.UUID, err)
					}
				}
			}
		}
		if songMigratedAny {
			songsMigrated++
		}
		if songBlockedAny && !songMigratedAny {
			songsBlocked++
		}
	}

	fmt.Printf("\nsummary\n  songs with legacy marks   %d\n  songs migrated            %d\n  songs blocked             %d\n  marks anchored            %d\n  marks blocked             %d\n  marks over-no-run (kept)  %d\n",
		songsWithLegacy, songsMigrated, songsBlocked, marksMigrated, marksBlocked, marksOverNoRun)
	if *apply {
		fmt.Printf("  write errors              %d\n", writeErrors)
		if writeErrors > 0 {
			os.Exit(1)
		}
	}
}
