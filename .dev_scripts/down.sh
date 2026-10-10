#!/usr/bin/env bash
# Stop and remove the container. The database volume and the image are kept, so
# `up.sh` brings everything back with your data.
#   down.sh           this machine
#   down.sh --prod    the server
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

say "Stopping $LABEL (data is kept)"
dc down --remove-orphans
say "Stopped. Start again with .dev_scripts/up.sh$([[ $TARGET == prod ]] && echo ' --prod')"
