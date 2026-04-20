package service

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// fakeNotificationStore is an in-memory NotificationWriter for webhook service tests.
type fakeNotificationStore struct {
	notifications []model.NotificationCreate
	createErr     error
	duplicate     bool
}

func (f *fakeNotificationStore) Create(_ context.Context, n model.NotificationCreate) (model.Notification, error) {
	if f.duplicate {
		return model.Notification{}, apperrors.ErrDuplicateDelivery
	}
	if f.createErr != nil {
		return model.Notification{}, f.createErr
	}
	f.notifications = append(f.notifications, n)
	return model.Notification{ID: n.ID, UserID: n.UserID, Title: n.Title}, nil
}
