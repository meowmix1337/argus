# Social Notification Consumers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two NSQ consumers that deliver social notifications to followers (new post, new follower), a per-user mute preference store, and a settings API — all gated behind DB migrations that ship in an isolated PR first.

**Architecture:** PR 1 ships 4 migration files only (no app code). PR 2 ships all app code against the merged schema. Consumers define narrow interfaces (DIP) in the `events` package; services implement them. `NotificationService.CreateForUser()` handles all social notification inserts; duplicate suppression uses a new `reference_id` column + partial unique index.

**Tech Stack:** Go, SQLite (sqlx), NSQ (go-nsq), chi, httprate, go-playground/validator, google/uuid

**Spec:** `docs/superpowers/specs/2026-04-12-social-notification-consumers-design.md`

---

## PR 1 — DB Migrations

### Task 1: Create migration files

**Files:**
- Create: `backend/migrations/021_seed_social_provider_type.sql`
- Create: `backend/migrations/022_seed_social_event_types.sql`
- Create: `backend/migrations/023_add_reference_id_to_notifications.sql`
- Create: `backend/migrations/024_create_social_notification_prefs.sql`

- [ ] **Step 1: Create `021_seed_social_provider_type.sql`**

```sql
-- +goose Up
INSERT OR IGNORE INTO provider_types (id, label, sort_order) VALUES ('social', 'Social', 10);

-- +goose Down
DELETE FROM provider_types WHERE id = 'social';
```

- [ ] **Step 2: Create `022_seed_social_event_types.sql`**

```sql
-- +goose Up
INSERT OR IGNORE INTO notification_event_types (id, label, sort_order) VALUES
    ('social.post.created', 'New Post',     10),
    ('social.new_follower', 'New Follower', 11);

-- +goose Down
DELETE FROM notification_event_types WHERE id IN ('social.post.created', 'social.new_follower');
```

- [ ] **Step 3: Create `023_add_reference_id_to_notifications.sql`**

```sql
-- +goose Up
ALTER TABLE notifications ADD COLUMN reference_id TEXT;

-- Partial unique index: prevents duplicate social notifications on consumer retry.
-- NULL values are excluded (GitHub notifications don't set reference_id).
CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_reference
    ON notifications (user_id, event_type_id, reference_id)
    WHERE reference_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_notifications_reference;
-- SQLite does not support DROP COLUMN before 3.35.0; leave column in place on rollback.
```

- [ ] **Step 4: Create `024_create_social_notification_prefs.sql`**

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

- [ ] **Step 5: Verify migrations apply and roll back cleanly**

Run from `backend/`:
```bash
go run ./cmd/server
```
Expected: server starts without errors (goose runs on startup). Then confirm rollback with goose manually if needed.

- [ ] **Step 6: Commit migrations**

```bash
git add backend/migrations/021_seed_social_provider_type.sql
git add backend/migrations/022_seed_social_event_types.sql
git add backend/migrations/023_add_reference_id_to_notifications.sql
git add backend/migrations/024_create_social_notification_prefs.sql
git commit -m "feat: add social provider type, event types, reference_id, and notification prefs migrations"
```

- [ ] **Step 7: Open PR 1 targeting `main`**

```bash
git push -u origin HEAD
gh pr create --title "feat: social notification DB migrations" \
  --body "Isolated migration PR for social notifications (argus-v1r). Must merge before app code PR." \
  --base main
```

---

## PR 2 — App Code

> Start PR 2 only after PR 1 is merged. Create branch `feat/social-notifications-argus-v1r` from latest main.

### Task 2: Expand event payloads

**Files:**
- Modify: `backend/internal/events/payloads.go`

- [ ] **Step 1: Add `TopicFollowCreated`, expand `PostCreatedPayload`, add `FollowCreatedPayload`**

Replace the entire `payloads.go` content:

```go
package events

import "time"

// Event topics — stable strings, never rename in production.
const (
	TopicPostCreated  = "post.created"
	TopicPostLiked    = "post.liked"
	TopicUserFollowed = "user.followed"
	TopicFollowCreated = "follow.created"
)

// EventEnvelope wraps every published message with version, type, and timestamp.
type EventEnvelope struct {
	Version   int    `json:"v"`
	Type      string `json:"type"`
	Timestamp string `json:"ts"`
	Payload   any    `json:"payload"`
}

// NewEnvelope creates an EventEnvelope with version 1 and the current UTC timestamp.
func NewEnvelope(eventType string, payload any) EventEnvelope {
	return EventEnvelope{
		Version:   1,
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
}

// PostCreatedPayload is the payload for TopicPostCreated events.
type PostCreatedPayload struct {
	PostID         string `json:"postId"`
	UserID         string `json:"userId"`
	AuthorName     string `json:"authorName"`
	ContentPreview string `json:"contentPreview"` // first 100 runes of content
}

// PostLikedPayload is the payload for TopicPostLiked events.
type PostLikedPayload struct {
	PostID  string `json:"postId"`
	LikerID string `json:"likerId"`
	OwnerID string `json:"ownerId"`
}

// UserFollowedPayload is the payload for TopicUserFollowed events.
type UserFollowedPayload struct {
	FollowerID  string `json:"followerId"`
	FollowingID string `json:"followingId"`
}

// FollowCreatedPayload is the payload for TopicFollowCreated events.
type FollowCreatedPayload struct {
	FollowerID   string `json:"followerId"`
	FollowingID  string `json:"followingId"`
	FollowerName string `json:"followerName"`
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd backend && go build ./...
```
Expected: no errors. `FeedFanoutConsumer` and `FollowBackfillConsumer` still compile — they unmarshal into `PostCreatedPayload` and `UserFollowedPayload` respectively; new fields are silently ignored.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/events/payloads.go
git commit -m "feat: expand PostCreatedPayload with author/preview, add FollowCreatedPayload"
```

---

### Task 3: Update PostsService to populate new payload fields

**Files:**
- Modify: `backend/internal/service/posts.go`
- Modify: `backend/internal/service/posts_test.go`

- [ ] **Step 1: Update the publish call in `posts.go`**

Find the `PublishEvent` call near line 73 and replace it:

```go
	// Truncate content preview to 100 runes (UTF-8 safe).
	preview := []rune(content)
	if len(preview) > 100 {
		preview = preview[:100]
	}

	if pubErr := s.publisher.PublishEvent(events.TopicPostCreated, events.PostCreatedPayload{
		PostID:         post.ID,
		UserID:         post.UserID,
		AuthorName:     post.UserName,
		ContentPreview: string(preview),
	}); pubErr != nil {
		slog.Error("failed to publish post.created event", "error", pubErr, "post_id", post.ID)
	}
