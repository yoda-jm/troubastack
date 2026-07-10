// Package gitstore persists durable song history into a git repository via go-git
// (pure Go — preserves core's single static binary, unlike the cgo PDF libs).
//
// WHY GIT FITS (ADR 0002): the domain IS git's object model.
//
//	linear append-only revisions  ->  commits on one branch (I4; no branching)
//	"revert to revision N"         ->  a new commit equal to N (git revert, not reset; I4)
//	content-addressed assets       ->  git blobs dedup identical PDFs/images for free
//	setlist pins / song head       ->  refs/tags / branch tip
//	GC = reachability mark-sweep   ->  `git gc` prunes unreachable objects (I7)
//	inspectable history            ->  `git log` of the annotation timeline
//
// GRANULARITY: ONE COMMIT PER COMPLETED ACTION. Each accepted mutation (and each
// appended revision) is its own single-author commit whose message is the action
// summary, so `git log` IS the history-browser feed (design/01).
//
// MODEL FOR THIS SPIKE: per song we keep an append-only JSONL file in the working
// tree (songs/<id>.jsonl), one record per accepted action. Each Apply/AppendRevision
// appends a line and makes ONE commit (message = summary). HEAD hydrate = read the
// latest commit's tree and fold the JSONL. SnapshotAt(N) = read the commit tagged for
// revision N and fold its JSONL prefix. Pins = git tags. Reopening the repo (a new
// Git instance) reads the same commits, so HEAD is durable.
//
// HOT PATH (gitstore is the SINK; the apply engine sits ABOVE it — see internal/sync):
// the engine owns HEAD, serializes per song, resolves LWW (I5), assigns seq, then
// drains the ordered, already-resolved stream here. So gitstore only ever receives
// LINEAR, RECONCILED actions and COMMITS them — it never sees concurrency, never does
// LWW, never merges. R6 permits SYNCHRONOUS per-action commits for v1 (no WAL).
//
// Boundary: store + domain + go-git + stdlib.
package gitstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// RepackEvery is the background-committer knob: run `git gc`/repack roughly every N
// commits to tame loose objects. (Honored by Collect; tunable.)
const RepackEvery = 500

// Git is a go-git-backed Store — a PASSIVE history sink. The engine above owns HEAD,
// per-song serialization, and (in production) the WAL.
type Git struct {
	mu   sync.Mutex
	repo *git.Repository
	wt   *git.Worktree
}

// New opens or initialises a git repo at dir.
func New(dir string) (store.Store, error) {
	repo, err := git.PlainOpen(dir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(dir, false)
	}
	if err != nil {
		return nil, fmt.Errorf("gitstore: open %s: %w", dir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("gitstore: worktree: %w", err)
	}
	return &Git{repo: repo, wt: wt}, nil
}

// Capability assertion: Git is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*Git)(nil)

// record mirrors the filestore JSONL record so the on-disk format is identical.
type record struct {
	Type     string           `json:"type"` // "mut" | "rev"
	Mutation *domain.Mutation `json:"mutation,omitempty"`
	Revision *domain.Revision `json:"revision,omitempty"`
}

func songFile(songID string) string { return path.Join("songs", songID+".jsonl") }
func revTag(songID string, n uint64) string {
	return fmt.Sprintf("rev/%s/%d", songID, n)
}
func pinTag(songID, name string) string { return fmt.Sprintf("pin/%s/%s", songID, name) }

// readLog reads the JSONL log for a song from the current HEAD tree.
func (g *Git) readLog(songID string) ([]record, error) {
	head, err := g.repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return nil, nil // empty repo
	}
	if err != nil {
		return nil, err
	}
	commit, err := g.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	return readLogFromCommit(commit, songID)
}

func readLogFromCommit(commit *object.Commit, songID string) ([]record, error) {
	f, err := commit.File(songFile(songID))
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	contents, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return parseLog(contents)
}

func parseLog(contents string) ([]record, error) {
	var recs []record
	start := 0
	for i := 0; i <= len(contents); i++ {
		if i == len(contents) || contents[i] == '\n' {
			if i > start {
				var r record
				if err := json.Unmarshal([]byte(contents[start:i]), &r); err != nil {
					return nil, fmt.Errorf("gitstore: corrupt record: %w", err)
				}
				recs = append(recs, r)
			}
			start = i + 1
		}
	}
	return recs, nil
}

// fold splits records into the mutation log + revisions and materializes.
func foldRecords(recs []record) ([]domain.Mutation, []domain.Revision, []int) {
	var log []domain.Mutation
	var revs []domain.Revision
	var revLogLen []int
	for _, r := range recs {
		switch r.Type {
		case "mut":
			log = append(log, *r.Mutation)
		case "rev":
			revs = append(revs, *r.Revision)
			revLogLen = append(revLogLen, len(log))
		}
	}
	return log, revs, revLogLen
}

// commitRecord appends a record line to the song file and makes one commit.
func (g *Git) commitRecord(songID string, r record, message string) (plumbing.Hash, error) {
	recs, err := g.readLog(songID)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	recs = append(recs, r)
	var buf []byte
	for _, rec := range recs {
		b, err := json.Marshal(rec)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	fp := songFile(songID)
	wf, err := g.wt.Filesystem.Create(fp)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := wf.Write(buf); err != nil {
		wf.Close()
		return plumbing.ZeroHash, err
	}
	wf.Close()
	if _, err := g.wt.Add(fp); err != nil {
		return plumbing.ZeroHash, err
	}
	if message == "" {
		message = fmt.Sprintf("%s %s", r.Type, songID)
	}
	author := "unknown"
	if r.Mutation != nil && r.Mutation.AuthorID != "" {
		author = r.Mutation.AuthorID
	} else if r.Revision != nil && r.Revision.AuthorID != "" {
		author = r.Revision.AuthorID
	}
	return g.wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: author, Email: author + "@troubastack", When: time.Now()},
	})
}

