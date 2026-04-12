# Social Notification Consumers — Design Spec

**Issue:** argus-v1r  
**Date:** 2026-04-12  
**Status:** Approved

---

## Summary

Phase 5 adds two NSQ consumers that deliver social notifications to users when someone posts or follows them. The work also expands the notification service with a `CreateForUser` convenience method, adds a per-user mute/opt-out preference store, and exposes a settings API.

Per project convention, **DB migrations ship in an isolated PR** (PR 1) that must merge before the app code PR (PR 2).

---

## PR 1 — DB Migrations (isolated)

Three sequential migration files:

### `021_seed_social_provider_type.sql`
Seeds `social` into the `provider_types` lookup table.
```sql
-- +goose Up
INSERT OR IGNORE INTO provider_types (id, label, sort_order) VALUES ('social', 'Social', 10);

-- +goose Down
DELETE FROM provider_types WHERE id = 'social';
```

### `022_seed_social_event_types.sql`
Seeds the two new event types into `notification_event_types`.
```sql
-- +goose Up
INSERT OR IGNORE INTO notification_event_types (id, label, sort_order) VALUES
    ('social.post.created', 'New Post',      10),
    ('social.new_follower', 'New Follower',  11);

-- +goose Down
DELETE FROM notification_event_types WHERE id IN ('social.post.created', 'social.new_follower');
```

### `023_add_reference_id_to_notifications.sql`
Adds a nullable `reference_id` column to `notifications` for deduplication by social consumers. Uses `ALTER TABLE ADD COLUMN` (safe for nullable columns in SQLite).
```sql
-- +goose Up
ALTER TABLE notifications ADD COLUMN reference_id TEXT;

-- Partial unique index: prevents duplicate social notifications on consumer retry.
-- NULL reference_id values are excluded (GitHub notifications don't use this field).
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_reference
    ON notifications (user_id, event_type_id, reference_id)
    WHERE reference_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_notifications_reference;
-- SQLite does not support DROP COLUMN before 3.35.0; leave column in place on rollback.
```

