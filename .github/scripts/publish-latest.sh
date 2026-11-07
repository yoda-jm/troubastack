#!/usr/bin/env bash
# publish-latest.sh <sha> <notes-file> <asset>... — publish the rolling `latest` pre-release at <sha> with these assets.
#
# The apps read https://api.github.com/repos/<repo>/releases/tags/latest (A88/WIN04: the commit from target_commitish,
# the asset by name, and "newer" = published_at later than the installed build). published_at is set when a release is
# first published and an edit never moves it, so `latest` can't be updated in place: it must be a new release each
# time. To keep the window without a `latest` as short as possible, the new release is built BESIDE the old one, under
# the tag latest-next (published, so its published_at is now, every asset uploaded, the body naming the commit), and
# only then swapped in: delete the old release and its tag, wait until both are gone, rename the new release's tag to
# latest. Between the delete and the rename (two API calls), a check finds no release ("no update") and the site's
# download links 404. Nobody ever sees the new commit beside the old assets: they live on the same release.
#
# Recovery comes first, so one failed run can't wedge the next: no `latest` but a latest-next → finish that rename; a
# leftover latest-next beside a `latest` → delete it and build again. Every failure exits non-zero and says what state
# it left. After the rename, the result is read back and checked.
#
# Env: GITHUB_REPOSITORY (owner/name). gh authenticates from GH_TOKEN, which is never echoed. TAG/NEXT override the tag
# names (tests only).
set -euo pipefail

sha=${1:?usage: publish-latest.sh <sha> <notes-file> <asset>...}; notes=${2:?notes file}; shift 2
assets=("$@"); [ ${#assets[@]} -gt 0 ] || { echo "::error::publish-latest: no assets given"; exit 1; }
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY}; tag=${TAG:-latest}; next=${NEXT:-$tag-next}
title="Latest debug build"

log() { echo "publish-latest: $*"; }
fail() { echo "::error::publish-latest: $*" >&2; exit 1; }   # stderr: it may run inside $(…)

# GET a release by tag: prints its JSON, or nothing when GitHub answers 404. Any other failure stops the script.
release_by_tag() {
  local out
  if out=$(gh api "repos/$repo/releases/tags/$1" 2>&1); then printf '%s' "$out"; return 0; fi
  case "$out" in *"HTTP 404"*|*"Not Found"*) return 0 ;; esac
  fail "reading the release at tag $1 failed: $out"
}
# A tag ref: prints its JSON, or nothing on 404.
tag_ref() {
  local out
  if out=$(gh api "repos/$repo/git/refs/tags/$1" 2>&1); then printf '%s' "$out"; return 0; fi
  case "$out" in *"HTTP 404"*|*"Not Found"*) return 0 ;; esac
  fail "reading the tag ref $1 failed: $out"
}
field() { python3 -c 'import json,sys
v = json.loads(sys.argv[1])
for k in sys.argv[2].split("."): v = v[k]
print(v)' "$1" "$2"; }
asset_names() { python3 -c 'import json,sys; print(" ".join(sorted(a["name"] for a in json.loads(sys.argv[1])["assets"] if a.get("state","uploaded")=="uploaded")))' "$1"; }
delete_release_and_tag() {   # <release id> <tag>: both, errors shown; a tag already gone is fine
  log "delete the release $1 and its tag $2"
  gh api -X DELETE "repos/$repo/releases/$1" >/dev/null || fail "deleting the release $1 (tag $2) failed — it may still be there"
  local out
  if ! out=$(gh api -X DELETE "repos/$repo/git/refs/tags/$2" 2>&1); then
    case "$out" in *"HTTP 404"*|*"HTTP 422"*|*"Not Found"*|*"Reference does not exist"*) ;; *) fail "the release $1 is deleted but deleting its tag $2 failed: $out" ;; esac
  fi
}
rename_to_tag() {   # <release id>: the release gets tag $tag at $sha, 3 tries
  local i
  for i in 1 2 3; do
    if gh api -X PATCH "repos/$repo/releases/$1" -f tag_name="$tag" -f target_commitish="$sha" >/dev/null; then return 0; fi
    log "rename try $i failed"; sleep 2
  done
  return 1
}

# 0. Recovery.
old=$(release_by_tag "$tag"); nxt=$(release_by_tag "$next")
if [ -z "$old" ] && [ -n "$nxt" ]; then
  id=$(field "$nxt" id)
  log "recovery: no $tag but a $next (release $id, commit $(field "$nxt" target_commitish)) — finishing that rename first"
  rename_to_tag "$id" || fail "recovery: renaming $next (release $id) to $tag failed; there is still no $tag. Fix: gh api -X PATCH repos/$repo/releases/$id -f tag_name=$tag -f target_commitish=$(field "$nxt" target_commitish)"
  gh api -X DELETE "repos/$repo/git/refs/tags/$next" >/dev/null 2>&1 || true
  old=$(release_by_tag "$tag"); nxt=""
