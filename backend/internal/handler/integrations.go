package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// repoNameRe matches valid GitHub "owner/repo" full names.
var repoNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// IntegrationsHandler handles GitHub integration management endpoints.
type IntegrationsHandler struct {
	githubSvc *service.GitHubIntegrationService
	validate  *validator.Validate
}

// NewIntegrationsHandler creates a new IntegrationsHandler.
func NewIntegrationsHandler(githubSvc *service.GitHubIntegrationService, validate *validator.Validate) *IntegrationsHandler {
	return &IntegrationsHandler{githubSvc: githubSvc, validate: validate}
}

// AddRoutes registers the integration management routes (all require an active session).
func (h *IntegrationsHandler) AddRoutes(r chi.Router) {
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Delete("/api/integrations/github", h.Disconnect)
	r.Get("/api/integrations/github/repos", h.ListRepos)
	r.With(httprate.LimitByIP(mutationRateLimit, rateLimitWindow)).Put("/api/integrations/github/repos", h.UpdateWatchedRepos)
}

// Disconnect handles DELETE /api/integrations/github.
func (h *IntegrationsHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.githubSvc.Disconnect(r.Context(), userID); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrIntegrationNotFound):
			response.WriteError(w, http.StatusNotFound, "github integration not found")
		default:
			slog.Error("integrations: disconnect failed", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListRepos handles GET /api/integrations/github/repos.
func (h *IntegrationsHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	repos, err := h.githubSvc.ListUserRepos(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrIntegrationNotFound):
			response.WriteError(w, http.StatusNotFound, "github integration not found")
		default:
			slog.Error("integrations: list repos failed", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	response.WriteJSON(w, http.StatusOK, repos)
}

// UpdateWatchedRepos handles PUT /api/integrations/github/repos.
func (h *IntegrationsHandler) UpdateWatchedRepos(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req UpdateWatchedReposRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Repos) > maxWatchedRepos {
		response.WriteError(w, http.StatusBadRequest, fmt.Sprintf("max %d repos allowed", maxWatchedRepos))
		return
	}
	for _, r := range req.Repos {
		if !repoNameRe.MatchString(r) {
			response.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid repo name: %q", r))
			return
		}
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "validation failed")
		return
	}
	if err := h.githubSvc.UpdateWatchedRepos(r.Context(), userID, req.Repos); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrIntegrationNotFound):
			response.WriteError(w, http.StatusNotFound, "github integration not found")
		default:
			slog.Error("integrations: update watched repos failed", "error", err)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
