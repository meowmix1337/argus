# Social Feed Feature — Full Implementation Plan

**Epic:** `argus-fi5`
**Date:** 2026-04-05
**Status:** Approved — ready for implementation
**Complexity:** HIGH
**Scope:** Full-stack feature (4 sequential PRs, ~40 new files)

---

## Table of Contents

1. [Context & Decisions](#context--decisions)
2. [bd Task Structure](#bd-task-structure)
3. [Guardrails](#guardrails)
4. [Phase 1 — DB Migrations](#phase-1-database-migrations)
5. [Phase 2 — Backend Services + Handlers](#phase-2-backend-services--handlers)
6. [Phase 3 — Frontend Social Section](#phase-3-frontend-social-section)
7. [Phase 4 — NSQ Feed Fanout Consumer](#phase-4-nsq-feed-fanout-consumer)
8. [Risk Mitigations](#risk-mitigations)
9. [Verification Commands](#verification-commands)
10. [Overall Success Criteria](#overall-success-criteria)

---

## Context & Decisions

Argus is a personal dashboard app (Go backend + React/TS frontend, SQLite, Google OAuth). This plan adds a Twitter-like social feed: users can post short messages (128 chars max), follow other users, like posts, search via FTS5, and view a reverse-chronological feed of posts from followed users. NSQ provides server-side event publishing and fanout. The frontend renders a full-width "Social" section below the existing dashboard card grid.

### Key Design Decisions (resolved during planning sessions)

| Decision | Choice | Rationale |
|---|---|---|
| Event system | **NSQ** (`nsqd` + `nsqlookupd` in docker-compose) | Go-native, lightweight, at-least-once delivery, no single point of failure |
| UI placement | **Full-width section below the dashboard card grid** | More real estate than a card; doesn't compete for grid space |
| Feed pagination | **Cursor-based** (opaque UUID v7 post ID) | No offset drift when new posts arrive at the top |
| Search | **SQLite FTS5** (`posts_fts` virtual table + sync triggers) | No external dependency; built into modernc.org/sqlite |
| Real-time updates | **Polling** (30s React Query refetch interval) | MVP simplicity; WebSocket/SSE deferred to Phase 5+ |
| Post visibility | **Private to followers only** | Feed, search, and `/api/users/{id}/posts` all filter by follow relationship |
| Single-post access | **Open to any authenticated user** | `GET /api/posts/{id}` has no follower-check (enables shared links) |
| Feed history on follow | **All historical posts** | Following someone shows their full history; no created_at filter vs follow date |

### NSQ Consumer Architecture Decisions (resolved during deep interview)

| Decision | Choice |
|---|---|
| Message format | JSON `EventEnvelope{v, type, ts, payload}` wrapping typed payload structs |
| Consumer framework | General-purpose `MessageHandler` interface + `ConsumerManager` (not social-feed-specific) |
| Retry policy | Exponential backoff: 5s → 10s → 20s → 40s → 80s; discard after `maxRetries=5` |
| Channel naming | Stable descriptive slugs per consumer type (`"feed-fanout"`, `"notifications"`, etc.) |
| Testing strategy | Split `HandleMessage` (NSQ plumbing) from `process()` (business logic); unit test `process()` without NSQ |

### Codebase Conventions (enforced throughout)

- **Layer boundaries:** `handler/` → `service/` → `repository/`. No cross-layer imports.
- **Store interfaces:** Defined inside `internal/service/`. Repositories implement via Go duck typing.
- **DTOs:** All request/response structs in `<handler>_dto.go` files.
- **DB schema:** UUID v7 PKs (`CHAR(36)`), `created_at`/`updated_at`/`deleted_at`, partial unique indexes for soft-delete, lookup tables for enums, `CHECK` constraints, `-- +goose Up/Down` format.
- **Error handling:** Sentinel errors in `internal/errors/errors.go`. Service wraps; handler maps to HTTP status.
- **Rate limiting:** `httprate.LimitByIP` middleware in `AddRoutes`. Never inline rate limiters.
- **Logging:** `log/slog` only. Never `"log"` package.
- **Validation:** `go-playground/validator` struct tags on DTOs; `validate.Struct()` in handler.
- **Frontend:** Inline styles only (no Tailwind classes in JSX). React Query for data fetching. Components receive data as props.
- **Testing:** Service test coverage ≥ 80%. Handler tests: happy path, pagination, 401, 400.
- **Migrations:** Isolated in their own PR. Never mixed with application code.

---

## bd Task Structure

```
argus-fi5 (epic)   Social Feed Feature (Twitter-like posts)
  └── argus-aff    Phase 1: DB migrations                   ← START HERE (unblocked)
        └── argus-7e7   Phase 2: Backend services + handlers
              └── argus-mex   Phase 3: Frontend Social section
                    └── argus-2np   Phase 4: NSQ fanout consumer
```

Each phase = exactly one PR targeting `main`. Close bd task only after PR is merged.

---

## Guardrails

### Must Have
- 128-char post limit enforced at validation layer AND DB `CHECK` constraint
- HTML sanitization via `bluemonday.StrictPolicy()` before storage (strip all tags)
- Cursor-based pagination using UUID v7 time-sortability (no offset)
- All endpoints behind `requireAuth` middleware
- User ID always from session context — never from request body or path param
- `parent_post_id` column on posts (nullable, unused in MVP — for future replies)
- `media_urls` column on posts (nullable, unused in MVP — for future media)
- NSQ events published for `post.created`, `post.liked`, `user.followed`
- Soft-delete everywhere (followers = unfollow, likes = unlike)
- Posts private to followers only (search + profile queries include follower-check WHERE clause)

### Visibility Rules
- **Feed, search, `/api/users/{id}/posts`**: filtered to followed users + self
- **`GET /api/posts/{id}`**: open to any authenticated user (no follower-check)
- **Feed history**: all historical posts shown after follow (no date cutoff)
- **Follower-check WHERE clause** (used in Search + ListByUser queries):
  ```sql
  AND (p.user_id = viewerUserID OR p.user_id IN (
      SELECT following_id FROM followers WHERE follower_id = viewerUserID AND deleted_at IS NULL
  ))
  ```

### Must NOT Have (MVP)
- No reply threading UI or API (column exists for future use)
- No media upload (column exists for future use)
- No notification consumption from NSQ (publish-only for `post.liked`, `user.followed`)
- No WebSocket/SSE — polling only
- No algorithmic feed ranking — pure reverse chronological
- No blocking/muting users

---

## Phase 1: Database Migrations

**bd task:** `argus-aff`
**Branch:** `feat/social-feed-migrations-argus-fi5`
**PR scope:** Migration files only. Zero application code.

### Migration 016 — posts table

**File:** `backend/migrations/016_create_posts.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS posts (
    id              CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    user_id         CHAR(36) NOT NULL REFERENCES users(id),
    content         TEXT NOT NULL CHECK(length(content) >= 1 AND length(content) <= 128),
    parent_post_id  CHAR(36) REFERENCES posts(id),   -- future: replies
    like_count      INTEGER NOT NULL DEFAULT 0,
    media_urls      TEXT,                              -- future: JSON array
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at      TEXT
);

-- Feed query: user's own posts + followed users, newest first, cursor-based
CREATE INDEX IF NOT EXISTS idx_posts_user_created
    ON posts (user_id, created_at DESC) WHERE deleted_at IS NULL;

-- Single post by ID (for like, delete, get)
CREATE INDEX IF NOT EXISTS idx_posts_id_active
    ON posts (id) WHERE deleted_at IS NULL;

-- +goose StatementBegin
CREATE TRIGGER posts_updated_at
    AFTER UPDATE ON posts
    FOR EACH ROW
    WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE posts
       SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
     WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS posts_updated_at;
DROP INDEX  IF EXISTS idx_posts_id_active;
DROP INDEX  IF EXISTS idx_posts_user_created;
DROP TABLE  IF EXISTS posts;
```

### Migration 017 — post_likes table

**File:** `backend/migrations/017_create_post_likes.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS post_likes (
    id         CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    post_id    CHAR(36) NOT NULL REFERENCES posts(id),
    user_id    CHAR(36) NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at TEXT
);

-- One active like per user per post
CREATE UNIQUE INDEX IF NOT EXISTS uq_post_likes_active
    ON post_likes (post_id, user_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_post_likes_post_active
    ON post_likes (post_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_post_likes_user_post
    ON post_likes (user_id, post_id) WHERE deleted_at IS NULL;

-- Trigger: increment like_count on insert
-- +goose StatementBegin
CREATE TRIGGER post_likes_increment
    AFTER INSERT ON post_likes
    FOR EACH ROW
    WHEN NEW.deleted_at IS NULL
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
END;
-- +goose StatementEnd

-- Trigger: decrement like_count on soft-delete (unlike)
-- +goose StatementBegin
CREATE TRIGGER post_likes_decrement
    AFTER UPDATE OF deleted_at ON post_likes
    FOR EACH ROW
    WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL
BEGIN
    UPDATE posts SET like_count = MAX(0, like_count - 1) WHERE id = NEW.post_id;
END;
-- +goose StatementEnd

-- Trigger: re-increment when un-soft-deleted (re-like)
-- +goose StatementBegin
CREATE TRIGGER post_likes_reincrement
    AFTER UPDATE OF deleted_at ON post_likes
    FOR EACH ROW
    WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
END;
-- +goose StatementEnd

-- Reconciliation query (run manually if like_count drifts):
-- UPDATE posts SET like_count = (SELECT COUNT(*) FROM post_likes WHERE post_id = posts.id AND deleted_at IS NULL);

-- +goose Down
DROP TRIGGER IF EXISTS post_likes_reincrement;
DROP TRIGGER IF EXISTS post_likes_decrement;
DROP TRIGGER IF EXISTS post_likes_increment;
DROP INDEX  IF EXISTS idx_post_likes_user_post;
DROP INDEX  IF EXISTS idx_post_likes_post_active;
DROP UNIQUE INDEX IF EXISTS uq_post_likes_active;
DROP TABLE  IF EXISTS post_likes;
```

### Migration 018 — followers table

**File:** `backend/migrations/018_create_followers.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS followers (
    id           CHAR(36) NOT NULL PRIMARY KEY CHECK(length(id) = 36),
    follower_id  CHAR(36) NOT NULL REFERENCES users(id),
    following_id CHAR(36) NOT NULL REFERENCES users(id),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    deleted_at   TEXT,

    -- Cannot follow yourself
    CHECK(follower_id != following_id)
);

-- One active follow per pair
CREATE UNIQUE INDEX IF NOT EXISTS uq_followers_active
    ON followers (follower_id, following_id) WHERE deleted_at IS NULL;

-- "Who do I follow?" — feed query + following list
CREATE INDEX IF NOT EXISTS idx_followers_follower_active
    ON followers (follower_id) WHERE deleted_at IS NULL;

-- "Who follows me?" — followers list
CREATE INDEX IF NOT EXISTS idx_followers_following_active
    ON followers (following_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_followers_following_active;
DROP INDEX IF EXISTS idx_followers_follower_active;
DROP UNIQUE INDEX IF EXISTS uq_followers_active;
DROP TABLE IF EXISTS followers;
```

### Migration 019 — FTS5 virtual table

**File:** `backend/migrations/019_create_posts_fts.sql`

```sql
-- +goose Up

-- External-content FTS5 table for full-text search over post content.
CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
    content,
    content='posts',
    content_rowid='rowid'
);

-- +goose StatementBegin
CREATE TRIGGER posts_fts_insert
    AFTER INSERT ON posts
    FOR EACH ROW
BEGIN
    INSERT INTO posts_fts(rowid, content) VALUES (NEW.rowid, NEW.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_fts_delete
    AFTER DELETE ON posts
    FOR EACH ROW
BEGIN
    INSERT INTO posts_fts(posts_fts, rowid, content) VALUES ('delete', OLD.rowid, OLD.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_fts_update
    AFTER UPDATE OF content ON posts
    FOR EACH ROW
BEGIN
    INSERT INTO posts_fts(posts_fts, rowid, content) VALUES ('delete', OLD.rowid, OLD.content);
    INSERT INTO posts_fts(rowid, content) VALUES (NEW.rowid, NEW.content);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS posts_fts_update;
DROP TRIGGER IF EXISTS posts_fts_delete;
DROP TRIGGER IF EXISTS posts_fts_insert;
DROP TABLE   IF EXISTS posts_fts;
```

### Acceptance Criteria (Phase 1)

- [ ] `go test -race ./...` passes with migrations applied
- [ ] INSERT into `posts` with content > 128 chars rejected by CHECK constraint
- [ ] INSERT into `followers` with `follower_id = following_id` rejected by CHECK constraint
- [ ] INSERT into `post_likes` increments `posts.like_count`; soft-delete decrements it
- [ ] FTS5 `SELECT * FROM posts_fts WHERE posts_fts MATCH 'hello'` returns matching rows
- [ ] All `-- +goose Down` migrations cleanly reverse the schema
- [ ] No application code in this PR

---

## Phase 2: Backend Services + Handlers

**bd task:** `argus-7e7`
**Branch:** `feat/social-feed-backend-argus-fi5`
**PR scope:** All Go application code. No migration files.

### Step 2.1 — Dependencies + config

**Files to modify:**
- `backend/go.mod` — add `github.com/microcosm-cc/bluemonday` and `github.com/nsqio/go-nsq`
- `backend/internal/config/config.go` — add fields:

```go
NSQNsqdAddr   string // NSQ_NSQD_ADDR, default "localhost:4150"
NSQLookupAddr string // NSQ_LOOKUPD_ADDR, default "localhost:4161"
```

- `README.md` and `.env.example` — add new env vars (see Documentation Updates)

### Step 2.2 — Event bus package (`internal/events/`)

**4 new files:**

```
backend/internal/events/
├── events.go      # Publisher interface, EventBus, topic constants, PublishEvent helper
├── noop.go        # NoopPublisher (dev fallback when NSQ_NSQD_ADDR is empty)
├── payloads.go    # EventEnvelope + typed payload structs
└── consumer.go    # MessageHandler interface, ConsumerManager, retry logic
```

**`events.go` — Publisher interface and topic constants:**
```go
type Publisher interface {
    PublishEvent(topic string, payload any) error
    Stop()
}

const (
    TopicPostCreated  = "post.created"
    TopicPostLiked    = "post.liked"
    TopicUserFollowed = "user.followed"
)
```

**`payloads.go` — Message envelope and typed payloads:**
```go
// EventEnvelope wraps all NSQ messages. Version field enables future schema evolution.
type EventEnvelope struct {
    Version    int             `json:"v"`       // schema version, start at 1
    EventType  string          `json:"type"`    // matches topic constant
    OccurredAt string          `json:"ts"`      // RFC3339 UTC
    Payload    json.RawMessage `json:"payload"` // typed per EventType
}

type PostCreatedPayload struct {
    PostID   string `json:"postId"`
    AuthorID string `json:"authorId"`
    Content  string `json:"content"` // sanitized; for future analytics/notification consumers
}

type PostLikedPayload struct {
    PostID string `json:"postId"`
    UserID string `json:"userId"`
    Liked  bool   `json:"liked"` // true=liked, false=unliked
}

type UserFollowedPayload struct {
    FollowerID  string `json:"followerId"`
    FollowingID string `json:"followingId"`
}
```

**`events.go` — PublishEvent helper:**
```go
func (b *EventBus) PublishEvent(topic string, payload any) error {
    raw, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    env := EventEnvelope{
        Version:    1,
        EventType:  topic,
        OccurredAt: time.Now().UTC().Format(time.RFC3339),
        Payload:    raw,
    }
    data, err := json.Marshal(env)
    if err != nil {
        return err
    }
    return b.producer.Publish(topic, data)
}
```

**`consumer.go` — General-purpose consumer framework:**
```go
// MessageHandler is the interface all NSQ consumers implement.
// To add a new consumer: implement this interface + cm.Register() in main.go.
type MessageHandler interface {
    Topic() string   // NSQ topic
    Channel() string // stable slug, e.g. "feed-fanout", "notifications", "analytics"
    process(body []byte) error // business logic — unit-testable without NSQ
}

const (
    maxRetries  = 5
    baseBackoff = 5 * time.Second
    maxBackoff  = 10 * time.Minute
)

// backoffDelay sequence: 5s, 10s, 20s, 40s, 80s (capped at 10m)
func backoffDelay(attempt uint16) time.Duration { ... }

// handleWithRetry: on error, requeue with backoff; after maxRetries, log + ACK to discard
func handleWithRetry(h MessageHandler, msg *nsq.Message) error { ... }

type ConsumerManager struct { ... }
func NewConsumerManager(lookupAddr string) *ConsumerManager
func (m *ConsumerManager) Register(h MessageHandler) error
func (m *ConsumerManager) Start() error  // ConnectToNSQLookupd
func (m *ConsumerManager) Stop()         // graceful shutdown
```

**NSQ constructor:** `NewEventBus(nsqdAddr string)` — if `nsqdAddr` is empty, returns `NoopPublisher` (graceful degradation; NSQ is optional in dev).

### Step 2.3 — Sentinel errors

**File:** `backend/internal/errors/errors.go`

```go
// Domain errors
var ErrPostNotFound    = errors.New("post not found")
var ErrFollowNotFound  = errors.New("follow relationship not found")
var ErrSelfFollow      = errors.New("cannot follow yourself")

// Validation errors
var ErrPostValidation  = errors.New("post validation failed")

// Conflict errors
var ErrAlreadyFollowing = errors.New("already following this user")
```

### Step 2.4 — Models

**File:** `backend/internal/model/models.go`

```go
type Post struct {
    ID           string `json:"id"`
    UserID       string `json:"userId"`
    AuthorName   string `json:"authorName"`
    AuthorAvatar string `json:"authorAvatar"`
    Content      string `json:"content"`
    LikeCount    int    `json:"likeCount"`
    LikedByMe    bool   `json:"likedByMe"`
    CreatedAt    string `json:"createdAt"`
}

type PostCreate struct {
    ID      string
    UserID  string
    Content string
}

type UserSummary struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    AvatarURL   string `json:"avatarUrl"`
    IsFollowing bool   `json:"isFollowing"`
}

type FeedPage struct {
    Posts      []Post  `json:"posts"`
    NextCursor *string `json:"nextCursor"` // nil = no more pages
}

type UserListPage struct {
    Users      []UserSummary `json:"users"`
    NextCursor *string       `json:"nextCursor"`
}
```

### Step 2.5 — PostsService + PostsRepository

**New files:**
- `backend/internal/service/posts.go`
- `backend/internal/repository/sqlite_posts_repository.go`

**Store interface (in `service/posts.go`):**
```go
type PostStore interface {
    Create(ctx context.Context, p model.PostCreate) (model.Post, error)
    GetByID(ctx context.Context, postID, viewerUserID string) (model.Post, error)
    Delete(ctx context.Context, postID, userID string) (int64, error)
    ListByUser(ctx context.Context, userID, viewerUserID string, limit int, cursor string) ([]model.Post, error)
    ListFeed(ctx context.Context, userID string, limit int, cursor string) ([]model.Post, error)
    Search(ctx context.Context, query, viewerUserID string, limit int, cursor string) ([]model.Post, error)
    ToggleLike(ctx context.Context, postID, userID string) (liked bool, err error)
}
```

**PostsService key behaviors:**
- `Create`: trim whitespace → `bluemonday.StrictPolicy()` sanitize → validate length (1–128 after sanitize) → UUID v7 → store → publish `TopicPostCreated`
- `Delete`: store → check `rowsAffected==0` → `ErrPostNotFound`
- `ToggleLike`: store → publish `TopicPostLiked`
- `ListFeed`: feed query uses cursor (`id DESC`, UUID v7 time-sortable)
- `Search` + `ListByUser`: include follower-check WHERE clause (visibility model)

**Feed query (cursor-based, follower-filtered):**
```sql
SELECT p.id, p.user_id, u.name, u.avatar_url, p.content, p.like_count, p.created_at,
       CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me
FROM posts p
JOIN users u ON u.id = p.user_id
LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND (p.user_id = ? OR p.user_id IN (
      SELECT following_id FROM followers WHERE follower_id = ? AND deleted_at IS NULL
  ))
  AND (? = '' OR p.id < ?)
ORDER BY p.id DESC
LIMIT ?
```

**Search query (FTS5 + follower-check):**
```sql
SELECT p.id, p.user_id, u.name, u.avatar_url, p.content, p.like_count, p.created_at,
       CASE WHEN pl.id IS NOT NULL THEN 1 ELSE 0 END AS liked_by_me
FROM posts p
JOIN users u ON u.id = p.user_id
JOIN posts_fts fts ON fts.rowid = p.rowid
LEFT JOIN post_likes pl ON pl.post_id = p.id AND pl.user_id = ? AND pl.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND posts_fts MATCH ?
  AND (p.user_id = ? OR p.user_id IN (
      SELECT following_id FROM followers WHERE follower_id = ? AND deleted_at IS NULL
  ))
  AND (? = '' OR p.id < ?)
ORDER BY p.id DESC
LIMIT ?
```

**Toggle like pattern:** single transaction — check active like → soft-delete if exists; else check soft-deleted row → un-soft-delete if exists; else insert new row.

### Step 2.6 — FollowService + FollowRepository

**New files:**
- `backend/internal/service/follow.go`
- `backend/internal/repository/sqlite_follow_repository.go`

**Store interface:**
```go
type FollowStore interface {
    Follow(ctx context.Context, followerID, followingID string) error
    Unfollow(ctx context.Context, followerID, followingID string) (int64, error)
    ListFollowers(ctx context.Context, userID, viewerUserID string, limit int, cursor string) ([]model.UserSummary, error)
    ListFollowing(ctx context.Context, userID, viewerUserID string, limit int, cursor string) ([]model.UserSummary, error)
    IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
    Discover(ctx context.Context, userID string, limit int, cursor string) ([]model.UserSummary, error)
}
```

**Key behaviors:**
- `Follow`: validate not self-follow → `ErrSelfFollow`; check for soft-deleted row (re-follow = clear `deleted_at`); else insert new; if active row exists → `ErrAlreadyFollowing`; publish `TopicUserFollowed`
- `Unfollow`: soft-delete; `rowsAffected==0` → `ErrFollowNotFound`
- `Discover`: users not followed by current user, excluding self

### Step 2.7 — Handlers + DTOs

**New files:**
- `backend/internal/handler/posts.go` + `posts_dto.go`
- `backend/internal/handler/follow.go` + `follow_dto.go`

**PostsHandler routes:**
```go
func (h *PostsHandler) AddRoutes(r chi.Router) {
    r.With(httprate.LimitByIP(postsCreateRateLimit, time.Minute)).Post("/api/posts", h.Create)
    r.Get("/api/posts/search", h.Search)   // before /{id} to avoid route conflict
    r.Get("/api/posts/{id}", h.GetByID)
    r.Delete("/api/posts/{id}", h.Delete)
    r.With(httprate.LimitByIP(postsLikeRateLimit, time.Minute)).Post("/api/posts/{id}/like", h.ToggleLike)
    r.Get("/api/feed", h.Feed)
}
```

**FollowHandler routes:**
```go
func (h *FollowHandler) AddRoutes(r chi.Router) {
    r.Get("/api/users/discover", h.Discover)   // before /{id} to avoid route conflict
    r.With(httprate.LimitByIP(followRateLimit, time.Minute)).Post("/api/users/{id}/follow", h.Follow)
    r.Delete("/api/users/{id}/follow", h.Unfollow)
    r.Get("/api/users/{id}/followers", h.ListFollowers)
    r.Get("/api/users/{id}/following", h.ListFollowing)
    r.Get("/api/users/{id}/posts", h.ListUserPosts)
}
```

**Rate limit + pagination constants (`handler/constants.go`):**
```go
const (
    postsCreateRateLimit = 10  // per minute per IP
    postsLikeRateLimit   = 60
    followRateLimit      = 30

    defaultFeedLimit     = 20
    maxFeedLimit         = 50
    defaultSearchLimit   = 20
    maxSearchLimit       = 50
    defaultFollowLimit   = 20
    maxFollowLimit       = 50
    defaultDiscoverLimit = 10
    maxDiscoverLimit     = 30
    maxPostContentLength = 128
)
```

**DTOs (`posts_dto.go`):**
```go
type CreatePostRequest struct {
    Content string `json:"content" validate:"required,min=1,max=128"`
}

type PostResponse struct {
    ID           string `json:"id"`
    UserID       string `json:"userId"`
    AuthorName   string `json:"authorName"`
    AuthorAvatar string `json:"authorAvatar"`
    Content      string `json:"content"`
    LikeCount    int    `json:"likeCount"`
    LikedByMe    bool   `json:"likedByMe"`
    CreatedAt    string `json:"createdAt"`
}

type FeedResponse struct {
    Posts      []PostResponse `json:"posts"`
    NextCursor *string        `json:"nextCursor"`
}

type ToggleLikeResponse struct {
    Liked     bool `json:"liked"`
    LikeCount int  `json:"likeCount"`
}
```

**DTOs (`follow_dto.go`):**
```go
type UserSummaryResponse struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    AvatarURL   string `json:"avatarUrl"`
    IsFollowing bool   `json:"isFollowing"`
}

type UserListResponse struct {
    Users      []UserSummaryResponse `json:"users"`
    NextCursor *string               `json:"nextCursor"`
}
```

**Error → HTTP status mapping:**
| Error | Status |
|---|---|
| `ErrPostNotFound`, `ErrFollowNotFound` | 404 |
| `ErrPostValidation`, `ErrSelfFollow` | 400 |
| `ErrAlreadyFollowing` | 409 |

### Step 2.8 — Wire into server.go

**File:** `backend/internal/server/server.go`

1. Create `EventBus` (or `NoopPublisher` if `NSQNsqdAddr` empty) in `setupRoutes`
2. Create `PostsRepository`, `FollowRepository`
3. Create `PostsService(postRepo, publisher)`, `FollowService(followRepo, publisher)`
4. Create `PostsHandler`, `FollowHandler`
5. Call `postsH.AddRoutes(protected)` and `followH.AddRoutes(protected)` inside `requireAuth` group

### Step 2.9 — docker-compose NSQ services

**File:** `docker-compose.yml`

```yaml
nsqlookupd:
  image: nsqio/nsq:v1.3.0
  command: /nsqlookupd
  ports:
    - "4160:4160"
    - "4161:4161"
  restart: unless-stopped

nsqd:
  image: nsqio/nsq:v1.3.0
  command: /nsqd --lookupd-tcp-address=nsqlookupd:4160 --data-path=/data
  ports:
    - "4150:4150"
    - "4151:4151"
  depends_on:
    - nsqlookupd
  volumes:
    - nsqdata:/data
  restart: unless-stopped
```

Add `nsqdata` to volumes. Add `NSQ_NSQD_ADDR=nsqd:4150` and `NSQ_LOOKUPD_ADDR=nsqlookupd:4161` to backend service env.

### Step 2.10 — Tests

**New test files:**
- `backend/internal/service/posts_test.go`
- `backend/internal/service/follow_test.go`
- `backend/internal/handler/posts_test.go`
- `backend/internal/handler/follow_test.go`

**Service test cases (mock store + mock publisher):**
- Create: happy path, empty content, content too long, HTML tags (verify sanitized), publisher called
- Delete: rowsAffected=1 (ok), rowsAffected=0 (ErrPostNotFound)
- ToggleLike: like→unlike→like cycle; publisher called each time
- ListFeed: with cursor, without cursor, empty feed
- Search: with results, no results
- Follow: happy path, self-follow, already following, publisher called
- Unfollow: happy path, not found
- Discover: excludes self, excludes already-followed

**Handler test cases:**
- `POST /api/posts` — 201, 400 (empty), 400 (too long), 401
- `DELETE /api/posts/{id}` — 204, 404, 401
- `POST /api/posts/{id}/like` — 200, 401
- `GET /api/feed` — 200, 200 with cursor, 401
- `GET /api/posts/search` — 200, 400 (missing q), 401
- `POST /api/users/{id}/follow` — 200, 400 (self), 409 (duplicate), 401
- `DELETE /api/users/{id}/follow` — 204, 404, 401
- `GET /api/users/{id}/followers` — 200, 401
- `GET /api/users/discover` — 200, 401

### Acceptance Criteria (Phase 2)

- [ ] `PostsService.Create` with `<script>` stores sanitized version
- [ ] `PostsService.Create` > 128 chars returns `ErrPostValidation`
- [ ] `PostsService.ToggleLike` is idempotent: like→unlike→like cycle correct
- [ ] `ListFeed` returns only followed+own posts; cursor returns posts with ID < cursor
- [ ] Search returns only posts from followed users + self
- [ ] `ErrSelfFollow` on self-follow; `ErrAlreadyFollowing` on duplicate follow
- [ ] Unfollow + re-follow reuses soft-deleted row
- [ ] NSQ events published on create, like, follow
- [ ] `backoffDelay(1)` = 5s, `backoffDelay(5)` = 80s
- [ ] `go test -race ./internal/service/...` coverage ≥ 80%
- [ ] `golangci-lint run ./...` passes
- [ ] `go build ./...` passes

---

## Phase 3: Frontend Social Section

**bd task:** `argus-mex`
**Branch:** `feat/social-feed-frontend-argus-fi5`
**PR scope:** Frontend code only.

### Step 3.1 — TypeScript types

**File:** `frontend/src/types/dashboard.ts`

```typescript
export interface FeedPost {
  id: string;
  userId: string;
  authorName: string;
  authorAvatar: string;
  content: string;
  likeCount: number;
  likedByMe: boolean;
  createdAt: string;
}

export interface FeedResponse {
  posts: FeedPost[];
  nextCursor: string | null;
}

export interface ToggleLikeResponse {
  liked: boolean;
  likeCount: number;
}

export interface DiscoverUser {
  id: string;
  name: string;
  avatarUrl: string;
  isFollowing: boolean;
}

export interface UserListResponse {
  users: DiscoverUser[];
  nextCursor: string | null;
}
```

### Step 3.2 — API client functions

**File:** `frontend/src/api/client.ts`

```typescript
// Posts
export function createPost(content: string): Promise<FeedPost>
export function deletePost(id: string): Promise<void>
export function togglePostLike(id: string): Promise<ToggleLikeResponse>

// Feed + search
export function fetchFeed(cursor?: string, limit?: number): Promise<FeedResponse>
export function searchPosts(q: string, cursor?: string, limit?: number): Promise<FeedResponse>

// Social graph
export function followUser(id: string): Promise<void>
export function unfollowUser(id: string): Promise<void>
export function discoverUsers(cursor?: string, limit?: number): Promise<UserListResponse>
```

### Step 3.3 — React Query hooks

**New files:**
- `frontend/src/hooks/useFeed.ts` — `useInfiniteQuery`, cursor-based, 30s refetch, 60s stale
- `frontend/src/hooks/useSocialMutations.ts` — create/delete/like/follow/unfollow mutations with optimistic updates
- `frontend/src/hooks/useDiscover.ts` — `useQuery` for discover users
- `frontend/src/hooks/useSearch.ts` — `useInfiniteQuery`, debounced (300ms)

**useFeed pattern:**
```typescript
export function useFeed() {
  return useInfiniteQuery<FeedResponse>({
    queryKey: ['feed'],
    queryFn: ({ pageParam }) => fetchFeed(pageParam as string | undefined),
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    initialPageParam: undefined,
    staleTime: 60_000,
    refetchInterval: 30_000,
  });
}
```

### Step 3.4 — UI Components

**New directory:** `frontend/src/components/social/`

**New files:**
| File | Purpose |
|---|---|
| `SocialSection.tsx` | Full-width container rendered below card grid |
| `PostComposer.tsx` | Textarea with 128-char counter + submit button |
| `PostItem.tsx` | Single post: avatar, author, content, timestamp ("2m ago"), like button, delete (own posts only) |
| `FeedList.tsx` | Maps posts → PostItem; "Load more" button when `nextCursor` exists |
| `FollowButton.tsx` | Follow/Following toggle with hover state |
| `DiscoverPeople.tsx` | Horizontal scrollable row of user cards with FollowButton |
| `SearchBar.tsx` | Debounced search input (300ms); shows results in FeedList format |

**SocialSection layout:**
```
+------------------------------------------------------------------+
| [SearchBar]                                                       |
+------------------------------------------------------------------+
| Post Composer (textarea + char counter + Submit)                  |
+------------------------------------------------------------------+
| Discover People (horizontal scroll: avatar + name + FollowBtn)   |
| Note: shows name/avatar only — no post previews (posts private)  |
+------------------------------------------------------------------+
| Feed                                                              |
|   PostItem                                                        |
|   PostItem                                                        |
|   PostItem                                                        |
|   [Load More]                                                     |
+------------------------------------------------------------------+
```

**Styling rules:**
- All inline styles — no Tailwind classes in JSX (per project convention)
- Glass-morphism shell: `background: var(--bg-card)`, `border: 1px solid var(--bg-card-border)`, `borderRadius: 16`
- Use CSS custom properties from existing theme system
- Staggered fade-in animation consistent with existing cards

### Step 3.5 — Wire into Dashboard

**File:** `frontend/src/components/Dashboard.tsx`

```tsx
// Add below the card grid closing </div>, before footer:
<SocialSection style={{ marginTop: 32 }} />
```

The section manages its own loading/error states. No loading gate needed at Dashboard level.

### Acceptance Criteria (Phase 3)

- [ ] Social section renders below the card grid
- [ ] PostComposer shows char count; submit disabled when empty or > 128 chars
- [ ] Submitting a post clears composer and prepends post to feed (optimistic)
- [ ] Like button toggles optimistically; count updates immediately
- [ ] "Load more" fetches next cursor page and appends
- [ ] Search shows results after 300ms debounce
- [ ] DiscoverPeople shows unfollowed users (name + avatar only); Follow updates immediately
- [ ] Deleting own post removes from feed (optimistic)
- [ ] All components use inline styles only
- [ ] `npm run lint` passes with zero errors
- [ ] `npm run build` succeeds

---

## Phase 4: NSQ Feed Fanout Consumer

**bd task:** `argus-2np`
**Branch:** `feat/social-feed-nsq-consumer-argus-fi5`
**PR scope:** Consumer implementation + main.go wiring.

### Step 4.1 — FeedFanoutConsumer

**New file:** `backend/internal/events/feed_fanout.go`

```go
type FeedFanoutConsumer struct {
    store FollowStoreReader // injected interface for querying followers
}

func (c *FeedFanoutConsumer) Topic()   string { return TopicPostCreated }
func (c *FeedFanoutConsumer) Channel() string { return "feed-fanout" } // NEVER rename in prod

// process is the unit-testable business logic — no NSQ dependency.
func (c *FeedFanoutConsumer) process(body []byte) error {
    var env EventEnvelope
    if err := json.Unmarshal(body, &env); err != nil {
        return fmt.Errorf("unmarshal envelope: %w", err)
    }
    var p PostCreatedPayload
    if err := json.Unmarshal(env.Payload, &p); err != nil {
        return fmt.Errorf("unmarshal payload: %w", err)
    }
    // MVP: log fanout. Feed computed at read-time via SQL JOIN — no DB write needed here.
    // Phase 5+: query followers and write to a materialized per-user feed table.
    slog.Info("feed fanout received", "postId", p.PostID, "authorId", p.AuthorID)
    return nil
}
```

**New test file:** `backend/internal/events/feed_fanout_test.go`
```go
func TestFeedFanoutConsumer_process_valid(t *testing.T)        { ... }
func TestFeedFanoutConsumer_process_malformedJSON(t *testing.T) { ... }
func TestFeedFanoutConsumer_process_unknownVersion(t *testing.T) { ... }
```

**Channel naming rule:** Each `MessageHandler` defines its own stable slug. Never change a channel name in production without draining the old channel first.

| Consumer | Topic | Channel |
|---|---|---|
| FeedFanoutConsumer | `post.created` | `"feed-fanout"` |
| NotificationConsumer (future) | `post.liked` | `"notifications"` |
| AnalyticsConsumer (future) | `post.created` | `"analytics"` |

### Step 4.2 — Wire ConsumerManager in main.go

**File:** `backend/cmd/server/main.go`

```go
if cfg.NSQLookupAddr != "" {
    cm := events.NewConsumerManager(cfg.NSQLookupAddr)

    if err := cm.Register(&events.FeedFanoutConsumer{Store: followRepo}); err != nil {
        slog.Error("failed to register feed fanout consumer", "err", err)
    }
    // To add future consumers: cm.Register(&events.NotificationConsumer{...})

    if err := cm.Start(); err != nil {
        slog.Warn("NSQ consumers failed to start, continuing without", "err", err)
    } else {
        defer cm.Stop() // before httpServer.Shutdown
    }
}
```

### Retry policy (defined in `consumer.go`, Phase 2)

| Attempt | Delay |
|---|---|
| 1 | 5s |
| 2 | 10s |
| 3 | 20s |
| 4 | 40s |
| 5 | 80s |
| > 5 | discard: `slog.Error` + ACK |

NSQ has no native DLQ. Failed messages are logged and dropped after `maxRetries = 5`.

### Acceptance Criteria (Phase 4)

- [ ] `FeedFanoutConsumer.process()` unit tests pass with no NSQ running
- [ ] Valid envelope → no error + slog output
- [ ] Malformed JSON → error returned (triggers requeue in prod)
- [ ] `ConsumerManager` skipped entirely when `NSQ_LOOKUPD_ADDR` is empty
- [ ] Consumer connects to lookupd and receives `post.created` messages when NSQ is running
- [ ] Consumer gracefully shuts down via `cm.Stop()` on SIGTERM
- [ ] `backoffDelay(1)` = 5s, `backoffDelay(5)` = 80s (unit tested)
- [ ] `go test -race ./internal/events/...` passes
- [ ] `golangci-lint run ./...` passes

---

## Documentation Updates

**In Phase 2 PR:**
- `README.md` — Add Social Feed to Architecture section; add `NSQ_NSQD_ADDR`, `NSQ_LOOKUPD_ADDR` to Environment Variables table; update docker-compose instructions
- `.env.example` — Add `NSQ_NSQD_ADDR=localhost:4150` and `NSQ_LOOKUPD_ADDR=localhost:4161`
- `CLAUDE.md` — Add `events/` package to backend structure description; update "Adding a new widget" to mention social feed pattern

---

## Risk Mitigations

| Risk | Mitigation |
|---|---|
| FTS5 not in SQLite build | `modernc.org/sqlite` includes FTS5 by default. Verify: `SELECT * FROM pragma_compile_options WHERE compile_options LIKE '%FTS5%'` |
| `like_count` drift | Triggers handle all cases. Reconciliation query in migration comment: `UPDATE posts SET like_count = (SELECT COUNT(*) FROM post_likes WHERE post_id = posts.id AND deleted_at IS NULL)` |
| NSQ unavailable in dev | `NoopPublisher` fallback. App starts and functions fully without NSQ. |
| Feed query slow at scale | JOIN-based feed is fine for MVP (< 1000 followers). `EXPLAIN QUERY PLAN` to monitor. Phase 4 consumer prepares path to materialized feed table. |
| Cursor tampering | Cursor is a UUID v7 post ID in a prepared statement. Invalid UUIDs return empty page — no injection risk. |
| NSQ message deduplication | NSQ is at-least-once. `process()` must be safe to call multiple times. For Phase 4 (logging only) this is inherently safe. Phase 5+ DB writes should use INSERT OR IGNORE / upsert. |
| Stale channel names | Channel slugs are permanent in NSQ. Document them in this plan. Changing requires draining the old channel first. |

---

## Verification Commands

### Phase 1
```bash
cd backend
go test -race ./...
# sqlite3 verify:
# INSERT INTO posts (...content of 129 chars...) -- should fail CHECK constraint
# INSERT INTO followers (follower_id, following_id, ...) WHERE follower_id = following_id -- should fail
```

### Phase 2
```bash
cd backend
go build ./...
golangci-lint run ./...
go test -race ./...
go test -coverprofile=coverage.out ./internal/service/...
go tool cover -func=coverage.out | grep total  # must be >= 80%
```

### Phase 3
```bash
cd frontend
npm run lint    # zero errors required
npm run build   # must succeed
```

### Phase 4
```bash
cd backend
go build ./...
golangci-lint run ./...
go test -race ./internal/events/...
# Manual integration:
# docker compose up nsqlookupd nsqd
# go run ./cmd/server
# POST /api/posts (create a post)
# Verify "feed fanout received" in server logs
```

---

## Overall Success Criteria

1. A user can compose and post a 128-character message
2. HTML in post content is stripped before storage — no XSS possible
3. A user can follow/unfollow other users
4. The feed shows posts from followed users + own posts, newest first, all historical
5. Posts are private to followers — search and profile post lists filter by follow relationship
6. `GET /api/posts/{id}` is accessible to any authenticated user (shared links work)
7. Cursor-based pagination works correctly (no duplicates, no gaps on page boundaries)
8. Like toggle is idempotent and count is always accurate
9. FTS5 search returns relevant posts by content (follower-filtered)
10. All endpoints return 401 without a valid session cookie
11. Rate limits enforced: 10 posts/min, 60 likes/min, 30 follows/min
12. NSQ events published for post creation, likes, and follows
13. NSQ consumer framework is general-purpose — future consumers implement `MessageHandler` and register in `main.go`
14. Frontend Social section renders below the dashboard grid with compose, feed, search, and discover
15. Backend service test coverage ≥ 80%
16. All quality gates pass: `go build`, `golangci-lint`, `go test -race`, `npm run lint`, `npm run build`
