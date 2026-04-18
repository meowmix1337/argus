package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// FollowHandler handles follow/unfollow endpoints.
type FollowHandler struct {
	service  *service.FollowService
	validate *validator.Validate
}

// NewFollowHandler creates a new FollowHandler.
func NewFollowHandler(svc *service.FollowService, v *validator.Validate) *FollowHandler {
	return &FollowHandler{service: svc, validate: v}
}

// AddRoutes registers follow routes on the given router.
func (h *FollowHandler) AddRoutes(r chi.Router) {
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/follow", h.Follow)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/follow/{id}", h.Unfollow)
	r.Get("/api/follow/status/{id}", h.IsFollowing)
	r.Get("/api/follow/{id}/followers", h.ListFollowers)
	r.Get("/api/follow/{id}/following", h.ListFollowing)
}

func (h *FollowHandler) Follow(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req FollowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Follow(r.Context(), sess.UserID, req.FollowingID, sess.Name); err != nil {
		switch {
		case errors.Is(err, apperrors.ErrSelfFollow):
			response.WriteError(w, http.StatusBadRequest, "cannot follow yourself")
		case errors.Is(err, apperrors.ErrAlreadyFollowing):
			response.WriteError(w, http.StatusConflict, "already following this user")
		default:
			slog.Error("failed to follow", "error", err, "follower_id", sess.UserID, "following_id", req.FollowingID)
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FollowHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	followingID := chi.URLParam(r, "id")
	if err := h.service.Unfollow(r.Context(), userID, followingID); err != nil {
		if errors.Is(err, apperrors.ErrNotFollowing) {
			response.WriteError(w, http.StatusNotFound, "not following this user")
			return
		}
		slog.Error("failed to unfollow", "error", err, "follower_id", userID, "following_id", followingID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FollowHandler) IsFollowing(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID := chi.URLParam(r, "id")
	following, err := h.service.IsFollowing(r.Context(), userID, targetID)
	if err != nil {
		slog.Error("failed to check follow status", "error", err, "follower_id", userID, "following_id", targetID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, FollowStatusResponse{Following: following})
}

func (h *FollowHandler) ListFollowers(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID := chi.URLParam(r, "id")
	limit, offset := parsePagination(r, defaultFollowLimit, maxFollowLimit)

	users, total, err := h.service.ListFollowers(r.Context(), targetID, limit, offset)
	if err != nil {
		slog.Error("failed to list followers", "error", err, "user_id", targetID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, FollowListResponse{
		Users:  userSummariesToResponse(users),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *FollowHandler) ListFollowing(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	targetID := chi.URLParam(r, "id")
	limit, offset := parsePagination(r, defaultFollowLimit, maxFollowLimit)

	users, total, err := h.service.ListFollowing(r.Context(), targetID, limit, offset)
	if err != nil {
		slog.Error("failed to list following", "error", err, "user_id", targetID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, FollowListResponse{
		Users:  userSummariesToResponse(users),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
