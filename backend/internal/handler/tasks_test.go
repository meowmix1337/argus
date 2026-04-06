package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// fakeTaskStore is an in-memory TaskStore for handler tests.
type fakeTaskStore struct {
	tasks     map[string]model.Task
	listErr   error
	getErr    error
	createErr error
	updateErr error
	deleteErr error
}

func (f *fakeTaskStore) List(_ context.Context, _ string, limit, offset int) ([]model.Task, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	all := make([]model.Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	if offset >= total {
		return []model.Task{}, total, nil
	}
	end := offset + limit
	if limit == 0 || end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (f *fakeTaskStore) Get(_ context.Context, id, _ string) (model.Task, error) {
	if f.getErr != nil {
		return model.Task{}, f.getErr
	}
	t, ok := f.tasks[id]
	if !ok {
		return model.Task{}, apperrors.ErrTaskNotFound
	}
	return t, nil
}

func (f *fakeTaskStore) Create(_ context.Context, t model.TaskCreate) (model.Task, error) {
	if f.createErr != nil {
		return model.Task{}, f.createErr
	}
	task := model.Task{ID: t.ID, Text: t.Text, Done: t.Done, Priority: t.PriorityID}
	if f.tasks == nil {
		f.tasks = make(map[string]model.Task)
	}
	f.tasks[t.ID] = task
	return task, nil
}

func (f *fakeTaskStore) Update(_ context.Context, id, _ string, u model.TaskUpdate) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	t, ok := f.tasks[id]
	if !ok {
		return apperrors.ErrTaskNotFound
	}
	if u.Done != nil {
		t.Done = *u.Done
	}
	if u.Text != nil {
		t.Text = *u.Text
	}
	if u.PriorityID != nil {
		t.Priority = *u.PriorityID
	}
	f.tasks[id] = t
	return nil
}

func (f *fakeTaskStore) Delete(_ context.Context, id, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.tasks, id)
	return nil
}

func newTestTasksHandler(store service.TaskStore) *TasksHandler {
	svc := service.NewTasksService(store)
	return NewTasksHandler(svc, validator.New())
}

func TestListTasks(t *testing.T) {
	allTasks := map[string]model.Task{
		"t1": {ID: "t1", Text: "task 1", Done: false, Priority: "high"},
		"t2": {ID: "t2", Text: "task 2", Done: true, Priority: "medium"},
		"t3": {ID: "t3", Text: "task 3", Done: false, Priority: "low"},
	}

	tests := []struct {
		name       string
		query      string
		noSession  bool
		listErr    error
		wantStatus int
		wantTotal  int
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "no params returns 200 with all tasks",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  defaultTaskLimit,
			wantOffset: 0,
		},
		{
			name:       "limit=2 offset=0 returns first page",
			query:      "?limit=2&offset=0",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  2,
			wantOffset: 0,
		},
		{
			name:       "offset past end returns empty tasks with correct total",
			query:      "?limit=5&offset=100",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  5,
			wantOffset: 100,
		},
		{
			name:       "limit exceeding max is clamped to maxTaskLimit",
			query:      "?limit=9999",
			wantStatus: http.StatusOK,
			wantTotal:  3,
			wantLimit:  maxTaskLimit,
			wantOffset: 0,
		},
		{
			name:       "service error returns 500",
			listErr:    fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTaskStore{tasks: allTasks, listErr: tt.listErr}
			h := newTestTasksHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/tasks"+tt.query, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp TaskListResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Total != tt.wantTotal {
				t.Errorf("total = %d, want %d", resp.Total, tt.wantTotal)
			}
			if resp.Limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", resp.Limit, tt.wantLimit)
			}
			if resp.Offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", resp.Offset, tt.wantOffset)
			}
		})
	}
}

func TestCreateTask(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		noSession  bool
		createErr  error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing text field returns 400",
			body:       `{"priority":"medium"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid priority returns 400",
			body:       `{"text":"buy milk","priority":"urgent"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid body returns 201 with task ID",
			body:       `{"text":"buy milk","priority":"high"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid body without priority returns 201",
			body:       `{"text":"buy milk"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "service error returns 500",
			body:       `{"text":"buy milk","priority":"high"}`,
			createErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTaskStore{createErr: tt.createErr}
			h := newTestTasksHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Create(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}

			var resp TaskResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ID == "" {
				t.Error("expected non-empty task ID in response")
			}
		})
	}
}

func TestUpdateTask(t *testing.T) {
	existingTask := model.Task{ID: "task-1", Text: "buy milk", Done: false, Priority: "medium"}

	tests := []struct {
		name       string
		taskID     string
		body       string
		noSession  bool
		tasks      map[string]model.Task
		updateErr  error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			taskID:     "task-1",
			body:       `{"done":true}`,
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			taskID:     "task-1",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "task not found returns 404",
			taskID:     "missing",
			body:       `{"done":true}`,
			tasks:      map[string]model.Task{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid priority returns 400",
			taskID:     "task-1",
			body:       `{"priority":"urgent"}`,
			tasks:      map[string]model.Task{"task-1": existingTask},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid update returns 200 with updated task",
			taskID:     "task-1",
			body:       `{"done":true}`,
			tasks:      map[string]model.Task{"task-1": existingTask},
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error returns 500",
			taskID:     "task-1",
			body:       `{"done":true}`,
			tasks:      map[string]model.Task{"task-1": existingTask},
			updateErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTaskStore{tasks: tt.tasks, updateErr: tt.updateErr}
			h := newTestTasksHandler(store)

			req := httptest.NewRequest(http.MethodPatch, "/api/tasks/"+tt.taskID, bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.taskID)
			w := httptest.NewRecorder()
			h.Update(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp TaskResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ID != tt.taskID {
				t.Errorf("response ID = %q, want %q", resp.ID, tt.taskID)
			}
		})
	}
}

func TestDeleteTask(t *testing.T) {
	existingTask := model.Task{ID: "task-1", Text: "buy milk", Done: false, Priority: "medium"}

	tests := []struct {
		name       string
		taskID     string
		noSession  bool
		tasks      map[string]model.Task
		deleteErr  error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			taskID:     "task-1",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "task not found returns 404",
			taskID:     "missing",
			tasks:      map[string]model.Task{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "existing task returns 204 no content",
			taskID:     "task-1",
			tasks:      map[string]model.Task{"task-1": existingTask},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "service error returns 500",
			taskID:     "task-1",
			tasks:      map[string]model.Task{"task-1": existingTask},
			deleteErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeTaskStore{tasks: tt.tasks, deleteErr: tt.deleteErr}
			h := newTestTasksHandler(store)

			req := httptest.NewRequest(http.MethodDelete, "/api/tasks/"+tt.taskID, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.taskID)
			w := httptest.NewRecorder()
			h.Delete(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
