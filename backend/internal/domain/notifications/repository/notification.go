package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// NotificationStore defines the data-access contract for notifications.
// This is the canonical interface used by both the service layer and any
// consumer-side code that needs to write notifications.
type NotificationStore interface {
	Create(ctx context.Context, n model.NotificationCreate) (model.Notification, error)
	List(ctx context.Context, userID, state, query, providerID, eventTypeID string, limit, offset int) ([]model.Notification, int, error)
	GetByID(ctx context.Context, id, userID string) (model.Notification, error)
	MarkRead(ctx context.Context, id, userID string) (int64, error)
	MarkDismissed(ctx context.Context, id, userID string) (int64, error)
	MarkAllRead(ctx context.Context, userID string) (int64, error)
	CountUnread(ctx context.Context, userID string) (int, error)
}
