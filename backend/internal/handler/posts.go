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
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

// PostsHandler handles social feed post endpoints.
type PostsHandler struct {
	service  *service.PostsService
	validate *validator.Validate
}

// NewPostsHandler creates a new PostsHandler.
func NewPostsHandler(svc *service.PostsService, v *validator.Validate) *PostsHandler {
	return &PostsHandler{service: svc, validate: v}
}

// AddRoutes registers post routes on the given router.
func (h *PostsHandler) AddRoutes(r chi.Router) {
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/posts", h.Create)
	r.Get("/api/posts/{id}", h.GetByID)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Delete("/api/posts/{id}", h.Delete)
	r.With(httprate.LimitByIP(middleware.MutationRateLimit, middleware.RateLimitWindow)).Post("/api/posts/{id}/like", h.ToggleLike)
	r.Get("/api/posts/user/{id}", h.ListByUser)
	r.With(httprate.LimitByIP(middleware.SearchRateLimit, middleware.RateLimitWindow)).Get("/api/posts/search", h.Search)
}

func (h *PostsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.service.Create(r.Context(), userID, req.Content, req.ParentPostID)
	if err != nil {
		if errors.Is(err, apperrors.ErrPostValidation) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Error("failed to create post", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusCreated, postToResponse(post))
}

func (h *PostsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postID := chi.URLParam(r, "id")
	post, err := h.service.GetByID(r.Context(), postID, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrPostNotFound) {
			response.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		slog.Error("failed to get post", "error", err, "post_id", postID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, postToResponse(post))
}

func (h *PostsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postID := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), postID, userID); err != nil {
		if errors.Is(err, apperrors.ErrPostNotFound) {
			response.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		slog.Error("failed to delete post", "error", err, "user_id", userID, "post_id", postID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PostsHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	postID := chi.URLParam(r, "id")
	post, err := h.service.ToggleLike(r.Context(), postID, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrPostNotFound) {
			response.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		slog.Error("failed to toggle like", "error", err, "user_id", userID, "post_id", postID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, postToResponse(post))
}

func (h *PostsHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	authorID := chi.URLParam(r, "id")
	limit, offset := parsePagination(r, defaultPostLimit, maxPostLimit)

	posts, total, err := h.service.ListByUser(r.Context(), authorID, viewerID, limit, offset)
	if err != nil {
		slog.Error("failed to list posts by user", "error", err, "author_id", authorID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, PostListResponse{
		Posts:  postsToResponse(posts),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *PostsHandler) Search(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		response.WriteError(w, http.StatusBadRequest, "search query is required")
		return
	}

	limit, offset := parsePagination(r, defaultPostLimit, maxPostLimit)

	posts, total, err := h.service.Search(r.Context(), query, viewerID, limit, offset)
	if err != nil {
		if errors.Is(err, apperrors.ErrPostValidation) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Error("failed to search posts", "error", err, "query", query)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, PostSearchResponse{
		Posts:  postsToResponse(posts),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// parsePagination extracts limit/offset from query params, clamped to defaults.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
