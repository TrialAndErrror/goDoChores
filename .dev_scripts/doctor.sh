#!/usr/bin/env bash
# Check that everything needed is in place, and say what to fix.
#   doctor.sh           this machine
#   doctor.sh --prod    also the SSH connection and Docker on the server
source "$(dirname "$0")/lib.sh"
parse_target "$@"

failed=0
ok()   { printf '  \033[32mok\033[0m   %s\n' "$*"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; failed=1; }
note() { printf '  \033[33mnote\033[0m %s\n' "$*"; }

say "This machine"
command -v docker >/dev/null && ok "docker: $(env -u DOCKER_HOST docker --version)" || bad "docker is not installed"
env -u DOCKER_HOST docker compose version >/dev/null 2>&1 && ok "docker compose: $(env -u DOCKER_HOST docker compose version --short)" || bad "docker compose plugin is missing"
env -u DOCKER_HOST docker info >/dev/null 2>&1 && ok "docker daemon is reachable" || bad "cannot talk to the docker daemon (is it running? are you in the docker group?)"
env -u DOCKER_HOST docker buildx version >/dev/null 2>&1 && ok "docker buildx is installed" || note "docker buildx is not installed: builds still work, with a deprecation notice from the legacy builder"

say "Settings ($ENV_FILE)"
if [[ -f $ENV_FILE ]]; then
    ok "$ENV_FILE exists"
    secret=$(read_var JWT_SECRET)
    if [[ -z $secret || $secret == change-me* ]]; then bad "JWT_SECRET is missing or still the example value (openssl rand -hex 32)"
    elif (( ${#secret} < 32 )); then bad "JWT_SECRET is shorter than 32 characters"
    else ok "JWT_SECRET is set"; fi
    if [[ $TARGET == prod ]]; then
        [[ $(read_var COOKIE_SECURE) == true ]] && ok "COOKIE_SECURE=true" || note "COOKIE_SECURE is not true; set it when the site is served over HTTPS"
        [[ -n $(read_var TRUSTED_ORIGINS) ]] && ok "TRUSTED_ORIGINS is set" || note "TRUSTED_ORIGINS is empty; needed only if your proxy changes the Host header and forms return 403"
    fi
else
    bad "$ENV_FILE not found (cp .env.example $ENV_FILE)"
fi

if [[ $TARGET == prod ]]; then
    say "Server ($DEPLOY_HOST)"
    if ssh -o BatchMode=yes -o ConnectTimeout=8 "$DEPLOY_HOST" true 2>/dev/null; then
        ok "ssh works without a password prompt"
        if ssh "$DEPLOY_HOST" 'docker info >/dev/null 2>&1'; then
            ok "docker works for that user: $(ssh "$DEPLOY_HOST" 'docker --version')"
            ssh "$DEPLOY_HOST" 'command -v gunzip >/dev/null' && ok "gunzip is installed there" || bad "gunzip missing on the server"
            if DOCKER_HOST=$DOCKER_REMOTE docker info >/dev/null 2>&1; then ok "docker over ssh:// works (used for all commands)"
            else bad "docker over ssh:// failed; the server's docker CLI must be reachable as 'docker system dial-stdio'"; fi
            ssh "$DEPLOY_HOST" 'docker volume inspect $VOLUME >/dev/null 2>&1' && ok "data volume exists (app has been deployed before)" || note "no data volume yet (first deploy)"
            # Free space where Docker keeps images and volumes (not necessarily /).
            # df -P guarantees one line per filesystem; the 4th column is KiB available.
            docker_root=$(ssh "$DEPLOY_HOST" "docker info --format '{{.DockerRootDir}}'" 2>/dev/null || true)
            free_kb=$(ssh "$DEPLOY_HOST" "df -Pk '${docker_root:-/}' | awk 'NR==2 {print \$4}'" 2>/dev/null || true)
            if [[ ! $free_kb =~ ^[0-9]+$ ]]; then
                note "could not read free disk space on the server (check it yourself: df -h ${docker_root:-/})"
            elif (( free_kb < 512 * 1024 )); then
                bad "only $((free_kb / 1024)) MiB free on the server where Docker stores data (${docker_root:-/}); the image needs about 100 MiB plus room for the database"
            elif (( free_kb < 2 * 1024 * 1024 )); then
                note "$((free_kb / 1024)) MiB free on the server (${docker_root:-/}); enough, but getting low"
            else
                ok "$((free_kb / 1024 / 1024)) GiB free on the server (${docker_root:-/})"
            fi
        else
            bad "that user cannot run docker (is docker installed? is the user in the docker group?)"
        fi
    else
        bad "cannot ssh to $DEPLOY_HOST non-interactively (check ~/.ssh/config and your key; or set DEPLOY_HOST=user@host)"
    fi
fi

echo
if (( failed )); then die "some checks failed (see FAIL above)"; fi
say "All good"
