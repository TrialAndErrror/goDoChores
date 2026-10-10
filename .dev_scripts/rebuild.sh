#!/usr/bin/env bash
# Rebuild everything from scratch: no layer cache, fresh base images, new
# container. The database volume is kept, so no data is lost.
#   rebuild.sh           this machine
#   rebuild.sh --prod    the server
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; require_env

build_image --no-cache --pull
ship_image
say "Recreating the container on $LABEL (data is kept)"
dc up -d --no-build --force-recreate --remove-orphans
wait_healthy
summary