### `024_create_social_notification_prefs.sql`
Per-user opt-out table. Uses `user_id` as PK — this is a 1:1 user settings table, not a domain entity, so a UUID v7 surrogate key is not appropriate here.
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS social_notification_prefs (
    user_id      TEXT PRIMARY KEY REFERENCES users(id),
    mute_posts   INTEGER NOT NULL DEFAULT 0,
    mute_follows INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS social_notification_prefs;
```

---

## PR 2 — App Code

### 1. Event Payload Changes (`internal/events/payloads.go`)

**New constant:**
```go
TopicFollowCreated = "follow.created"
```

**Expand `PostCreatedPayload`** — add author name and content preview so the `FollowerNotificationConsumer` requires no DB lookup:
```go
type PostCreatedPayload struct {
    PostID         string `json:"postId"`
    UserID         string `json:"userId"`
    AuthorName     string `json:"authorName"`
    ContentPreview string `json:"contentPreview"` // first 100 chars of content
}
```
`PostsService.Create()` truncates content to 100 chars and populates both fields at publish time. `FeedFanoutConsumer` and `FollowBackfillConsumer` ignore the new fields (Go JSON unmarshal is additive).

**New payload type:**
```go
type FollowCreatedPayload struct {
    FollowerID   string `json:"followerId"`
    FollowingID  string `json:"followingId"`
    FollowerName string `json:"followerName"`
}
```
`UserFollowedPayload` is left unchanged (still used by `FollowBackfillConsumer`).

---

### 2. FollowService Signature Change

`FollowService.Follow()` gains a `followerName string` parameter:
```go
func (s *FollowService) Follow(ctx context.Context, followerID, followingID, followerName string) error
```

After the existing `user.followed` publish, add a second publish:
```go
s.publisher.PublishEvent(events.TopicFollowCreated, events.FollowCreatedPayload{
    FollowerID:   followerID,
    FollowingID:  followingID,
    FollowerName: followerName,
})
```

`follow.go` handler extracts the name from the session (`sess.Name`) and passes it to `Follow()`. A helper `sessionFromRequest(r)` is introduced (or the existing pattern extended) to expose the full `session.Data`.

---

### 3. NotificationService Expansion

New method on `NotificationService`:
```go
func (s *NotificationService) CreateForUser(
    ctx context.Context,
    userID, providerID, eventTypeID, title string,
    body, url, referenceID *string,
) error
```
Generates a UUID v7 internally, builds `model.NotificationCreate` (including `ReferenceID`), and delegates to `s.store.Create()`. Returns error only (callers log and continue — a failed notification must never block the consumer's primary work).

`model.NotificationCreate` gains a `ReferenceID *string` field. The `NotificationRepository.Create()` implementation passes it through to the `INSERT` statement.

---

### 4. Social Prefs Layer

**Model** (`internal/model/notification.go`):
```go
type SocialNotificationPrefs struct {
    UserID      string
    MutePosts   bool
    MuteFollows bool
    CreatedAt   string
    UpdatedAt   string
}
```

**Store interface** (in `internal/service/social_prefs.go`):
```go
type SocialNotificationPrefsStore interface {
    GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
    UpsertPrefs(ctx context.Context, prefs model.SocialNotificationPrefs) error
}
```
`GetPrefs` returns a zero-value struct with `MutePosts=false, MuteFollows=false` when no row exists — not an error.

**Service** (`internal/service/social_prefs.go`):
- `GetPrefs(ctx, userID)` — delegates to store
- `UpsertPrefs(ctx, prefs)` — sets `CreatedAt`/`UpdatedAt` timestamps, delegates to store

**Repository** (`internal/repository/social_prefs_repository.go`):
- SQLite `INSERT OR REPLACE` upsert on `social_notification_prefs`

---

### 5. Consumer Interfaces (in `internal/events/`)

Following the DIP pattern used by existing consumers, define two narrow interfaces in the events package:

```go
// NotificationCreator creates a social notification for a user.
type NotificationCreator interface {
    CreateForUser(ctx context.Context,
        userID, providerID, eventTypeID, title string,
        body, url, referenceID *string,
    ) error
}

// SocialPrefsReader returns a user's social notification mute preferences.
type SocialPrefsReader interface {
    GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
}
```

`NotificationService` and `SocialPrefsService` both satisfy these interfaces at wiring time.

---

### 6. FollowerNotificationConsumer

**File:** `internal/events/follower_notification.go`  
**Topic:** `post.created` | **Channel:** `follower-notifications`

Dependencies injected:
- `FanoutFollowStore` (already defined — reused)
- `NotificationCreator`
- `SocialPrefsReader`

`process(payload PostCreatedPayload)`:
1. Fetch all follower IDs via `FanoutFollowStore.GetFollowerIDs()`
2. For each follower: call `SocialPrefsReader.GetPrefs()` — skip if `MutePosts == true`
3. Call `NotificationCreator.CreateForUser()`:
   - `providerID = "social"`
   - `eventTypeID = "social.post.created"`
   - `title = "<AuthorName> posted something"`
   - `body = &payload.ContentPreview` (non-nil)
   - `url = nil`
   - `referenceID = &payload.PostID` (deduplicates per follower per post)

Duplicate suppression is handled at the DB layer via the `uq_notifications_reference` partial unique index on `(user_id, event_type_id, reference_id)`. Pass `reference_id = postID` in the `CreateForUser` call so that a re-delivered `post.created` message produces no duplicate rows.

---

### 7. FollowNotificationConsumer

**File:** `internal/events/follow_notification.go`  
**Topic:** `follow.created` | **Channel:** `follow-notifications`

Dependencies injected:
- `NotificationCreator`
- `SocialPrefsReader`

`process(payload FollowCreatedPayload)`:
1. Call `SocialPrefsReader.GetPrefs(ctx, payload.FollowingID)` — skip if `MuteFollows == true`
2. Call `NotificationCreator.CreateForUser()`:
   - `providerID = "social"`
   - `eventTypeID = "social.new_follower"`
   - `title = "<FollowerName> started following you"`
   - `body = nil`
   - `url = nil`
   - `referenceID = &payload.FollowerID` (deduplicates per follower; one notification even if they refollow)

---

### 8. Mute Prefs API

**Handler files:** `internal/handler/social_prefs.go` + `internal/handler/social_prefs_dto.go`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/settings/social-notifications` | required | Returns current prefs (defaults false/false if no row) |
| `PUT` | `/api/settings/social-notifications` | required | Upserts prefs |

DTOs:
```go
// Request (PUT)
type UpdateSocialPrefsRequest struct {
    MutePosts   bool `json:"mutePosts"`
    MuteFollows bool `json:"muteFollows"`
}

// Response (GET)
type SocialPrefsResponse struct {
    MutePosts   bool `json:"mutePosts"`
    MuteFollows bool `json:"muteFollows"`
}
```

`PUT` returns `204 No Content`. Rate-limited with `httprate.LimitByIP` on mutation endpoint. Both routes are under `requireAuth`.

---

### 9. Server Wiring (`internal/server/server.go`)

In `setupRoutes()`:
```go
socialPrefsRepo := repository.NewSQLiteSocialPrefsRepository(s.db)
socialPrefsSvc := service.NewSocialPrefsService(socialPrefsRepo)
```

In the NSQ consumer block, add:
```go
events.NewFollowerNotificationConsumer(followRepo, notificationSvc, socialPrefsSvc),
events.NewFollowNotificationConsumer(notificationSvc, socialPrefsSvc),
```

Register the settings handler:
```go
socialPrefsH := handler.NewSocialPrefsHandler(socialPrefsSvc, v)
socialPrefsH.AddRoutes(protected)
```

---

### 10. Testing

**Consumer tests** (table-driven, `process()` unit tests with fakes — no NSQ):

For both consumers:
| Scenario | Expected |
|----------|----------|
| Valid payload, user not muted | `CreateForUser` called with correct fields |
| Valid payload, user muted | `CreateForUser` not called |
| Malformed JSON envelope | `process()` returns error |
| `GetFollowerIDs` returns empty list (FollowerNotification only) | No notifications created |

**Settings handler tests** (`internal/handler/social_prefs_test.go`):
| Scenario | Expected |
|----------|----------|
| `GET` — no prefs row | `200` with `{mutePosts: false, muteFollows: false}` |
| `GET` — prefs exist | `200` with stored values |
| `PUT` valid body | `204` |
| `PUT` invalid JSON | `400` |
| `GET` unauthenticated | `401` |
| `PUT` unauthenticated | `401` |

---

## Key Constraints

- Notification creation failure in a consumer must be **logged and swallowed** — never propagate to the retry loop (a failed notification is not worth retrying the entire message).
- Consumers must use a `30s` context timeout (`context.WithTimeout`) matching the existing consumer pattern.
- `PostsService` must truncate content preview to exactly 100 UTF-8 characters (use `[]rune` slicing, not byte slicing).
