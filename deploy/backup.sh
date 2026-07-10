#!/usr/bin/env bash
# TroubaStack backup/restore (OPS01). The ENTIRE server state is the data dir
# (TROUBA_DATA_DIR) when both backends are `file`: app.json (users/bands/songs/
# memberships/invites), songs/*.jsonl (annotations), blobs/ (files), bakes/. One tar
# is a full backup; untar onto a fresh dir is a full restore.
#
# IMPORTANT: stop the server first. filerepo writes app.json as a single whole file
# (atomic rename), and a bake can be mid-write — a hot copy risks a torn bakes/ tree.
#
#   ./backup.sh backup  [DATA_DIR] [OUT_DIR]      # default DATA_DIR=./troubadata, OUT_DIR=.
#   ./backup.sh restore <ARCHIVE> <DATA_DIR>      # DATA_DIR must be empty/absent
#
# Docker-volume deploy: the data lives in the `troubadata` volume, so run this against a
# bind mount of it, e.g.:
#   docker run --rm -v troubadata:/data -v "$PWD":/backup alpine \
#     tar czf /backup/troubastack-backup.tgz -C /data .
set -euo pipefail

cmd="${1:-}"

usage() {
  sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-2}"
}

case "$cmd" in
  backup)
    data_dir="${2:-./troubadata}"
    out_dir="${3:-.}"
    [ -d "$data_dir" ] || { echo "backup: data dir not found: $data_dir" >&2; exit 1; }
    ts="$(date -u +%Y%m%dT%H%M%SZ)"
    archive="$out_dir/troubastack-backup-$ts.tgz"
    mkdir -p "$out_dir"
    # -C into the data dir so the archive holds its CONTENTS at the root (restore-anywhere).
    tar czf "$archive" -C "$data_dir" .
    echo "backup: wrote $archive ($(du -h "$archive" | cut -f1))"
    echo "backup: reminder — take backups with the server stopped (app.json is single-writer)."
    ;;
  restore)
    archive="${2:-}"
    data_dir="${3:-}"
    [ -n "$archive" ] && [ -n "$data_dir" ] || usage 2
    [ -f "$archive" ] || { echo "restore: archive not found: $archive" >&2; exit 1; }
    if [ -d "$data_dir" ] && [ -n "$(ls -A "$data_dir" 2>/dev/null)" ]; then
      echo "restore: refusing — $data_dir is not empty (restore onto a fresh dir)" >&2
      exit 1
    fi
    mkdir -p "$data_dir"
    tar xzf "$archive" -C "$data_dir"
    echo "restore: unpacked $archive into $data_dir — start the server against it."
    ;;
  ""|-h|--help|help)
    usage 0
    ;;
  *)
    echo "unknown command: $cmd" >&2
    usage 2
    ;;
esac
