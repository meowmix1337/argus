package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

type NewsHandler struct {
	service *service.NewsService
}

func NewNewsHandler(svc *service.NewsService) *NewsHandler {
	return &NewsHandler{service: svc}
}

func (h *NewsHandler) AddRoutes(r chi.Router) {
	r.Get("/api/news", h.Get)
}

func (h *NewsHandler) Get(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Fetch(r.Context())
	if err != nil {
		slog.Error("news fetch error", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, data)
}
