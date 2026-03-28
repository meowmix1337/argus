package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// WatchedRepoRepository defines the data-access contract for watched repositories.
type WatchedRepoRepository interface {
	Create(ctx context.Context, w model.WatchedRepoCreate) (model.WatchedRepo, error)
	GetByID(ctx context.Context, id, userID string) (model.WatchedRepo, error)
	ListByIntegration(ctx context.Context, integrationID, userID string) ([]model.WatchedRepo, error)
	GetByOwnerRepo(ctx context.Context, owner, repo string) ([]model.WatchedRepo, error)
	Delete(ctx context.Context, id, userID string) (int64, error)
}
