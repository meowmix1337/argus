package events

import (
	"context"
	"encoding/json"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// rawEventEnvelope is used for decoding only — Payload is kept as raw JSON to avoid
// a double marshal/unmarshal round-trip when deserializing into concrete payload types.
type rawEventEnvelope struct {
	Version   int             `json:"v"`
	Type      string          `json:"type"`
	Timestamp string          `json:"ts"`
	Payload   json.RawMessage `json:"payload"`
}

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

// FanoutFollowStore looks up follower IDs for a given user.
// Used by FollowerNotificationConsumer.
type FanoutFollowStore interface {
	GetFollowerIDs(ctx context.Context, userID string) ([]string, error)
}
