Use 'bd' for task tracking

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

Always use the `bd` bead tracking tool for task updates as specified in AGENTS.md. Never skip bead tracking even when focused on code changes.

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed!

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

For more details, see README.md and docs/QUICKSTART.md.

<!-- END BEADS INTEGRATION -->

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   ~/.local/bin/bd backup && git add .beads/backup/ && git diff --cached --quiet || git commit -m "chore: sync bd backup"
   git pull --rebase
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

## Commands

### Backend (run from `backend/`)
```bash
go run ./cmd/server          # Start dev server on :8080
go run ./cmd/devtoken        # Mint a signed session cookie for local endpoint testing (reads SESSION_SECRET from .env)
go build ./...               # Compile all packages
golangci-lint run ./...      # Lint (install: https://golangci-lint.run/welcome/install/)
go test -race ./...          # Run tests
go build -o bin/server ./cmd/server  # Build binary
govulncheck ./...            # Scan for vulnerabilities (install: go install golang.org/x/vuln/cmd/govulncheck@latest)
```

### Frontend (run from `frontend/`)
```bash
npm run dev      # Vite dev server on :5173 (proxies /api → :8080)
npm run build    # TypeScript check + production build → dist/
npm run lint     # ESLint
```

### Make shortcuts (from repo root)
```bash
make dev-backend     # go run ./cmd/server
make dev-frontend    # npm run dev
make docker-up       # docker compose up -d (production)
make docker-dev      # docker compose with hot-reload overrides
make lint            # golangci-lint + npm run lint
make test            # go test -race ./...
make install-hooks   # install pre-commit hooks via pre-commit framework (run once after clone)
```

