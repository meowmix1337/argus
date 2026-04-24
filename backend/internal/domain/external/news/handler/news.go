package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	newssvc "github.com/meowmix1337/argus/backend/internal/domain/external/news/service"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
)

type NewsHandler struct {
	service *newssvc.NewsService
}

func NewNewsHandler(svc *newssvc.NewsService) *NewsHandler {
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