// Head folds the current HEAD tree's JSONL.
func (g *Git) Head(songID string) (domain.Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	recs, err := g.readLog(songID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	log, revs, _ := foldRecords(recs)
	return store.Fold(log, uint64(len(revs))), nil
}

// Apply commits one accepted mutation (I2 idempotent by seq).
func (g *Git) Apply(songID string, mut domain.Mutation) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if mut.Seq != 0 {
		recs, err := g.readLog(songID)
		if err != nil {
			return err
		}
		for _, r := range recs {
			if r.Type == "mut" && r.Mutation.Seq == mut.Seq {
				return nil
			}
		}
	}
	m := mut.Clone()
	_, err := g.commitRecord(songID, record{Type: "mut", Mutation: &m}, mut.Summary)
	return err
}

// SnapshotAt reads the commit tagged for revision N and folds its JSONL (I4).
func (g *Git) SnapshotAt(songID string, revision uint64) (domain.Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ref, err := g.repo.Tag(revTag(songID, revision))
	if errors.Is(err, git.ErrTagNotFound) {
		return domain.Snapshot{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Snapshot{}, err
	}
	commit, err := g.commitFromTag(ref)
	if err != nil {
		return domain.Snapshot{}, err
	}
	recs, err := readLogFromCommit(commit, songID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	log, _, _ := foldRecords(recs)
	return store.Fold(log, revision), nil
}

func (g *Git) commitFromTag(ref *plumbing.Reference) (*object.Commit, error) {
	// Annotated tag → tag object → commit; lightweight tag → commit directly.
	if tagObj, err := g.repo.TagObject(ref.Hash()); err == nil {
		return tagObj.Commit()
	}
	return g.repo.CommitObject(ref.Hash())
}

// AppendRevision commits a revision marker and tags it rev/<song>/<N> (I4).
func (g *Git) AppendRevision(songID string, r domain.Revision) (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	recs, err := g.readLog(songID)
	if err != nil {
		return 0, err
	}
	_, revs, _ := foldRecords(recs)
	r.Number = uint64(len(revs)) + 1
	if r.Number > 1 {
		r.Parent = r.Number - 1
	}
	msg := r.Summary
	if msg == "" {
		msg = fmt.Sprintf("revision %d", r.Number)
	}
	hash, err := g.commitRecord(songID, record{Type: "rev", Revision: &r}, msg)
	if err != nil {
		return 0, err
	}
	if _, err := g.repo.CreateTag(revTag(songID, r.Number), hash, nil); err != nil {
		return 0, err
	}
	return r.Number, nil
}

// Revisions returns the linear history oldest→newest.
func (g *Git) Revisions(songID string) ([]domain.Revision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	recs, err := g.readLog(songID)
	if err != nil {
		return nil, err
	}
	_, revs, _ := foldRecords(recs)
	return revs, nil
}

// MovePin points a pin tag at the revision's commit (I4); never deletes its target.
func (g *Git) MovePin(songID, pinName string, revision uint64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	ref, err := g.repo.Tag(revTag(songID, revision))
	if errors.Is(err, git.ErrTagNotFound) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}
	tag := pinTag(songID, pinName)
	// A pin moves: drop the old tag, recreate at the new target (the rev tag retains
	// the target commit, so nothing is orphaned — I7).
	_ = g.repo.DeleteTag(tag)
	commit, err := g.commitFromTag(ref)
	if err != nil {
		return err
	}
	_, err = g.repo.CreateTag(tag, commit.Hash, nil)
	return err
}

// Pins lists the pins for a song by scanning pin/<song>/* tags.
func (g *Git) Pins(songID string) ([]domain.Pin, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	iter, err := g.repo.Tags()
	if err != nil {
		return nil, err
	}
	prefix := "pin/" + songID + "/"
	revByCommit := map[plumbing.Hash]uint64{}
	if err := g.indexRevCommits(songID, revByCommit); err != nil {
		return nil, err
	}
	var out []domain.Pin
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if len(name) <= len(prefix) || name[:len(prefix)] != prefix {
			return nil
		}
		commit, err := g.commitFromTag(ref)
		if err != nil {
			return err
		}
		out = append(out, domain.Pin{
			SongID:         songID,
			Name:           name[len(prefix):],
			RevisionNumber: revByCommit[commit.Hash],
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (g *Git) indexRevCommits(songID string, dst map[plumbing.Hash]uint64) error {
	iter, err := g.repo.Tags()
	if err != nil {
		return err
	}
	prefix := "rev/" + songID + "/"
	return iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if len(name) <= len(prefix) || name[:len(prefix)] != prefix {
			return nil
		}
		var n uint64
		if _, err := fmt.Sscanf(name[len(prefix):], "%d", &n); err != nil {
			return nil
		}
		commit, err := g.commitFromTag(ref)
		if err != nil {
			return err
		}
		dst[commit.Hash] = n
		return nil
	})
}

// Collect runs reachability GC — currently a reference-safe no-op (refs/tags ARE the
// root set, so nothing reachable is dropped; keep-all is the default and always legal).
// A real prune is DEFERRED (P202): go-git exposes no porcelain git.Repository.GC(), so
// a true repack means driving the storer's packfile writer directly (or shelling out to
// `git gc`, which fights the pure-Go single-binary goal) — a design call of its own.
// storetest's ReachabilityI7 subtest locks the "every root still reconstructs" guarantee.
func (g *Git) Collect(_ store.RootSet, _ store.RetentionPolicy) error {
	return nil
}
