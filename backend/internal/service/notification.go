package service

import (
	"context"
	"fmt"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// NotificationStore defines the data-access contract for notifications.
type NotificationStore interface {
	Create(ctx context.Context, n model.NotificationCreate) (model.Notification, error)
	List(ctx context.Context, userID, state string, limit, offset int) ([]model.Notification, int, error)
	GetByID(ctx context.Context, id, userID string) (model.Notification, error)
	MarkRead(ctx context.Context, id, userID string) (int64, error)
	MarkDismissed(ctx context.Context, id, userID string) (int64, error)
	MarkAllRead(ctx context.Context, userID string) (int64, error)
	CountUnread(ctx context.Context, userID string) (int, error)
}

// NotificationService manages user notifications via a store.
type NotificationService struct {
	store NotificationStore
}

// NewNotificationService creates a new NotificationService backed by the given store.
func NewNotificationService(store NotificationStore) *NotificationService {
	return &NotificationService{store: store}
}

// List returns a paginated list of notifications filtered by state.
// Valid state values: "unread" (default), "read", "dismissed", "all".
func (s *NotificationService) List(ctx context.Context, userID, state string, limit, offset int) ([]model.Notification, int, error) {
	notifications, total, err := s.store.List(ctx, userID, state, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return notifications, total, nil
}

// GetByID returns a single notification by ID, scoped to the given user.
func (s *NotificationService) GetByID(ctx context.Context, id, userID string) (model.Notification, error) {
	n, err := s.store.GetByID(ctx, id, userID)
	if err != nil {
		return model.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	return n, nil
}

// MarkRead marks a notification as read. Returns ErrNotificationNotFound if the
// notification does not exist, is not owned by the user, or is already read.
func (s *NotificationService) MarkRead(ctx context.Context, id, userID string) error {
	rows, err := s.store.MarkRead(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if rows == 0 {
		return apperrors.ErrNotificationNotFound
	}
	return nil
}

// MarkDismissed marks a notification as dismissed. Returns ErrNotificationNotFound if the
// notification does not exist, is not owned by the user, or is already dismissed.
func (s *NotificationService) MarkDismissed(ctx context.Context, id, userID string) error {
	rows, err := s.store.MarkDismissed(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification dismissed: %w", err)
	}
	if rows == 0 {
		return apperrors.ErrNotificationNotFound
	}
	return nil
}

// MarkAllRead marks all unread notifications as read for the given user.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.store.MarkAllRead(ctx, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// CountUnread returns the count of unread notifications for the given user.
func (s *NotificationService) CountUnread(ctx context.Context, userID string) (int, error) {
	count, err := s.store.CountUnread(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// Create persists a new notification. Returns ErrDuplicateDelivery if a notification
// with the same github_delivery_id already exists.
func (s *NotificationService) Create(ctx context.Context, n model.NotificationCreate) (model.Notification, error) {
	result, err := s.store.Create(ctx, n)
	if err != nil {
		return model.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return result, nil
}
