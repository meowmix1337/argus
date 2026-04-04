# Argus

A personal daily life dashboard with a Go backend and React/TypeScript frontend. Displays weather, calendar, tasks, news, stocks/crypto, and a daily quote — all in a single dark-themed view.

![screenshot placeholder](docs/screenshot.png)

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.26 + chi router |
| Frontend | React 19 + TypeScript + Vite 6 |
| Styling | Tailwind CSS 4 |
| State | TanStack Query v5 |
| Database | SQLite (via sqlx + goose migrations) |
| Auth | Google OAuth 2.0 |
| Containers | Docker + Docker Compose v2 |
| Proxy | Nginx 1.27 |

## Architecture

```
Browser → Nginx (frontend) → Go API (backend)
                                 ├─ Open-Meteo        (weather, no auth)
                                 ├─ GNews             (headlines, API key)
                                 ├─ Finnhub           (stocks, API key)
                                 ├─ CoinGecko         (BTC, no auth)
                                 ├─ Sunrise-Sunset.org (no auth)
                                 ├─ API Ninjas        (daily quote, API key)
                                 └─ ICS Calendar      (today's events, URL)
```

## Quick Start

```bash
cp .env.example .env
# Edit .env and add your API keys
make docker-up
# Open http://localhost:3000
```

## Local Development

**Terminal 1 — backend:**
```bash
make dev-backend
# Starts Go server at http://localhost:8080
```

**Terminal 2 — frontend:**
```bash
cd frontend && npm install
make dev-frontend
# Starts Vite dev server at http://localhost:5173
```

### Testing authenticated endpoints locally

Argus uses Google OAuth for login, which requires a browser flow. For local API testing (e.g. `curl`) you can mint a valid signed session cookie without going through OAuth:

```bash
cd backend && go run ./cmd/devtoken
# Prints a signed session token, e.g.:
# eyJ1c2VySUQiOiJkZXYtdGVzdC11c2VyLTAwMDAwMDAwIiwiZ....<sig>
```

Use the token as a cookie in your requests:

```bash
TOKEN=$(cd backend && go run ./cmd/devtoken)
curl -s -H "Cookie: session=$TOKEN" http://localhost:8080/api/dashboard | jq .
```

The token is valid for 1 hour and uses `SESSION_SECRET` from your `.env` file. **Never use this tool in production.**

### Testing GitHub Webhooks Locally

There are three ways to deliver GitHub webhook events to your local server:

#### Option A — `gh webhook forward` (real GitHub events)

