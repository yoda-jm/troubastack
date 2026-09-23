// Command rerender-charts re-renders every GENERATED chart from its stored source, so a change to the
// renderer (2026-09-12: the no-directive body size 11 → 13 pt) reaches charts that already exist.
//
// WHY A TOOL AND NOT A BAKE: a bake rasterises the STORED PDF blob — `baker.go` calls DownloadSongFile,
// which re-renders only when the blob is MISSING. A generated chart therefore carries the bytes it had
// when its source was last saved, and nothing short of saving it again moves them.
//
// WHAT IT COSTS, stated because the operator must see it before pressing --apply: re-rendering changes the
// page geometry, and a mark only follows the text if it carries a T145 anchor. On the library this was
// written for, 15 of 18 marks on generated charts carry NO anchor — those keep their coordinates while the
// words move. The tool therefore prints, per file, how many marks are on it and how many of them will NOT
// follow, and refuses to be quiet about it. It also names any chart whose pages a rehearsal note is keyed
// to (T170): re-rendering changes the raster hash, so that note reads "not in the current bake" afterwards.
//
// SAFETY: dry-run by DEFAULT; --apply writes. filerepo is a single-writer whole-file store, so the server
// MUST be stopped — the tool refuses to run while something is listening on the configured port. Old blobs
// are never deleted: they are content-addressed, so the previous bytes stay reachable and a revert is a
// matter of putting the old hash back.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
)

type row struct {
	band, song, file string
	fileID           string
	oldHash, newHash string
	oldPages, pages  int
	marks, unanchor  int
	noteKeyed        bool
}

func main() {
	data := flag.String("data", "", "server data dir (contains app.json + blobs/)")
	bands := flag.String("bands", "", "bands dir (annotation documents; used only to COUNT marks)")
	notes := flag.String("notes-index", "", "optional: a tablet notes index.json, to flag charts a rehearsal note is keyed to")
	apply := flag.Bool("apply", false, "persist the re-renders (default: dry-run, no writes)")
	port := flag.Int("guard-port", 8080, "refuse to run if something is listening here (the server must be stopped)")
	flag.Parse()
	if *data == "" {
		fmt.Fprintln(os.Stderr, "rerender-charts: --data is required")
		os.Exit(2)
	}
	if *apply {
		if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port)); err != nil {
			fmt.Fprintf(os.Stderr, "rerender-charts: something is listening on :%d — stop the server first "+
				"(filerepo is a single-writer whole-file store; it would overwrite these edits on its next flush)\n", *port)
			os.Exit(1)
		} else {
			_ = ln.Close()
		}
	}

	repo, err := filerepo.New(*data)
	must(err)
	blobs, err := blob.NewFile(filepath.Join(*data, "blobs"))
	must(err)

	markCount, unanchored := loadMarks(*bands)
	noted := loadNotedRasters(*notes)

	files, err := repo.AllSongFiles()
	must(err)
	sort.Slice(files, func(i, j int) bool { return files[i].ID < files[j].ID })

	var rows []row
	skipped, unchanged := 0, 0
	for _, f := range files {
		if !f.Generated {
			continue
		}
		src, err := repo.GetChartSource(f.ID)
		if err != nil || strings.TrimSpace(src) == "" {
			skipped++ // generated but sourceless: nothing to render from, leave it exactly as it is
			continue
		}
		pdf, err := chartpdf.Render(src)
		if err != nil {
			fmt.Printf("!! %s: render failed, left untouched: %v\n", f.ID, err)
			skipped++
			continue
		}
		newHash := blob.HashOf(pdf)
		if newHash == f.BlobHash {
			unchanged++
			continue
		}
		song, _ := repo.GetSong(f.SongID)
		band, _ := repo.GetBand(f.BandID)
		old, _ := blobs.Get(f.BlobHash)
		r := row{
			band: band.Name, song: song.Title, file: f.Filename, fileID: f.ID,
			oldHash: f.BlobHash, newHash: newHash,
			oldPages: countPages(old), pages: countPages(pdf),
			marks: markCount[song.Slug+"|"+f.Filename], unanchor: unanchored[song.Slug+"|"+f.Filename],
			noteKeyed: noted[f.SongID],
		}
		rows = append(rows, r)
		if *apply {
			if _, err := blobs.Put(pdf); err != nil {
				must(err)
			}
			f.BlobHash = newHash
			f.Size = int64(len(pdf))
			f.Revision++ // the ?rev= URL is what stops a browser serving the old PDF
			must(repo.UpdateSongFile(f))
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].unanchor > rows[j].unanchor })
	fmt.Printf("\n%-20s %-34s %-16s %7s %7s  %s\n", "BAND", "SONG", "FILE", "PAGES", "MARKS", "CONSEQUENCE")
	for _, r := range rows {
		cons := "—"
		switch {
		case r.unanchor > 0 && r.noteKeyed:
			cons = fmt.Sprintf("%d mark(s) will NOT follow the text; a rehearsal note is keyed to this chart", r.unanchor)
		case r.unanchor > 0:
			cons = fmt.Sprintf("%d of %d mark(s) will NOT follow the text", r.unanchor, r.marks)
		case r.noteKeyed:
			cons = "a rehearsal note is keyed to this chart — it will read \"not in the current bake\""
		case r.marks > 0:
			cons = fmt.Sprintf("%d mark(s), all anchored — they re-project", r.marks)
		}
		pages := fmt.Sprintf("%d", r.pages)
		if r.oldPages != r.pages {
			pages = fmt.Sprintf("%d→%d", r.oldPages, r.pages)
		}
		fmt.Printf("%-20s %-34s %-16s %7s %7d  %s\n", trunc(r.band, 20), trunc(r.song, 34), trunc(r.file, 16), pages, r.marks, cons)
	}

	var totMarks, totUn, totNoted, pageMoves int
	for _, r := range rows {
		totMarks += r.marks
		totUn += r.unanchor
		if r.noteKeyed {
			totNoted++
		}
		if r.oldPages != r.pages {
			pageMoves++
		}
	}
	fmt.Printf("\n%d chart(s) re-render, %d already current, %d skipped (no source / render error)\n", len(rows), unchanged, skipped)
	fmt.Printf("%d mark(s) on them: %d will NOT follow the text, %d re-project\n", totMarks, totUn, totMarks-totUn)
	fmt.Printf("%d chart(s) carry a rehearsal note; %d chart(s) change page count\n", totNoted, pageMoves)
	if *apply {
		fmt.Println("\nAPPLIED. Old blobs are kept (content-addressed) — a revert is putting the old hash back.")
		fmt.Println("Re-bake every concert so the bundles pick up the new pages.")
	} else {
		fmt.Println("\nDRY RUN — nothing written. Re-run with --apply.")
	}
}

