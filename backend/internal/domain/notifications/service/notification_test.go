package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// strPtr returns a pointer to the given string — used for nullable fields in tests.
func strPtr(s string) *string { return &s }

// fakeNotificationStore is an in-memory NotificationStore for service tests.
type fakeNotificationStore struct {
	notifications     []model.Notification
	markReadRows      int64
	markDismissedRows int64
	markAllReadRows   int64
	unreadCount       int
	markReadErr       error
	markDismissedErr  error
	markAllReadErr    error
	countErr          error
	createErr         error
	listErr           error
}

func (f *fakeNotificationStore) Create(_ context.Context, n model.NotificationCreate) (model.Notification, error) {
	if f.createErr != nil {
		return model.Notification{}, f.createErr
	}
	notif := model.Notification{ID: n.ID, Title: n.Title}
	f.notifications = append(f.notifications, notif)
	return notif, nil
}

func (f *fakeNotificationStore) List(_ context.Context, _, _, _, _, _ string, _, _ int) ([]model.Notification, int, error) {
	return f.notifications, len(f.notifications), f.listErr
}

func (f *fakeNotificationStore) GetByID(_ context.Context, id, _ string) (model.Notification, error) {
	for _, n := range f.notifications {
		if n.ID == id {
			return n, nil
		}
	}
	return model.Notification{}, fmt.Errorf("not found")
}

func (f *fakeNotificationStore) MarkRead(_ context.Context, _, _ string) (int64, error) {
	return f.markReadRows, f.markReadErr
}

func (f *fakeNotificationStore) MarkDismissed(_ context.Context, _, _ string) (int64, error) {
	return f.markDismissedRows, f.markDismissedErr
}

func (f *fakeNotificationStore) MarkAllRead(_ context.Context, _ string) (int64, error) {
	return f.markAllReadRows, f.markAllReadErr
}

func (f *fakeNotificationStore) CountUnread(_ context.Context, _ string) (int, error) {
	return f.unreadCount, f.countErr
}

// ---- MarkRead ----

// TestNotificationService_MarkRead_ZeroRows verifies that when the repository
// returns 0 rows affected (notification not found or already read), the service
// returns ErrNotificationNotFound rather than a generic error.
func TestNotificationService_MarkRead_ZeroRows_ReturnsNotFound(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markReadRows: 0})
	err := svc.MarkRead(context.Background(), "n1", "user1")
	if !errors.Is(err, apperrors.ErrNotificationNotFound) {
		t.Errorf("expected ErrNotificationNotFound, got %v", err)
	}
}

func TestNotificationService_MarkRead_Success(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markReadRows: 1})
	if err := svc.MarkRead(context.Background(), "n1", "user1"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
}

func TestNotificationService_MarkRead_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markReadErr: fmt.Errorf("db failure")})
	if err := svc.MarkRead(context.Background(), "n1", "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- MarkDismissed ----

func TestNotificationService_MarkDismissed_ZeroRows_ReturnsNotFound(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markDismissedRows: 0})
	err := svc.MarkDismissed(context.Background(), "n1", "user1")
	if !errors.Is(err, apperrors.ErrNotificationNotFound) {
		t.Errorf("expected ErrNotificationNotFound, got %v", err)
	}
}

func TestNotificationService_MarkDismissed_Success(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markDismissedRows: 1})
	if err := svc.MarkDismissed(context.Background(), "n1", "user1"); err != nil {
		t.Fatalf("MarkDismissed: %v", err)
	}
}

func TestNotificationService_MarkDismissed_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markDismissedErr: fmt.Errorf("db failure")})
	if err := svc.MarkDismissed(context.Background(), "n1", "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- CountUnread ----

func TestNotificationService_CountUnread_ReturnsCount(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{unreadCount: 7})
	count, err := svc.CountUnread(context.Background(), "user1")
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if count != 7 {
		t.Errorf("count = %d, want 7", count)
	}
}

func TestNotificationService_CountUnread_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{countErr: fmt.Errorf("db failure")})
	if _, err := svc.CountUnread(context.Background(), "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- MarkAllRead ----

func TestNotificationService_MarkAllRead_Success(t *testing.T) {
	store := &fakeNotificationStore{markAllReadRows: 3}
	svc := NewNotificationService(store)
	if err := svc.MarkAllRead(context.Background(), "user1"); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
}

// ---- List ----

func TestNotificationService_List_ReturnsAll(t *testing.T) {
	store := &fakeNotificationStore{
		notifications: []model.Notification{
			{ID: "n1", Title: "First"},
			{ID: "n2", Title: "Second"},
		},
	}
	svc := NewNotificationService(store)
	notifs, total, err := svc.List(context.Background(), "user1", "unread", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(notifs) != 2 {
		t.Errorf("expected 2 notifications, got total=%d len=%d", total, len(notifs))
	}
}

// ---- List (error path) ----

func TestNotificationService_List_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{listErr: fmt.Errorf("db failure")})
	if _, _, err := svc.List(context.Background(), "user1", "unread", "", "", "", 10, 0); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- MarkAllRead (error path) ----

func TestNotificationService_MarkAllRead_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{markAllReadErr: fmt.Errorf("db failure")})
	if err := svc.MarkAllRead(context.Background(), "user1"); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- Create (error path) ----

func TestNotificationService_Create_StoreError_Propagates(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{createErr: fmt.Errorf("db failure")})
	if _, err := svc.Create(context.Background(), model.NotificationCreate{ID: "n1", Title: "test"}); err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}

// ---- GetByID ----

func TestNotificationService_GetByID_Success(t *testing.T) {
	store := &fakeNotificationStore{
		notifications: []model.Notification{{ID: "n1", Title: "PR opened"}},
	}
	svc := NewNotificationService(store)
	n, err := svc.GetByID(context.Background(), "n1", "user1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if n.Title != "PR opened" {
		t.Errorf("Title = %q, want %q", n.Title, "PR opened")
	}
}

func TestNotificationService_GetByID_NotFound_ReturnsError(t *testing.T) {
	svc := NewNotificationService(&fakeNotificationStore{})
	if _, err := svc.GetByID(context.Background(), "nonexistent", "user1"); err == nil {
		t.Error("expected error for missing notification, got nil")
	}
}

// ---- Create ----

func TestNotificationService_Create_Success(t *testing.T) {
	store := &fakeNotificationStore{}
	svc := NewNotificationService(store)
	n, err := svc.Create(context.Background(), model.NotificationCreate{
		ID:    "n1",
		Title: "PR #1 opened",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.Title != "PR #1 opened" {
		t.Errorf("Title = %q, want %q", n.Title, "PR #1 opened")
	}
}

// ---- CreateForUser ----

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
