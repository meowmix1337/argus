package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// TaskLabelStore defines the data-access contract for task labels.
type TaskLabelStore interface {
	// List returns all active labels for the given user.
	List(ctx context.Context, userID string) ([]model.TaskLabel, error)
	// Get returns a single active label by ID for the given user.
	Get(ctx context.Context, id string, userID string) (model.TaskLabel, error)
	// Create inserts a new label and returns the created label.
	Create(ctx context.Context, l model.TaskLabelCreate) (model.TaskLabel, error)
	// Update applies partial updates to a label.
	Update(ctx context.Context, id string, userID string, u model.TaskLabelUpdate) error
	// Delete soft-deletes a label.
	Delete(ctx context.Context, id string, userID string) error
	// ListForTask returns all active labels assigned to the given task, scoped to the user.
	ListForTask(ctx context.Context, taskID string, userID string) ([]model.TaskLabel, error)
	// AssignLabel creates an assignment of a label to a task.
	AssignLabel(ctx context.Context, a model.TaskLabelAssignmentCreate) error
	// RemoveLabel soft-deletes the assignment of a label from a task, scoped to the user.
	RemoveLabel(ctx context.Context, taskID string, labelID string, userID string) error
}
