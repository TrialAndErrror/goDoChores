#!/usr/bin/env bash
# Ship the current code to the production server: back up the live database,
# build the image here, send it over SSH, and restart. Same as `up.sh --prod`
# with a backup first.
#   deploy.sh [--no-backup]
source "$(dirname "$0")/lib.sh"
backup=true
for arg in "$@"; do
    case $arg in
        --no-backup) backup=false ;;
        *) die "unknown option: $arg" ;;
    esac
done
parse_target --prod
need docker; require_env

if $backup; then
    if app_running; then
        say "Backing up the live database first"
        say "Backup saved: $(backup_db)"
    else
        warn "the app is not running on $LABEL, so there is nothing to back up (first deploy?)"
    fi
fi

build_image
ship_image
say "Starting on $LABEL"
dc up -d --no-build --remove-orphans
wait_healthy
summary
