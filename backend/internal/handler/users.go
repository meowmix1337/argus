package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

const (
	minUserSearchQuery = 2
	maxUserSearchQuery = 100
)

// UsersHandler handles user-search endpoints.
type UsersHandler struct {
	service *service.UserService
}

// NewUsersHandler creates a new UsersHandler.
func NewUsersHandler(svc *service.UserService) *UsersHandler {
	return &UsersHandler{service: svc}
}

// AddRoutes registers user routes on the given router.
func (h *UsersHandler) AddRoutes(r chi.Router) {
	r.With(httprate.LimitByIP(middleware.SearchRateLimit, middleware.RateLimitWindow)).Get("/api/users/search", h.Search)
}

func (h *UsersHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query().Get("q")
	if len(q) < minUserSearchQuery {
		response.WriteError(w, http.StatusBadRequest, "q must be at least 2 characters")
		return
	}
	if len(q) > maxUserSearchQuery {
		response.WriteError(w, http.StatusBadRequest, "q must be at most 100 characters")
		return
	}

	limit := defaultFollowLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= maxFollowLimit {
			limit = parsed
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, total, err := h.service.Search(r.Context(), userID, q, limit, offset)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to search users")
		return
	}

	response.WriteJSON(w, http.StatusOK, UserSearchResponse{
		Users:  userSummariesToResponse(users),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
