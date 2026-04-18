package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// NotificationsHandler handles notification inbox endpoints.
type NotificationsHandler struct {
	service  *service.NotificationService
	validate *validator.Validate
}

// NewNotificationsHandler creates a new NotificationsHandler.
func NewNotificationsHandler(svc *service.NotificationService, v *validator.Validate) *NotificationsHandler {
	return &NotificationsHandler{service: svc, validate: v}
}

// AddRoutes registers notification routes on the given router.
func (h *NotificationsHandler) AddRoutes(r chi.Router) {
	r.With(httprate.LimitByIP(middleware.SearchRateLimit, middleware.RateLimitWindow)).Get("/api/notifications", h.List)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Patch("/api/notifications/{id}", h.Patch)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/notifications/mark-all-read", h.MarkAllRead)
}

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		state = "unread"
	}
	switch state {
	case "unread", "read", "dismissed", "all":
	default:
		response.WriteError(w, http.StatusBadRequest, "invalid state: must be unread, read, dismissed, or all")
		return
	}

	limit := defaultNotificationLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxNotificationLimit {
		limit = maxNotificationLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > maxNotificationSearchQuery {
		response.WriteError(w, http.StatusBadRequest, "search query too long")
		return
	}

	providerID := r.URL.Query().Get("provider")
	if providerID != "" {
		if _, ok := allowedNotificationProviders[providerID]; !ok {
			response.WriteError(w, http.StatusBadRequest, "invalid provider value")
			return
		}
	}

	eventTypeID := r.URL.Query().Get("event_type")
	if eventTypeID != "" {
		if _, ok := allowedNotificationEventTypes[eventTypeID]; !ok {
			response.WriteError(w, http.StatusBadRequest, "invalid event_type value")
			return
		}
	}

	notifications, total, err := h.service.List(r.Context(), userID, state, q, providerID, eventTypeID, limit, offset)
	if err != nil {
		slog.Error("failed to list notifications", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, NotificationListResponse{
		Notifications: notificationsToResponse(notifications),
		Total:         total,
		Limit:         limit,
		Offset:        offset,
	})
}

func (h *NotificationsHandler) Patch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req PatchNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	switch req.Action {
	case "read":
		err = h.service.MarkRead(r.Context(), id, userID)
	case "dismissed":
		err = h.service.MarkDismissed(r.Context(), id, userID)
	}

	if err != nil {
		if errors.Is(err, apperrors.ErrNotificationNotFound) {
			response.WriteError(w, http.StatusNotFound, "notification not found")
		} else {
			slog.Error("failed to patch notification", "error", err, "user_id", userID, "notification_id", id)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationsHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.MarkAllRead(r.Context(), userID); err != nil {
		slog.Error("failed to mark all notifications read", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
