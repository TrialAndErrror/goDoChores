#!/usr/bin/env bash
# Shared helpers for the commands in .dev_scripts. Source it; do not run it.
#
# Every command works on one of two targets:
#   local        this machine's Docker, settings from .env
#   --prod       the server in DEPLOY_HOST, settings from .env.production
# For --prod, Docker commands run against the server over SSH
# (DOCKER_HOST=ssh://...), so nothing but the image needs copying there.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

IMAGE=${IMAGE:-gochores:latest}
PROJECT=${COMPOSE_PROJECT_NAME:-gochores}     # compose project; prefixes the volume name
VOLUME=${PROJECT}_chores-data
DEPLOY_HOST=${DEPLOY_HOST:-do-green-family}   # an ssh host alias or user@host
BACKUP_DIR=${BACKUP_DIR:-$ROOT/backups}
export IMAGE

# ---- output ----------------------------------------------------------------
say()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# ---- target selection ------------------------------------------------------
TARGET=local
REST=()   # the arguments left over after --prod is removed

# parse_target "$@": sets TARGET, REST, ENV_FILE and friends.
parse_target() {
    TARGET=local
    REST=()
    local arg
    for arg in "$@"; do
        if [[ $arg == --prod ]]; then TARGET=prod; else REST+=("$arg"); fi
    done

    if [[ $TARGET == prod ]]; then
        ENV_FILE=${ENV_FILE:-.env.production}
        DOCKER_REMOTE=ssh://$DEPLOY_HOST
        LABEL="production ($DEPLOY_HOST)"
    else
        ENV_FILE=${ENV_FILE:-.env}
        DOCKER_REMOTE=
        LABEL=local
    fi
    export ENV_FILE
}

# dk ARGS...: docker, aimed at the chosen target.
dk() {
    if [[ $TARGET == prod ]]; then
        DOCKER_HOST=$DOCKER_REMOTE docker "$@"
    else
        env -u DOCKER_HOST docker "$@"
    fi
}

# dc ARGS...: docker compose, aimed at the chosen target, using the env file.
dc() {
    dk compose --env-file "$ENV_FILE" "$@"
}

# ---- checks ----------------------------------------------------------------
need() { command -v "$1" >/dev/null 2>&1 || die "$1 is not installed (or not on PATH)"; }

