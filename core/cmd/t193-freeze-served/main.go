// Command t193-freeze-served is the T193 (anchoring stage 0) one-off: before the frozen binary is deployed,
// it writes the position each mark is SERVED at today as its STORED position, so freezing moves nothing VLL
// sees.
//
// Why: frozen, serve and bake hand out a mark's stored Points/Page. A mark whose stored render hash is
// stale (its chart was re-rendered since) is served today at its PROJECTION (chartpdf.Reproject against
// the current render), so the freeze would visibly move it. For every live mark on a generated chart whose
// served position differs from its stored one, this tool writes the served Points/Page as stored, with
// PointsRenderHash = the chart's current blob hash, through the apply engine as a normal mutation (a new
// revision, so history and revert see it). The Anchor is kept as is (stage 4 reads it).
//
// The served position is computed exactly as the pre-freeze serve path did (httpapi reprojectSnapshot):
// only for a mark with an Anchor, on a layer whose file is a generated chart whose fresh render reproduces
// the stored blob byte-for-byte (app.ChartAnchorsIfCurrent); a mark whose run is gone was served at its
// stored coordinates, so it is not listed.
//
// SAFETY:
//   - dry run by DEFAULT; -apply writes;
//   - refuses to run while a server holds the data dir (the app.lock writer lock the server takes for its
//     lifetime, the same check as OPS07's offline CLIs); stop the server, back the data dir up, dry run,
//     apply, then deploy the frozen binary;
//   - file backends only (TROUBA_APP_STORE=file, TROUBA_STORE=file), the deployed configuration;
//   - output is ids and numbers only: no band, song or lyric text (this repo is public).
//
// Usage:
//
//	t193-freeze-served -data <dataDir> [-apply] [-actor t193-freeze]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "t193-freeze-served:", err)
		os.Exit(1)
	}
}

// errLive is returned when a server holds the data dir.
var errLive = errors.New("a server holds this data dir (app.lock) — stop it first; this tool never writes behind a live server")

// report is what one pass found (and, with -apply, wrote).
type report struct {
	Scanned, Differ, Written int
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("t193-freeze-served", flag.ContinueOnError)
	fs.SetOutput(out)
	dir := fs.String("data", "", "the data dir (TROUBA_DATA_DIR) of a STOPPED server")
	apply := fs.Bool("apply", false, "write the served positions (default: dry run, no writes)")
	actor := fs.String("actor", "t193-freeze", "author id stamped on the writes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" || fs.NArg() != 0 {
		return errors.New("usage: t193-freeze-served -data <dataDir> [-apply] [-actor id]")
	}
	if _, err := os.Stat(*dir); err != nil {
		return err
	}
	lock, err := filerepo.LockDir(*dir)
	if errors.Is(err, filerepo.ErrLocked) {
		return errLive
	}
	if err != nil {
		return err
	}
	defer lock.Close()

	repo, err := filerepo.New(*dir)
	if err != nil {
		return fmt.Errorf("open app repo: %w", err)
	}
	ha, ok := filestore.New(*dir).(store.HistoryAware)
	if !ok {
		return errors.New("annotation store is not history-aware")
	}
	_, err = freeze(repo, engine.New(ha), *apply, *actor, out)
	return err
}

// chartRender is one generated file's current render, resolved once.
type chartRender struct {
	anchors []chartpdf.Anchor
	hash    string
	ok      bool // the fresh render reproduces the stored blob (else serve left stored coordinates)
}

func freeze(repo *filerepo.Repo, eng *engine.Engine, apply bool, actor string, out io.Writer) (report, error) {
	var rep report
	files, err := repo.AllSongFiles()
	if err != nil {
		return rep, err
	}
	svc := app.NewService(repo)
	generated := map[string]app.SongFile{}
	songSet := map[string]bool{}
	for _, f := range files {
		if f.Generated {
			generated[f.ID] = f
			songSet[f.SongID] = true
		}
	}
	songs := make([]string, 0, len(songSet))
	for id := range songSet {
		songs = append(songs, id)
	}
	sort.Strings(songs)

	renders := map[string]chartRender{}
	renderOf := func(fileID string) chartRender {
		if r, seen := renders[fileID]; seen {
			return r
		}
		var r chartRender
		if sf, src, err := svc.ChartSourceForFile(fileID); err == nil {
			if anchors, ok := app.ChartAnchorsIfCurrent(src, sf.BlobHash); ok {
				r = chartRender{anchors: anchors, hash: sf.BlobHash, ok: true}
			}
		}
		renders[fileID] = r
		return r
	}

	mode := "DRY RUN (no writes)"
	if apply {
		mode = "APPLY (writing served positions)"
	}
	fmt.Fprintf(out, "T193 freeze: store the served position — %s\n", mode)
	fmt.Fprintf(out, "%-40s %-40s %-9s %-10s %-10s %s\n", "object", "file", "page", "stored_y", "served_y", "delta_y")

	for _, songID := range songs {
		snap, err := eng.Head(songID)
		if err != nil {
			return rep, fmt.Errorf("song %s: %w", songID, err)
		}
		fileByLayer := map[string]string{}
		for _, l := range snap.Layers {
			fileByLayer[l.ID] = l.FileID
		}
		for _, o := range snap.LiveObjects() {
			fileID := fileByLayer[o.LayerID]
			if _, gen := generated[fileID]; !gen {
				continue // uploaded / song-level: never projected
			}
			rep.Scanned++
			if o.Anchor == nil {
				continue // never projected: served = stored
			}
			r := renderOf(fileID)
			if !r.ok {
				continue // the render diverges from the blob: serve left stored coordinates
			}
			// The pre-freeze serve path, unfrozen: what VLL sees today.
			rp, _ := chartpdf.Reproject([]domain.Object{o}, r.anchors, r.hash)
			served := rp[0]
			if served.Page == o.Page && reflect.DeepEqual(served.Points, o.Points) {
				continue
			}
			rep.Differ++
			sy, vy := topY(o.Points), topY(served.Points)
			fmt.Fprintf(out, "%-40s %-40s %-9s %-10.4f %-10.4f %+.4f\n", o.UUID, fileID,
				fmt.Sprintf("%d->%d", o.Page, served.Page), sy, vy, vy-sy)
			if !apply {
				continue
			}
			served.PointsRenderHash = r.hash
			served.Version = o.Version + 1
			m := domain.Mutation{
				Kind:        domain.KindMove,
				UUID:        o.UUID,
				Object:      &served,
				BaseVersion: o.Version,
				AuthorID:    actor,
				Summary:     "T193 freeze: store the served position of " + o.UUID,
				ClientTS:    time.Now().UnixMilli(),
			}
			if _, err := eng.Apply(songID, m); err != nil {
				return rep, fmt.Errorf("write %s: %w", o.UUID, err)
			}
			rep.Written++
		}
	}
	fmt.Fprintf(out, "summary: %d mark(s) on generated charts scanned, %d served away from their stored position, %d written\n",
		rep.Scanned, rep.Differ, rep.Written)
	return rep, nil
}

// topY is a mark's top edge (smallest y) on its page.
func topY(pts []domain.Point) float64 {
	if len(pts) == 0 {
		return 0
	}
	y := pts[0].Y
	for _, p := range pts[1:] {
		if p.Y < y {
			y = p.Y
		}
	}
	return y
}
