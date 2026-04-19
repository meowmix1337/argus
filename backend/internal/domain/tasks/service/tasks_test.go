package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// fakeTaskStore is an in-memory TaskStore used by service-layer tests.
// It avoids any database dependency while exercising real service logic.
type fakeTaskStore struct {
	tasks     map[string]model.Task
	listErr   error
	createErr error
	getErr    error
	updateErr error
	deleteErr error
}

func newFakeTaskStore(tasks ...model.Task) *fakeTaskStore {
	s := &fakeTaskStore{tasks: make(map[string]model.Task)}
	for _, t := range tasks {
		s.tasks[t.ID] = t
	}
	return s
}

func (f *fakeTaskStore) List(_ context.Context, _ string, limit, offset int) ([]model.Task, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	out := make([]model.Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		out = append(out, t)
	}
	total := len(out)
	if limit > 0 && offset < total {
		out = out[offset:min(offset+limit, total)]
	}
	return out, total, nil
}

func (f *fakeTaskStore) Get(_ context.Context, id string, _ string) (model.Task, error) {
	if f.getErr != nil {
		return model.Task{}, f.getErr
	}
	t, ok := f.tasks[id]
	if !ok {
		return model.Task{}, apperrors.ErrTaskNotFound
	}
	return t, nil
}

func (f *fakeTaskStore) Create(_ context.Context, tc model.TaskCreate) (model.Task, error) {
	if f.createErr != nil {
		return model.Task{}, f.createErr
	}
	task := model.Task{
		ID:       tc.ID,
		Text:     tc.Text,
		Done:     tc.Done,
		Priority: tc.PriorityID,
	}
	f.tasks[tc.ID] = task
	return task, nil
}

func (f *fakeTaskStore) Update(_ context.Context, id string, _ string, u model.TaskUpdate) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	t, ok := f.tasks[id]
	if !ok {
		return apperrors.ErrTaskNotFound
	}
	if u.Text != nil {
		t.Text = *u.Text
	}
	if u.Done != nil {
		t.Done = *u.Done
	}
	if u.PriorityID != nil {
		t.Priority = *u.PriorityID
	}
	f.tasks[id] = t
	return nil
}

func (f *fakeTaskStore) Delete(_ context.Context, id string, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.tasks, id)
	return nil
}

// ---- Create ----

func TestTasksService_Create_EmptyText(t *testing.T) {
	svc := NewTasksService(newFakeTaskStore())
	_, err := svc.Create(context.Background(), "user1", "", "medium")
	if !errors.Is(err, ErrTaskValidation) {
		t.Errorf("expected ErrTaskValidation, got %v", err)
	}
}

func TestTasksService_Create_WhitespaceOnlyText(t *testing.T) {
	svc := NewTasksService(newFakeTaskStore())
	_, err := svc.Create(context.Background(), "user1", "   ", "medium")
	if !errors.Is(err, ErrTaskValidation) {
		t.Errorf("expected ErrTaskValidation for whitespace-only text, got %v", err)
	}
}

func TestTasksService_Create_InvalidPriority(t *testing.T) {
	svc := NewTasksService(newFakeTaskStore())
	_, err := svc.Create(context.Background(), "user1", "Buy groceries", "urgent")
	if !errors.Is(err, ErrTaskValidation) {
		t.Errorf("expected ErrTaskValidation for invalid priority, got %v", err)
	}
}

func TestTasksService_Create_DefaultsToMediumPriority(t *testing.T) {
	// An empty priority string must default to "medium", not error.
	svc := NewTasksService(newFakeTaskStore())
	task, err := svc.Create(context.Background(), "user1", "Buy groceries", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.Priority != "medium" {
		t.Errorf("Priority = %q, want %q", task.Priority, "medium")
	}
}

func TestTasksService_Create_AllValidPriorities(t *testing.T) {
	for _, p := range []string{"high", "medium", "low"} {
		svc := NewTasksService(newFakeTaskStore())
		task, err := svc.Create(context.Background(), "user1", "Do something", p)
		if err != nil {
			t.Errorf("priority %q: unexpected error: %v", p, err)
			continue
		}
		if task.Priority != p {
			t.Errorf("priority %q: got %q back from store", p, task.Priority)
		}
	}
}

// ---- Update ----

func TestTasksService_Update_EmptyText(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "original"})
	svc := NewTasksService(store)
	empty := ""
	_, err := svc.Update(context.Background(), "t1", "user1", nil, &empty, nil)
	if !errors.Is(err, ErrTaskValidation) {
		t.Errorf("expected ErrTaskValidation for empty text update, got %v", err)
	}
}

