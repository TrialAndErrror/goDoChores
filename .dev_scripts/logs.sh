#!/usr/bin/env bash
# Follow the app's log (the last 100 lines, then new ones). Extra arguments go to
# `docker compose logs`, e.g. --since 1h or --timestamps. Ctrl-C to stop.
#   logs.sh [--prod] [--since 1h] ...
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

dc logs --follow --tail 100 ${REST[@]+"${REST[@]}"} app
