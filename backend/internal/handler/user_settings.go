package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// UserSettingsHandler handles reading and updating user-scoped settings.
type UserSettingsHandler struct {
	service  *service.UserSettingsService
	validate *validator.Validate
}

// NewUserSettingsHandler creates a new UserSettingsHandler.
func NewUserSettingsHandler(svc *service.UserSettingsService, v *validator.Validate) *UserSettingsHandler {
	return &UserSettingsHandler{service: svc, validate: v}
}

// AddRoutes registers user settings routes on the given router.
func (h *UserSettingsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/settings", h.Get)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Put("/api/settings", h.Upsert)
	r.Get("/api/settings/news-categories", h.GetNewsCategories)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Put("/api/settings/news-categories", h.SetNewsCategories)
}

func (h *UserSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	settings, err := h.service.Get(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get user settings", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, settingsToResponse(settings))
}

func (h *UserSettingsHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req UpsertSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	upsert := model.UserSettingsUpsert{
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		CalendarICSURL: req.CalendarICSURL,
		Timezone:       req.Timezone,
	}

	settings, err := h.service.Upsert(r.Context(), userID, upsert)
	if err != nil {
		if errors.Is(err, apperrors.ErrSettingsValidation) {
			response.WriteError(w, http.StatusBadRequest, "invalid request body")
		} else {
			slog.Error("failed to upsert user settings", "error", err, "user_id", userID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, settingsModelToResponse(settings))
}

func (h *UserSettingsHandler) GetNewsCategories(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	available, err := h.service.ListAllCategories(r.Context())
	if err != nil {
		slog.Error("failed to list all news categories", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	selected, err := h.service.ListSelectedCategories(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list selected news categories", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := NewsCategoriesResponse{
		Available: categoriesToResponse(available),
		Selected:  categoriesToResponse(selected),
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

func (h *UserSettingsHandler) SetNewsCategories(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req SetNewsCategoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.SetSelectedCategories(r.Context(), userID, req.CategoryIDs); err != nil {
		if errors.Is(err, apperrors.ErrCategoryNotFound) {
			response.WriteError(w, http.StatusBadRequest, "invalid category id")
		} else {
			slog.Error("failed to set news categories", "error", err, "user_id", userID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func settingsToResponse(s *model.UserSettings) UserSettingsResponse {
	if s == nil {
		return UserSettingsResponse{}
	}
	return settingsModelToResponse(*s)
}

func settingsModelToResponse(s model.UserSettings) UserSettingsResponse {
	return UserSettingsResponse{
		Latitude:       s.Latitude,
		Longitude:      s.Longitude,
		CalendarICSURL: s.CalendarICSURL,
		Timezone:       s.Timezone,
	}
}

func categoriesToResponse(cats []model.NewsCategoryType) []NewsCategoryTypeResponse {
	resp := make([]NewsCategoryTypeResponse, 0, len(cats))
	for _, c := range cats {
		resp = append(resp, NewsCategoryTypeResponse{
			ID:        c.ID,
			Label:     c.Label,
			SortOrder: c.SortOrder,
		})
	}
	return resp
}
