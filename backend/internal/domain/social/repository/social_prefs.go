package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// SocialNotificationPrefsStore defines the data-access contract for social notification prefs.
type SocialNotificationPrefsStore interface {
	GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
	UpsertPrefs(ctx context.Context, prefs model.SocialNotificationPrefs) error
}
