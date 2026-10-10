# Deployment guide

How to run goDoChores in Docker, on your own machine and on your server, and how to rebuild, tear down, back up and restore it.

- [How it works](#how-it-works)
- [Quick start (local)](#quick-start-local)
- [Command reference](#command-reference)
- [Settings](#settings)
- [Setting up the server](#setting-up-the-server)
- [Everyday workflows](#everyday-workflows)
- [Backups and restoring](#backups-and-restoring)
- [Moving from the old bare-metal setup](#moving-from-the-old-bare-metal-setup)
- [Reverse proxy and HTTPS](#reverse-proxy-and-https)
- [Troubleshooting](#troubleshooting)
- [Security notes](#security-notes)
- [Known limits](#known-limits)

## How it works

```
 your machine                                   the server
 ───────────────────────                        ───────────────────────────
 .dev_scripts/*.sh                              Docker engine
   │  docker build  → image gochores:latest       │
   │  docker save | gzip | ssh ... docker load ──►│  image
   │  DOCKER_HOST=ssh://server docker compose ───►│  container "gochores-app-1"
   │  reads .env.production (stays here)          │    └─ volume gochores_chores-data  →  /data/chores.db
   └─ backups/ (snapshots of the database)        │  listens on 127.0.0.1:3000 ◄── your reverse proxy
```

- **One container, one volume.** The app is a single Go binary in a small [distroless](https://github.com/GoogleContainerTools/distroless) image. It runs as an unprivileged user on a read-only filesystem. The SQLite database is `/data/chores.db` in the Docker volume `gochores_chores-data`, which is why rebuilding the image never touches your data.
- **The image is built on your machine** and sent to the server over SSH. The server never builds anything and never gets your source code.
- **Every command runs here.** For the server, the scripts point Docker at it with `DOCKER_HOST=ssh://...`. Your settings file `.env.production` is read on your machine and handed to the container, so secrets are never written to the server's disk.
- **Two targets, same commands.** No flag means this machine (settings from `.env`). `--prod` means the server (settings from `.env.production`).

Files involved:

| File | Purpose |
|---|---|
| `Dockerfile` | Builds the image (Go build, `templ generate`, tiny runtime image). |
| `docker-compose.yml` | Container settings: volume, port, restart policy, hardening. |
| `.dockerignore` | Keeps secrets, databases and local files out of the image. |
| `.dev_scripts/*.sh` | The commands below. `lib.sh` is shared code they load. |
| `.env`, `.env.production` | Your settings. **Git-ignored.** Never committed, never in the image. |
| `backups/` | Database snapshots made by `backup.sh`/`nuke.sh`. **Git-ignored.** |

## Quick start (local)

Prerequisites: Docker with the Compose plugin, and `bash`.

```bash
cp .env.example .env
# edit .env: set JWT_SECRET to a long random string
openssl rand -hex 32            # paste the output as JWT_SECRET

.dev_scripts/doctor.sh          # checks Docker and your settings
.dev_scripts/up.sh              # builds the image, starts the app, waits until healthy
.dev_scripts/user.sh create     # asks for username, email and password
```

Open <http://127.0.0.1:3000/login>. The first build takes a couple of minutes (it compiles the SQLite driver); later builds reuse cached layers and take seconds when only your code changed.

## Command reference

Run everything from anywhere; the scripts find the project themselves. Add `--prod` to any command to run it against the server instead of this machine (except `deploy.sh`, which is always for the server).

| Command | What it does | Data |
|---|---|---|
| `up.sh` | Build the image (cached), start or update the app, wait until healthy. | kept |
| `rebuild.sh` | Rebuild **from scratch**: no layer cache, fresh base images, new container. | kept |
| `down.sh` | Stop and remove the container. The volume and image stay, so `up.sh` brings it all back. | kept |
| `nuke.sh [--yes] [--no-backup]` | Delete **everything**: container, image and the data volume. Saves a backup first (if the app is running) and asks you to type `local` (or the server's name) to confirm. | **deleted** |
| `deploy.sh [--no-backup]` | Production only: back up the live database, build, ship, restart. | kept |
| `user.sh create\|list\|delete ...` | Manage users in the running app. `create` takes `-u`, `-e`, `-p` or prompts; `delete USERNAME` asks you to retype the name (`--yes` skips). | changes users |
| `backup.sh` | Save a consistent snapshot of the live database to `backups/`. Safe while the app is serving. | kept |
| `restore.sh [--yes] FILE` | Replace the database with a backup. Stops the app briefly. The replaced database is kept inside the volume. | **replaced** |
| `logs.sh [args]` | Follow the log. Extra arguments go to `docker compose logs` (`--since 1h`, `--timestamps`). | – |
| `status.sh` | Container health, image, volume, and the user list. | – |
| `doctor.sh` | Checks Docker, your settings and (with `--prod`) SSH and Docker on the server, and says what to fix. | – |
| `build.sh [--no-cache] [--pull]` | Only build the image; extra arguments go to `docker build`. | – |

Examples:

```bash
.dev_scripts/up.sh --prod             # first deploy to the server
.dev_scripts/deploy.sh                # later deploys (with a backup first)
.dev_scripts/user.sh --prod list
.dev_scripts/logs.sh --prod --since 1h
.dev_scripts/rebuild.sh               # clean local rebuild, data kept
.dev_scripts/nuke.sh                  # wipe the local install (asks first)
```

The scripts print a clear error and change nothing if a prerequisite is missing (no Docker, a missing or placeholder `JWT_SECRET`, the app not running when a command needs it).

## Settings

Put these in `.env` (local) or `.env.production` (server). Start from `.env.example`.

| Setting | Needed | Meaning |
|---|---|---|
| `JWT_SECRET` | **yes** | Signs login tokens. At least 32 characters; the scripts refuse to start with a missing, example or short value. Generate with `openssl rand -hex 32`. Changing it signs everyone out. Use a different value for local and production. |
| `COOKIE_SECURE` | for HTTPS | Set `true` when the site is served over HTTPS so the login cookie is only sent over HTTPS. |
| `TRUSTED_ORIGINS` | sometimes | Comma-separated public origins, e.g. `https://chores.example.com`. Only needed if your reverse proxy rewrites the `Host` header and form submissions start returning `403`. See [Reverse proxy](#reverse-proxy-and-https). |
| `HOST_PORT` | no | Port published on the host. Default `3000`. |
| `BIND_ADDR` | no | Address the port is published on. Default `127.0.0.1` (this machine only). Use `0.0.0.0` only if something on another machine must reach it directly. |
| `PORT`, `DB_PATH` | no | Ignored in Docker: the container always uses port `3000` and `/data/chores.db`. They only matter when running the binary without Docker. |

Environment variables for the scripts themselves (set them in your shell, not in the env files):

| Variable | Default | Meaning |
|---|---|---|
| `DEPLOY_HOST` | `do-green-family` | SSH host for `--prod`: an alias from `~/.ssh/config` or `user@host`. |
| `IMAGE` | `gochores:latest` | Image name and tag. |
| `COMPOSE_PROJECT_NAME` | `gochores` | Prefix for container, network and volume names. |
| `BACKUP_DIR` | `./backups` | Where snapshots are written. |
| `ENV_FILE` | `.env` / `.env.production` | Use a different settings file. |

## Setting up the server

Do this once.

**On the server**

1. Install Docker Engine.
2. Let the SSH user run Docker without `sudo`: `sudo usermod -aG docker USER`, then log out and back in.
3. Make sure `gunzip` exists (it does on practically every Linux).

You do **not** need Compose, Go, templ or the source code on the server.

**On your machine**

1. Make sure `ssh <host>` works without a password prompt (key-based login) and that the host name matches `DEPLOY_HOST`. The default `do-green-family` is the alias your old scripts used; set `DEPLOY_HOST=user@host` if you use something else.
2. Create the production settings:
   ```bash
   cp .env.example .env.production
   ```
   Set a **new** `JWT_SECRET` (`openssl rand -hex 32`), `COOKIE_SECURE=true` once HTTPS is in front, and `TRUSTED_ORIGINS` if needed.
3. Check everything:
   ```bash
   .dev_scripts/doctor.sh --prod
   ```
   It verifies SSH, Docker on the server, Docker-over-SSH (the mechanism every command uses) and your settings, and tells you what to fix.
4. First deploy and first user:
   ```bash
   .dev_scripts/up.sh --prod
   .dev_scripts/user.sh --prod create
   ```
5. Point your reverse proxy at `127.0.0.1:3000` on the server ([see below](#reverse-proxy-and-https)).

## Everyday workflows

**Ship a change**

```bash
git pull                       # or just commit your work
.dev_scripts/deploy.sh
```

This backs up the live database to `backups/`, builds the image, sends it (skipped if the server already has that exact image), restarts the container, and waits for the health check. The app restarts in a few seconds. Database tables update themselves when the new version starts.

**Roll back a bad deploy**

There is no separate rollback command. Check out the last good commit and deploy it again:

```bash
git checkout <good-commit>
.dev_scripts/deploy.sh --no-backup
git checkout main
```

If the bad version changed or damaged data, restore the backup `deploy.sh` just made ([below](#backups-and-restoring)). Note that table changes are additive (new columns), so an older version can normally read a newer database, but check before relying on it.

**Start completely clean (local)**

```bash
.dev_scripts/nuke.sh           # asks you to type "local"; saves a backup first
.dev_scripts/up.sh
.dev_scripts/user.sh create
```

**Rebuild from scratch but keep the data**

```bash
.dev_scripts/rebuild.sh        # add --prod for the server
```

**Stop everything but keep the data**

```bash
.dev_scripts/down.sh           # .dev_scripts/up.sh brings it back
```

**Look at what is happening**

```bash
.dev_scripts/status.sh --prod
.dev_scripts/logs.sh --prod
```

The container restarts itself if it crashes and when the server reboots (`restart: unless-stopped`). `docker stop` and `down` shut it down gracefully: running requests get up to 10 seconds to finish.

## Backups and restoring

```bash
.dev_scripts/backup.sh --prod      # → backups/prod-YYYYMMDD-HHMMSS.db
```

A backup is a consistent SQLite snapshot taken by the app itself (not a file copy), so it is safe while people are using the site, and it is verified before it is saved. Backups are made automatically by `deploy.sh` and `nuke.sh`.

To restore:

```bash
.dev_scripts/restore.sh --prod backups/prod-20261010-120000.db
```

You must type the server's name to confirm. The app is stopped, the backup is checked (it must be a healthy SQLite file with this app's tables, otherwise nothing is changed), the database is swapped, and the app is started again. The database that was replaced is kept inside the volume as `/data/chores.db.before-restore-<time>`, so a mistaken restore can itself be undone.

**Restore after a `nuke`:** `restore.sh` rebuilds and ships the image itself if it is missing, so this works on a completely empty install:

```bash
.dev_scripts/nuke.sh --prod        # prints the path of the backup it saved
.dev_scripts/restore.sh --prod backups/prod-....db
```

Things to know:
- Backups contain **password hashes** and all user data. `backups/` is git-ignored and each file is readable only by you. Keep copies somewhere you trust, and delete old ones you don't need.
- Keep backups off the server too. They are already on your machine because the scripts write them there; copy them to a second place as well.
- The `.before-restore-*` files pile up inside the volume if you restore often. The container has no shell, so clear them with a throwaway container on the server, once you are sure you don't need them: `docker run --rm -v gochores_chores-data:/data alpine sh -c 'rm -f /data/*.before-restore-*'`.

## Moving from the old bare-metal setup

Your old setup ran `./goDoChores` with `nohup` in `/home/green_family/chores`, with `test.db` next to it. To move to Docker without losing users or chores:

1. Do [Setting up the server](#setting-up-the-server) steps 1–3 first, but don't create users yet.
2. Stop the old process on the server (`ps aux | grep goDoChores`, then `kill <pid>`). It must be stopped so the database isn't being written to and the old port is free.
3. Copy the database to your machine:
   ```bash
   scp do-green-family:/home/green_family/chores/test.db ./old-test.db
   ```
4. Deploy and restore it:
   ```bash
   .dev_scripts/up.sh --prod
   .dev_scripts/restore.sh --prod ./old-test.db
   .dev_scripts/user.sh --prod list       # your users should be listed
   ```
5. Point the reverse proxy at `127.0.0.1:3000`, then try logging in.
6. Keep `old-test.db` (and the old folder on the server) until you are happy, then delete them.

Everyone will have to log in again once, because the container uses a new `JWT_SECRET`. The old `build.sh` and `deploy.sh` were replaced by the commands in this guide.

## Reverse proxy and HTTPS

The container publishes only `127.0.0.1:3000` **on the server**. Put your existing reverse proxy in front of it and terminate HTTPS there. The app does not look at `X-Forwarded-*` headers, so set `COOKIE_SECURE=true` yourself.

Example configurations (adapt to your own setup):

```nginx
# nginx
location / {
    proxy_pass http://127.0.0.1:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

```
# Caddy
chores.example.com {
    reverse_proxy 127.0.0.1:3000
}
```

**CSRF and `TRUSTED_ORIGINS`.** Form submissions are checked to make sure they come from your own site, using the `Sec-Fetch-Site` header browsers send over HTTPS, or comparing the `Origin` header with the `Host` header. Keep `proxy_set_header Host $host` (as above) and it just works. If your proxy forwards a different `Host` and logins or edits start failing with `403 Forbidden`, set `TRUSTED_ORIGINS=https://chores.example.com` in `.env.production` and run `deploy.sh`. Several origins can be comma-separated.

If the proxy is on a different machine from the container, set `BIND_ADDR=0.0.0.0` and firewall the port so only the proxy can reach it.

## Troubleshooting

Start with `.dev_scripts/doctor.sh` (add `--prod` for the server) and `.dev_scripts/logs.sh`.

| Symptom | Likely cause and fix |
|---|---|
| `JWT_SECRET ... is still the example value` / `shorter than 32 characters` | Edit `.env`/`.env.production` and set a real secret: `openssl rand -hex 32`. |
| `.env.production not found` | `cp .env.example .env.production` and fill it in. |
| `cannot ssh to ... non-interactively` | Set up key-based SSH login, check `~/.ssh/config`, or use `DEPLOY_HOST=user@host`. |
| `that user cannot run docker` | Add the SSH user to the `docker` group on the server and log in again. |
| `docker over ssh:// failed` | The server's Docker is not reachable through SSH for that user. Check `ssh HOST docker info` works. |
| The app starts, then `did not become healthy` | The log printed above the error says why. Most often `JWT_SECRET` is missing, the port is already in use (`HOST_PORT=3001`), or the volume isn't writable. |
| `port is already allocated` / `address already in use` | Something else is on that port. On a migrating server, the old bare-metal process: stop it, or change `HOST_PORT`. |
| Logging in or saving forms gives `403 Forbidden` behind the proxy | See [CSRF and TRUSTED_ORIGINS](#reverse-proxy-and-https). |
| Login works, but you are logged out immediately | `COOKIE_SECURE=true` while visiting over plain HTTP. Use HTTPS or set it to `false`. |
| `the app is not running on ...` | `user.sh`, `backup.sh` and similar need a running app. Run `up.sh` first. |
| `DEPRECATED: The legacy builder` message during a build | Harmless; your Docker has no `buildx` plugin. Installing `docker-buildx` removes the notice. |
| Everyone was signed out after a deploy | Expected if `JWT_SECRET` changed. Logging out, or changing a user's password with `user.sh create`, also signs that user out of every device. |
| Disk filling up on the server | Old images pile up after many deploys: `ssh HOST docker image prune -f` removes only unused, untagged ones. |

Manual access when you need it:

```bash
docker ps --filter name=gochores                     # on the server
docker logs gochores-app-1 --tail 200                # on the server
docker exec -it gochores-app-1 /app/goDoChores list-users
```

The image has no shell by design, so `docker exec ... sh` won't work. Everything you need is a subcommand of the app: run it without arguments to start the server, or with `create-user`, `list-users`, `delete-user`, `backup`, `restore`.

## Security notes

- The container runs as an unprivileged user (uid 65532), with a read-only root filesystem, no Linux capabilities and `no-new-privileges`. Only `/data` (the volume) and `/tmp` are writable.
- The published port is bound to `127.0.0.1` by default, so the app is only reachable through your reverse proxy.
- Secrets live in `.env.production` on **your machine**. They are passed to the container when it is created; they are not baked into the image and not copied to the server's disk. The image contains no `.env` files or databases (`.dockerignore`).
- Anyone who can run `docker` on the server can read the container's environment, including `JWT_SECRET`. Treat the `docker` group as root-equivalent.
- Logging out, or changing a password, invalidates all of that user's logins. Login tokens expire after 24 hours.
- Cross-site form submissions are rejected (see [CSRF](#reverse-proxy-and-https)).
- Back up regularly and keep copies off the server; see [Backups](#backups-and-restoring).

## Known limits

- **The `--prod` path has not been tested against a real server.** Everything was exercised locally against Docker (build, start, health, users, down/up, rebuild, nuke, backup, restore). The server path uses the same code with `DOCKER_HOST=ssh://...` and an `ssh` image transfer. Run `doctor.sh --prod` first and try `up.sh --prod` before relying on it.
- **Single instance.** SQLite means one container; don't run two against the same volume.
- **No rollback command** and no scheduled backups. To get them regularly, run `backup.sh --prod` from cron on your machine (it needs your SSH key available without a prompt), e.g. `0 3 * * * cd /path/to/goDoChores && .dev_scripts/backup.sh --prod`.
- **Downtime is a few seconds** while the container is replaced during deploys.
