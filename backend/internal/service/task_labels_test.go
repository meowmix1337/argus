package service

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeTaskLabelStore is an in-memory TaskLabelStore for service tests.
type fakeTaskLabelStore struct {
	labels         map[string]model.TaskLabel
	assignments    map[string][]model.TaskLabel // taskID → labels
	listErr        error
	createErr      error
	getErr         error
	updateErr      error
	deleteErr      error
	assignErr      error
	listForTaskErr error
	removeLabelErr error
}

func newFakeTaskLabelStore(labels ...model.TaskLabel) *fakeTaskLabelStore {
	s := &fakeTaskLabelStore{
		labels:      make(map[string]model.TaskLabel),
		assignments: make(map[string][]model.TaskLabel),
	}
	for _, l := range labels {
		s.labels[l.ID] = l
	}
	return s
}

func (f *fakeTaskLabelStore) List(_ context.Context, _ string) ([]model.TaskLabel, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]model.TaskLabel, 0, len(f.labels))
	for _, l := range f.labels {
		out = append(out, l)
	}
	return out, nil
}

func (f *fakeTaskLabelStore) Get(_ context.Context, id string, _ string) (model.TaskLabel, error) {
	if f.getErr != nil {
		return model.TaskLabel{}, f.getErr
	}
	l, ok := f.labels[id]
	if !ok {
		return model.TaskLabel{}, apperrors.ErrLabelNotFound
	}
	return l, nil
}

func (f *fakeTaskLabelStore) Create(_ context.Context, l model.TaskLabelCreate) (model.TaskLabel, error) {
	if f.createErr != nil {
		return model.TaskLabel{}, f.createErr
	}
	label := model.TaskLabel{ID: l.ID, Name: l.Name, Color: l.Color}
	f.labels[l.ID] = label
	return label, nil
}

func (f *fakeTaskLabelStore) Update(_ context.Context, id string, _ string, u model.TaskLabelUpdate) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	l, ok := f.labels[id]
	if !ok {
		return apperrors.ErrLabelNotFound
	}
	if u.Name != nil {
		l.Name = *u.Name
	}
	if u.Color != nil {
		l.Color = *u.Color
	}
	f.labels[id] = l
	return nil
}

func (f *fakeTaskLabelStore) Delete(_ context.Context, id string, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.labels, id)
	return nil
}

func (f *fakeTaskLabelStore) ListForTask(_ context.Context, taskID string, _ string) ([]model.TaskLabel, error) {
	if f.listForTaskErr != nil {
		return nil, f.listForTaskErr
	}
	return f.assignments[taskID], nil
}

func (f *fakeTaskLabelStore) AssignLabel(_ context.Context, a model.TaskLabelAssignmentCreate) error {
	if f.assignErr != nil {
		return f.assignErr
	}
	l, ok := f.labels[a.LabelID]
	if !ok {
		return apperrors.ErrLabelNotFound
	}
	f.assignments[a.TaskID] = append(f.assignments[a.TaskID], l)
	return nil
}

func (f *fakeTaskLabelStore) RemoveLabel(_ context.Context, taskID, labelID, _ string) error {
	if f.removeLabelErr != nil {
		return f.removeLabelErr
	}
	labels := f.assignments[taskID]
	for i, l := range labels {
		if l.ID == labelID {
			f.assignments[taskID] = append(labels[:i], labels[i+1:]...)
			return nil
		}
	}
	return nil
}

// ---- Create ----

func TestTaskLabelsService_Create_EmptyName(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	_, err := svc.Create(context.Background(), "user1", "", "#ff0000")
	if !errors.Is(err, ErrLabelValidation) {
		t.Errorf("expected ErrLabelValidation for empty name, got %v", err)
	}
}

func TestTaskLabelsService_Create_WhitespaceOnlyName(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	_, err := svc.Create(context.Background(), "user1", "   ", "#ff0000")
	if !errors.Is(err, ErrLabelValidation) {
		t.Errorf("expected ErrLabelValidation for whitespace-only name, got %v", err)
	}
}

func TestTaskLabelsService_Create_NameTooLong(t *testing.T) {
	// 17 characters — one over the 16-char limit.
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	_, err := svc.Create(context.Background(), "user1", "seventeen-chars!!", "#ff0000")
	if !errors.Is(err, ErrLabelValidation) {
		t.Errorf("expected ErrLabelValidation for name > 16 chars, got %v", err)
	}
}

