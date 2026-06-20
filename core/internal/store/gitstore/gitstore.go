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
// GRANULARITY: ONE COMMIT PER COMPLETED ACTION. The mid-gesture firehose — stroke
// points while the pen moves, drag interpolation — lives ONLY in the display/wet layer
// and is NEVER persisted; the persisted unit is one mutation per COMPLETED gesture
// (commit-on-release). So there is no write-time flood: each action maps cleanly to its
// own single-author commit — no coalescing/amend machinery, no co-author case.
//
//	revision        = commit (I4: linear, append-only, no branching)
//	revert(N)        = a revert-commit equal to N (git revert, not reset; I4)
//	one commit       = one action by ONE author -> native `git blame`, no Co-authored-by
//	pin / song head  = a ref/tag onto a commit / branch tip
//	GC               = `git gc` reachability prune (I7)
//
// Layout (bare repo): refs/heads/song/<songID> = head; refs/tags/pin/<…> = pins.
// Implements store.Store. Boundary: store + domain + go-git (later) + stdlib.
//
// HOT PATH (gitstore is the SINK; the apply engine sits ABOVE it — see internal/sync):
// the engine owns the in-memory HEAD, serializes per song, resolves LWW (I5), assigns
// seq, durably logs the ordered action (WAL), acks/echoes, then drains that ordered,
// already-resolved stream into gitstore ASYNC — off the edit hot path. So gitstore only
// ever receives LINEAR, RECONCILED actions and COMMITS them (one per action; revision =
// commit) — it never sees concurrency, never does LWW, never merges. Commits are cheap
// (content-addressed: only CHANGED objects written). At start the engine HYDRATES HEAD
// from gitstore (Head/SnapshotAt).
//
// MULTI-EDITOR & AUDIT: git is downstream of conflict resolution — the sync layer
// settles concurrent edits by LWW (I5) into a total order (server `seq`) BEFORE
// anything is committed, so git never merges. Each action is its own single-author
// commit even under simultaneous editing (interleaved by seq, never merged). The
// append-only mutation log (author_id + client_ts + seq) is the audit source of truth.
//
// CONFLICTS: there are NONE at the git level, by construction. Git merge conflicts
// require divergent branches + a merge; we never branch (I4) and never merge — git is
// a linear, single-writer, append-only log. Concurrent edits to the same object are
// resolved UPSTREAM by LWW (I5) before they ever reach a commit. Revert appends a
// commit whose tree = the target snapshot (computed by us), NOT the 3-way `git revert`
// porcelain — so no merge step exists. The only git-level race is concurrent REF
// updates, avoided by single-writer-per-song ownership (one goroutine/lock per song;
// shard song ownership if multiple server nodes). RULE: git is STORAGE, not the
// replication/merge transport — never git-push between nodes (that reintroduces
// divergence + real conflicts); replication rides the sync protocol.
//
// TIMELINE & SIZE: many commits => a noisy `git log` and repo growth. Handled by
// (a) Mutation.checkpoint, which tags a commit as a notable MILESTONE for a human-
// readable timeline view; (b) periodic `git gc`/repack; (c) the OPTIONAL smart-squash
// GC tier (collapse old history below the oldest pin) as long-term compaction. All
// reference-safe (I7).
package gitstore

import "troubastack/core/internal/store"

// Background-committer knob (tunable). See the package doc.
const (
	RepackEvery = 500 // run `git gc`/repack roughly every N commits to tame loose objects
)

// Git is a go-git-backed Store — a PASSIVE history sink. TODO: hold the
// *git.Repository (go-git v5) and repack bookkeeping. HEAD, per-song serialization,
// and the WAL live in the apply engine ABOVE the store (internal/sync), not here.
type Git struct{ dir string }

// New opens or initialises a git repo at dir. TODO: wire go-git
// (github.com/go-git/go-git/v5); kept as a stub so core builds offline.
func New(dir string) (store.Store, error) { return &Git{dir: dir}, nil }

// Capability assertion: Git is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*Git)(nil)

// Git is Store + HistoryAware + Collector (it retains full history).
func (*Git) Head(string) (any, error) { return nil, store.ErrTODO }

// Apply persists one already-ordered, already-resolved action (HEAD/serialize/WAL live
// in the engine above). For gitstore this is a git commit (revision = commit), driven
// async by the engine off the hot path.
func (*Git) Apply(string, any) error { return store.ErrTODO }

func (*Git) SnapshotAt(string, string) (any, error) { return nil, store.ErrTODO }

// AppendRevision tags a milestone commit (revision = commit; this names a notable one).
func (*Git) AppendRevision(string, any) error     { return store.ErrTODO }
func (*Git) MovePin(string, string, string) error { return store.ErrTODO }

// Collect runs `git gc` over refs (the root set IS the refs) (I7).
func (*Git) Collect(store.RootSet, store.RetentionPolicy) error { return store.ErrTODO }
