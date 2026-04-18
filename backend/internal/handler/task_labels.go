package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// TaskLabelsHandler handles CRUD for user-scoped task labels and label assignments.
type TaskLabelsHandler struct {
	service  *service.TaskLabelsService
	validate *validator.Validate
}

// NewTaskLabelsHandler creates a new TaskLabelsHandler.
func NewTaskLabelsHandler(svc *service.TaskLabelsService, v *validator.Validate) *TaskLabelsHandler {
	return &TaskLabelsHandler{service: svc, validate: v}
}

// AddRoutes registers task label routes on the given router.
func (h *TaskLabelsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/labels", h.List)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/labels", h.Create)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Patch("/api/labels/{id}", h.Update)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/labels/{id}", h.Delete)

	r.Get("/api/tasks/{taskID}/labels", h.ListForTask)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/tasks/{taskID}/labels", h.AssignLabel)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/tasks/{taskID}/labels/{labelID}", h.RemoveLabel)
}

func (h *TaskLabelsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	labels, err := h.service.List(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list labels", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, labelsToResponse(labels))
}

func (h *TaskLabelsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req CreateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	label, err := h.service.Create(r.Context(), userID, req.Name, req.Color)
	if err != nil {
		if errors.Is(err, apperrors.ErrLabelValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to create label", "error", err, "user_id", userID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, labelToResponse(label))
}

func (h *TaskLabelsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req UpdateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	label, err := h.service.Update(r.Context(), id, userID, req.Name, req.Color)
	if err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			response.WriteError(w, http.StatusNotFound, "label not found")
		} else if errors.Is(err, apperrors.ErrLabelValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to update label", "error", err, "user_id", userID, "label_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, labelToResponse(label))
}

func (h *TaskLabelsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), id, userID); err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			response.WriteError(w, http.StatusNotFound, "label not found")
		} else {
			slog.Error("failed to delete label", "error", err, "user_id", userID, "label_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskLabelsHandler) ListForTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	labels, err := h.service.ListForTask(r.Context(), taskID, userID)
	if err != nil {
		slog.Error("failed to list labels for task", "error", err, "user_id", userID, "task_id", taskID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, labelsToResponse(labels))
}

func (h *TaskLabelsHandler) AssignLabel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req AssignLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.AssignLabel(r.Context(), taskID, req.LabelID, userID); err != nil {
		if errors.Is(err, apperrors.ErrLabelNotFound) {
			response.WriteError(w, http.StatusNotFound, "label not found")
		} else if errors.Is(err, apperrors.ErrLabelAlreadyAssigned) {
			response.WriteError(w, http.StatusConflict, "label already assigned to task")
		} else {
			slog.Error("failed to assign label", "error", err, "user_id", userID, "task_id", taskID, "label_id", req.LabelID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskLabelsHandler) RemoveLabel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := chi.URLParam(r, "taskID")
	labelID := chi.URLParam(r, "labelID")

	if err := h.service.RemoveLabel(r.Context(), taskID, labelID, userID); err != nil {
		slog.Error("failed to remove label", "error", err, "user_id", userID, "task_id", taskID, "label_id", labelID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func labelToResponse(l model.TaskLabel) LabelResponse {
	return LabelResponse{
		ID:        l.ID,
		Name:      l.Name,
		Color:     l.Color,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}

func labelsToResponse(labels []model.TaskLabel) []LabelResponse {
	resp := make([]LabelResponse, 0, len(labels))
	for _, l := range labels {
		resp = append(resp, labelToResponse(l))
	}
	return resp
}