```

- [ ] **Step 2: Update tests that assert on published PostCreatedPayload fields**

Open `backend/internal/service/posts_test.go`. Find any test that checks `pub.events[0]` payload fields and update the payload struct initialization to include `AuthorName` and `ContentPreview` where relevant. If `fakePublisher` captures `payload any`, the existing tests likely only check the topic, not payload fields — confirm they still pass.

- [ ] **Step 3: Run tests**

```bash
cd backend && go test -race ./internal/service/...
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service/posts.go backend/internal/service/posts_test.go
git commit -m "feat: populate AuthorName and ContentPreview in post.created payload"
```

---

### Task 4: Update FollowService signature + tests

**Files:**
- Modify: `backend/internal/service/follow.go`
- Modify: `backend/internal/service/follow_test.go`

- [ ] **Step 1: Update `Follow()` signature in `follow.go`**

Change the `Follow` method signature and add a second publish call:

```go
// Follow creates a follow relationship. Returns error if self-follow or already following.
func (s *FollowService) Follow(ctx context.Context, followerID, followingID, followerName string) error {
	if followerID == followingID {
		return apperrors.ErrSelfFollow
	}

	already, err := s.store.IsFollowing(ctx, followerID, followingID)
	if err != nil {
		return fmt.Errorf("check follow: %w", err)
	}
	if already {
		return apperrors.ErrAlreadyFollowing
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate follow id: %w", err)
	}

	if err := s.store.Follow(ctx, id.String(), followerID, followingID); err != nil {
		if errors.Is(err, apperrors.ErrAlreadyFollowing) {
			return apperrors.ErrAlreadyFollowing
		}
		return fmt.Errorf("follow: %w", err)
	}

	if pubErr := s.publisher.PublishEvent(events.TopicUserFollowed, events.UserFollowedPayload{
		FollowerID:  followerID,
		FollowingID: followingID,
	}); pubErr != nil {
		slog.Error("failed to publish user.followed event", "error", pubErr)
	}

	if pubErr := s.publisher.PublishEvent(events.TopicFollowCreated, events.FollowCreatedPayload{
		FollowerID:   followerID,
		FollowingID:  followingID,
		FollowerName: followerName,
	}); pubErr != nil {
		slog.Error("failed to publish follow.created event", "error", pubErr)
	}

	return nil
}
```

- [ ] **Step 2: Update all `Follow()` call sites in `follow_test.go`**

Add `"UserOne"` (or any name string) as the third argument to every `svc.Follow(...)` call:

```go
// TestFollowService_Follow_Success
err := svc.Follow(context.Background(), "user1", "user2", "User One")

// TestFollowService_Follow_SelfFollow
err := svc.Follow(context.Background(), "user1", "user1", "User One")

// TestFollowService_Follow_AlreadyFollowing
err := svc.Follow(context.Background(), "user1", "user2", "User One")

// TestFollowService_Follow_IsFollowingError_Propagates
err := svc.Follow(context.Background(), "user1", "user2", "User One")