The [GitHub CLI](https://cli.github.com/) can forward live webhook deliveries from GitHub to your local server without exposing a public URL:

```bash
gh webhook forward \
  --repo=<owner>/<repo> \
  --events=pull_request,issue_comment,pull_request_review_comment \
  --url=http://localhost:8080/api/webhooks/github
```

Events are forwarded in real time and validated via HMAC just like production. Requires `gh` to be installed and authenticated (`gh auth login`).

#### Option B — ngrok (real GitHub events via public tunnel)

Expose your local server with a public URL so GitHub delivers webhooks directly:

```bash
ngrok http 8080
```

Copy the `Forwarding` URL (e.g. `https://abc123.ngrok-free.app`) and:

1. Set `GITHUB_WEBHOOK_URL=https://abc123.ngrok-free.app/api/webhooks/github` in your `.env`
2. Configure the webhook in your GitHub repo settings (**Settings → Webhooks → Add webhook**) pointing to that URL

Events are HMAC-validated end-to-end just like production.

#### Option C — `_simulate` endpoint (synthetic events, no HMAC)

When `APP_ENV=development`, the server exposes a debug endpoint that injects a synthetic event directly into the notification pipeline — no HMAC signature required:

```bash
curl -s -X POST http://localhost:8080/api/webhooks/github/_simulate \
  -H "Content-Type: application/json" \
  -d '{
    "event_type": "pull_request",
    "payload": {
      "action": "opened",
      "number": 42,
      "pull_request": {
        "title": "Test PR",
        "html_url": "https://github.com/owner/repo/pull/42",
        "user": { "login": "octocat" }
      },
      "repository": {
        "full_name": "owner/repo"
      }
    }
  }' | jq .
```

> **Prerequisite:** The `full_name` in the payload must match a repository the user has added via the GitHub integration settings. If no watched repo matches, the endpoint returns `400`.

The route is not registered when `APP_ENV` is not set to `development` — requests to that path will receive the server's default not-found response.

## Obtaining API Keys & Config

Cards that require credentials show an unavailable state when their key/URL is not set — no crashes or mock data.

### GNews (headlines)

1. Go to [gnews.io](https://gnews.io) and click **Get API Key**
2. Sign up for a free account (100 requests/day on the free tier)
3. Copy the key from your dashboard and set `GNEWS_API_KEY` in `.env`

### Finnhub (stocks)

1. Go to [finnhub.io](https://finnhub.io) and click **Get free API key**
2. Sign up — no credit card required (60 requests/minute on the free tier)
3. Copy the key from **Dashboard → API Key** and set `FINNHUB_API_KEY` in `.env`

### API Ninjas (daily quote)

1. Go to [api-ninjas.com](https://api-ninjas.com) and create a free account
2. Copy your API key from the dashboard and set `API_NINJAS_API_KEY` in `.env`

### Calendar (ICS URL)

The calendar card reads any standard ICS/iCal feed — no OAuth required.

**Google Calendar:**
1. Open [Google Calendar](https://calendar.google.com) → Settings (gear icon)
2. Select the calendar you want to display under **Settings for my calendars**
3. Scroll to **Integrate calendar** → copy the **Secret address in iCal format**
4. Set `CALENDAR_ICS_URL` in `.env` to that URL

**Apple iCloud Calendar:**
1. In the Calendar app, right-click (or Ctrl-click) the calendar → **Share Calendar**
2. Enable **Public Calendar** and copy the link
3. Replace `webcal://` with `https://` and set `CALENDAR_ICS_URL` in `.env`

> The ICS URL is private — treat it like a password. Calendar data is cached for 15 minutes.

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `ENCRYPTION_KEY` | AES-256 key for encrypting sensitive user settings. Generate with `openssl rand -hex 32` | **Yes** | — |
| `GNEWS_API_KEY` | GNews API key for headlines | No | — (unavailable state) |
| `FINNHUB_API_KEY` | Finnhub API key for stocks | No | — (unavailable state) |
| `API_NINJAS_API_KEY` | API Ninjas key for daily quote | No | — (unavailable state) |
| `CALENDAR_ICS_URL` | Private ICS URL for calendar events (stored encrypted) | No | — (unavailable state) |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID | No | — (auth disabled) |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret | No | — (auth disabled) |
| `GOOGLE_CALLBACK_URL` | OAuth redirect URL (e.g. `http://localhost:8080/auth/google/callback`) | No | — |
| `SESSION_KEY` | Secret for signing session cookies | No | random (insecure for prod) |
| `LATITUDE` | Your location latitude | No | 37.7749 (SF) |
| `LONGITUDE` | Your location longitude | No | -122.4194 (SF) |
| `TIMEZONE` | IANA timezone for calendar date filtering (e.g. `America/New_York`) | No | server local (UTC in Docker) |
| `PORT` | Backend server port | No | 8080 |
| `CORS_ORIGIN` | Allowed CORS origin for the API | No | `http://localhost:5173` |
| `FRONTEND_URL` | Frontend URL for OAuth redirects | No | `http://localhost:5173` |
| `SECURE_COOKIES` | Set `true` in production (HTTPS only cookies) | No | false |
| `GITHUB_CLIENT_ID` | GitHub OAuth App client ID (for notification center integration) | No | — (integration disabled) |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret | No | — (integration disabled) |
| `GITHUB_CALLBACK_URL` | GitHub OAuth redirect URL (e.g. `http://localhost:8080/api/auth/github/callback`) | No | — |
| `GITHUB_WEBHOOK_URL` | Public URL where GitHub delivers webhook events (must be reachable by GitHub; use ngrok for local dev) | No | — (webhooks not installed) |
| `APP_ENV` | Set to `development` to enable the `POST /api/webhooks/github/_simulate` debug endpoint | No | — (simulate disabled) |

## API Endpoints

### Public
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/auth/google` | Initiate Google OAuth flow |
| GET | `/auth/google/callback` | OAuth callback |
| POST | `/auth/logout` | Clear session |
| POST | `/api/webhooks/github` | Receive GitHub webhook deliveries (HMAC-validated) |

### Protected (requires auth session)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/me` | Current user info |
| GET | `/api/dashboard` | All data aggregated |
| GET | `/api/weather` | Current weather + forecast |
| GET | `/api/news` | Top headlines by category |
| GET | `/api/stocks` | Stock + crypto quotes |
| GET | `/api/stocks/symbols` | Watchlist symbols |
| POST | `/api/stocks/symbols` | Add symbol to watchlist |
| DELETE | `/api/stocks/symbols/{symbol}` | Remove symbol |
| GET | `/api/stocks/symbols/search` | Search Finnhub symbols |
| GET | `/api/calendar` | Today's events |
| GET | `/api/meta` | Sunrise/sunset + daily quote |
| GET | `/api/tasks` | Task list |
| POST | `/api/tasks` | Create task |
| PATCH | `/api/tasks/{id}` | Update task |
| DELETE | `/api/tasks/{id}` | Delete task |
| GET | `/api/labels` | All user labels |
| POST | `/api/labels` | Create label |
| PATCH | `/api/labels/{id}` | Update label |
| DELETE | `/api/labels/{id}` | Delete label |
| GET | `/api/tasks/{taskID}/labels` | Labels on a task |
| POST | `/api/tasks/{taskID}/labels` | Assign label to task |
| DELETE | `/api/tasks/{taskID}/labels/{labelID}` | Remove label from task |
| GET | `/api/settings` | User settings |
| PUT | `/api/settings` | Update user settings |
| GET | `/api/settings/news-categories` | Available + selected news categories |
| PUT | `/api/settings/news-categories` | Set selected news categories |

### Development only (`APP_ENV=development`)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/webhooks/github/_simulate` | Inject a synthetic GitHub event without HMAC validation |

## Project Structure

```
argus/
├── backend/
│   ├── cmd/server/main.go        # Entrypoint
│   ├── db/migrations/            # goose SQL migrations
│   └── internal/
│       ├── config/config.go      # Env config
│       ├── errors/errors.go      # Sentinel domain errors
│       ├── handler/              # HTTP handlers + DTOs
│       ├── httpclient/           # HTTPClient interface + wrapper
│       ├── middleware/           # CORS, auth, logging
│       ├── model/                # Domain types (shared across layers)
│       ├── repository/           # SQLite data access (implements service interfaces)
│       ├── response/             # WriteJSON / WriteError helpers
│       ├── server/server.go      # Router + dependency wiring
│       ├── service/              # Business logic + external API clients
│       └── validate/             # Shared validator instance
├── frontend/
│   └── src/
│       ├── api/client.ts         # API fetch wrapper
│       ├── components/           # React components
│       ├── hooks/                # React Query hooks
│       └── types/dashboard.ts    # TypeScript interfaces
├── deploy/
│   ├── backend.Dockerfile
│   ├── frontend.Dockerfile
│   └── nginx.conf
├── docker-compose.yml
├── .env.example
└── Makefile
```

## Deployment

Deploy on any Linux VPS:

```bash
git clone <your-repo>
cd argus
cp .env.example .env
# Fill in API keys
docker compose up -d
```

The stack runs on ports 3000 (frontend) and 8080 (backend). Point a reverse proxy at port 3000.
