package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

// socialPrefsService is the interface the handler requires from SocialPrefsService.
type socialPrefsService interface {
	GetPrefs(ctx context.Context, userID string) (model.SocialNotificationPrefs, error)
	UpsertPrefs(ctx context.Context, userID string, mutePosts, muteFollows bool) error
}

// SocialPrefsHandler handles social notification preference endpoints.
type SocialPrefsHandler struct {
	service  socialPrefsService
	validate *validator.Validate
}

// NewSocialPrefsHandler creates a new SocialPrefsHandler.
func NewSocialPrefsHandler(svc socialPrefsService, v *validator.Validate) *SocialPrefsHandler {
	return &SocialPrefsHandler{service: svc, validate: v}
}

// AddRoutes registers social notification preference routes on the given router.
func (h *SocialPrefsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/settings/social-notifications", h.GetPrefs)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Put("/api/settings/social-notifications", h.UpdatePrefs)
}

func (h *SocialPrefsHandler) GetPrefs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	prefs, err := h.service.GetPrefs(r.Context(), userID)
	if err != nil {
		slog.Error("failed to get social prefs", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, SocialPrefsResponse{
		MutePosts:   prefs.MutePosts,
		MuteFollows: prefs.MuteFollows,
	})
}

func (h *SocialPrefsHandler) UpdatePrefs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req UpdateSocialPrefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpsertPrefs(r.Context(), userID, req.MutePosts, req.MuteFollows); err != nil {
		slog.Error("failed to update social prefs", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
