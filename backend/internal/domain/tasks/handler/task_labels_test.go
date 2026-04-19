package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	tasksrepo "github.com/meowmix1337/argus/backend/internal/domain/tasks/repository"
	taskssvc "github.com/meowmix1337/argus/backend/internal/domain/tasks/service"
	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
)

// fakeLabelStore is an in-memory TaskLabelStore for handler tests.
type fakeLabelStore struct {
	labels            map[string]model.TaskLabel
	listErr           error
	getErr            error
	createResult      model.TaskLabel
	createErr         error
	updateErr         error
	deleteErr         error
	listForTaskResult []model.TaskLabel
	listForTaskErr    error
	assignLabelErr    error
	removeLabelErr    error
}

func (f *fakeLabelStore) List(_ context.Context, _ string) ([]model.TaskLabel, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	all := make([]model.TaskLabel, 0, len(f.labels))
	for _, l := range f.labels {
		all = append(all, l)
	}
	return all, nil
}

func (f *fakeLabelStore) Get(_ context.Context, id string, _ string) (model.TaskLabel, error) {
	if f.getErr != nil {
		return model.TaskLabel{}, f.getErr
	}
	l, ok := f.labels[id]
	if !ok {
		return model.TaskLabel{}, apperrors.ErrLabelNotFound
	}
	return l, nil
}

func (f *fakeLabelStore) Create(_ context.Context, _ model.TaskLabelCreate) (model.TaskLabel, error) {
	return f.createResult, f.createErr
}

func (f *fakeLabelStore) Update(_ context.Context, id string, _ string, u model.TaskLabelUpdate) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	l, ok := f.labels[id]
	if !ok {
		return nil
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

func (f *fakeLabelStore) Delete(_ context.Context, id string, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.labels, id)
	return nil
}

func (f *fakeLabelStore) ListForTask(_ context.Context, _ string, _ string) ([]model.TaskLabel, error) {
	if f.listForTaskErr != nil {
		return nil, f.listForTaskErr
	}
	return f.listForTaskResult, nil
}

func (f *fakeLabelStore) AssignLabel(_ context.Context, _ model.TaskLabelAssignmentCreate) error {
	return f.assignLabelErr
}

func (f *fakeLabelStore) RemoveLabel(_ context.Context, _ string, _ string, _ string) error {
	return f.removeLabelErr
}

func newTestTaskLabelsHandler(store tasksrepo.TaskLabelStore) *TaskLabelsHandler {
	svc := taskssvc.NewTaskLabelsService(store)
	return NewTaskLabelsHandler(svc, validator.New())
}