// loadMarks counts, per song-file id, how many annotation objects sit on it and how many of those carry NO
// T145 anchor. The documents key a layer by FILENAME, not by file id, so the join is (song slug → its files
// → filename); a layer whose filename matches nothing is counted nowhere rather than guessed at.
func loadMarks(bandsDir string) (total, unanchored map[string]int) {
	total, unanchored = map[string]int{}, map[string]int{}
	if bandsDir == "" {
		return
	}
	// The count is advisory: it is printed so a human can weigh the cost, never used to decide anything.
	docs, _ := filepath.Glob(filepath.Join(bandsDir, "*", "annotations", "*.json"))
	for _, p := range docs {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var d struct {
			Layers []struct {
				ID   string `json:"id"`
				File string `json:"file"`
			} `json:"layers"`
			Objects []struct {
				Layer   string          `json:"layer"`
				Anchor  json.RawMessage `json:"anchor"`
				Deleted bool            `json:"deleted"`
			} `json:"objects"`
		}
		if json.Unmarshal(b, &d) != nil {
			continue
		}
		layerFile := map[string]string{}
		for _, l := range d.Layers {
			layerFile[l.ID] = l.File
		}
		slug := strings.TrimSuffix(filepath.Base(p), ".json")
		for _, o := range d.Objects {
			if o.Deleted {
				continue
			}
			fn := layerFile[o.Layer]
			// T79 strips the extension from the stored pool name; older documents kept it on the layer.
			// Count under both spellings so the join cannot silently miss and report a reassuring zero.
			for _, key := range []string{slug + "|" + fn, slug + "|" + strings.TrimSuffix(fn, filepath.Ext(fn))} {
				total[key]++
				if len(o.Anchor) == 0 || string(o.Anchor) == "null" {
					unanchored[key]++
				}
				if filepath.Ext(fn) == "" {
					break // both spellings are the same; do not double-count
				}
			}
		}
	}
	return
}

// loadNotedRasters reads a tablet notes index and returns the SONG ids it holds notes for. The caller keys
// by file id, so this is resolved to files by the song they belong to — a note is keyed to a page raster,
// and every page of a song's chart re-renders together.
func loadNotedRasters(path string) map[string]bool {
	out := map[string]bool{}
	if path == "" {
		return out
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: could not read %s (%v) — charts carrying rehearsal notes will NOT be flagged\n", path, err)
		return out
	}
	var idx struct {
		Entries []struct {
			SongID string `json:"songId"`
		} `json:"entries"`
	}
	if json.Unmarshal(b, &idx) != nil {
		return out
	}
	for _, e := range idx.Entries {
		out[e.SongID] = true
	}
	return out
}

func countPages(pdf []byte) int {
	if len(pdf) == 0 {
		return 0
	}
	return strings.Count(string(pdf), "/Type /Page") - strings.Count(string(pdf), "/Type /Pages")
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "rerender-charts:", err)
		os.Exit(1)
	}
}
