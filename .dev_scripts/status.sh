#!/usr/bin/env bash
# Show whether the app is running and healthy, and what the data volume holds.
#   status.sh [--prod]
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

say "Container on $LABEL"
dc ps --all
say "Image"
dk image ls "$IMAGE"
say "Data volume"
dk volume ls --filter "name=$VOLUME"
if app_running; then
    say "Users"
    dc exec -T app /app/goDoChores list-users
fi
