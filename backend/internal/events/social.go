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