func TestListLabels(t *testing.T) {
	tests := []struct {
		name       string
		noSession  bool
		listErr    error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "with session returns 200",
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error returns 500",
			listErr:    fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{labels: map[string]model.TaskLabel{}, listErr: tt.listErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/labels", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestListLabelsWithData(t *testing.T) {
	label := model.TaskLabel{ID: "label-1", Name: "bug", Color: "#ff0000"}
	store := &fakeLabelStore{labels: map[string]model.TaskLabel{"label-1": label}}
	h := newTestTaskLabelsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/labels", nil)
	req = withSession(req, "user1")
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp []LabelResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("len(labels) = %d, want 1", len(resp))
	}
	if resp[0].ID != "label-1" {
		t.Errorf("label ID = %q, want %q", resp[0].ID, "label-1")
	}
	if resp[0].Name != "bug" {
		t.Errorf("label Name = %q, want %q", resp[0].Name, "bug")
	}
	if resp[0].Color != "#ff0000" {
		t.Errorf("label Color = %q, want %q", resp[0].Color, "#ff0000")
	}
}

func TestCreateLabel(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		noSession    bool
		createResult model.TaskLabel
		createErr    error
		wantStatus   int
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
			name:       "name too long returns 400",
			body:       `{"name":"thisnamelongerthan16chars","color":"#ff0000"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:         "valid body returns 201",
			body:         `{"name":"bug","color":"#ff0000"}`,
			createResult: model.TaskLabel{ID: "label-1", Name: "bug", Color: "#ff0000"},
			wantStatus:   http.StatusCreated,
		},
		{
			name:       "service error returns 500",
			body:       `{"name":"bug","color":"#ff0000"}`,
			createErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{createResult: tt.createResult, createErr: tt.createErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/labels", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			w := httptest.NewRecorder()
			h.Create(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestUpdateLabel(t *testing.T) {
	existingLabel := model.TaskLabel{ID: "label-1", Name: "bug", Color: "#ff0000"}

	tests := []struct {
		name       string
		labelID    string
		body       string
		noSession  bool
		labels     map[string]model.TaskLabel
		getErr     error
		updateErr  error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			labelID:    "label-1",
			body:       `{"name":"feature"}`,
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty body returns 400",
			labelID:    "label-1",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "label not found returns 404",
			labelID:    "label-1",
			body:       `{"name":"feature"}`,
			labels:     map[string]model.TaskLabel{},
			getErr:     apperrors.ErrLabelNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "valid update returns 200",
			labelID:    "label-1",
			body:       `{"name":"feature"}`,
			labels:     map[string]model.TaskLabel{"label-1": existingLabel},
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error returns 500",
			labelID:    "label-1",
			body:       `{"name":"feature"}`,
			labels:     map[string]model.TaskLabel{"label-1": existingLabel},
			updateErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{labels: tt.labels, getErr: tt.getErr, updateErr: tt.updateErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodPatch, "/api/labels/"+tt.labelID, bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.labelID)
			w := httptest.NewRecorder()
			h.Update(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestDeleteLabel(t *testing.T) {
	existingLabel := model.TaskLabel{ID: "label-1", Name: "bug", Color: "#ff0000"}

	tests := []struct {
		name       string
		labelID    string
		noSession  bool
		labels     map[string]model.TaskLabel
		getErr     error
		deleteErr  error
		wantStatus int
	}{
		{
			name:       "no session returns 401",
			labelID:    "label-1",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "label not found returns 404",
			labelID:    "label-1",
			labels:     map[string]model.TaskLabel{},
			getErr:     apperrors.ErrLabelNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "existing label returns 204",
			labelID:    "label-1",
			labels:     map[string]model.TaskLabel{"label-1": existingLabel},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "service error returns 500",
			labelID:    "label-1",
			labels:     map[string]model.TaskLabel{"label-1": existingLabel},
			deleteErr:  fmt.Errorf("db error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{labels: tt.labels, getErr: tt.getErr, deleteErr: tt.deleteErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodDelete, "/api/labels/"+tt.labelID, nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "id", tt.labelID)
			w := httptest.NewRecorder()
			h.Delete(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestListForTask(t *testing.T) {
	tests := []struct {
		name           string
		noSession      bool
		listForTaskErr error
		wantStatus     int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "with session returns 200",
			wantStatus: http.StatusOK,
		},
		{
			name:           "service error returns 500",
			listForTaskErr: fmt.Errorf("db error"),
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{listForTaskErr: tt.listForTaskErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/api/tasks/task-1/labels", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "taskID", "task-1")
			w := httptest.NewRecorder()
			h.ListForTask(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestAssignLabel(t *testing.T) {
	existingLabel := model.TaskLabel{ID: "00000000-0000-0000-0000-000000000001", Name: "bug", Color: "#ff0000"}

	tests := []struct {
		name              string
		body              string
		noSession         bool
		labels            map[string]model.TaskLabel
		getErr            error
		listForTaskResult []model.TaskLabel
		wantStatus        int
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
			name:       "invalid UUID for label_id returns 400",
			body:       `{"label_id":"not-a-uuid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "label not found returns 404",
			body:       `{"label_id":"00000000-0000-0000-0000-000000000001"}`,
			labels:     map[string]model.TaskLabel{},
			getErr:     apperrors.ErrLabelNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:              "label already assigned returns 409",
			body:              `{"label_id":"00000000-0000-0000-0000-000000000001"}`,
			labels:            map[string]model.TaskLabel{"00000000-0000-0000-0000-000000000001": existingLabel},
			listForTaskResult: []model.TaskLabel{{ID: "00000000-0000-0000-0000-000000000001"}},
			wantStatus:        http.StatusConflict,
		},
		{
			name:              "valid assignment returns 204",
			body:              `{"label_id":"00000000-0000-0000-0000-000000000001"}`,
			labels:            map[string]model.TaskLabel{"00000000-0000-0000-0000-000000000001": existingLabel},
			listForTaskResult: []model.TaskLabel{},
			wantStatus:        http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{
				labels:            tt.labels,
				getErr:            tt.getErr,
				listForTaskResult: tt.listForTaskResult,
			}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/api/tasks/task-1/labels", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			req = withChiParam(req, "taskID", "task-1")
			w := httptest.NewRecorder()
			h.AssignLabel(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestRemoveLabel(t *testing.T) {
	tests := []struct {
		name           string
		noSession      bool
		removeLabelErr error
		wantStatus     int
	}{
		{
			name:       "no session returns 401",
			noSession:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "success returns 204",
			wantStatus: http.StatusNoContent,
		},
		{
			name:           "service error returns 500",
			removeLabelErr: fmt.Errorf("db error"),
			wantStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeLabelStore{removeLabelErr: tt.removeLabelErr}
			h := newTestTaskLabelsHandler(store)

			req := httptest.NewRequest(http.MethodDelete, "/api/tasks/task-1/labels/label-1", nil)
			if !tt.noSession {
				req = withSession(req, "user1")
			}
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("taskID", "task-1")
			rctx.URLParams.Add("labelID", "label-1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			w := httptest.NewRecorder()
			h.RemoveLabel(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
