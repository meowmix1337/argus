# Plan: Phase 0 — Local Dev Environment (Docker + NSQ + Persistent DB)

## Context

Before implementing the social feed feature (argus-fi5), the local dev environment needs to be
hardened. Currently:

- `docker-compose.yml` (prod) has **no SQLite volume** — DB is lost on container restart
- `docker-compose.dev.yml` has **no NSQ services** — NSQ is planned for Phase 2 but devs need
  it running locally from day one
- No single command spins up the complete stack (backend + frontend + NSQ)
- DB persistence in dev relies on an implicit side-effect of the `./backend:/src` bind mount
- Backend dev uses `go run ./cmd/server` — this restarts on container start but **does not
  hot-reload on file changes**; proper hot reload requires `air`

This PR establishes a consistent, one-command local dev environment as a prerequisite to all
social feed phases. It blocks all argus-fi5 child tasks.

---

## What changes

### 1. `docker-compose.yml` — Add NSQ services (base/prod)

Add three NSQ services to the base compose file so they are available in both prod and dev stacks:

```yaml
nsqlookupd:
  image: nsqio/nsq:v1.3.0
  command: /nsqlookupd
  ports:
    - "4160:4160"   # TCP
    - "4161:4161"   # HTTP
  restart: unless-stopped

nsqd:
  image: nsqio/nsq:v1.3.0
  command: /nsqd --lookupd-tcp-address=nsqlookupd:4160 --data-path=/data
  ports:
    - "4150:4150"   # TCP (producer/consumer connect here)
    - "4151:4151"   # HTTP
  depends_on:
    - nsqlookupd
  volumes:
    - nsqdata:/data
  restart: unless-stopped

nsqadmin:
  image: nsqio/nsq:v1.3.0
  command: /nsqadmin --lookupd-http-address=nsqlookupd:4161
  ports:
    - "4171:4171"   # Web UI for inspecting topics/channels/messages
  depends_on:
    - nsqlookupd
  restart: unless-stopped
```

Also update the `backend` service in `docker-compose.yml`:
```yaml
backend:
  depends_on:
    - nsqd           # add (waits for nsqd before starting)
  environment:
    - NSQ_NSQD_ADDR=nsqd:4150
    - NSQ_LOOKUPD_ADDR=nsqlookupd:4161
```

Add top-level `volumes: nsqdata:` entry.

**Note:** No SQLite volume added to the base file. Production DB persistence is left for a
dedicated ops/infra task. The backend's `NoopPublisher` fallback means it starts fine even
if NSQ is not connected.

---

### 2. `deploy/backend.Dockerfile` — Install `air` in builder stage

Add `air` installation to the `builder` stage (before `COPY . .` so Docker layer caches it):

```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

# Install air for hot reload (dev only — used via docker-compose.dev.yml command override)
RUN go install github.com/air-verse/air@latest

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /bin/server ./cmd/server
```

`air` is installed at `/go/bin/air` (on PATH). The prod runtime stage ignores it — only the
builder stage is used in dev.

---

