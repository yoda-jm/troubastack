# OPS06 — the image content gate misses `deploy/apps/`, and an allowlist will drift again

**Lane:** web-core (CI). **Size:** XS. **Status:** spec, not started.
**Raised by:** Fable in the OPS04 re-review (`48a7fb48`); filed at VLL's instruction, 2026-09-02.

## The gap

OPS04's publish gate republishes `latest` only when the push touched image content:

```sh
grep -qE '^(core/|web/studio/|web/ink/|web/bake/|Dockerfile$)'
```

But the **final** stage of the `Dockerfile` also does `COPY deploy/apps/ /app/apps/` (line 84),
served at runtime via `TROUBA_APPS_DIR=/app/apps`. **`deploy/apps/` is not in the list**, so a
change there ships nothing: CI goes green, the tag does not move, and the published image keeps
serving the old contents with no signal anywhere.

**Nothing is broken today, and the reason is stronger than I first wrote.** My original framing said
"the failure appears the first time a real app is dropped in". **That is wrong, and I am correcting my
own spec rather than leaving it to mislead whoever picks this up.** `deploy/apps/*.apk` is
**gitignored** (`.gitignore:54`) — only `.gitkeep` is committed. An APK dropped there never appears in
a git diff and never reaches CI at all: embedding one is a *local* `docker compose build`, and a
CI-built image is designed to contain no APK ("build without an APK ⇒ the manifest is empty and the
card is hidden", `deploy/README.md`). So the gate could not miss an APK change even in principle.

**What is actually left is hygiene, and it is worth one line anyway.** The allowlist is supposed to
mirror the Dockerfile's build-context `COPY` set and does not: `deploy/apps/` is copied in and is not
listed. Today only `.gitkeep` could change there, so the impact is nil — but the list will drift
again at the next `COPY`, silently, and in the direction that looks safe (publishing nothing). This
task is about the drift, not about a bug waiting to happen. **Size it accordingly: XS, filler, no
urgency.**

**One thing this task must not do:** add `proto/` to the list. It looks like the same gap — the
Dockerfile copies it at line 40 — but core imports no codegen output (the proto types are
hand-mirrored, I1/P203, and the Dockerfile says so in a comment), and the runtime stage takes only
`/out/troubacore`. Checked before filing. Recorded here as a negative so it does not get "fixed".

## The design question, which is the actual point

Adding one alternation fixes today. It does not fix the shape: **an allowlist has to be updated
every time someone adds a `COPY` to the Dockerfile, and forgetting is silent and looks safe.**

Two options; pick one deliberately and write down why:

- **Extend the allowlist** with `deploy/apps/`. One line. Keeps the gate tight and cheap, and keeps
  a docs commit from republishing. Drifts again on the next `COPY`.
- **Invert to a denylist** — publish unless the diff touches *only* known-irrelevant paths
  (`docs/`, `app/`, `web/site/`, `.github/`, `README.md`, `deploy/*.md`). Fails **safe**: a new
  `COPY` is republished by default, and the worst case is a redundant push rather than a stale
  image. Costs some redundant publishes.

The recommendation is the **denylist**, because the two failure modes are not symmetric: a redundant
republish is a few wasted minutes, and a silently stale public image is a bug report nobody can
reproduce. But it is a judgement call and the churn cost is real — VLL restricted the APK publish
for exactly that reason, so record whichever way it goes.

Whatever is chosen, **tie it to the Dockerfile in a comment** — name the `COPY` lines the pattern is
derived from, so the next person editing either one sees the other.

## Done when

- A commit touching **only** `deploy/apps/` publishes a new image; verify by reading the run's
  `does image content change?` step and by checking that the Docker Hub tag's `tag_last_pushed`
  actually moved. Not by reading the regex. (In practice that means committing a change to
  `.gitkeep` or a sibling non-`.apk` file, since APKs are gitignored.)
- A commit touching **only** `docs/` still does **not** publish — the OPS04 behaviour must survive.
  This is the control arm; a change that publishes on everything has removed the gate, not fixed it.
- The pattern carries a comment naming the Dockerfile `COPY` lines it mirrors.
- `proto/` is still absent from the publish trigger, with the reason recorded.