// TestFollowService_Follow_StoreError_Propagates
err := svc.Follow(context.Background(), "user1", "user2", "User One")
```

- [ ] **Step 3: Update the success test to assert 2 published events**

`TestFollowService_Follow_Success` currently asserts `len(pub.events) == 1` and checks `user.followed`. Update it:

```go
func TestFollowService_Follow_Success(t *testing.T) {
	store := newFakeFollowStore()
	pub := &fakePublisher{}
	svc := NewFollowService(store, pub)

	err := svc.Follow(context.Background(), "user1", "user2", "User One")
	if err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if !store.follows["user1"]["user2"] {
		t.Error("expected follow relationship to be stored")
	}
	if len(pub.events) != 2 {
		t.Fatalf("expected 2 published events, got %d", len(pub.events))
	}
	topics := map[string]bool{}
	for _, e := range pub.events {
		topics[e.topic] = true
	}
	if !topics["user.followed"] {
		t.Error("expected user.followed event to be published")
	}
	if !topics["follow.created"] {
		t.Error("expected follow.created event to be published")
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -race ./internal/service/...
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/follow.go backend/internal/service/follow_test.go
git commit -m "feat: add followerName param to FollowService.Follow, publish follow.created event"
```

---

### Task 5: Update follow handler to pass followerName

**Files:**
- Modify: `backend/internal/handler/tasks.go`
- Modify: `backend/internal/handler/follow.go`

- [ ] **Step 1: Add `sessionFromRequest` helper to `tasks.go`**

Add the following import to `tasks.go`'s import block:
```go
"github.com/meowmix1337/argus/backend/internal/session"
```

Add the new helper function at the bottom of `tasks.go` (after `userIDFromRequest`):

```go
// sessionFromRequest extracts the full session data from the request context.
func sessionFromRequest(r *http.Request) (session.Data, bool) {
	return middleware.SessionFromContext(r.Context())
}
```

- [ ] **Step 2: Update the `Follow` method in `follow.go`**

Replace the `Follow` handler method:

```go
func (h *FollowHandler) Follow(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req FollowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Follow(r.Context(), sess.UserID, req.FollowingID, sess.Name); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrSelfFollow):
			response.WriteError(w, http.StatusBadRequest, "cannot follow yourself")
		case errors.Is(err, apperrors.ErrAlreadyFollowing):
			response.WriteError(w, http.StatusConflict, "already following this user")
		default:
			slog.Error("failed to follow", "error", err, "follower_id", sess.UserID, "following_id", req.FollowingID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Build + run tests**

```bash
cd backend && go build ./...
cd backend && go test -race ./internal/handler/...
```
Expected: PASS. (Follow handler tests set up session context — they should still work since `sessionFromRequest` is a thin wrapper over the same middleware call.)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handler/tasks.go backend/internal/handler/follow.go
git commit -m "feat: extract follower name from session in Follow handler"
```

---

### Task 6: Add reference_id to notification model + repository

**Files:**
- Modify: `backend/internal/model/notification.go`
- Modify: `backend/internal/repository/sqlite_notification_repository.go`

- [ ] **Step 1: Add `ReferenceID` to `model.Notification` and `model.NotificationCreate`**

In `backend/internal/model/notification.go`, update the two structs:

```go
// Notification is a single inbox notification for a user.
type Notification struct {
	ID               string  `db:"id"`
	UserID           string  `db:"user_id"`
	ProviderID       string  `db:"provider_id"`   // FK to provider_types
	EventTypeID      string  `db:"event_type_id"` // FK to notification_event_types
	Title            string  `db:"title"`
	Body             *string `db:"body"`
	URL              *string `db:"url"`
	ReferenceID      *string `db:"reference_id"`      // deduplication key for social events
	ReadAt           *string `db:"read_at"`
	DismissedAt      *string `db:"dismissed_at"`
	GitHubDeliveryID *string `db:"github_delivery_id"`
	CreatedAt        string  `db:"created_at"`
	UpdatedAt        string  `db:"updated_at"`
	DeletedAt        *string `db:"deleted_at"`
}

// NotificationCreate holds the fields for inserting a new notification.
type NotificationCreate struct {
	ID               string
	UserID           string
	ProviderID       string
	EventTypeID      string
	Title            string
	Body             *string
	URL              *string
	ReferenceID      *string // set for social events; nil for GitHub webhook events
	GitHubDeliveryID *string
}
```

- [ ] **Step 2: Update `sqlite_notification_repository.go`**

Update `sqliteNotificationRow` to add `ReferenceID`:

```go
type sqliteNotificationRow struct {
	ID               string         `db:"id"`
	UserID           string         `db:"user_id"`
	ProviderID       string         `db:"provider_id"`
	EventTypeID      string         `db:"event_type_id"`
	Title            string         `db:"title"`
	Body             sql.NullString `db:"body"`
	URL              sql.NullString `db:"url"`
	ReferenceID      sql.NullString `db:"reference_id"`
	ReadAt           sql.NullString `db:"read_at"`
	DismissedAt      sql.NullString `db:"dismissed_at"`
	GitHubDeliveryID sql.NullString `db:"github_delivery_id"`
	CreatedAt        string         `db:"created_at"`
	UpdatedAt        string         `db:"updated_at"`
	DeletedAt        sql.NullString `db:"deleted_at"`
}
```

Update `notificationColumns` constant:

```go
const notificationColumns = `id, user_id, provider_id, event_type_id, title, body, url,
	reference_id, read_at, dismissed_at, github_delivery_id, created_at, updated_at, deleted_at`
```

Update `toModel()` to map the new field:

```go
func (r *sqliteNotificationRow) toModel() model.Notification {
	m := model.Notification{
		ID:          r.ID,
		UserID:      r.UserID,
		ProviderID:  r.ProviderID,
		EventTypeID: r.EventTypeID,
		Title:       r.Title,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if r.Body.Valid {
		m.Body = &r.Body.String
	}
	if r.URL.Valid {
		m.URL = &r.URL.String
	}
	if r.ReferenceID.Valid {
		m.ReferenceID = &r.ReferenceID.String
	}
	if r.ReadAt.Valid {
		m.ReadAt = &r.ReadAt.String
	}
	if r.DismissedAt.Valid {
		m.DismissedAt = &r.DismissedAt.String
	}
	if r.GitHubDeliveryID.Valid {
		m.GitHubDeliveryID = &r.GitHubDeliveryID.String
	}
	if r.DeletedAt.Valid {
		m.DeletedAt = &r.DeletedAt.String
	}
	return m
}
```

Update the `Create()` INSERT statement to include `reference_id`:

```go
func (r *SQLiteNotificationRepository) Create(ctx context.Context, n model.NotificationCreate) (model.Notification, error) {
	now := time.Now().UTC().Format(timeFormat)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications
		 (id, user_id, provider_id, event_type_id, title, body, url, reference_id, github_delivery_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.UserID, n.ProviderID, n.EventTypeID, n.Title, n.Body, n.URL, n.ReferenceID, n.GitHubDeliveryID, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return model.Notification{}, apperrors.ErrDuplicateDelivery
		}
		return model.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return r.GetByID(ctx, n.ID, n.UserID)
}
```

- [ ] **Step 3: Build**

```bash
cd backend && go build ./...
```
Expected: PASS.

- [ ] **Step 4: Run existing notification tests**

```bash
cd backend && go test -race ./internal/service/... ./internal/repository/...
```
Expected: PASS. (The fake store in service tests receives `model.NotificationCreate` with nil `ReferenceID` — this is valid since the field is a pointer.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/model/notification.go \
        backend/internal/repository/sqlite_notification_repository.go
git commit -m "feat: add reference_id to Notification model and repository for social dedup"
```

---

### Task 7: Add CreateForUser() to NotificationService

**Files:**
- Modify: `backend/internal/service/notification.go`
- Modify: `backend/internal/service/notification_test.go`

- [ ] **Step 1: Write the failing test**

Add to `backend/internal/service/notification_test.go`:

```go
func TestNotificationService_CreateForUser_Success(t *testing.T) {
	store := &fakeNotificationStore{}
	svc := NewNotificationService(store)

	body := "first 100 chars"
	err := svc.CreateForUser(context.Background(),
		"user-1", "social", "social.post.created", "Alice posted something",
		&body, nil, strPtr("post-abc"),
	)
	if err != nil {
		t.Fatalf("CreateForUser: %v", err)
	}
	if len(store.notifications) != 1 {
		t.Fatalf("expected 1 notification stored, got %d", len(store.notifications))
	}
	if store.notifications[0].Title != "Alice posted something" {
		t.Errorf("title = %q, want %q", store.notifications[0].Title, "Alice posted something")
	}
}

func TestNotificationService_CreateForUser_DuplicateSwallowed(t *testing.T) {
	store := &fakeNotificationStore{createErr: apperrors.ErrDuplicateDelivery}
	svc := NewNotificationService(store)

	err := svc.CreateForUser(context.Background(),
		"user-1", "social", "social.post.created", "Alice posted something",
		nil, nil, strPtr("post-abc"),
	)
	if err != nil {
		t.Errorf("expected duplicate to be swallowed, got error: %v", err)
	}
}

func TestNotificationService_CreateForUser_StoreError_Propagates(t *testing.T) {
	store := &fakeNotificationStore{createErr: errors.New("db error")}
	svc := NewNotificationService(store)

	err := svc.CreateForUser(context.Background(),
		"user-1", "social", "social.post.created", "title",
		nil, nil, nil,
	)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// strPtr is a test helper that returns a pointer to the given string.
func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run to confirm it fails**

```bash
cd backend && go test -race ./internal/service/... -run TestNotificationService_CreateForUser
```
Expected: FAIL — `CreateForUser undefined`.

- [ ] **Step 3: Implement `CreateForUser` in `notification.go`**

Add the following imports to `notification.go` (add `errors` and `uuid`):
```go
import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)
```

Add the method:

```go
// CreateForUser creates a social notification for the given user.
// Duplicate deliveries (same user + event type + reference_id) are silently ignored.
func (s *NotificationService) CreateForUser(
	ctx context.Context,
	userID, providerID, eventTypeID, title string,
	body, url, referenceID *string,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate notification id: %w", err)
	}
	_, err = s.store.Create(ctx, model.NotificationCreate{
		ID:          id.String(),
		UserID:      userID,
		ProviderID:  providerID,
		EventTypeID: eventTypeID,
		Title:       title,
		Body:        body,
		URL:         url,
		ReferenceID: referenceID,
	})
	if err != nil {
		if errors.Is(err, apperrors.ErrDuplicateDelivery) {
			return nil // idempotent — duplicate suppressed
		}
		return fmt.Errorf("create notification for user: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -race ./internal/service/... -run TestNotificationService_CreateForUser
```
Expected: all 3 PASS.

- [ ] **Step 5: Run full service tests**

```bash
cd backend && go test -race ./internal/service/...
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/notification.go backend/internal/service/notification_test.go
git commit -m "feat: add CreateForUser to NotificationService with duplicate suppression"
```

---

### Task 8: Social prefs model + service + tests

**Files:**
- Modify: `backend/internal/model/notification.go`
- Create: `backend/internal/service/social_prefs.go`
- Create: `backend/internal/service/social_prefs_test.go`

- [ ] **Step 1: Add `SocialNotificationPrefs` to the model**

Append to `backend/internal/model/notification.go`:

```go
// SocialNotificationPrefs holds a user's social notification mute preferences.
type SocialNotificationPrefs struct {
	UserID      string
	MutePosts   bool
	MuteFollows bool
	CreatedAt   string
	UpdatedAt   string
}
```

- [ ] **Step 2: Write failing tests for `SocialPrefsService`**

Create `backend/internal/service/social_prefs_test.go`:

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

type fakeSocialPrefsStore struct {
	prefs     model.SocialNotificationPrefs
	getErr    error
	upsertErr error
	upserted  *model.SocialNotificationPrefs
}

func (f *fakeSocialPrefsStore) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.getErr != nil {
		return model.SocialNotificationPrefs{}, f.getErr
	}
	if f.prefs.UserID == "" {
		return model.SocialNotificationPrefs{UserID: userID}, nil // default
	}
	return f.prefs, nil
}

func (f *fakeSocialPrefsStore) UpsertPrefs(_ context.Context, prefs model.SocialNotificationPrefs) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.upserted = &prefs
	return nil
}

func TestSocialPrefsService_GetPrefs_DefaultsWhenNoRow(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{})
	prefs, err := svc.GetPrefs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetPrefs: %v", err)
	}
	if prefs.MutePosts || prefs.MuteFollows {
		t.Errorf("expected default false prefs, got mutePosts=%v muteFollows=%v", prefs.MutePosts, prefs.MuteFollows)
	}
}

func TestSocialPrefsService_GetPrefs_ReturnsStored(t *testing.T) {
	stored := model.SocialNotificationPrefs{UserID: "user-1", MutePosts: true, MuteFollows: false}
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{prefs: stored})
	prefs, err := svc.GetPrefs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetPrefs: %v", err)
	}
	if !prefs.MutePosts {
		t.Error("expected MutePosts=true")
	}
}

func TestSocialPrefsService_GetPrefs_StoreError_Propagates(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{getErr: errors.New("db error")})
	_, err := svc.GetPrefs(context.Background(), "user-1")
	if err == nil {
		t.Error("expected error to propagate")
	}
}

func TestSocialPrefsService_UpsertPrefs_Success(t *testing.T) {
	store := &fakeSocialPrefsStore{}
	svc := NewSocialPrefsService(store)
	err := svc.UpsertPrefs(context.Background(), "user-1", true, false)
	if err != nil {
		t.Fatalf("UpsertPrefs: %v", err)
	}
	if store.upserted == nil {
		t.Fatal("expected prefs to be upserted")
	}
	if !store.upserted.MutePosts {
		t.Error("expected MutePosts=true in upserted prefs")
	}
	if store.upserted.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", store.upserted.UserID)
	}
}

func TestSocialPrefsService_UpsertPrefs_StoreError_Propagates(t *testing.T) {
	svc := NewSocialPrefsService(&fakeSocialPrefsStore{upsertErr: errors.New("db error")})
	err := svc.UpsertPrefs(context.Background(), "user-1", false, true)
	if err == nil {
		t.Error("expected error to propagate")
	}
}
```

- [ ] **Step 3: Run to confirm failure**

```bash
cd backend && go test -race ./internal/service/... -run TestSocialPrefsService
```
Expected: FAIL — `NewSocialPrefsService undefined`.

- [ ] **Step 4: Implement `social_prefs.go`**

Create `backend/internal/service/social_prefs.go`:

```go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// SocialNotificationPrefsStore defines the data-access contract for social notification prefs.
type SocialNotificationPrefsStore interface {
	GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
	UpsertPrefs(ctx context.Context, prefs model.SocialNotificationPrefs) error
}

// SocialPrefsService manages social notification mute preferences.
type SocialPrefsService struct {
	store SocialNotificationPrefsStore
}

// NewSocialPrefsService creates a new SocialPrefsService.
func NewSocialPrefsService(store SocialNotificationPrefsStore) *SocialPrefsService {
	return &SocialPrefsService{store: store}
}

// GetPrefs returns the social notification prefs for the given user.
// Returns default (all false) prefs when no row exists — not an error.
func (s *SocialPrefsService) GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error) {
	prefs, err := s.store.GetPrefs(ctx, userID)
	if err != nil {
		return model.SocialNotificationPrefs{}, fmt.Errorf("get social prefs: %w", err)
	}
	return prefs, nil
}

// UpsertPrefs creates or updates the social notification prefs for the given user.
func (s *SocialPrefsService) UpsertPrefs(ctx context.Context, userID string, mutePosts, muteFollows bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.store.UpsertPrefs(ctx, model.SocialNotificationPrefs{
		UserID:      userID,
		MutePosts:   mutePosts,
		MuteFollows: muteFollows,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		return fmt.Errorf("upsert social prefs: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test -race ./internal/service/... -run TestSocialPrefsService
```
Expected: all 5 PASS.

- [ ] **Step 6: Run full service suite**

```bash
cd backend && go test -race ./internal/service/...
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/model/notification.go \
        backend/internal/service/social_prefs.go \
        backend/internal/service/social_prefs_test.go
git commit -m "feat: add SocialPrefsService with GetPrefs and UpsertPrefs"
```

---

### Task 9: SQLite social prefs repository

**Files:**
- Create: `backend/internal/repository/sqlite_social_prefs_repository.go`

- [ ] **Step 1: Create the repository**

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/model"
)

type sqliteSocialPrefsRow struct {
	UserID      string `db:"user_id"`
	MutePosts   bool   `db:"mute_posts"`
	MuteFollows bool   `db:"mute_follows"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

// SQLiteSocialPrefsRepository implements SocialNotificationPrefsStore backed by SQLite.
type SQLiteSocialPrefsRepository struct {
	db *sqlx.DB
}

// NewSQLiteSocialPrefsRepository creates a new SQLiteSocialPrefsRepository.
func NewSQLiteSocialPrefsRepository(db *sqlx.DB) *SQLiteSocialPrefsRepository {
	return &SQLiteSocialPrefsRepository{db: db}
}

// GetPrefs returns the social notification prefs for the given user.
// Returns zero-value prefs (all false) when no row exists — not an error.
func (r *SQLiteSocialPrefsRepository) GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error) {
	var row sqliteSocialPrefsRow
	err := r.db.GetContext(ctx, &row,
		`SELECT user_id, mute_posts, mute_follows, created_at, updated_at
		 FROM social_notification_prefs WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SocialNotificationPrefs{UserID: userID}, nil
		}
		return model.SocialNotificationPrefs{}, fmt.Errorf("get social prefs: %w", err)
	}
	return model.SocialNotificationPrefs{
		UserID:      row.UserID,
		MutePosts:   row.MutePosts,
		MuteFollows: row.MuteFollows,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// UpsertPrefs creates or updates the social notification prefs for the given user.
// Uses INSERT ... ON CONFLICT to preserve created_at on updates.
func (r *SQLiteSocialPrefsRepository) UpsertPrefs(ctx context.Context, prefs model.SocialNotificationPrefs) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO social_notification_prefs (user_id, mute_posts, mute_follows, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		     mute_posts   = excluded.mute_posts,
		     mute_follows = excluded.mute_follows,
		     updated_at   = excluded.updated_at`,
		prefs.UserID, prefs.MutePosts, prefs.MuteFollows, prefs.CreatedAt, prefs.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert social prefs: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Build**

```bash
cd backend && go build ./...
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/sqlite_social_prefs_repository.go
git commit -m "feat: add SQLiteSocialPrefsRepository for social notification prefs"
```

---

### Task 10: Consumer interfaces + FollowerNotificationConsumer

**Files:**
- Create: `backend/internal/events/social.go`
- Create: `backend/internal/events/follower_notification.go`
- Create: `backend/internal/events/follower_notification_test.go`

- [ ] **Step 1: Create consumer interfaces in `events/social.go`**

```go
package events

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// NotificationCreator creates a social notification for a user.
// Implemented by service.NotificationService.
type NotificationCreator interface {
	CreateForUser(ctx context.Context,
		userID, providerID, eventTypeID, title string,
		body, url, referenceID *string,
	) error
}

// SocialPrefsReader returns a user's social notification mute preferences.
// Implemented by service.SocialPrefsService.
type SocialPrefsReader interface {
	GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
}
```

- [ ] **Step 2: Write failing tests for `FollowerNotificationConsumer`**

Create `backend/internal/events/follower_notification_test.go`:

```go
package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeNotifCreator records CreateForUser calls for test assertions.
type fakeNotifCreator struct {
	calls []notifCall
	err   error
}

type notifCall struct {
	userID      string
	eventTypeID string
	title       string
	referenceID *string
}

func (f *fakeNotifCreator) CreateForUser(_ context.Context,
	userID, _, eventTypeID, title string,
	_, _, referenceID *string,
) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, notifCall{
		userID:      userID,
		eventTypeID: eventTypeID,
		title:       title,
		referenceID: referenceID,
	})
	return nil
}

// fakePrefsReader returns configurable prefs per user ID.
type fakePrefsReader struct {
	prefs map[string]model.SocialNotificationPrefs
	err   error
}

func (f *fakePrefsReader) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.err != nil {
		return model.SocialNotificationPrefs{}, f.err
	}
	if p, ok := f.prefs[userID]; ok {
		return p, nil
	}
	return model.SocialNotificationPrefs{UserID: userID}, nil
}

// Note: buildEnvelope is defined in feed_fanout_test.go (same package) and is reused here.

func TestFollowerNotificationConsumer_Process_ValidPayload(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{"follower-1", "follower-2"}}
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{}}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, prefs)

	body := buildEnvelope(t, PostCreatedPayload{
		PostID:         "post-1",
		UserID:         "author-1",
		AuthorName:     "Alice",
		ContentPreview: "Hello world",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 2 {
		t.Fatalf("expected 2 notifications (one per follower), got %d", len(notifier.calls))
	}
	for _, call := range notifier.calls {
		if call.title != "Alice posted something" {
			t.Errorf("title = %q, want %q", call.title, "Alice posted something")
		}
		if call.eventTypeID != "social.post.created" {
			t.Errorf("eventTypeID = %q, want social.post.created", call.eventTypeID)
		}
		if call.referenceID == nil || *call.referenceID != "post-1" {
			t.Errorf("referenceID = %v, want post-1", call.referenceID)
		}
	}
}

func TestFollowerNotificationConsumer_Process_MutedFollowerSkipped(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{"muted-user", "active-user"}}
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{
		"muted-user": {UserID: "muted-user", MutePosts: true},
	}}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, prefs)

	body := buildEnvelope(t, PostCreatedPayload{
		PostID:         "post-2",
		UserID:         "author-1",
		AuthorName:     "Bob",
		ContentPreview: "content",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification (muted user skipped), got %d", len(notifier.calls))
	}
	if notifier.calls[0].userID != "active-user" {
		t.Errorf("notification sent to %q, want active-user", notifier.calls[0].userID)
	}
}

