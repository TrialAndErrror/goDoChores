# ---- build stage -----------------------------------------------------------
# go.mod says which Go the project needs; keep this in step with it.
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Install the templ generator at the exact version go.mod pins, so the code it
# generates matches the library the app is compiled against.
RUN go install github.com/a-h/templ/cmd/templ@$(go list -m -f '{{.Version}}' github.com/a-h/templ)

# Compiling the SQLite driver (cgo) takes a minute. Do it in its own layer, with
# the same flags as the real build, so it is cached until go.mod changes.
RUN CGO_ENABLED=1 go build -trimpath github.com/mattn/go-sqlite3

COPY . .
# The *_templ.go files are not in git; they are generated here.
RUN templ generate

# cgo is required by the SQLite driver (go-sqlite3).
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/goDoChores .

# An empty data directory to copy into the final image with the right owner.
RUN mkdir /out/data

# ---- runtime stage ---------------------------------------------------------
# distroless: just glibc and the binary. No shell, no package manager, and it
# runs as the unprivileged "nonroot" user (uid 65532).
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app

COPY --from=build /out/goDoChores /app/goDoChores
# The database lives here; mount a volume on it to keep data across rebuilds.
COPY --from=build --chown=65532:65532 /out/data /data

ENV PORT=3000 \
    DB_PATH=/data/chores.db
EXPOSE 3000

# The image has no curl/wget, so the binary checks itself.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/goDoChores", "healthcheck"]

# `docker run IMAGE` starts the server; `docker run IMAGE list-users` etc. work too.
ENTRYPOINT ["/app/goDoChores"]
