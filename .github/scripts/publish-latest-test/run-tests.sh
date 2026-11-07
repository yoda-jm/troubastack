#!/usr/bin/env bash
# Offline tests of publish-latest.sh against the stand-in gh (bin/gh). No network, nothing live.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd); script=${1:?path to publish-latest.sh}
export PATH="$here/bin:$PATH" GITHUB_REPOSITORY=owner/repo GH_TOKEN=secret-token-do-not-print
w=$(mktemp -d); export FAKE_GH_STATE=$w/state.json FAKE_GH_LOG=$w/calls.log
for a in troubastage-debug.apk troubastage-windows.msi troubastage-linux-x86_64.AppImage troubastage-macos.dmg; do echo "$a" > "$w/$a"; done
A=("$w/troubastage-debug.apk" "$w/troubastage-windows.msi" "$w/troubastage-linux-x86_64.AppImage" "$w/troubastage-macos.dmg")
sha() { printf '%040d' "$1"; }
notes() { printf 'Commit: %s\n' "$1" > "$w/notes.md"; echo "$w/notes.md"; }
state() { python3 - "$FAKE_GH_STATE" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
rel = sorted(f"{r['tag_name']}{'(draft)' if r['draft'] else ''}#{r['id']}@{r['target_commitish'][-2:]} pub={r['published_at']} assets={len(r['assets'])}" for r in s["releases"])
print("releases:", rel, "| tags:", {k: v[-2:] for k, v in sorted(s["tags"].items())})
PY
}
pass=0; failn=0
check() { if eval "$2"; then echo "  PASS $1"; pass=$((pass+1)); else echo "  FAIL $1"; failn=$((failn+1)); fi; }
run() { : > "$FAKE_GH_LOG"; out=$("$script" "$@" 2>&1); code=$?; }
latest_sha() { python3 -c 'import json,sys; s=json.load(open(sys.argv[1])); r=[r for r in s["releases"] if r["tag_name"]=="latest" and not r["draft"]]; print(r[0]["target_commitish"] if r else "none")' "$FAKE_GH_STATE"; }
latest_assets() { python3 -c 'import json,sys; s=json.load(open(sys.argv[1])); r=[r for r in s["releases"] if r["tag_name"]=="latest" and not r["draft"]]; print(len(r[0]["assets"]) if r else 0)' "$FAKE_GH_STATE"; }
count_releases() { python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["releases"]))' "$FAKE_GH_STATE"; }
# The swap went straight through: no "renaming anyway", and the wait after the old tag's DELETE ended at its first
# check (one read of releases/tags/latest before the rename). A tag lookup that can't see `latest` gone (the plural
# git/refs prefix match) waits the full 30 s and renames anyway; only test 11 takes that path, on purpose.
reads_before_rename() { awk '/-X DELETE repos\/owner\/repo\/git\/refs\/tags\/latest$/{on=1;next} /-X PATCH repos\/owner\/repo\/releases\//{on=0} on && $0=="api repos/owner/repo/releases/tags/latest"{n++} END{print n+0}' "$FAKE_GH_LOG"; }
direct() { echo "    reads of releases/tags/latest between the delete and the rename: $(reads_before_rename)"
  check "the swap went straight through (the wait ended at its first check)" '! echo "$out" | grep -q "renaming anyway" && [ "$(reads_before_rename)" = 1 ]'; }

echo "== 1. first publish (no latest yet)"
run "$(sha 1)" "$(notes "$(sha 1)")" "${A[@]}"; echo "$out" | sed 's/^/    /' | tail -3; state
check "exit 0" '[ $code = 0 ]'; check "latest at sha 1 with 4 assets" '[ "$(latest_sha)" = "$(sha 1)" ] && [ "$(latest_assets)" = 4 ]'; check "one release, no latest-next tag" '[ "$(count_releases)" = 1 ] && ! grep -q "latest-next\": " $FAKE_GH_STATE'

echo "== 2. update (sha 2) — and the call order"
run "$(sha 2)" "$(notes "$(sha 2)")" "${A[@]}"; echo "$out" | tail -1 | sed 's/^/    /'; state
check "exit 0" '[ $code = 0 ]'; check "latest at sha 2, 4 assets, one release" '[ "$(latest_sha)" = "$(sha 2)" ] && [ "$(latest_assets)" = 4 ] && [ "$(count_releases)" = 1 ]'
order=$(grep -nE "release create latest-next|DELETE repos/owner/repo/releases/[0-9]+$|DELETE repos/owner/repo/git/refs/tags/latest$|PATCH repos/owner/repo/releases/" "$FAKE_GH_LOG" | sed 's/ -f .*//' | tr '\n' ';')
echo "    order: $order"
check "create next, then delete old release, then its tag, then rename" 'echo "$order" | grep -qE "release create latest-next.*;.*DELETE repos/owner/repo/releases/[0-9]+;.*DELETE repos/owner/repo/git/refs/tags/latest;.*PATCH repos/owner/repo/releases/"'
check "the token never printed" '! echo "$out" | grep -q secret-token && ! grep -q secret-token "$FAKE_GH_LOG"'
direct

echo "== 3. second update (sha 3)"
run "$(sha 3)" "$(notes "$(sha 3)")" "${A[@]}"; state
check "exit 0, latest at sha 3" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 3)" ]'
direct

