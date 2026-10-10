#!/usr/bin/env bash
# Manage users in the running app.
#   user.sh [--prod] create [-u NAME] [-e EMAIL] [-p PASSWORD]
#   user.sh [--prod] list
#   user.sh [--prod] delete USERNAME [--yes]
source "$(dirname "$0")/lib.sh"
parse_target "$@"
need docker; lenient_env

[[ ${#REST[@]} -ge 1 ]] || die "usage: user.sh [--prod] create|list|delete [args]"
case ${REST[0]} in
    create) command=create-user ;;
    list)   command=list-users ;;
    delete) command=delete-user ;;
    *) die "unknown action '${REST[0]}' (use create, list or delete)" ;;
esac
args=("${REST[@]:1}")
require_running

# exec allocates a terminal when there is one, so the hidden password prompt and
# the delete confirmation work as they do in a normal shell.
dc exec app /app/goDoChores "$command" ${args[@]+"${args[@]}"}