func TestTasksService_Update_InvalidPriority(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "original"})
	svc := NewTasksService(store)
	bad := "critical"
	_, err := svc.Update(context.Background(), "t1", "user1", nil, nil, &bad)
	if !errors.Is(err, ErrTaskValidation) {
		t.Errorf("expected ErrTaskValidation for invalid priority, got %v", err)
	}
}

func TestTasksService_Update_NotFound(t *testing.T) {
	svc := NewTasksService(newFakeTaskStore()) // empty store
	done := true
	_, err := svc.Update(context.Background(), "nonexistent", "user1", &done, nil, nil)
	if !errors.Is(err, apperrors.ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTasksService_Update_TrimsWhitespaceFromText(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "original"})
	svc := NewTasksService(store)
	padded := "  updated text  "
	task, err := svc.Update(context.Background(), "t1", "user1", nil, &padded, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if task.Text != "updated text" {
		t.Errorf("Text = %q, want %q (should be trimmed)", task.Text, "updated text")
	}
}

// ---- Delete ----

func TestTasksService_Delete_NotFound(t *testing.T) {
	svc := NewTasksService(newFakeTaskStore())
	err := svc.Delete(context.Background(), "nonexistent", "user1")
	if !errors.Is(err, apperrors.ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTasksService_Delete_Success(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "buy milk"})
	svc := NewTasksService(store)
	if err := svc.Delete(context.Background(), "t1", "user1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, exists := store.tasks["t1"]; exists {
		t.Error("expected task to be removed from store after Delete")
	}
}

// ---- List ----

func TestTasksService_List_ReturnsAllTasks(t *testing.T) {
	store := newFakeTaskStore(
		model.Task{ID: "t1", Text: "task one"},
		model.Task{ID: "t2", Text: "task two"},
	)
	svc := NewTasksService(store)
	tasks, total, err := svc.List(context.Background(), "user1", 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(tasks))
	}
}

func TestTasksService_List_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskStore()
	store.listErr = errors.New("db failure")
	svc := NewTasksService(store)
	if _, _, err := svc.List(context.Background(), "user1", 10, 0); err == nil {
		t.Error("expected list store error to propagate, got nil")
	}
}

func TestTasksService_Create_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskStore()
	store.createErr = errors.New("db failure")
	svc := NewTasksService(store)
	if _, err := svc.Create(context.Background(), "user1", "Buy milk", "medium"); err == nil {
		t.Error("expected create store error to propagate, got nil")
	}
}

func TestTasksService_Update_GenericGetError_Propagates(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "original"})
	store.getErr = errors.New("db failure")
	svc := NewTasksService(store)
	done := true
	if _, err := svc.Update(context.Background(), "t1", "user1", &done, nil, nil); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestTasksService_Update_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "original"})
	store.updateErr = errors.New("db failure")
	svc := NewTasksService(store)
	done := true
	if _, err := svc.Update(context.Background(), "t1", "user1", &done, nil, nil); err == nil {
		t.Error("expected update store error to propagate, got nil")
	}
}

func TestTasksService_Delete_GenericGetError_Propagates(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1"})
	store.getErr = errors.New("db failure")
	svc := NewTasksService(store)
	if err := svc.Delete(context.Background(), "t1", "user1"); err == nil {
		t.Error("expected generic get error to propagate, got nil")
	}
}

func TestTasksService_Delete_StoreError_Propagates(t *testing.T) {
	store := newFakeTaskStore(model.Task{ID: "t1", Text: "task"})
	store.deleteErr = errors.New("db failure")
	svc := NewTasksService(store)
	if err := svc.Delete(context.Background(), "t1", "user1"); err == nil {
		t.Error("expected delete store error to propagate, got nil")
	}
}
