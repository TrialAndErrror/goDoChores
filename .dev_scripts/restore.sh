#!/usr/bin/env bash
# Replace the database with a backup made by backup.sh (or nuke.sh). The app is
# stopped for a moment and started again. The database being replaced is kept
# inside the volume as chores.db.before-restore-<time>.
#   restore.sh [--prod] [--yes] backups/local-20260101-120000.db
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; require_env

yes=false; file=
for arg in ${REST[@]+"${REST[@]}"}; do
    case $arg in
        --yes) yes=true ;;
        -*) die "unknown option: $arg" ;;
        *) [[ -z $file ]] || die "give exactly one backup file"; file=$arg ;;
    esac
done
[[ -n $file ]] || die "usage: restore.sh [--prod] [--yes] BACKUP_FILE"
[[ -f $file ]] || die "$file does not exist"
[[ $(head -c 15 "$file") == "SQLite format 3" ]] || die "$file is not a SQLite database"

if ! $yes; then
    expect=local; [[ $TARGET == prod ]] && expect=$DEPLOY_HOST
    confirm "$expect" "This replaces the database on $LABEL with $file."
fi

ensure_image   # after a nuke there is no image yet

if app_running; then
    say "Stopping the app"
    dc stop app
fi

# The data goes in over stdin to a one-off container that shares the volume, and
# the app itself checks the file before it replaces anything.
say "Restoring $file"
dc run --rm --no-deps -T app restore - < "$file"

say "Starting the app"
dc up -d --no-build --remove-orphans
wait_healthy
summary
