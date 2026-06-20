# Commit model — per-action commits with an in-memory HEAD

- **Status:** Current (WIP).
- **Relates to:** I4 (linear append-only), I5 (LWW), I7 (GC), I6 (server authority).

## Context
Real-time editing *looks* like it would flood git with a commit per stroke. It doesn't: **the
mid-gesture firehose — stroke points while the pen moves, drag interpolation — lives only in the
display/wet layer and is NEVER persisted.** The persisted unit is one mutation per **completed**
gesture (commit-on-release). So per-action commits carry no flood, and the simplest model wins.

## Decision
**One git commit per completed action.** Concretely (the maintainer's model):

1. The server holds the authoritative song **HEAD in memory**.
2. Mutations are applied to HEAD **serialized** per song (single writer); **LWW (I5)** resolves any
   concurrency here, in memory.
3. Each completed action is persisted to the per-song **WAL before it is acked/echoed** (durability),
   then echoed fast.
4. A **background committer** drains the WAL into **one commit per action**, recording HEAD after each
   change. Git I/O stays off the hot path.

Why this is simple:
- **No coalescing/amend machinery** — `revision = commit`.
- **One commit = one action = one author** → native `git blame`; no `Co-authored-by:` needed.
- **`checkpoint`** (on `Mutation`) tags a notable **milestone** for a human-readable timeline view
  over the fine-grained log.

## Conflicts — there are none, by construction
Git merge conflicts need divergent branches + a merge. We never branch (I4) and never merge: git is
a **linear, single-writer, append-only journal** of the in-memory HEAD. Concurrency is resolved
upstream by LWW before anything is committed. Revert appends a commit whose tree = the target
snapshot (computed by us), not the 3-way `git revert` porcelain. The only git-level race is
concurrent **ref updates**, eliminated by single-writer-per-song ownership (shard ownership across
nodes if scaled). **Rule:** git is storage, not the replication/merge transport — never git-push
between nodes (that reintroduces divergence + real conflicts); replication rides the sync protocol.

## Consequences / costs
- Many commits → noisier `git log` + repo growth. Handled by: `checkpoint` milestone view; periodic
  `git gc`/repack (`RepackEvery`); and the **optional** smart-squash GC tier (collapse old history
  below the oldest pin) as long-term compaction — all reference-safe (I7).
- Commits are cheap: git is content-addressed, so each commit writes only the **changed** objects.
- If `go-git` per-commit latency proves low enough, the WAL could be skipped and commits made
  synchronously before ack (simpler, slightly slower acks) — deferred until the gitstore spike.
