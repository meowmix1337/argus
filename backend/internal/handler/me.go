package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

// MeHandler serves GET /api/auth/me — returns the current user from the session cookie.
type MeHandler struct{}

func NewMeHandler() *MeHandler { return &MeHandler{} }

func (h *MeHandler) AddRoutes(r chi.Router) {
	r.Get("/api/auth/me", h.Get)
}

func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]string{
		"user_id":    sess.UserID,
		"email":      sess.Email,
		"name":       sess.Name,
		"avatar_url": sess.AvatarURL,
	})
}