func TestTaskLabelsService_Create_DefaultColor(t *testing.T) {
	// An empty color string must fall back to the default indigo (#6366f1).
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	label, err := svc.Create(context.Background(), "user1", "Work", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if label.Color != "#6366f1" {
		t.Errorf("Color = %q, want default %q", label.Color, "#6366f1")
	}
}

func TestTaskLabelsService_Create_CustomColor(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	label, err := svc.Create(context.Background(), "user1", "Urgent", "#ff0000")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if label.Color != "#ff0000" {
		t.Errorf("Color = %q, want %q", label.Color, "#ff0000")
	}
}

// ---- Update ----

func TestTaskLabelsService_Update_EmptyName(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	svc := NewTaskLabelsService(store)
	empty := ""
	_, err := svc.Update(context.Background(), "l1", "user1", &empty, nil)
	if !errors.Is(err, ErrLabelValidation) {
		t.Errorf("expected ErrLabelValidation for empty name update, got %v", err)
	}
}

func TestTaskLabelsService_Update_NameTooLong(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	svc := NewTaskLabelsService(store)
	tooLong := "seventeen-chars!!"
	_, err := svc.Update(context.Background(), "l1", "user1", &tooLong, nil)
	if !errors.Is(err, ErrLabelValidation) {
		t.Errorf("expected ErrLabelValidation for name > 16 chars, got %v", err)
	}
}

func TestTaskLabelsService_Update_NotFound(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore()) // empty store
	name := "Updated"
	_, err := svc.Update(context.Background(), "nonexistent", "user1", &name, nil)
	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("expected ErrLabelNotFound, got %v", err)
	}
}

// ---- Delete ----

func TestTaskLabelsService_Delete_NotFound(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	if err := svc.Delete(context.Background(), "nonexistent", "user1"); !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("expected ErrLabelNotFound, got %v", err)
	}
}

func TestTaskLabelsService_Delete_Success(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	svc := NewTaskLabelsService(store)
	if err := svc.Delete(context.Background(), "l1", "user1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, exists := store.labels["l1"]; exists {
		t.Error("expected label to be removed from store after Delete")
	}
}

// ---- AssignLabel ----

func TestTaskLabelsService_AssignLabel_LabelNotFound(t *testing.T) {
	svc := NewTaskLabelsService(newFakeTaskLabelStore())
	if err := svc.AssignLabel(context.Background(), "task1", "nonexistent", "user1"); !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("expected ErrLabelNotFound, got %v", err)
	}
}

// TestTaskLabelsService_AssignLabel_AlreadyAssigned verifies the service-level
// duplicate check that runs before calling the store.
func TestTaskLabelsService_AssignLabel_AlreadyAssigned(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	store.assignments["task1"] = []model.TaskLabel{{ID: "l1"}}
	svc := NewTaskLabelsService(store)

	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); !errors.Is(err, ErrLabelAlreadyAssigned) {
		t.Errorf("expected ErrLabelAlreadyAssigned, got %v", err)
	}
}

func TestTaskLabelsService_AssignLabel_Success(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	svc := NewTaskLabelsService(store)

	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); err != nil {
		t.Fatalf("AssignLabel: %v", err)
	}
	if len(store.assignments["task1"]) != 1 {
		t.Errorf("expected 1 assigned label, got %d", len(store.assignments["task1"]))
	}
}

// ---- List ----

func TestTaskLabelsService_List_ReturnsAll(t *testing.T) {
	store := newFakeTaskLabelStore(
		model.TaskLabel{ID: "l1", Name: "Work"},
		model.TaskLabel{ID: "l2", Name: "Personal"},
	)
	svc := NewTaskLabelsService(store)
	labels, err := svc.List(context.Background(), "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(labels))
	}
}

// ---- ListForTask ----

func TestTaskLabelsService_ListForTask_ReturnsAssigned(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	store.assignments["task1"] = []model.TaskLabel{{ID: "l1", Name: "Work"}}
	svc := NewTaskLabelsService(store)

	labels, err := svc.ListForTask(context.Background(), "task1", "user1")
	if err != nil {
		t.Fatalf("ListForTask: %v", err)
	}
	if len(labels) != 1 || labels[0].ID != "l1" {
		t.Errorf("expected label l1, got: %+v", labels)
	}
}