echo "== 4. an upload fails (a missing asset)"
run "$(sha 4)" "$(notes "$(sha 4)")" "${A[@]:0:3}" "$w/missing.dmg"; echo "$out" | grep -E "::error::" | sed 's/^/    /'; state
check "exit non-zero, names the state" '[ $code != 0 ] && echo "$out" | grep -q "latest is untouched"'
check "latest still at sha 3 with 4 assets" '[ "$(latest_sha)" = "$(sha 3)" ] && [ "$(latest_assets)" = 4 ]'
echo "   … next run (good):"
run "$(sha 4)" "$(notes "$(sha 4)")" "${A[@]}"; echo "$out" | grep "recovery" | sed 's/^/    /'; state
check "recovers (the leftover draft deleted) and publishes sha 4" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 4)" ] && [ "$(count_releases)" = 1 ]'
direct

echo "== 5. deleting the old release fails"
FAKE_GH_FAIL='-X DELETE repos/owner/repo/releases/[0-9]+$@1' run "$(sha 5)" "$(notes "$(sha 5)")" "${A[@]}"; echo "$out" | grep -E "::error::" | sed 's/^/    /'; state
check "exit non-zero; latest still at sha 4" '[ $code != 0 ] && [ "$(latest_sha)" = "$(sha 4)" ]'
run "$(sha 5)" "$(notes "$(sha 5)")" "${A[@]}"; echo "$out" | grep "recovery" | sed 's/^/    /'; state
check "next run: the leftover latest-next deleted and rebuilt; latest at sha 5" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 5)" ] && [ "$(count_releases)" = 1 ]'
direct

echo "== 6. the rename fails after the delete (every try)"
FAKE_GH_FAIL='-X PATCH repos/owner/repo/releases/@all' run "$(sha 6)" "$(notes "$(sha 6)")" "${A[@]}"; echo "$out" | grep -E "::error::" | sed 's/^/    /'; state
check "exit non-zero; says there is NO latest and prints the one-command fix" '[ $code != 0 ] && echo "$out" | grep -q "there is NO latest now. Fix: gh api -X PATCH repos/owner/repo/releases/[0-9]* -f tag_name=latest -f target_commitish=$(sha 6)"'
check "state: no latest, a latest-next at sha 6" '[ "$(latest_sha)" = none ] && grep -q "\"latest-next\"" $FAKE_GH_STATE'
direct
run "$(sha 7)" "$(notes "$(sha 7)")" "${A[@]}"; echo "$out" | grep "recovery" | sed 's/^/    /'; state
check "next run: finishes the rename first, then publishes sha 7" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 7)" ] && [ "$(count_releases)" = 1 ] && echo "$out" | grep -q "finishing that rename first"'
direct

echo "== 7. the rename fails twice, then succeeds (bounded retry)"
FAKE_GH_FAIL='-X PATCH repos/owner/repo/releases/@1,-X PATCH repos/owner/repo/releases/@2' run "$(sha 8)" "$(notes "$(sha 8)")" "${A[@]}"; echo "$out" | grep "rename try" | sed 's/^/    /'
check "exit 0, latest at sha 8 after 2 failed tries" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 8)" ] && [ "$(echo "$out" | grep -c "rename try")" = 2 ]'
direct

echo "== 8. the read-back check: published_at not advancing is caught"
FAKE_GH_FREEZE_CLOCK=1 run "$(sha 9)" "$(notes "$(sha 9)")" "${A[@]}"; echo "$out" | grep -E "::error::" | sed 's/^/    /'
check "exit non-zero, names published_at" '[ $code != 0 ] && echo "$out" | grep -q "published_at .* is not later"'
direct

echo "== 9. the read-back of the git tag: a tag created at the wrong commit is caught"
FAKE_GH_TAG_DRIFT="$(sha 99)" run "$(sha 10)" "$(notes "$(sha 10)")" "${A[@]}"; echo "$out" | grep -E "::error::" | sed 's/^/    /'
check "exit non-zero, names the tag's commit and the fix" '[ $code != 0 ] && echo "$out" | grep -q "git/refs/tags/latest points at $(sha 99), expected $(sha 10)" && echo "$out" | grep -q "Fix: gh api -X PATCH repos/owner/repo/git/refs/tags/latest"'
direct

echo "== 10. the stand-in prefix-matches git/refs as GitHub does (read live on 2026-11-07), so tests 2-9 cover it"
p=$w/prefix.json; echo '{"releases": [], "tags": {"latest-next": "'"$(sha 11)"'"}, "next_id": 1, "clock": 0, "calls": {}}' > "$p"
plural=$(FAKE_GH_STATE=$p gh api repos/owner/repo/git/refs/tags/latest 2>&1); singular=$(FAKE_GH_STATE=$p gh api repos/owner/repo/git/ref/tags/latest 2>&1); sc=$?
echo "    git/refs/tags/latest: $plural | git/ref/tags/latest: $singular"
check "plural answers [latest-next], singular 404" 'echo "$plural" | grep -q "^\[.*refs/tags/latest-next" && [ $sc != 0 ] && echo "$singular" | grep -q "HTTP 404"'

echo "== 11. the old tag still reads after the delete (a lagging read): the rename is attempted anyway"
FAKE_GH_STALE_REFS=1 run "$(sha 12)" "$(notes "$(sha 12)")" "${A[@]}"; echo "$out" | grep -E "still shows|rename|done:" | sed 's/^/    /'; state
check "exit 0, latest at sha 12 with 4 assets, the tag at sha 12" '[ $code = 0 ] && [ "$(latest_sha)" = "$(sha 12)" ] && [ "$(latest_assets)" = 4 ] && FAKE_GH_STATE=$FAKE_GH_STATE gh api repos/owner/repo/git/ref/tags/latest | grep -q "$(sha 12)"'
check "it said it was renaming anyway" 'echo "$out" | grep -q "still shows after 30 s - renaming anyway"'

echo "== $pass passed, $failn failed"; rm -rf "$w"; [ $failn = 0 ]
