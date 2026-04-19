package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// UserSearchStore defines the data-access contract for user search.
type UserSearchStore interface {
	SearchUsers(ctx context.Context, viewerID, q string, limit, offset int) ([]model.UserSummary, int, error)
}
