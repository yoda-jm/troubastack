// Command t191-reanchor is the T191 §3 one-off repair runner. Marks created before T191 that were later
// MOVED or RESIZED kept their create-time anchor (the bug), so Reproject drags them back to their old words
// at the next text edit. This re-anchors each such mark from its CURRENT Points, but ONLY when its Points are
// known current — PointsRenderHash equals the file's current render hash — so we re-anchor from exactly what
// VLL sees today. A mark whose hash is stale (its file changed since it was last placed) is LEFT and listed.
//
// SAFETY: dry-run by DEFAULT. --apply writes (one normal move revision per mark, via the engine). Run against
// a COPY of the store with the server STOPPED (filerepo is single-writer, whole-file), and back up app.json +
// songs/ first. Mirrors cmd/migrate-anchors.
//
// PRIVACY: prints marks by id only — never song titles or lyrics. The id→title mapping goes to VLL directly.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	svc *app.Service
	eng *engine.Engine
	rep *filerepo.Repo
}

func open(dir string) (*stores, error) {
	ha, ok := filestore.New(dir).(store.HistoryAware)
	if !ok {
		return nil, fmt.Errorf("%s: store is not HistoryAware", dir)
	}
	rep, err := filerepo.New(dir)
	if err != nil {
		return nil, err
	}
	blobs, err := blob.NewFile(filepath.Join(dir, "blobs"))
	if err != nil {
		return nil, err
	}
	return &stores{svc: app.NewService(rep).WithBlobStore(blobs), eng: engine.New(ha), rep: rep}, nil
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

func runText(a *domain.SourceAnchor) string {
	if a == nil {
		return "(none)"
	}
	off := ""
	if a.Offset != nil {
		off = fmt.Sprintf("+off[%.2f..%.2f]", a.Offset.RelY0, a.Offset.RelY1)
	}
	return fmt.Sprintf("run#%d%s", a.Occurrence, off) // NB: RunText (lyrics) deliberately omitted
}

func kindName(t domain.ObjectType) string {
	switch t {
	case domain.TypeFreehand:
		return "freehand"
	case domain.TypeRect:
		return "rect"
	case domain.TypeLine:
		return "line"
	case domain.TypeHighlight:
		return "highlight"
	case domain.TypeText:
		return "text"
	case domain.TypeIcon:
		return "icon"
	default:
		return fmt.Sprintf("type%d", int(t))
	}
}

func pointsBounds(pts []domain.Point) (x0, y0, x1, y1 float64) {
	if len(pts) == 0 {
		return
	}
	x0, y0, x1, y1 = pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts[1:] {
		if p.X < x0 {
			x0 = p.X
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.Y > y1 {
			y1 = p.Y
		}
	}
	return
}

func absf(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

func maxf(vs ...float64) float64 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func main() {
	dir := flag.String("dir", "", "store dir (operate on a COPY, not the served store)")
	apply := flag.Bool("apply", false, "persist the re-anchors (default: dry-run, no writes)")
	actor := flag.String("actor", "t191-repair", "author id stamped on an --apply write")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: t191-reanchor --dir <copyDir> [--apply]")
		os.Exit(2)
	}
	s, err := open(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	songIDs, err := annotatedSongIDs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list songs:", err)
		os.Exit(1)
	}

	mode := "DRY-RUN (no writes)"
	if *apply {
		mode = "APPLY (writing re-anchors)"
	}
	fmt.Printf("T191 §3 re-anchor repair — %s\n  dir: %s\n  songs: %d\n\n", mode, *dir, len(songIDs))

	var anchored, reanchor, cleared, keptStale, keptDivergent, alreadyOK, writeErrs int

	for _, songID := range songIDs {
		snap, err := s.eng.Head(songID)
		if err != nil {
			continue
		}
		fileByLayer := map[string]string{}
		for _, l := range snap.Layers {
			fileByLayer[l.ID] = l.FileID
		}
		for _, o := range snap.LiveObjects() {
			if o.Anchor == nil {
				continue
			}
			anchored++
			fileID := fileByLayer[o.LayerID]
			if fileID == "" {
				continue
			}
			_, src, err := s.svc.ChartSourceForFile(fileID)
			if err != nil {
				continue // uploaded PDF / no source — never anchored by us
			}
			f, err := s.rep.GetSongFile(fileID)
			if err != nil {
				continue
			}
			anchors, ok := app.ChartAnchorsIfCurrent(src, f.BlobHash)
			if !ok {
				keptDivergent++
				fmt.Printf("  [kept:divergent] song %s file %s obj %s (%s): render does not reproduce stored blob\n",
					songID, fileID, o.UUID, kindName(o.Type))
				continue
			}
			if o.PointsRenderHash != f.BlobHash {
				keptStale++
				fmt.Printf("  [kept:stale-hash] song %s file %s obj %s (%s): PointsRenderHash != file render — leave & review\n",
					songID, fileID, o.UUID, kindName(o.Type))
				continue
			}
			// Staleness test: does the STORED anchor still describe where the mark sits? Project it onto the
			// current render and compare to the mark's current Points. If they diverge, the anchor is stale —
			// the mark was moved/resized away from it and WILL jump to the projected spot at the next edit.
			ox0, oy0, ox1, oy1 := pointsBounds(o.Points)
			pg, px0, py0, px1, py1, pok := chartpdf.Project(*o.Anchor, anchors)
			const eps = 0.004 // ~a quarter line-height on a full page; a move smaller than this is noise
			var dmax float64
			stale := false
			if !pok {
				// The anchored run was edited away. The mark shows at its Points and Reproject leaves it; the
				// dangling anchor is harmless on the current render — out of scope (§4), list it.
				keptDivergent++
				fmt.Printf("  [kept:run-gone] song %s file %s obj %s (%s): anchored run no longer in the render\n",
					songID, fileID, o.UUID, kindName(o.Type))
				continue
			}
			dmax = maxf(absf(px0-ox0), absf(py0-oy0), absf(px1-ox1), absf(py1-oy1))
			stale = pg != o.Page || dmax > eps
			if !stale {
				alreadyOK++
				continue
			}
			ro := chartpdf.Reanchor(o.Clone(), anchors, f.BlobHash)
			oldR, newR := runText(o.Anchor), runText(ro.Anchor)
			if ro.Anchor == nil {
				cleared++
				fmt.Printf("  [re-anchor] song %s file %s obj %s (%s) y=%.3f dmax=%.3f: %s -> (cleared, empty space)\n",
					songID, fileID, o.UUID, kindName(o.Type), oy0, dmax, oldR)
			} else {
				reanchor++
				fmt.Printf("  [re-anchor] song %s file %s obj %s (%s) y=%.3f dmax=%.3f: %s -> %s\n",
					songID, fileID, o.UUID, kindName(o.Type), oy0, dmax, oldR, newR)
			}
			if *apply {
				ro.Version = o.Version + 1
				if _, err := s.eng.Apply(songID, domain.Mutation{
					Kind: domain.KindMove, UUID: o.UUID, Object: &ro, AuthorID: *actor,
					BaseVersion: o.Version, Summary: "T191 re-anchor repair",
				}); err != nil {
					writeErrs++
					fmt.Printf("    ! write failed: %v\n", err)
				}
			}
		}
	}

	fmt.Printf("\nSummary: %d anchored marks on generated charts\n", anchored)
	fmt.Printf("  re-anchor (stale anchor, Points current): %d  (of which cleared to empty space: %d)\n", reanchor+cleared, cleared)
	fmt.Printf("  already correct (anchor matches Points):   %d\n", alreadyOK)
	fmt.Printf("  kept & listed — stale render hash:         %d\n", keptStale)
	fmt.Printf("  kept & listed — divergent render / run gone: %d\n", keptDivergent)
	if *apply {
		fmt.Printf("  write errors: %d\n", writeErrs)
	}
}
