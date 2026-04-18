Use `~/.local/bin/bd` for ALL task tracking. Always use `--json` flag. Never create markdown TODOs.

## Landing the Plane (Session Completion)

Work is NOT complete until `git push` succeeds.

1. File bd issues for remaining work
2. Run quality gates (tests, lint, build)
3. Close finished bd issues (only after PR merged)
4. Push: `git pull --rebase && git push`

## Commands

```bash
# Backend (from backend/)
go run ./cmd/server          # dev server :8080
go build ./...               # compile
golangci-lint run ./...      # lint
go test -race ./...          # test

# Frontend (from frontend/)
npm run dev      # Vite :5173
npm run build    # TS check + build
npm run lint     # ESLint

# Make (from root)
make dev-start   # build + start dev stack (Docker + air + NSQ)
make dev-stop    # stop dev stack
make dev-logs    # tail dev logs
make prod-up     # start production stack
make lint        # both linters
make test        # go test -race
make coverage    # test + per-package coverage report
```

## Architecture

### Request Flow
```
Browser → Vite proxy (:5173/api) → Go API (:8080)   [dev]
Browser → Nginx (:3000/api)      → Go API (:8080)   [prod]
```

Dashboard `GET /api/dashboard` fans out concurrently via `errgroup`; failed services return zero value, not error.

### Backend Layer Boundaries (strict)
- `handler/` → `model/`, `service/`, `internal/errors`, `internal/response` only
- `service/` → `model/`, `internal/errors`, `internal/httpclient` only — defines own store interfaces (DIP), zero imports from `repository/`
- `repository/` → `model/`, `internal/errors` only — implements store interfaces via duck typing
- `model/` → stdlib only

### Backend Patterns
- All DI in `internal/server/server.go:setupRoutes()`
- Every handler exposes `AddRoutes(r chi.Router)`; routes registered on public or `requireAuth` group
- DTOs in `<handler>_dto.go`; sentinel errors in `internal/errors/errors.go`
- Repository `Update`/`Delete` returns `(int64, error)`; service checks `rowsAffected == 0` for `ErrNotFound`
- Rate limit mutation endpoints via `httprate.LimitByIP` in `AddRoutes`
- Logging via `log/slog` only

### Adding a new widget
`internal/service/foo.go` → `internal/handler/foo.go` + `foo_dto.go` → `fooH.AddRoutes(r)` in `server.go` → field in `model.DashboardResponse` → goroutine in `handler/dashboard.go`

### Frontend
- `Dashboard.tsx` calls `useDashboard()` (60s stale, 30s refetch); cards are props-only, no fetching
- Inline styles exclusively (no Tailwind in JSX)
- UI primitives: `Card.tsx`, `CardHeader.tsx`, `MiniStat.tsx`
- When user describes UI positioning, clarify "within layout" vs "above/outside layout" before implementing

## Git Workflow
- Feature branches only — never commit to `main`
- Branch naming: `feat/<description>-<bd-ticket-id>`
- Verify branch with `git branch --show-current` before any work
- PR base is always `main`

## Rules
- Update `README.md` and `.env.example` for any new env var or external API
- Skill files: `.claude/skills/<name>/SKILL.md` (subdirectory format)
- Concurrent external API calls: use rate limiting + sequential fallbacks
- DB enums: lookup tables + FK, never `CHECK (col IN (...))`. FK columns end in `_id`
- Soft-delete tables: partial unique index (`WHERE deleted_at IS NULL`), never plain `UNIQUE`
- No `SELECT *` — explicit columns only
- Scope all DB queries to `userID`

## Known Limitations
- News uses sequential fetching — GNews free tier ~1 req/s; 9 categories × 3h cache = ~9s on miss
