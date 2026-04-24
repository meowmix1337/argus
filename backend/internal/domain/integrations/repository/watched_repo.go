package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// WatchedRepoStore defines the data-access contract for watched repositories.
type WatchedRepoStore interface {
	Create(ctx context.Context, w model.WatchedRepoCreate) (model.WatchedRepo, error)
	GetByID(ctx context.Context, id, userID string) (model.WatchedRepo, error)
	ListByIntegration(ctx context.Context, integrationID, userID string) ([]model.WatchedRepo, error)
	// GetByOwnerRepo returns all watched repos for the given owner/repo across all users.
	// Intentionally omits userID scoping — webhook dispatch receives an owner/repo from the
	// payload with no user context, and must match every user watching that repo.
	GetByOwnerRepo(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error)
	Delete(ctx context.Context, id, userID string) (int64, error)
}
