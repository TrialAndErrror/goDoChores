#!/usr/bin/env bash
# Delete EVERYTHING for a target: the container, the image, and the database
# volume (all users and chores). A backup is saved to backups/ first when the app
# is running.
#   nuke.sh [--prod] [--yes] [--no-backup]
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

yes=false; backup=true
for arg in ${REST[@]+"${REST[@]}"}; do
    case $arg in
        --yes) yes=true ;;
        --no-backup) backup=false ;;
        *) die "unknown option: $arg" ;;
    esac
done

if ! $yes; then
    expect=local; [[ $TARGET == prod ]] && expect=$DEPLOY_HOST
    confirm "$expect" "This permanently deletes the app, its image and ALL its data on $LABEL."
fi

if $backup && app_running; then
    say "Saving a backup first"
    saved=$(backup_db)
    say "Backup saved: $saved"
elif $backup; then
    warn "the app is not running, so no backup could be taken"
fi

say "Removing the container, image and data volume on $LABEL"
dc down --volumes --rmi all --remove-orphans
dk image rm "$IMAGE" >/dev/null 2>&1 || true
say "Everything is gone. Start fresh with .dev_scripts/up.sh$([[ $TARGET == prod ]] && echo ' --prod')"
[[ -z ${saved:-} ]] || say "To get your data back afterwards:  .dev_scripts/restore.sh $saved$([[ $TARGET == prod ]] && echo ' --prod')"
