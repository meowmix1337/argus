package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// UserSettingsStore defines the data-access contract for user settings.
// This is the merged superset of the former UserSettingsRepository and UserSettingsStore interfaces.
type UserSettingsStore interface {
	// Get returns the settings for the given user.
	Get(ctx context.Context, userID string) (model.UserSettings, error)
	// Upsert creates or updates settings for the given user, returning the final state.
	Upsert(ctx context.Context, userID string, u model.UserSettingsUpsert) (model.UserSettings, error)
	// ListAllCategories returns all news category types ordered by sort_order.
	ListAllCategories(ctx context.Context) ([]model.NewsCategoryType, error)
	// ListSelectedCategories returns the news categories the user has selected.
	ListSelectedCategories(ctx context.Context, userID string) ([]model.NewsCategoryType, error)
	// SetSelectedCategories replaces the user's selected categories.
	SetSelectedCategories(ctx context.Context, userID string, categoryIDs []string) error
}
