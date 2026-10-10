#!/usr/bin/env bash
# Build the Docker image on this machine.
#   build.sh                 use the layer cache
#   build.sh --no-cache      rebuild every layer
#   build.sh --pull          also refresh the base images
# Any other arguments are passed to `docker build`.
source "$(dirname "$0")/lib.sh"
parse_target "$@"
build_image ${REST[@]+"${REST[@]}"}
say "Built $IMAGE"
