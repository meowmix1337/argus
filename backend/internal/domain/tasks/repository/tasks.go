package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// TaskStore defines the data-access contract for tasks.
type TaskStore interface {
	List(ctx context.Context, userID string, limit, offset int) ([]model.Task, int, error)
	Get(ctx context.Context, id string, userID string) (model.Task, error)
	Create(ctx context.Context, t model.TaskCreate) (model.Task, error)
	Update(ctx context.Context, id string, userID string, u model.TaskUpdate) error
	Delete(ctx context.Context, id string, userID string) error
}