// ---- Update (success + error paths) ----

func TestTaskLabelsService_Update_Success(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work", Color: "#ff0000"})
	svc := NewTaskLabelsService(store)
	newName := "Updated Work"
	label, err := svc.Update(context.Background(), "l1", "user1", &newName, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if label.Name != "Updated Work" {
		t.Errorf("Name = %q, want %q", label.Name, "Updated Work")
	}
}

func TestTaskLabelsService_Update_NilName_UpdatesColorOnly(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work", Color: "#ff0000"})
	svc := NewTaskLabelsService(store)
	newColor := "#00ff00"
	label, err := svc.Update(context.Background(), "l1", "user1", nil, &newColor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if label.Color != "#00ff00" {
		t.Errorf("Color = %q, want %q", label.Color, "#00ff00")
	}
}

func TestTaskLabelsService_Update_GenericGetError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.getErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	name := "New Name"
	if _, err := svc.Update(context.Background(), "l1", "user1", &name, nil); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestTaskLabelsService_Update_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1", Name: "Work"})
	store.updateErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	name := "New Name"
	if _, err := svc.Update(context.Background(), "l1", "user1", &name, nil); err == nil {
		t.Error("expected update store error to propagate, got nil")
	}
}

// ---- List (error path) ----

func TestTaskLabelsService_List_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore()
	store.listErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if _, err := svc.List(context.Background(), "user1"); err == nil {
		t.Error("expected list store error to propagate, got nil")
	}
}

// ---- Create (error path) ----

func TestTaskLabelsService_Create_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore()
	store.createErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if _, err := svc.Create(context.Background(), "user1", "Work", "#ff0000"); err == nil {
		t.Error("expected create store error to propagate, got nil")
	}
}

// ---- Delete (error paths) ----

func TestTaskLabelsService_Delete_GenericGetError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.getErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if err := svc.Delete(context.Background(), "l1", "user1"); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestTaskLabelsService_Delete_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.deleteErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if err := svc.Delete(context.Background(), "l1", "user1"); err == nil {
		t.Error("expected delete store error to propagate, got nil")
	}
}

// ---- AssignLabel (error paths) ----

func TestTaskLabelsService_AssignLabel_GenericGetError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.getErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestTaskLabelsService_AssignLabel_ListForTaskError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.listForTaskErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); err == nil {
		t.Error("expected listForTask error to propagate, got nil")
	}
}

// TestTaskLabelsService_AssignLabel_StoreConflict_ReturnsAlreadyAssigned covers the
// store-level duplicate check (race condition path where the service loop doesn't catch it).
func TestTaskLabelsService_AssignLabel_StoreConflict_ReturnsAlreadyAssigned(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.assignErr = apperrors.ErrLabelAlreadyAssigned // store returns conflict
	svc := NewTaskLabelsService(store)
	// No existing assignment in store.assignments — service loop passes, store rejects.
	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); !errors.Is(err, ErrLabelAlreadyAssigned) {
		t.Errorf("expected ErrLabelAlreadyAssigned from store conflict, got %v", err)
	}
}

func TestTaskLabelsService_AssignLabel_StoreGenericError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.assignErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if err := svc.AssignLabel(context.Background(), "task1", "l1", "user1"); err == nil {
		t.Error("expected store assign error to propagate, got nil")
	}
}

// ---- ListForTask (error path) ----

func TestTaskLabelsService_ListForTask_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskLabelStore()
	store.listForTaskErr = errors.New("db failure")
	svc := NewTaskLabelsService(store)
	if _, err := svc.ListForTask(context.Background(), "task1", "user1"); err == nil {
		t.Error("expected listForTask store error to propagate, got nil")
	}
}

// ---- RemoveLabel ----

func TestTaskLabelsService_RemoveLabel_Success(t *testing.T) {
	store := newFakeTaskLabelStore(model.TaskLabel{ID: "l1"})
	store.assignments["task1"] = []model.TaskLabel{{ID: "l1"}}
	svc := NewTaskLabelsService(store)

	if err := svc.RemoveLabel(context.Background(), "task1", "l1", "user1"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	if len(store.assignments["task1"]) != 0 {
		t.Error("expected label to be removed from task assignments")
	}
}
