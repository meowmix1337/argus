package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
	"github.com/meowmix1337/argus/backend/internal/session"
)

// TasksHandler handles CRUD for user-scoped tasks.
type TasksHandler struct {
	service  *service.TasksService
	validate *validator.Validate
}

// NewTasksHandler creates a new TasksHandler.
func NewTasksHandler(svc *service.TasksService, v *validator.Validate) *TasksHandler {
	return &TasksHandler{service: svc, validate: v}
}

// AddRoutes registers task routes on the given router.
func (h *TasksHandler) AddRoutes(r chi.Router) {
	r.Get("/api/tasks", h.List)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/tasks", h.Create)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Patch("/api/tasks/{id}", h.Update)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/tasks/{id}", h.Delete)
}

func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := defaultTaskLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxTaskLimit {
		limit = maxTaskLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	tasks, total, err := h.service.List(r.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("failed to list tasks", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, TaskListResponse{
		Tasks:  tasksToResponse(tasks),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *TasksHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096) // 4 KB is generous for these small JSON bodies
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.service.Create(r.Context(), userID, req.Text, req.Priority)
	if err != nil {
		if errors.Is(err, apperrors.ErrTaskValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to create task", "error", err, "user_id", userID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, taskToResponse(task))
}

func (h *TasksHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 4096) // 4 KB is generous for these small JSON bodies
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := h.service.Update(r.Context(), id, userID, req.Done, req.Text, req.Priority)
	if err != nil {
		if errors.Is(err, apperrors.ErrTaskNotFound) {
			response.WriteError(w, http.StatusNotFound, "task not found")
		} else if errors.Is(err, apperrors.ErrTaskValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to update task", "error", err, "user_id", userID, "task_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, taskToResponse(task))
}

func (h *TasksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id, userID); err != nil {
		if errors.Is(err, apperrors.ErrTaskNotFound) {
			response.WriteError(w, http.StatusNotFound, "task not found")
		} else {
			slog.Error("failed to delete task", "error", err, "user_id", userID, "task_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// sessionFromRequest extracts the full session data from the request context.
func sessionFromRequest(r *http.Request) (session.Data, bool) {
	return middleware.SessionFromContext(r.Context())
}

func taskToResponse(t model.Task) TaskResponse {
	return TaskResponse{
		ID:       t.ID,
		Text:     t.Text,
		Done:     t.Done,
		Priority: t.Priority,
	}
}

func tasksToResponse(tasks []model.Task) []TaskResponse {
	resp := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, taskToResponse(t))
	}
	return resp
}
