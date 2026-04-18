package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// ErrLabelNotFound is returned when a label does not exist or does not belong to the user.
var ErrLabelNotFound = apperrors.ErrLabelNotFound

// ErrLabelValidation is returned when label input fails validation.
var ErrLabelValidation = apperrors.ErrLabelValidation

// ErrLabelAlreadyAssigned is returned when a label is already assigned to a task.
var ErrLabelAlreadyAssigned = apperrors.ErrLabelAlreadyAssigned

// TaskLabelStore defines the data-access contract for task labels.
type TaskLabelStore interface {
	List(ctx context.Context, userID string) ([]model.TaskLabel, error)
	Get(ctx context.Context, id string, userID string) (model.TaskLabel, error)
	Create(ctx context.Context, l model.TaskLabelCreate) (model.TaskLabel, error)
	Update(ctx context.Context, id string, userID string, u model.TaskLabelUpdate) error
	Delete(ctx context.Context, id string, userID string) error
	ListForTask(ctx context.Context, taskID string, userID string) ([]model.TaskLabel, error)
	AssignLabel(ctx context.Context, a model.TaskLabelAssignmentCreate) error
	RemoveLabel(ctx context.Context, taskID string, labelID string, userID string) error
}

// TaskLabelsService manages task labels via a store.
type TaskLabelsService struct {
	store TaskLabelStore
}

// NewTaskLabelsService creates a new TaskLabelsService backed by the given store.
func NewTaskLabelsService(store TaskLabelStore) *TaskLabelsService {
	return &TaskLabelsService{store: store}
}

// List returns all active labels for the given user.
func (s *TaskLabelsService) List(ctx context.Context, userID string) ([]model.TaskLabel, error) {
	labels, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

// Create adds a new label for the given user.
func (s *TaskLabelsService) Create(ctx context.Context, userID string, name string, color string) (model.TaskLabel, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.TaskLabel{}, fmt.Errorf("%w: label name cannot be empty", ErrLabelValidation)
	}
	if len(name) > 16 {
		return model.TaskLabel{}, fmt.Errorf("%w: label name exceeds 16 characters", ErrLabelValidation)
	}

	if color == "" {
		color = "#6366f1"
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("failed to generate UUID v7", "error", err)
		return model.TaskLabel{}, fmt.Errorf("generate label id: %w", err)
	}

	label, err := s.store.Create(ctx, model.TaskLabelCreate{
		ID:     id.String(),
		UserID: userID,
		Name:   name,
		Color:  color,
	})
	if err != nil {
		return model.TaskLabel{}, fmt.Errorf("create label: %w", err)
	}
	return label, nil
}

// Update modifies a label by ID, scoped to the given user.
func (s *TaskLabelsService) Update(ctx context.Context, id string, userID string, name *string, color *string) (model.TaskLabel, error) {
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return model.TaskLabel{}, fmt.Errorf("%w: label name cannot be empty", ErrLabelValidation)
		}
		if len(trimmed) > 16 {
			return model.TaskLabel{}, fmt.Errorf("%w: label name exceeds 16 characters", ErrLabelValidation)
		}
		name = &trimmed
	}

	// Verify the label exists and belongs to this user.
	if _, err := s.store.Get(ctx, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			return model.TaskLabel{}, ErrLabelNotFound
		}
		return model.TaskLabel{}, fmt.Errorf("get label: %w", err)
	}

	if err := s.store.Update(ctx, id, userID, model.TaskLabelUpdate{
		Name:  name,
		Color: color,
	}); err != nil {
		return model.TaskLabel{}, fmt.Errorf("update label: %w", err)
	}

	label, err := s.store.Get(ctx, id, userID)
	if err != nil {
		return model.TaskLabel{}, fmt.Errorf("fetch updated label: %w", err)
	}
	return label, nil
}

// Delete soft-deletes a label by ID, scoped to the given user.
func (s *TaskLabelsService) Delete(ctx context.Context, id string, userID string) error {
	// Verify the label exists and belongs to this user.
	if _, err := s.store.Get(ctx, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			return ErrLabelNotFound
		}
		return fmt.Errorf("get label: %w", err)
	}

	if err := s.store.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return nil
}

// ListForTask returns all active labels assigned to the given task, scoped to the user.
func (s *TaskLabelsService) ListForTask(ctx context.Context, taskID string, userID string) ([]model.TaskLabel, error) {
	labels, err := s.store.ListForTask(ctx, taskID, userID)
	if err != nil {
		return nil, fmt.Errorf("list labels for task: %w", err)
	}
	return labels, nil
}

// AssignLabel assigns a label to a task.
func (s *TaskLabelsService) AssignLabel(ctx context.Context, taskID string, labelID string, userID string) error {
	// Verify the label exists and belongs to this user.
	if _, err := s.store.Get(ctx, labelID, userID); err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			return ErrLabelNotFound
		}
		return fmt.Errorf("get label: %w", err)
	}

	// Check if already assigned.
	existing, err := s.store.ListForTask(ctx, taskID, userID)
	if err != nil {
		return fmt.Errorf("list labels for task: %w", err)
	}
	for _, l := range existing {
		if l.ID == labelID {
			return ErrLabelAlreadyAssigned
		}
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("failed to generate UUID v7", "error", err)
		return fmt.Errorf("generate assignment id: %w", err)
	}

	if err := s.store.AssignLabel(ctx, model.TaskLabelAssignmentCreate{
		ID:      id.String(),
		TaskID:  taskID,
		LabelID: labelID,
	}); err != nil {
		if errors.Is(err, apperrors.ErrLabelAlreadyAssigned) {
			return ErrLabelAlreadyAssigned
		}
		return fmt.Errorf("assign label: %w", err)
	}
	return nil
}

// RemoveLabel removes a label assignment from a task.
func (s *TaskLabelsService) RemoveLabel(ctx context.Context, taskID string, labelID string, userID string) error {
	if err := s.store.RemoveLabel(ctx, taskID, labelID, userID); err != nil {
		return fmt.Errorf("remove label: %w", err)
	}
	return nil
}