elif [ -n "$nxt" ]; then
  log "recovery: a leftover $next (release $(field "$nxt" id)) beside $tag — deleting it to build again"
  delete_release_and_tag "$(field "$nxt" id)" "$next"
fi
# gh release create with assets makes a draft first and publishes it after the uploads, so a failed upload leaves a
# draft under $next (invisible by tag): delete those too.
for id in $(gh api "repos/$repo/releases" | python3 -c 'import json,sys; print(" ".join(str(r["id"]) for r in json.load(sys.stdin) if r.get("draft") and r.get("tag_name")==sys.argv[1]))' "$next"); do
  log "recovery: a leftover draft $next (release $id) — deleting it"
  gh api -X DELETE "repos/$repo/releases/$id" >/dev/null || fail "a leftover draft $next (release $id) could not be deleted"
done
[ -z "$(tag_ref "$next")" ] || { gh api -X DELETE "repos/$repo/git/refs/tags/$next" >/dev/null || fail "a leftover tag $next could not be deleted"; }
old_published=""; old_id=""
if [ -n "$old" ]; then old_published=$(field "$old" published_at); old_id=$(field "$old" id); log "$tag is release $old_id, commit $(field "$old" target_commitish), published $old_published"
else log "no $tag yet: the first publish"; fi

# 1. The new release, beside the old one: published, at $sha, every asset uploaded.
log "create $next at $sha with ${#assets[@]} assets"
gh release create "$next" "${assets[@]}" --repo "$repo" --target "$sha" --prerelease --title "$title" --notes-file "$notes" \
  || fail "creating $next failed; $tag is untouched (${old_id:-none}); a partial $next is removed by the next run"
new=$(release_by_tag "$next"); [ -n "$new" ] || fail "$next was created but can't be read back; $tag is untouched"
new_id=$(field "$new" id)
want=$(for a in "${assets[@]}"; do basename "$a"; done | sort | tr '\n' ' ' | sed 's/ $//')
got=$(asset_names "$new")
[ "$got" = "$want" ] || fail "$next (release $new_id) has assets [$got], expected [$want]; $tag is untouched"
[ "$(field "$new" target_commitish)" = "$sha" ] || fail "$next (release $new_id) targets $(field "$new" target_commitish), expected $sha; $tag is untouched"

# 2. The swap: the old release and its tag go, then the new release takes the tag.
if [ -n "$old_id" ]; then
  delete_release_and_tag "$old_id" "$tag"
  for i in $(seq 1 15); do [ -z "$(release_by_tag "$tag")" ] && [ -z "$(tag_ref "$tag")" ] && break; sleep 2; done
  [ -z "$(release_by_tag "$tag")" ] && [ -z "$(tag_ref "$tag")" ] || fail "the old $tag (release $old_id) still shows after 30 s; $next (release $new_id) is ready. Fix: delete it, then gh api -X PATCH repos/$repo/releases/$new_id -f tag_name=$tag -f target_commitish=$sha"
fi
log "rename $next (release $new_id) to $tag"
rename_to_tag "$new_id" || fail "the old $tag is deleted but renaming $next (release $new_id) failed: there is NO $tag now. Fix: gh api -X PATCH repos/$repo/releases/$new_id -f tag_name=$tag -f target_commitish=$sha"
gh api -X DELETE "repos/$repo/git/refs/tags/$next" >/dev/null 2>&1 || true   # the rename leaves the old tag name behind

# 3. Read it back.
got_rel=$(release_by_tag "$tag"); [ -n "$got_rel" ] || fail "after the rename, $tag can't be read (release $new_id)"
[ "$(field "$got_rel" tag_name)" = "$tag" ] || fail "$tag reads back with tag_name $(field "$got_rel" tag_name)"
[ "$(field "$got_rel" target_commitish)" = "$sha" ] || fail "$tag reads back at $(field "$got_rel" target_commitish), expected $sha"
# target_commitish is only what the release says; the tag itself is what downloads and the source archives follow.
ref=$(tag_ref "$tag"); [ -n "$ref" ] || fail "after the rename there is no git tag $tag (release $new_id says $sha)"
[ "$(field "$ref" object.sha)" = "$sha" ] || fail "git/refs/tags/$tag points at $(field "$ref" object.sha), expected $sha (release $new_id). Fix: gh api -X PATCH repos/$repo/git/refs/tags/$tag -f sha=$sha -F force=true"
pub=$(field "$got_rel" published_at)
[ -z "$old_published" ] || [[ "$pub" > "$old_published" ]] || fail "$tag's published_at $pub is not later than the old one's $old_published — the apps would not see it as newer"
[ "$(asset_names "$got_rel")" = "$want" ] || fail "$tag reads back with assets [$(asset_names "$got_rel")], expected [$want]"
log "done: $tag is release $new_id at $sha, published $pub (was ${old_published:-none}), assets [$want]"