### 3. `backend/.air.toml` — Hot reload config (new file)

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/server"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor", "testdata"]
  include_ext = ["go", "toml"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[log]
  time = false

[misc]
  clean_on_exit = true
```

Air watches `/src` (the bind-mounted `./backend/`), recompiles on `.go` changes, and restarts
the server binary. The `tmp/` directory lives inside the container on the bind mount.

---

### 4. `docker-compose.dev.yml` — Add SQLite persistence, NSQ wiring, and air command

Replace `go run ./cmd/server` with `air` and add the `./data:/data` bind-mount:

```yaml
services:
  backend:
    command: air                      # replaces go run ./cmd/server — uses .air.toml
    depends_on:
      - nsqd                          # add (NSQ now in base)
    environment:
      SQLITE_PATH: /data/dashboard.db # override default path
    volumes:
      - ./backend:/src                # existing (hot reload source)
      - ./data:/data                  # add: DB lives at ./data/dashboard.db on host
```

`./data/` is created automatically by Docker on first `make docker-dev-up`. Add it to `.gitignore`.

No changes needed for the `frontend` service — it already runs Vite dev with hot reload on :5173.

---

### 5. `Makefile` — Add dev convenience targets

```makefile
docker-dev-up:   ## Start full dev stack detached (NSQ + hot-reload backend + Vite frontend)
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

docker-dev-down: ## Stop dev stack (data preserved in ./data/)
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

docker-dev-reset: ## Wipe local dev data (removes ./data/ — fresh DB on next up)
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v
	rm -rf ./data

docker-dev-logs: ## Tail all dev stack container logs
	docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f
```

Update `docker-dev` description to clarify it's the attached (foreground) version.

---

### 6. `.gitignore` — Ignore local data and air tmp directories

Add:
```
# Local dev SQLite DB (created by docker-dev-up)
data/
# air hot-reload build artifacts
backend/tmp/
```

---

### 7. `.env.example` — Document new env vars + clarify dev setup

**Env var strategy:**
- Both `docker-compose.yml` and `docker-compose.dev.yml` already use `env_file: .env`
- `environment:` blocks in compose files override `.env` values for Docker networking
  (e.g. `NSQ_NSQD_ADDR=nsqd:4150` routes to the container name, not localhost)
- Developers must `cp .env.example .env` and fill in secrets; NSQ Docker values are
  auto-overridden by the compose `environment:` block — no manual edit needed for NSQ

Add to `.env.example`:
```env
# NSQ message queue (required for social feed events)
# In Docker (make docker-dev-up): these are overridden automatically by docker-compose.yml
# For local non-Docker dev: start nsqd locally or leave empty (NoopPublisher fallback)
NSQ_NSQD_ADDR=localhost:4150
NSQ_LOOKUPD_ADDR=localhost:4161
```

---

### 8. `README.md` — Update Local Dev section

Update the "Getting Started" / local dev section to document:
- `make docker-dev-up` as the recommended single-command dev start
- Service port map: backend :8080, frontend :5173, nsqadmin :4171
- `make docker-dev-reset` for wiping the DB (fresh start)
- Note that `./data/dashboard.db` is the dev DB file

---

## Dev service map after this PR

| Service     | Port  | Purpose                                        |
|-------------|-------|------------------------------------------------|
| backend     | 8080  | Go API (hot reload via `air` — auto-recompile) |
| frontend    | 5173  | Vite dev server (proxies /api→:8080)           |
| nsqd        | 4150  | NSQ broker (producer/consumer)                 |
| nsqlookupd  | 4160  | NSQ service discovery                          |
| nsqadmin    | 4171  | NSQ web UI                                     |

DB file: `./data/dashboard.db` (on host, bind-mounted into backend container at `/data/dashboard.db`)

---

## Verification

```bash
# 1. Start the full dev stack
make docker-dev-up

# 2. Verify all containers running
docker compose -f docker-compose.yml -f docker-compose.dev.yml ps
# Expect: backend, frontend, nsqd, nsqlookupd, nsqadmin all Up

# 3. Verify backend health
curl http://localhost:8080/api/health

# 4. Verify frontend accessible
open http://localhost:5173

# 5. Verify NSQ admin UI
open http://localhost:4171

# 6. Verify DB persists across restart
make docker-dev-down
make docker-dev-up
# Login and confirm data (tasks, settings) still present

# 7. Verify DB file exists on host
ls -lh ./data/dashboard.db

# 8. Verify fresh reset works
make docker-dev-reset
# ./data/ should be gone; next make docker-dev-up starts with empty DB

# 9. Verify hot reload works
# Touch a .go file in backend/ and watch the backend container log
# Expect: air detects change, recompiles, restarts server within ~2s
docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f backend
# (in another terminal) touch backend/internal/server/server.go
```

---

## Files to modify

| File | Change |
|---|---|
| `docker-compose.yml` | Add nsqlookupd, nsqd, nsqadmin services; add NSQ env vars + depends_on to backend; add nsqdata volume |
| `docker-compose.dev.yml` | Change command to `air`; add `./data:/data` bind mount + `SQLITE_PATH=/data/dashboard.db` + `depends_on: nsqd` |
| `deploy/backend.Dockerfile` | Add `go install github.com/air-verse/air@latest` to builder stage |
| `backend/.air.toml` | New file — air hot-reload config (watches `/src`, builds to `./tmp/main`) |
| `Makefile` | Add `docker-dev-up`, `docker-dev-down`, `docker-dev-reset`, `docker-dev-logs` targets |
| `.gitignore` | Add `data/` and `backend/tmp/` |
| `.env.example` | Add NSQ_NSQD_ADDR, NSQ_LOOKUPD_ADDR with Docker override note |
| `README.md` | Update local dev Getting Started section |
