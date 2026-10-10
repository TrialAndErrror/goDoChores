#!/usr/bin/env bash
# Save a consistent snapshot of the running app's database into backups/.
# Safe to run while the app is serving requests. Backups hold password hashes,
# so they are only readable by you and are git-ignored.
#   backup.sh [--prod]
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

saved=$(backup_db)
say "Backup saved: $saved ($(wc -c < "$saved") bytes)"