# read_var NAME: the last value of NAME in the env file, without quotes.
read_var() {
    local value
    value=$(grep -E "^$1=" "$ENV_FILE" 2>/dev/null | tail -n 1 | cut -d= -f2- | tr -d '\r' || true)
    value=${value%\"}; value=${value#\"}; value=${value%\'}; value=${value#\'}
    printf '%s' "$value"
}

# require_env: the env file exists and has a real secret.
require_env() {
    [[ -f $ENV_FILE ]] || die "$ENV_FILE not found. Create it with:  cp .env.example $ENV_FILE  and set JWT_SECRET (openssl rand -hex 32)"
    local secret
    secret=$(read_var JWT_SECRET)
    [[ -n $secret ]] || die "JWT_SECRET is not set in $ENV_FILE (try: openssl rand -hex 32)"
    [[ $secret != change-me* ]] || die "JWT_SECRET in $ENV_FILE is still the example value"
    (( ${#secret} >= 32 )) || die "JWT_SECRET in $ENV_FILE is shorter than 32 characters (try: openssl rand -hex 32)"
    if [[ $TARGET == prod ]]; then
        [[ $(read_var COOKIE_SECURE) == true ]] || warn "COOKIE_SECURE is not true in $ENV_FILE; set it if the site is served over HTTPS"
    fi
}

# lenient_env: for commands that only tear things down, a missing env file
# should not stop them (compose still wants to read the file it names).
lenient_env() {
    if [[ ! -f $ENV_FILE ]]; then ENV_FILE=/dev/null; export ENV_FILE; fi
}

# ---- image -----------------------------------------------------------------
# build_image [docker build args]: always builds on this machine.
build_image() {
    need docker
    say "Building $IMAGE"
    env -u DOCKER_HOST docker build "$@" -t "$IMAGE" .
}

local_image_id() { env -u DOCKER_HOST docker image inspect --format '{{.Id}}' "$IMAGE" 2>/dev/null || true; }

# ship_image: for --prod, copy the image to the server unless it already has
# exactly this one. A no-op for local.
ship_image() {
    [[ $TARGET == prod ]] || return 0
    need ssh; need gzip
    local want have
    want=$(local_image_id)
    [[ -n $want ]] || die "image $IMAGE has not been built"
    have=$(ssh "$DEPLOY_HOST" docker image inspect --format '{{.Id}}' "$IMAGE" 2>/dev/null || true)
    if [[ $want == "$have" ]]; then
        say "Server already has this image"
        return 0
    fi
    say "Sending image to $DEPLOY_HOST"
    env -u DOCKER_HOST docker save "$IMAGE" | gzip | ssh "$DEPLOY_HOST" 'gunzip | docker load' >/dev/null
}

# ensure_image: build only if there is no image yet, then ship it.
ensure_image() {
    [[ -n $(local_image_id) ]] || build_image
    ship_image
}

# ---- container -------------------------------------------------------------
app_running() { [[ -n $(dc ps -q --status running app 2>/dev/null || true) ]]; }

require_running() {
    app_running || die "the app is not running on $LABEL. Start it first:  .dev_scripts/up.sh$([[ $TARGET == prod ]] && echo ' --prod')"
}

# wait_healthy [seconds]: wait for the health check to pass.
wait_healthy() {
    local timeout=${1:-60} waited=0 cid status
    cid=$(dc ps -q app)
    [[ -n $cid ]] || die "the app container does not exist"
    printf 'Waiting for the app to become healthy'
    while (( waited < timeout )); do
        status=$(dk inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$cid" 2>/dev/null || echo missing)
        case $status in
            healthy) printf ' ok\n'; return 0 ;;
            unhealthy) break ;;
            none) printf ' (no health check)\n'; return 0 ;;
        esac
        printf '.'; sleep 2; waited=$((waited + 2))
    done
    printf ' FAILED\n'
    dc logs --tail 40 app || true
    die "the app did not become healthy; see the log above"
}

# published_address: the host address:port Docker actually published for the app
# (e.g. 127.0.0.1:3001). Asking Docker means this is right whatever HOST_PORT and
# BIND_ADDR are set to, without reading the settings file.
published_address() {
    dc port app 3000 2>/dev/null | head -n 1 || true
}

# summary: where the app is and how to reach it.
summary() {
    dc ps
    local addr
    addr=$(published_address)
    if [[ -z $addr ]]; then
        warn "could not work out which port the app is published on (see the PORTS column above)"
        return 0
    fi
    if [[ $TARGET == prod ]]; then
        say "Running on $DEPLOY_HOST, listening on $addr there (point your reverse proxy at it)"
    else
        say "Open http://${addr/0.0.0.0/127.0.0.1}/login"
        say "Create a user:  .dev_scripts/user.sh create"
    fi
}

# ---- data ------------------------------------------------------------------
# backup_db: save a consistent snapshot of the running app's database into
# BACKUP_DIR and print its path.
backup_db() {
    require_running
    mkdir -p "$BACKUP_DIR"; chmod 700 "$BACKUP_DIR"
    local out partial
    out=$BACKUP_DIR/$([[ $TARGET == prod ]] && echo prod || echo local)-$(date +%Y%m%d-%H%M%S).db
    partial=$out.partial
    if ! dc exec -T app /app/goDoChores backup - > "$partial"; then
        rm -f "$partial"
        die "backup failed"
    fi
    [[ $(head -c 15 "$partial") == "SQLite format 3" ]] || { rm -f "$partial"; die "backup did not produce a database file"; }
    chmod 600 "$partial"
    mv "$partial" "$out"
    printf '%s\n' "$out"
}

# confirm "text to type" "what will happen": ask before anything destructive.
confirm() {
    local expect=$1 what=$2 answer
    printf '%s\nType "%s" to continue: ' "$what" "$expect" >&2
    read -r answer || answer=
    [[ $answer == "$expect" ]] || die "aborted; nothing was changed"
}