func TestFollowerNotificationConsumer_Process_NoFollowers(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{}}
	notifier := &fakeNotifCreator{}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, &fakePrefsReader{})

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-3", UserID: "author-1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected 0 notifications for no-follower post, got %d", len(notifier.calls))
	}
}

func TestFollowerNotificationConsumer_Process_MalformedJSON(t *testing.T) {
	consumer := NewFollowerNotificationConsumer(&fakeFollowStore{}, &fakeNotifCreator{}, &fakePrefsReader{})
	err := consumer.Process([]byte(`not-json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFollowerNotificationConsumer_Process_UnknownVersion(t *testing.T) {
	env := EventEnvelope{Version: 99, Type: TopicPostCreated, Payload: map[string]string{}}
	body, _ := json.Marshal(env)
	consumer := NewFollowerNotificationConsumer(&fakeFollowStore{}, &fakeNotifCreator{}, &fakePrefsReader{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version: %v", err)
	}
}
```

- [ ] **Step 3: Run to confirm failure**

```bash
cd backend && go test -race ./internal/events/... -run TestFollowerNotification
```
Expected: FAIL — `NewFollowerNotificationConsumer undefined`.

- [ ] **Step 4: Implement `FollowerNotificationConsumer`**

Create `backend/internal/events/follower_notification.go`:

```go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// FollowerNotificationConsumer consumes post.created events and creates
// a notification for each follower of the post author.
type FollowerNotificationConsumer struct {
	followStore  FanoutFollowStore
	notifCreator NotificationCreator
	prefsReader  SocialPrefsReader
}

// NewFollowerNotificationConsumer creates a new FollowerNotificationConsumer.
func NewFollowerNotificationConsumer(
	followStore FanoutFollowStore,
	notifCreator NotificationCreator,
	prefsReader SocialPrefsReader,
) *FollowerNotificationConsumer {
	return &FollowerNotificationConsumer{
		followStore:  followStore,
		notifCreator: notifCreator,
		prefsReader:  prefsReader,
	}
}

// Topic implements MessageHandler.
func (c *FollowerNotificationConsumer) Topic() string { return TopicPostCreated }

// Channel implements MessageHandler.
func (c *FollowerNotificationConsumer) Channel() string { return "follower-notifications" }

// Process implements MessageHandler.
func (c *FollowerNotificationConsumer) Process(body []byte) error {
	var evt rawEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if evt.Version != 1 {
		slog.Warn("follower notification: unknown envelope version, skipping", "version", evt.Version)
		return nil
	}
	var payload PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal post created payload: %w", err)
	}
	return c.process(payload)
}

// process fans out notifications to each follower of the post author.
func (c *FollowerNotificationConsumer) process(payload PostCreatedPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	followerIDs, err := c.followStore.GetFollowerIDs(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("get followers for user %s: %w", payload.UserID, err)
	}
	if len(followerIDs) == 0 {
		return nil
	}

	title := payload.AuthorName + " posted something"
	body := &payload.ContentPreview
	postID := payload.PostID

	for _, followerID := range followerIDs {
		prefs, err := c.prefsReader.GetPrefs(ctx, followerID)
		if err != nil {
			slog.Warn("follower notification: failed to get prefs, skipping",
				"follower_id", followerID, "error", err)
			continue
		}
		if prefs.MutePosts {
			continue
		}
		if err := c.notifCreator.CreateForUser(ctx,
			followerID, "social", "social.post.created", title, body, nil, &postID,
		); err != nil {
			slog.Warn("follower notification: failed to create notification",
				"follower_id", followerID, "post_id", postID, "error", err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test -race ./internal/events/... -run TestFollowerNotification
```
Expected: all 5 PASS.

- [ ] **Step 6: Run full events suite**

```bash
cd backend && go test -race ./internal/events/...
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/events/social.go \
        backend/internal/events/follower_notification.go \
        backend/internal/events/follower_notification_test.go
git commit -m "feat: add FollowerNotificationConsumer for post.created → follower notifications"
```

---

### Task 11: FollowNotificationConsumer

**Files:**
- Create: `backend/internal/events/follow_notification.go`
- Create: `backend/internal/events/follow_notification_test.go`

- [ ] **Step 1: Write failing tests**

Create `backend/internal/events/follow_notification_test.go`:

```go
package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// buildFollowEnvelope marshals a FollowCreatedPayload into a raw EventEnvelope JSON message.
func buildFollowEnvelope(t *testing.T, payload FollowCreatedPayload) []byte {
	t.Helper()
	env := NewEnvelope(TopicFollowCreated, payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

func TestFollowNotificationConsumer_Process_ValidPayload(t *testing.T) {
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{}}
	consumer := NewFollowNotificationConsumer(notifier, prefs)

	body := buildFollowEnvelope(t, FollowCreatedPayload{
		FollowerID:   "follower-1",
		FollowingID:  "target-1",
		FollowerName: "Alice",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.userID != "target-1" {
		t.Errorf("notification userID = %q, want target-1", call.userID)
	}
	if call.title != "Alice started following you" {
		t.Errorf("title = %q, want %q", call.title, "Alice started following you")
	}
	if call.eventTypeID != "social.new_follower" {
		t.Errorf("eventTypeID = %q, want social.new_follower", call.eventTypeID)
	}
	if call.referenceID == nil || *call.referenceID != "follower-1" {
		t.Errorf("referenceID = %v, want follower-1", call.referenceID)
	}
}

func TestFollowNotificationConsumer_Process_MutedUserSkipped(t *testing.T) {
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{
		"target-1": {UserID: "target-1", MuteFollows: true},
	}}
	consumer := NewFollowNotificationConsumer(notifier, prefs)

	body := buildFollowEnvelope(t, FollowCreatedPayload{
		FollowerID:   "follower-1",
		FollowingID:  "target-1",
		FollowerName: "Bob",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected 0 notifications (muted), got %d", len(notifier.calls))
	}
}

func TestFollowNotificationConsumer_Process_MalformedJSON(t *testing.T) {
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, &fakePrefsReader{})
	err := consumer.Process([]byte(`not-json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFollowNotificationConsumer_Process_UnknownVersion(t *testing.T) {
	env := EventEnvelope{Version: 99, Type: TopicFollowCreated, Payload: map[string]string{}}
	body, _ := json.Marshal(env)
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, &fakePrefsReader{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version: %v", err)
	}
}

func TestFollowNotificationConsumer_Process_PrefsError_ReturnsError(t *testing.T) {
	prefs := &fakePrefsReader{}
	prefs.err = context.DeadlineExceeded
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, prefs)

	body := buildFollowEnvelope(t, FollowCreatedPayload{
		FollowerID:   "f1",
		FollowingID:  "t1",
		FollowerName: "Alice",
	})
	err := consumer.Process(body)
	if err == nil {
		t.Error("expected prefs error to propagate, got nil")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
cd backend && go test -race ./internal/events/... -run TestFollowNotification
```
Expected: FAIL — `NewFollowNotificationConsumer undefined`.

- [ ] **Step 3: Implement `FollowNotificationConsumer`**

Create `backend/internal/events/follow_notification.go`:

```go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// FollowNotificationConsumer consumes follow.created events and notifies
// the followed user that someone started following them.
type FollowNotificationConsumer struct {
	notifCreator NotificationCreator
	prefsReader  SocialPrefsReader
}

// NewFollowNotificationConsumer creates a new FollowNotificationConsumer.
func NewFollowNotificationConsumer(
	notifCreator NotificationCreator,
	prefsReader SocialPrefsReader,
) *FollowNotificationConsumer {
	return &FollowNotificationConsumer{
		notifCreator: notifCreator,
		prefsReader:  prefsReader,
	}
}

// Topic implements MessageHandler.
func (c *FollowNotificationConsumer) Topic() string { return TopicFollowCreated }

// Channel implements MessageHandler.
func (c *FollowNotificationConsumer) Channel() string { return "follow-notifications" }

// Process implements MessageHandler.
func (c *FollowNotificationConsumer) Process(body []byte) error {
	var evt rawEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if evt.Version != 1 {
		slog.Warn("follow notification: unknown envelope version, skipping", "version", evt.Version)
		return nil
	}
	var payload FollowCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal follow created payload: %w", err)
	}
	return c.process(payload)
}

// process sends a notification to the followed user if they have not muted follow notifications.
func (c *FollowNotificationConsumer) process(payload FollowCreatedPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefs, err := c.prefsReader.GetPrefs(ctx, payload.FollowingID)
	if err != nil {
		return fmt.Errorf("get prefs for user %s: %w", payload.FollowingID, err)
	}
	if prefs.MuteFollows {
		return nil
	}

	title := payload.FollowerName + " started following you"
	followerID := payload.FollowerID

	if err := c.notifCreator.CreateForUser(ctx,
		payload.FollowingID, "social", "social.new_follower", title, nil, nil, &followerID,
	); err != nil {
		slog.Warn("follow notification: failed to create notification",
			"following_id", payload.FollowingID, "follower_id", followerID, "error", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -race ./internal/events/... -run TestFollowNotification
```
Expected: all 5 PASS.

- [ ] **Step 5: Run full events suite**

```bash
cd backend && go test -race ./internal/events/...
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/events/follow_notification.go \
        backend/internal/events/follow_notification_test.go
git commit -m "feat: add FollowNotificationConsumer for follow.created → new follower notifications"
```

---

### Task 12: Social prefs handler + DTO + tests

**Files:**
- Create: `backend/internal/handler/social_prefs.go`
- Create: `backend/internal/handler/social_prefs_dto.go`
- Create: `backend/internal/handler/social_prefs_test.go`

- [ ] **Step 1: Create DTOs**

Create `backend/internal/handler/social_prefs_dto.go`:

```go
package handler

// UpdateSocialPrefsRequest is the request body for PUT /api/settings/social-notifications.
type UpdateSocialPrefsRequest struct {
	MutePosts   bool `json:"mutePosts"`
	MuteFollows bool `json:"muteFollows"`
}

// SocialPrefsResponse is the response body for GET /api/settings/social-notifications.
type SocialPrefsResponse struct {
	MutePosts   bool `json:"mutePosts"`
	MuteFollows bool `json:"muteFollows"`
}
```

- [ ] **Step 2: Write failing handler tests**

Create `backend/internal/handler/social_prefs_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/session"
	"github.com/meowmix1337/argus/backend/internal/validate"
)

type fakeSocialPrefsService struct {
	prefs      model.SocialNotificationPrefs
	getErr     error
	upsertErr  error
	upsertedID string
}

func (f *fakeSocialPrefsService) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.getErr != nil {
		return model.SocialNotificationPrefs{}, f.getErr
	}
	if f.prefs.UserID == "" {
		return model.SocialNotificationPrefs{UserID: userID}, nil
	}
	return f.prefs, nil
}

func (f *fakeSocialPrefsService) UpsertPrefs(_ context.Context, userID string, _, _ bool) error {
	f.upsertedID = userID
	return f.upsertErr
}

// withSession injects session.Data into the request context, simulating RequireAuth middleware.
// middleware.SessionKey is an exported constant; context.WithValue accepts it from any package.
func withSession(r *http.Request, userID, name string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.SessionKey, session.Data{UserID: userID, Name: name})
	return r.WithContext(ctx)
}

func TestSocialPrefsHandler_GetPrefs_DefaultsWhenNoRow(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	req = withSession(req, "user-1", "Alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp SocialPrefsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.MutePosts || resp.MuteFollows {
		t.Errorf("expected default false prefs, got %+v", resp)
	}
}

func TestSocialPrefsHandler_GetPrefs_ReturnsStored(t *testing.T) {
	stored := model.SocialNotificationPrefs{UserID: "user-1", MutePosts: true, MuteFollows: false}
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{prefs: stored}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	req = withSession(req, "user-1", "Alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp SocialPrefsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.MutePosts {
		t.Error("expected MutePosts=true in response")
	}
}

func TestSocialPrefsHandler_GetPrefs_Unauthorized(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/social-notifications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_Success(t *testing.T) {
	svc := &fakeSocialPrefsService{}
	h := NewSocialPrefsHandler(svc, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	body, _ := json.Marshal(UpdateSocialPrefsRequest{MutePosts: true, MuteFollows: false})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withSession(req, "user-1", "Alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if svc.upsertedID != "user-1" {
		t.Errorf("upserted userID = %q, want user-1", svc.upsertedID)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_InvalidJSON(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader([]byte(`not-json`)))
	req = withSession(req, "user-1", "Alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSocialPrefsHandler_UpdatePrefs_Unauthorized(t *testing.T) {
	h := NewSocialPrefsHandler(&fakeSocialPrefsService{}, validate.New())
	r := chi.NewRouter()
	h.AddRoutes(r)

	body, _ := json.Marshal(UpdateSocialPrefsRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/settings/social-notifications", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
```

> **Note:** The `withSession` helper calls `middleware.WithSession` — check whether that function exists in the middleware package. If it doesn't, look at how other handler tests inject the session (e.g., `notifications_test.go`) and use the same pattern.

- [ ] **Step 3: Run to confirm failure**

```bash
cd backend && go test -race ./internal/handler/... -run TestSocialPrefsHandler
```
Expected: FAIL — `NewSocialPrefsHandler undefined`.

- [ ] **Step 4: Create `social_prefs.go` handler**

Create `backend/internal/handler/social_prefs.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/response"
)

// socialPrefsService is the interface the handler requires from SocialPrefsService.
type socialPrefsService interface {
	GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
	UpsertPrefs(ctx context.Context, userID string, mutePosts, muteFollows bool) error
}

// SocialPrefsHandler handles social notification preference endpoints.
type SocialPrefsHandler struct {
	service  socialPrefsService
	validate *validator.Validate
}

// NewSocialPrefsHandler creates a new SocialPrefsHandler.
func NewSocialPrefsHandler(svc socialPrefsService, v *validator.Validate) *SocialPrefsHandler {
	return &SocialPrefsHandler{service: svc, validate: v}
}

// AddRoutes registers social notification preference routes on the given router.
func (h *SocialPrefsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/settings/social-notifications", h.GetPrefs)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Put("/api/settings/social-notifications", h.UpdatePrefs)
}

func (h *SocialPrefsHandler) GetPrefs(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	prefs, err := h.service.GetPrefs(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get social prefs", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, SocialPrefsResponse{
		MutePosts:   prefs.MutePosts,
		MuteFollows: prefs.MuteFollows,
	})
}

func (h *SocialPrefsHandler) UpdatePrefs(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req UpdateSocialPrefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpsertPrefs(r.Context(), userID, req.MutePosts, req.MuteFollows); err != nil {
		slog.Error("failed to update social prefs", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test -race ./internal/handler/... -run TestSocialPrefsHandler
```
Expected: all 6 PASS.

- [ ] **Step 6: Run full handler test suite**

```bash
cd backend && go test -race ./internal/handler/...
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/handler/social_prefs.go \
        backend/internal/handler/social_prefs_dto.go \
        backend/internal/handler/social_prefs_test.go
git commit -m "feat: add social notification prefs handler (GET/PUT /api/settings/social-notifications)"
```

---

### Task 13: Server wiring

**Files:**
- Modify: `backend/internal/server/server.go`

- [ ] **Step 1: Add social prefs service + new consumers to `setupRoutes()`**

In `server.go`, after the existing `notificationSvc` line (around line 92), add:

```go
socialPrefsRepo := repository.NewSQLiteSocialPrefsRepository(s.db)
socialPrefsSvc := service.NewSocialPrefsService(socialPrefsRepo)
```

In the NSQ consumer block, add the two new consumers to the slice:

```go
if s.cfg.NSQLookupdAddr != "" {
    cm := events.NewConsumerManager(s.cfg.NSQLookupdAddr)
    for _, consumer := range []events.MessageHandler{
        events.NewFeedFanoutConsumer(followRepo, feedRepo),
        events.NewFollowBackfillConsumer(postsRepo, feedRepo),
        events.NewFollowerNotificationConsumer(followRepo, notificationSvc, socialPrefsSvc),
        events.NewFollowNotificationConsumer(notificationSvc, socialPrefsSvc),
    } {
        if err := cm.Register(consumer); err != nil {
            slog.Warn("failed to register consumer", "topic", consumer.Topic(), "error", err)
        }
    }
    // ... rest unchanged
}
```

After `notificationsH := handler.NewNotificationsHandler(...)`, add:

```go
socialPrefsH := handler.NewSocialPrefsHandler(socialPrefsSvc, v)
```

In the protected routes section, add:

```go
socialPrefsH.AddRoutes(protected)
```

- [ ] **Step 2: Build**

```bash
cd backend && go build ./...
```
Expected: PASS. Fix any type mismatch errors — `notificationSvc` is `*service.NotificationService` which implements `NotificationCreator` (has `CreateForUser`). `socialPrefsSvc` is `*service.SocialPrefsService` which implements `SocialPrefsReader` (has `GetPrefs`). Both satisfy the interfaces via Go structural typing.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/server/server.go
git commit -m "feat: wire FollowerNotificationConsumer, FollowNotificationConsumer, and social prefs handler"
```

---

### Task 14: Quality gates + PR 2

- [ ] **Step 1: Run full test suite with race detector**

```bash
cd backend && go test -race ./...
```
Expected: all PASS.

- [ ] **Step 2: Check test coverage on service package**

```bash
cd backend && go test -coverprofile=coverage.out ./internal/service/... && go tool cover -func=coverage.out
```
Expected: overall coverage ≥ 80%. If below, add tests for uncovered paths.

- [ ] **Step 3: Run linter**

```bash
cd backend && golangci-lint run ./...
```
Expected: zero errors. Fix any issues before opening the PR.

- [ ] **Step 4: Sync bd backup and push**

```bash
~/.local/bin/bd backup && git add .beads/backup/
git commit -m "chore: sync bd backup"
git push
```

- [ ] **Step 5: Open PR 2 targeting `main`**

```bash
gh pr create --title "feat: social notification consumers and mute prefs API (argus-v1r)" \
  --body "$(cat <<'EOF'
## Summary
- Two NSQ consumers: FollowerNotificationConsumer (post.created → follower notifications) and FollowNotificationConsumer (follow.created → new follower notification)
- NotificationService.CreateForUser() for consumer use with duplicate suppression via reference_id
- SocialPrefsService + SQLite repository for per-user mute preferences
- GET/PUT /api/settings/social-notifications API
- FollowService.Follow() now publishes both user.followed and follow.created events

**Depends on:** Migration PR must be merged first (migrations 021–024)

## Test plan
- [ ] Run `go test -race ./...` — all green
- [ ] Run `golangci-lint run ./...` — zero errors
- [ ] Coverage ≥ 80% on `internal/service/`
- [ ] Verify consumer registration logs appear on server startup with NSQ configured
EOF
)" \
  --base main
```