### Pre-commit hooks
This project uses the [pre-commit](https://pre-commit.com/) framework. Run once after cloning:

```bash
pip install pre-commit   # or: brew install pre-commit
make install-hooks       # runs: pre-commit install
```

The config lives at `.pre-commit-config.yaml` (repo root). Hooks that run on staged `.go` files:
- **goimports** — auto-formats Go imports (`goimports -w .` in `backend/`)
- **golangci-lint** — runs the full linter suite (`golangci-lint run ./...` in `backend/`)

## Architecture

### Request Flow
```
Browser → Vite proxy (:5173/api) → Go API (:8080)   [dev]
Browser → Nginx (:3000/api)      → Go API (:8080)   [prod]
```

The frontend calls `GET /api/dashboard` which fans out to all backend services concurrently via `errgroup` and returns a single `model.DashboardResponse`. Individual services fail gracefully — a failed service returns a zero value, not an error response.

### Backend Structure

**Wiring**: All dependency injection happens in `internal/server/server.go:setupRoutes()`. Services and handlers are constructed there with a shared `httpclient.HTTPClient` wrapper (30s timeout) and a shared `*service.CacheService`.

**Layer boundaries (strict)**:
- `handler/` → imports `model/`, `service/`, `internal/errors`, `internal/response` only
- `service/` → imports `model/`, `internal/errors`, `internal/httpclient` only — defines its own store interfaces (DIP), zero imports from `repository/`
- `repository/` → imports `model/`, `internal/errors` only — implements service store interfaces via Go duck typing
- `model/` → stdlib only, zero internal imports

**Shared utility packages**:
- `internal/errors/` — centralized sentinel errors (`ErrTaskNotFound`, `ErrSettingsNotFound`, etc.)
- `internal/response/` — `WriteJSON(w, status, v)` and `WriteError(w, status, msg)` HTTP helpers
- `internal/validate/` — shared `go-playground/validator` instance
- `internal/httpclient/` — `HTTPClient` interface with `ClientOption`/`RequestOption` functional options, `HTTPError` type

**Service pattern**: Every external-API service follows the same shape:
```go
type XxxService struct { httpClient httpclient.HTTPClient; apiKey string; cache *CacheService }
func NewXxxService(...) *XxxService
func (s *XxxService) Fetch(ctx context.Context) (model.XxxData, error)
```
Each `Fetch` checks the cache first, calls the external API on miss, and returns an error (card shows unavailable state) if the API key is absent or the call fails. Cache TTLs: weather 15m, stocks 10s, news 3h, calendar 15m, sunrise 6h, quotes 24h.

**Handler pattern**: Every handler exposes `AddRoutes(r chi.Router)` which registers its own routes. `setupRoutes()` in `server.go` constructs handlers and calls each one's `AddRoutes` on either the public router or the `requireAuth` protected group — no route paths live in `server.go` itself. Request/response DTOs live in `<handler>_dto.go` files.

**Adding a new widget**: create `internal/service/foo.go` → `internal/handler/foo.go` + `internal/handler/foo_dto.go` (implement `AddRoutes`) → call `fooH.AddRoutes(r)` in `server.go` → add field to `model.DashboardResponse` → add goroutine in `handler/dashboard.go`.

### Frontend Structure

**Data flow**: `App.tsx` wraps everything in `QueryClientProvider`. `Dashboard.tsx` calls `useDashboard()` (60s stale, 30s refetch) and passes typed slices/structs down to each card component as props. Cards never fetch data themselves.

**State**: Only `TasksCard` has mutations — `useTasks.ts` wraps `PATCH /api/tasks/{id}` with optimistic updates against the `['dashboard']` query cache.

**Styling**: Components use inline styles exclusively (no Tailwind classes in JSX). Tailwind is imported in `index.css` via `@import "tailwindcss"` but the design system is all inline to match the exact pixel values from the mock.

**UI primitives**: `components/ui/Card.tsx` handles the glass-morphism card shell and staggered fade-in animation. `CardHeader.tsx` and `MiniStat.tsx` are the other shared primitives — import these for any new card.

When user describes UI positioning (e.g., 'at the top'), ask a clarifying question before implementing. Distinguish between 'within the current layout' vs 'above/outside the current layout'.

### External APIs
| Service | API | Key env var | Fallback |
|---------|-----|-------------|---------|
| Weather | Open-Meteo + AQI | none | unavailable state |
| News | GNews (9 categories, sequential 1 req/s) | `GNEWS_API_KEY` | unavailable state |
| Stocks | Finnhub (equities) + CoinGecko (BTC) | `FINNHUB_API_KEY` | unavailable state |
| Sunrise | sunrise-sunset.org | none | unavailable state |
| Quote | api.api-ninjas.com/v2/quoteoftheday | `API_NINJAS_API_KEY` | unavailable state |

### Config (env vars → `internal/config/config.go`)
`PORT` (default 8080), `GNEWS_API_KEY`, `FINNHUB_API_KEY`, `API_NINJAS_API_KEY`, `LATITUDE`/`LONGITUDE` (default SF 37.7749/-122.4194), `TIMEZONE` (IANA tz name, e.g. `America/New_York`; defaults to server local time — required for correct calendar event filtering), `ENCRYPTION_KEY` (**required** — AES-256-GCM key for encrypting sensitive user settings; generate with `openssl rand -hex 32`), `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_CALLBACK_URL` (OAuth), `SESSION_KEY` (session signing), `FRONTEND_URL`, `CORS_ORIGIN`.

## Known Limitations

- **News uses sequential fetching** — GNews free tier requires ~1 req/s; 9 categories × 3h cache means full refresh takes ~9s on cache miss.

## Git Workflow

- Always use feature branches — never commit directly to `main`. Create a branch with a descriptive name before making changes.
- Verify the current branch with `git branch` before making changes. Never apply backend changes on a frontend branch or vice versa.
- Before opening a PR, verify the base branch with `git log --oneline --graph` to avoid branch ancestry issues.
- **Commit small and often**: each commit should represent one logical change (add a model, add a test, wire a handler, etc.). Never batch unrelated changes into a single commit. Frequent small commits make reviews easier, bisects faster, and cherry-picks cleaner.

## Claude Skills

- Skill files must use the subdirectory format: `.claude/skills/<skill-name>/SKILL.md` — never create flat `.md` files.

## API Integration

- When making concurrent calls to external services, implement rate limiting and sequential fallbacks to avoid provider rate limits.

## Documentation Rule

**Always update `README.md` and `.env.example` when:**
- A new env var is added or removed (update the Environment Variables table and `.env.example`)
- A new external API or service is added or removed (update the Architecture section and Obtaining API Keys)
- The behavior of an existing service changes in a user-visible way

Keep `README.md` as the single source of truth for setup instructions.

## Common Mistakes & Lessons Learned

A running log of mistakes made during development. Each entry is a reminder to
avoid repeating the same error. **Add to this section whenever a mistake is
caught in review or causes a regression.**

### Architecture / Layer Boundaries

- **Importing across layers**: `service/` must never import from `handler/` or
  `repository/`. `handler/` must never import from `repository/`. Violations
  break the DIP and make testing impossible. Check your imports before
  committing.

- **Defining store interfaces outside service/**: Every service defines its own
  store interface (e.g. `BillStore`, `TaskStore`) inside `internal/service/`.
  Repositories implement these interfaces via Go duck typing — never import
  a repository type directly into a service.

- **Inline sentinel errors**: All domain errors belong in
  `internal/errors/errors.go`. Never define `errors.New(...)` inline in a
  handler or service. Pick the right category (domain, validation, conflict,
  auth) and add a comment if needed.

- **Error category mislabeling**: When adding errors to `internal/errors/`,
  place them under the correct comment block. E.g. `ErrBillNotFound` belongs
  under "Domain errors", not under "Auth errors".

### Handler / Service Patterns

- **Missing `_dto.go` file**: Every handler file (`foo.go`) must have a
  matching `foo_dto.go` for its request/response structs. Never put DTOs in the
  main handler file.

- **Forgetting `AddRoutes` registration**: After creating a new handler, call
  `fooH.AddRoutes(r)` (or `fooH.AddRoutes(protected)`) in
  `internal/server/server.go:setupRoutes()`. The handler silently does nothing
  if this step is skipped.

- **Not scoping DB queries to `userID`**: Every query that touches user data
  must include a `userID` filter at the repository layer. Missing this is a
  data-isolation bug — user A can read/modify user B's data.

### Services (External API / Cache)

- **Skipping the cache check**: Every `Fetch` method must check
  `s.cache.Get(key)` before calling the external API, and call `s.cache.Set`
  after a successful fetch. Forgetting this causes the API to be hammered on
  every dashboard load.

### Documentation / Config

- **Missing README / .env.example update**: Any new env var must appear in
  both `README.md` (Environment Variables table) and `.env.example`. CI will
  not catch this — it must be done manually.

- **Outdated Known Limitations**: Update the Known Limitations section when a limitation is resolved.

### Database / Migrations

- **CHECK constraints instead of lookup tables**: Never use `CHECK (col IN ('a','b','c'))` or raw `VARCHAR` for enumerated values. Any column representing a finite set of values (type, status, category, role, provider) **must** use a lookup table with a FK reference. Example:
  ```sql
  -- BAD: CHECK constraint — breaks when new values are added, no label/sort_order
  provider TEXT NOT NULL CHECK(provider IN ('github'))
  -- GOOD: lookup table + FK
  provider_id TEXT NOT NULL REFERENCES provider_types(id)
  ```
  Create the lookup table in its own migration, seed it with known values, then FK to it.

- **FK column naming**: Columns that reference a lookup or parent table must end in `_id` (e.g. `provider_id`, `event_type_id`, `category_id`). Never use a bare noun (`provider`, `event_type`) for a FK column.

- **Plain UNIQUE on soft-delete columns**: Never put a plain `UNIQUE` constraint on any column in a table that uses `deleted_at`. Use a partial unique index instead:
  ```sql
  CREATE UNIQUE INDEX uq_<table>_<col>_active ON <table>(<col>) WHERE deleted_at IS NULL;
  ```

### Quality Gates

- **Ignoring `go test -race`**: The `-race` flag catches concurrent map writes
  and data races that plain tests miss. Always use it.

- **Skipping coverage checks**: Backend test coverage must remain at or above **80%**
  across `internal/service`. Run `go test -coverprofile=coverage.out ./...` and
  `go tool cover -func=coverage.out` to verify before opening a PR. New service
  code without tests will not be accepted.

- **Writing coverage-padding tests**: Tests must verify real behaviour — correct
  outputs, error types, cache interactions, validation rules. Tests that exist only
  to tick a line-covered box will be rejected in review.
