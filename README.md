# goDoChores

goDoChores is a web server built in Golang. You can read an overview of the project in [my writeup and initial impressions after doing the project](https://write.as/thewadegreen/experimenting-with-webservers-in-golang)

## Overview

This project is designed to help users keep track of chores that need to be performed on a regular bases. Users create chores, and then create chore reminders to tell them to do the specific chore on a predetermined interval.

## Tools Used

Overall, this project is golang for the full stack. Apart from the golang standard library, I used:
- [Chi](https://go-chi.io/#/) for routing and middleware,
- [Templ](https://templ.guide/) for template rendering, and
- [GORM](https://gorm.io/) for database models and query management

## Deployment Instructions

Run in development:
- Clone the repo
- Copy `.env.example` to `.env` and set `JWT_SECRET` to a long random string
- `templ generate` to compile the templates
- `go run . create-user` to create a user for yourself (this also creates the database on a fresh install)
- `go run .` to start the dev server (it will default to port 3000)

Basic deployment:
- `templ generate` to compile the templates
- `go build .` to build a binary
- Copy the binary wherever you want it
- Copy the .env.example file to .env and fill in `JWT_SECRET` (required), `PORT`, and optionally `DB_PATH` (SQLite file, default `test.db`) and `COOKIE_SECURE=true` if you serve over HTTPS, and `TRUSTED_ORIGINS` if a reverse proxy changes the `Host` header (see Security)
- `./goDoChores create-user` to create the user (or whatever you called the binary, if you specified a name)
- `./goDoChores` to run the server

## Groups and tags

- **Groups** (the *Group* page in the navbar): create a group, rename it, and invite people by the email address on their account. An invitation shows up on their home page and Group page, where they can accept or reject it. A user belongs to at most one group. Members see each other's chore reminders on the home page, labelled with the owner's name (click one to see its details and mark it completed; only the owner can delete or edit it). The owner can delete the group; other members can leave.
- **Tags** are shared by everyone in a group. Without a group they are personal to you. On the chore create/edit form, pick tags from the *Tags* dropdown, or type a new name and click *Add New* to create it on the spot (it is added to the dropdown and ticked, no page reload). The *Tags to Show* section of the Group page lists the group's tags; unchecked tags are hidden from your home page, and the choice is saved per user. A reminder is hidden if its chore has any hidden tag. Untagged chores and tasks always show. Joining a group folds your personal tags into the group's (same name = same tag). Leaving a group, or the group being deleted, gives you personal copies of the tags your chores use, so you keep them.

## Managing users

There is no sign-up page, so users are managed from the command line. Every command creates the database and its tables first if they don't exist yet.

- `./goDoChores create-user` asks for a username, email and password (the password prompt is hidden and asked twice). Pass `-u/--username`, `-e/--email` and `-p/--password` to skip the questions; note that `--password` can end up in your shell history. Running it for an existing username updates that user's email and password.
- `./goDoChores list-users` shows every user with their ID, email, how many chores, reminders and tasks they have, and when they were created.
- `./goDoChores delete-user <username>` permanently deletes a user **and all of their chores, reminders and tasks**. You have to type the username to confirm, or pass `-y/--yes` to skip that. Their logins stop working immediately, and the username and email can be used again.


## Security notes

- **Logging out** ends all of that user's sessions on every device, not just the browser you clicked it in. Changing someone's password with `create-user` does the same. Logins last 24 hours otherwise.
- **Cross-site request forgery** is blocked for every non-GET request using Go's built-in `http.CrossOriginProtection`, which checks the browser's `Sec-Fetch-Site` header (or `Origin` against `Host`). Requests with neither header, such as `curl`, are allowed. If you run behind a reverse proxy that rewrites the `Host` header and your forms start returning `403`, set `TRUSTED_ORIGINS` to your public origin, e.g. `TRUSTED_ORIGINS=https://chores.example.com` (comma-separate several).

## Running in Docker

The full walkthrough (server setup, deploying, backups, migrating from the old setup, reverse proxy, troubleshooting) is in **[DEPLOYMENT.md](DEPLOYMENT.md)**. The short version:

The app ships as a small container (a [distroless](https://github.com/GoogleContainerTools/distroless) image, running as an unprivileged user with a read-only filesystem). The SQLite database lives in a Docker volume, so it survives rebuilds. The commands are shell scripts in `.dev_scripts/`:

| Command | What it does |
|---|---|
| `.dev_scripts/up.sh` | Build the image (cached), start the app, wait until it is healthy. Keeps data. |
| `.dev_scripts/rebuild.sh` | Rebuild **from scratch** (no cache, fresh base images) and recreate the container. Keeps data. |
| `.dev_scripts/down.sh` | Stop and remove the container. Keeps the data volume and image. |
| `.dev_scripts/nuke.sh` | Delete **everything**: container, image and the database volume. Saves a backup to `backups/` first and asks you to type a confirmation. |
| `.dev_scripts/user.sh create\|list\|delete` | Manage users in the running container. |
| `.dev_scripts/backup.sh` | Save a consistent snapshot of the live database into `backups/` (safe while the app is serving). |
| `.dev_scripts/restore.sh FILE` | Replace the database with a backup. The database being replaced is kept inside the volume. |
| `.dev_scripts/logs.sh`, `status.sh` | Follow the log; show container, image, volume and users. |
| `.dev_scripts/doctor.sh` | Check Docker, the settings file, and (with `--prod`) the server. |
| `.dev_scripts/build.sh` | Just build the image (`--no-cache` and `--pull` are passed through). |

**First time, locally:** `cp .env.example .env`, put a real value in `JWT_SECRET` (`openssl rand -hex 32`), then `.dev_scripts/up.sh` and `.dev_scripts/user.sh create`. The app is at http://127.0.0.1:3000 (set `HOST_PORT` in `.env` to change it).

### Deploying to the server

Add `--prod` to any command above to run it on the server instead (`up.sh --prod`, `logs.sh --prod`, `user.sh --prod list`, ...). The image is built on your machine and sent over SSH, and all Docker commands run on the server through `DOCKER_HOST=ssh://...`, so the server needs nothing but Docker and an SSH login that can run it. Nothing is built or copied there besides the image.

1. On the server's side: Docker installed, and your SSH user allowed to run `docker` (member of the `docker` group).
2. Here: `cp .env.example .env.production` and fill it in: a new `JWT_SECRET`, `COOKIE_SECURE=true`, and `TRUSTED_ORIGINS=https://your.domain` if your reverse proxy changes the `Host` header. This file stays on your machine (it is git-ignored) and is never copied to the server.
3. `DEPLOY_HOST` is the SSH host (default `do-green-family`, an alias from your `~/.ssh/config`). Override it with `DEPLOY_HOST=user@host .dev_scripts/doctor.sh --prod`.
4. `.dev_scripts/doctor.sh --prod` checks all of the above.
5. `.dev_scripts/deploy.sh` backs up the live database, then builds, ships and restarts. The first time, use `.dev_scripts/up.sh --prod`, then `.dev_scripts/user.sh --prod create`.

The app listens on `127.0.0.1:3000` **on the server** only; point your existing reverse proxy at it. Set `BIND_ADDR=0.0.0.0` in `.env.production` if the proxy runs somewhere else.

Backups in `backups/` contain password hashes. The folder is git-ignored; keep it somewhere you trust.
