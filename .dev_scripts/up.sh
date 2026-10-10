#!/usr/bin/env bash
# Build the image (cached), start or update the app, and wait until it is healthy.
# Data in the volume is kept.
#   up.sh           this machine
#   up.sh --prod    the server: also sends the image over SSH
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; require_env

build_image
ship_image
say "Starting on $LABEL"
dc up -d --no-build --remove-orphans
wait_healthy
summary
