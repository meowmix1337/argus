package handler

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	socialsvc "github.com/meowmix1337/argus/backend/internal/domain/social/service"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

// FeedHandler handles the social feed timeline endpoint.
type FeedHandler struct {
	service *socialsvc.FeedService
}

// NewFeedHandler creates a new FeedHandler.
func NewFeedHandler(svc *socialsvc.FeedService) *FeedHandler {
	return &FeedHandler{service: svc}
}

// AddRoutes registers feed routes on the given router.
func (h *FeedHandler) AddRoutes(r chi.Router) {
	r.Get("/api/feed", h.ListFeed)
}

func (h *FeedHandler) ListFeed(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := defaultFeedLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxFeedLimit {
		limit = maxFeedLimit
	}

	var cursor *model.FeedCursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		var c model.FeedCursor
		if err := json.Unmarshal(decoded, &c); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursor = &c
	}

	// Fetch limit+1 to detect if there's a next page.
	posts, err := h.service.ListFeed(r.Context(), userID, cursor, limit+1)
	if err != nil {
		slog.Error("failed to list feed", "error", err, "user_id", userID)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var nextCursor *string
	if len(posts) > limit {
		posts = posts[:limit]
		last := posts[len(posts)-1]
		cursorData, _ := json.Marshal(model.FeedCursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		})
		encoded := base64.RawURLEncoding.EncodeToString(cursorData)
		nextCursor = &encoded
	}

	response.WriteJSON(w, http.StatusOK, FeedResponse{
		Posts:      postsToResponse(posts),
		NextCursor: nextCursor,
	})
}
