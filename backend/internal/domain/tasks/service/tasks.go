package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/meowmix1337/argus/backend/internal/domain/tasks/repository"
	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// ErrTaskNotFound is returned when a task does not exist or does not belong to the user.
var ErrTaskNotFound = apperrors.ErrTaskNotFound

// ErrTaskValidation is returned when task input fails validation.
var ErrTaskValidation = apperrors.ErrTaskValidation

// TasksService manages tasks via a store.
type TasksService struct {
	store repository.TaskStore
}

// NewTasksService creates a new TasksService backed by the given store.
func NewTasksService(store repository.TaskStore) *TasksService {
	return &TasksService{store: store}
}

// List returns a page of tasks for the given user along with the total count.
func (s *TasksService) List(ctx context.Context, userID string, limit, offset int) ([]model.Task, int, error) {
	tasks, total, err := s.store.List(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, total, nil
}

// Create adds a new task for the given user.
func (s *TasksService) Create(ctx context.Context, userID string, text string, priority string) (model.Task, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return model.Task{}, fmt.Errorf("%w: task text cannot be empty", ErrTaskValidation)
	}

	if priority == "" {
		priority = "medium"
	}
	if priority != "high" && priority != "medium" && priority != "low" {
		return model.Task{}, fmt.Errorf("%w: invalid priority %q", ErrTaskValidation, priority)
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("failed to generate UUID v7", "error", err)
		return model.Task{}, fmt.Errorf("generate task id: %w", err)
	}

	task, err := s.store.Create(ctx, model.TaskCreate{
		ID:         id.String(),
		UserID:     userID,
		Text:       text,
		Done:       false,
		PriorityID: priority,
	})
	if err != nil {
		return model.Task{}, fmt.Errorf("create task: %w", err)
	}
	return task, nil
}

// Update modifies a task by ID, scoped to the given user.
func (s *TasksService) Update(ctx context.Context, id string, userID string, done *bool, text *string, priority *string) (model.Task, error) {
	if text != nil {
		trimmed := strings.TrimSpace(*text)
		if trimmed == "" {
			return model.Task{}, fmt.Errorf("%w: task text cannot be empty", ErrTaskValidation)
		}
		text = &trimmed
	}
	if priority != nil && *priority != "high" && *priority != "medium" && *priority != "low" {
		return model.Task{}, fmt.Errorf("%w: invalid priority %q", ErrTaskValidation, *priority)
	}

	// 1. Verify the task exists and belongs to this user.
	if _, err := s.store.Get(ctx, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrTaskNotFound) {
			return model.Task{}, ErrTaskNotFound
		}
		return model.Task{}, fmt.Errorf("get task: %w", err)
	}

	// 2. Apply the update.
	if err := s.store.Update(ctx, id, userID, model.TaskUpdate{
		Done:       done,
		Text:       text,
		PriorityID: priority,
	}); err != nil {
		return model.Task{}, fmt.Errorf("update task: %w", err)
	}

	// 3. Re-fetch to return the current state.
	task, err := s.store.Get(ctx, id, userID)
	if err != nil {
		return model.Task{}, fmt.Errorf("fetch updated task: %w", err)
	}
	return task, nil
}

// Delete soft-deletes a task by ID, scoped to the given user.
func (s *TasksService) Delete(ctx context.Context, id string, userID string) error {
	// 1. Verify the task exists and belongs to this user.
	if _, err := s.store.Get(ctx, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrTaskNotFound) {
			return ErrTaskNotFound
		}
		return fmt.Errorf("get task: %w", err)
	}

	// 2. Soft-delete it.
	if err := s.store.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}
