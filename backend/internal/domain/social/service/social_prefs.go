package service

import (
	"context"
	"fmt"
	"time"

	"github.com/meowmix1337/argus/backend/internal/domain/social/repository"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// SocialPrefsService manages social notification mute preferences.
type SocialPrefsService struct {
	store repository.SocialNotificationPrefsStore
}

// NewSocialPrefsService creates a new SocialPrefsService.
func NewSocialPrefsService(store repository.SocialNotificationPrefsStore) *SocialPrefsService {
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
